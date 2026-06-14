package cmd

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// linkListHandler serves GET /issue/{key}?fields=issuelinks with linked-issue
// summary/status expanded, plus an empty-links key.
func linkListHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/rest/api/2/myself" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"name":"alice","displayName":"Alice"}`)
		case path == "/rest/api/2/issue/PROJ-NOLINKS" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":"1","key":"PROJ-NOLINKS","fields":{"issuelinks":[]}}`)
		case strings.HasPrefix(path, "/rest/api/2/issue/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":"1","key":"PROJ-1","fields":{"issuelinks":[
				{"id":"link1","type":{"name":"Blocks","inward":"is blocked by","outward":"blocks"},"outwardIssue":{"key":"PROJ-99","fields":{"summary":"Downstream work","status":{"name":"To Do"}}}},
				{"id":"link2","type":{"name":"Relates","inward":"relates to","outward":"relates to"},"inwardIssue":{"key":"PROJ-88","fields":{"summary":"Upstream work","status":{"name":"Done"}}}}
			]}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestIssueLinkList(t *testing.T) {
	mockJiraServer(t, linkListHandler(t))

	t.Run("table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "link", "list", sampleIssueKey)
		if !containsAny(stdout, "link1", "blocks", "PROJ-99", "Downstream work", "To Do") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json untrusted", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "link", "list", sampleIssueKey)
		if !containsAny(stdout, "linkedKey", "PROJ-88", "Upstream work", "_untrusted", "linkedSummary") {
			t.Fatalf("stdout=%q", stdout)
		}
		if !strings.Contains(stdout, "inward") || !strings.Contains(stdout, "outward") {
			t.Fatalf("expected both directions: %s", stdout)
		}
	})

	t.Run("empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "link", "list", "PROJ-NOLINKS")
		if !containsAny(stdout, "No links") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueLinkList_APIError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/myself" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"name":"alice"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	issueRunSilent(t, ExitNotFound, "issue", "link", "list", sampleIssueKey)
}

func TestIssueLinkList_NotConfigured(t *testing.T) {
	setupTestHome(t)
	_, _, err := issueRunRoot(t, "issue", "link", "list", sampleIssueKey)
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitAuth)
	}
}

// sprintReportHandler serves the Greenhopper chart and the issue/sprint fallback.
func sprintReportHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/rest/api/2/myself":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"name":"alice","displayName":"Alice"}`)
		case strings.Contains(path, "sprintreport"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"contents":{
				"completedIssues":[{"key":"P-1"},{"key":"P-2"}],
				"issuesNotCompletedInCurrentSprint":[{"key":"P-3"}],
				"puntedIssues":[],
				"completedIssuesEstimateSum":{"value":8},
				"issuesNotCompletedEstimateSum":{"value":5}
			},"sprint":{"id":10,"name":"Sprint 10","state":"closed"}}`)
		case strings.HasSuffix(path, "/sprint/10/issue"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"issues":[
				{"id":"1","key":"P-1","fields":{"status":{"statusCategory":{"key":"done"}},"issuetype":{"name":"Task"}}},
				{"id":"2","key":"P-2","fields":{"status":{"statusCategory":{"key":"new"}},"issuetype":{"name":"Task"}}}
			]}`)
		case strings.HasSuffix(path, "/sprint/10"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":10,"name":"Sprint 10","state":"active"}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestSprintReport(t *testing.T) {
	mockJiraServer(t, sprintReportHandler(t))

	t.Run("bad id", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "sprint", "report", "abc")
	})

	t.Run("greenhopper table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "sprint", "report", "10", "--board", "1")
		if !containsAny(stdout, "Sprint 10", "greenhopper", "Story Points") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("greenhopper json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "sprint", "report", "10", "--board", "1")
		if !containsAny(stdout, `"source"`, "greenhopper", "committedPoints", "completedIssues") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("computed fallback no board", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "sprint", "report", "10")
		if !containsAny(stdout, "computed", `"completedIssues":1`) {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestSprintReport_NotConfigured(t *testing.T) {
	setupTestHome(t)
	_, _, err := issueRunRoot(t, "sprint", "report", "10")
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitAuth)
	}
}
