package cmd

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetCLIState(t *testing.T) {
	t.Helper()
	outputFormat = outputFormatJSON
	jsonCompatMode = false
	jsonMode = true
	compactMode = false
	dryRun = false
	forceMode = false
	confirmToken = ""
	quietMode = false
	output.CompactJSON = false
	output.ErrorJSON = false
	output.EnvelopeJSON = true
	output.Quiet = false
	lastExit = 0
	var reset func(*cobra.Command)
	reset = func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if sliceValue, ok := f.Value.(pflag.SliceValue); ok {
				_ = sliceValue.Replace(defaultSliceFlagValue(t, f))
			} else {
				_ = f.Value.Set(f.DefValue)
			}
			f.Changed = false
		})
		for _, c := range cmd.Commands() {
			reset(c)
		}
	}
	reset(rootCmd)
	resetFlagValue(rootCmd.PersistentFlags(), "confirm", "")
}

func defaultSliceFlagValue(t *testing.T, f *pflag.Flag) []string {
	t.Helper()
	if f.DefValue == "" || f.DefValue == "[]" {
		return []string{}
	}
	values, err := csv.NewReader(strings.NewReader(f.DefValue)).Read()
	if err != nil {
		t.Fatalf("parse default for --%s: %v", f.Name, err)
	}
	return values
}

func issueRunRoot(t *testing.T, args ...string) (stdout, stderr string, err error) {
	resetCLIState(t)
	return runRoot(t, textFormatUnlessSpecified(args)...)
}

func issueRunOK(t *testing.T, args ...string) (stdout, stderr string) {
	resetCLIState(t)
	return runRootOK(t, textFormatUnlessSpecified(args)...)
}

func issueRunSilent(t *testing.T, code int, args ...string) (stdout, stderr string) {
	resetCLIState(t)
	return runRootExpectSilent(t, code, textFormatUnlessSpecified(args)...)
}

const (
	sampleIssueKey = "PROJ-1"
	sampleFields   = `"summary":"Test issue","description":"Line one","status":{"id":"1","name":"To Do"},"issuetype":{"id":"1","name":"Task","subtask":false},"priority":{"id":"2","name":"High"},"assignee":{"name":"alice","displayName":"Alice"},"reporter":{"name":"bob","displayName":"Bob"},"created":"2024-01-01T10:00:00.000+0000","updated":"2024-01-02T12:00:00.000+0000","labels":["bug","cli"],"components":[{"id":"1","name":"Backend"}],"fixVersions":[{"id":"1","name":"v1.0"}],"attachment":[{"id":"att1","filename":"note.txt","size":128,"mimeType":"text/plain","author":{"displayName":"Alice"},"created":"2024-01-01T10:00:00.000+0000"}],"issuelinks":[{"id":"link1","type":{"name":"Blocks"},"outwardIssue":{"key":"PROJ-99"}},{"id":"link2","type":{"name":"Relates"},"inwardIssue":{"key":"PROJ-88"}},{"id":"link3","type":{"name":"Duplicate"}}]`
)

func sampleIssueJSON(key string) string {
	if key == "" {
		key = sampleIssueKey
	}
	return fmt.Sprintf(`{"id":"10001","key":%q,"self":"https://jira.test/rest/api/2/issue/%s","fields":{%s}}`, key, key, sampleFields)
}

func issueKeyFromPath(path string) string {
	const prefix = "/rest/api/2/issue/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if idx := strings.Index(rest, "/"); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}

func issueMockHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	fieldsJSON := `[{"id":"summary","name":"Summary","custom":false},{"id":"customfield_10001","name":"Story Points","custom":true}]`

	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		key := issueKeyFromPath(path)

		switch {
		case path == "/rest/api/2/myself" && method == http.MethodGet:
			writeJSON(w, http.StatusOK, `{"name":"alice","displayName":"Alice","emailAddress":"alice@example.com"}`)

		case path == "/rest/api/2/field" && method == http.MethodGet:
			writeJSON(w, http.StatusOK, fieldsJSON)

		case path == "/rest/api/2/search" && method == http.MethodPost:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			jql, _ := body["jql"].(string)
			if strings.Contains(jql, "project = \"EMPTY\"") {
				writeJSON(w, http.StatusOK, `{"startAt":0,"maxResults":50,"total":0,"issues":[]}`)
				return
			}
			if strings.Contains(jql, "project = \"BULKJQL\"") {
				writeJSON(w, http.StatusOK, `{"startAt":0,"maxResults":100,"total":2,"issues":[{"id":"1","key":"PROJ-1","fields":{"summary":"a","status":{"name":"To Do"},"issuetype":{"name":"Task"}}},{"id":"2","key":"PROJ-SKIP","fields":{"summary":"b","status":{"name":"To Do"},"issuetype":{"name":"Task"}}}]}`)
				return
			}
			writeJSON(w, http.StatusOK, fmt.Sprintf(`{"startAt":0,"maxResults":50,"total":1,"issues":[%s]}`, sampleIssueJSON(sampleIssueKey)))

		case path == "/rest/api/2/issue" && method == http.MethodPost:
			writeJSON(w, http.StatusCreated, `{"id":"10002","key":"PROJ-2"}`)

		case path == "/rest/api/2/issueLink" && method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)

		case path == "/rest/api/2/issueLinkType" && method == http.MethodGet:
			writeJSON(w, http.StatusOK, `{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"}]}`)

		case strings.HasPrefix(path, "/rest/api/2/issueLink/") && method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)

		case strings.HasSuffix(path, "/transitions"):
			if key == "PROJ-FAIL-T" {
				writeJSON(w, http.StatusInternalServerError, `{"errorMessages":["transitions failed"]}`)
				return
			}
			if key == "PROJ-SKIP" || key == "PROJ-NOTRANS" {
				writeJSON(w, http.StatusOK, `{"transitions":[{"id":"99","name":"Close","to":{"name":"Done"}}]}`)
				return
			}
			if method == http.MethodGet {
				writeJSON(w, http.StatusOK, `{"transitions":[{"id":"21","name":"Start Progress","to":{"name":"In Progress"}},{"id":"31","name":"Done","to":{"name":"Done"}}]}`)
				return
			}
			if key == "PROJ-FAIL-D" {
				writeJSON(w, http.StatusInternalServerError, `{"errorMessages":["transition failed"]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case strings.HasSuffix(path, "/assignee") && method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)

		case strings.HasSuffix(path, "/watchers"):
			switch method {
			case http.MethodGet:
				if key == "PROJ-NOWATCH" {
					writeJSON(w, http.StatusOK, `{"watchers":[]}`)
					return
				}
				writeJSON(w, http.StatusOK, `{"watchers":[{"name":"alice","displayName":"Alice","emailAddress":"alice@example.com"}]}`)
			case http.MethodPost:
				w.WriteHeader(http.StatusNoContent)
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			}

		case strings.HasSuffix(path, "/votes"):
			switch method {
			case http.MethodPost:
				w.WriteHeader(http.StatusCreated)
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			}

		case strings.HasSuffix(path, "/comment"):
			switch method {
			case http.MethodGet:
				if key == "PROJ-NOCOMMENT" {
					writeJSON(w, http.StatusOK, `{"comments":[],"total":0,"startAt":0,"maxResults":20}`)
					return
				}
				writeJSON(w, http.StatusOK, `{"comments":[{"id":"10010","author":{"displayName":"Alice"},"body":"Hello world","created":"2024-01-01T10:00:00.000+0000"}],"total":1,"startAt":0,"maxResults":20}`)
			case http.MethodPost:
				writeJSON(w, http.StatusCreated, `{"id":"10011","author":{"displayName":"Alice"},"body":"New comment","created":"2024-01-02T10:00:00.000+0000"}`)
			}

		case strings.Contains(path, "/comment/") && method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)

		case strings.HasSuffix(path, "/worklog"):
			switch method {
			case http.MethodGet:
				if key == "PROJ-NOWORK" {
					writeJSON(w, http.StatusOK, `{"worklogs":[],"total":0,"startAt":0,"maxResults":100}`)
					return
				}
				writeJSON(w, http.StatusOK, `{"worklogs":[{"id":"20010","author":{"displayName":"Alice"},"timeSpent":"1h","started":"2024-01-01T10:00:00.000+0000","comment":"Did work"}],"total":1,"startAt":0,"maxResults":100}`)
			case http.MethodPost:
				writeJSON(w, http.StatusCreated, `{"id":"20011","author":{"displayName":"Alice"},"timeSpent":"30m","timeSpentSeconds":1800,"started":"2024-01-02T10:00:00.000+0000","comment":"Quick fix"}`)
			}

		case strings.HasSuffix(path, "/remotelink"):
			switch method {
			case http.MethodGet:
				if key == "PROJ-NOREMOTE" {
					writeJSON(w, http.StatusOK, `[]`)
					return
				}
				writeJSON(w, http.StatusOK, `[{"id":1,"object":{"url":"https://docs.example.com","title":"Docs"}}]`)
			case http.MethodPost:
				writeJSON(w, http.StatusCreated, `{"id":2,"object":{"url":"https://new.example.com","title":"New Link"}}`)
			}

		case strings.HasSuffix(path, "/attachments") && method == http.MethodPost:
			writeJSON(w, http.StatusOK, `[{"id":"att2","filename":"upload.txt","size":64,"mimeType":"text/plain","author":{"displayName":"Alice"},"created":"2024-01-03T10:00:00.000+0000"}]`)

		case strings.HasPrefix(path, "/rest/api/2/issue/") && method == http.MethodGet:
			if key == "PROJ-NOATTACH" {
				writeJSON(w, http.StatusOK, `{"id":"10001","key":"PROJ-NOATTACH","fields":{"summary":"No files","status":{"name":"Open"},"issuetype":{"name":"Task"},"attachment":[]}}`)
				return
			}
			if key == "PROJ-CLONE" {
				writeJSON(w, http.StatusOK, sampleIssueJSON("PROJ-CLONE"))
				return
			}
			writeJSON(w, http.StatusOK, sampleIssueJSON(key))

		case strings.HasPrefix(path, "/rest/api/2/issue/") && method == http.MethodPut:
			if r.Header.Get("X-Fail-Edit") == "1" {
				writeJSON(w, http.StatusInternalServerError, `{"errorMessages":["edit failed"]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case strings.HasPrefix(path, "/rest/api/2/issue/") && method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected request: %s %s", method, path)
			writeJSON(w, http.StatusNotFound, `{"errorMessages":["not found"]}`)
		}
	}
}

func issueMockHandlerWithLinkFailure(t *testing.T) http.HandlerFunc {
	t.Helper()
	base := issueMockHandler(t)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issueLink" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"errorMessages":["link failed"]}`)
			return
		}
		base(w, r)
	}
}

func issueMockHandlerWithEditFailure(t *testing.T) http.HandlerFunc {
	t.Helper()
	base := issueMockHandler(t)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") && !strings.HasSuffix(r.URL.Path, "/assignee") {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			if strings.Contains(string(body), "customfield_") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"errorMessages":["edit failed"]}`)
				return
			}
		}
		base(w, r)
	}
}

func runRootWithStdinIssue(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetCLIState(t)
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("stdin pipe: %v", pipeErr)
	}
	go func() {
		_, _ = io.WriteString(w, stdin)
		_ = w.Close()
	}()
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })
	return runRoot(t, textFormatUnlessSpecified(args)...)
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestIssueCommands_NotConfigured(t *testing.T) {
	clearJiraEnv(t)

	_, _, err := issueRunRoot(t, "issue", "get", sampleIssueKey)
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitAuth)
	}
}

func TestIssueGet(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "get", sampleIssueKey)
		if !containsAny(stdout, sampleIssueKey, "Test issue", "To Do") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json flat", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "get", sampleIssueKey)
		if !containsAny(stdout, "PROJ-1", "Test issue") || containsAny(stdout, `"issuelinks"`, `"attachment"`) {
			t.Fatalf("expected flat JSON, got: %s", stdout)
		}
	})

	t.Run("json raw", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "get", sampleIssueKey, "--raw")
		if !strings.Contains(stdout, `"fields"`) {
			t.Fatalf("expected raw JSON, got: %s", stdout)
		}
	})

	t.Run("json fields", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "get", sampleIssueKey, "--fields", "key,summary,status")
		if !containsAny(stdout, `"key"`, `"summary"`, `"status"`) || strings.Contains(stdout, `"assignee"`) {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("expand", func(t *testing.T) {
		issueRunOK(t, "issue", "get", sampleIssueKey, "--expand", "transitions")
	})
}

func TestIssueList(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("missing project", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "list")
	})

	t.Run("invalid limit", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "list", "--project", "PROJ", "--limit", "0")
		issueRunSilent(t, ExitBadArgs, "issue", "list", "--project", "PROJ", "--limit", "101")
	})

	t.Run("table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "list", "--project", "PROJ", "--status", "Open", "--assignee", "alice", "--type", "Task", "--label", "bug", "--priority", "High", "--order-by", "updated", "--limit", "10")
		if !containsAny(stdout, sampleIssueKey, "Test issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "list", "--project", "EMPTY")
		if !containsAny(stdout, "No issues found") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json raw", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "list", "--project", "PROJ", "--raw")
		if !containsAny(stdout, `"fields"`, "PROJ-1") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json fields", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "list", "--project", "PROJ", "--fields", "key,summary")
		if !strings.Contains(stdout, `"summary"`) || strings.Contains(stdout, `"assignee"`) {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueCreate(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("missing required", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "create", "--project", "PROJ")
		issueRunSilent(t, ExitBadArgs, "issue", "create", "--summary", "x")
	})

	t.Run("dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "create", "--project", "PROJ", "--summary", "New")
		if !containsAny(stdout, "create issue", "dry-run") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("success plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "create", "--project", "PROJ", "--summary", "New", "--description", "Desc", "--assignee", "me", "--priority", "High", "--label", "a", "--label", "b", "--component", "Backend", "--fix-version", "v1.0", "--parent", "PROJ-1")
		if !containsAny(stdout, "Created PROJ-2", "/browse/PROJ-2") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("success json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "create", "--project", "PROJ", "--summary", "New")
		if !containsAny(stdout, "PROJ-2", "url") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("custom field invalid", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "create", "--project", "PROJ", "--summary", "New", "--field", "badfield")
		if !containsAny(stdout, "Created PROJ-2", "Custom fields failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("custom field edit failure json", func(t *testing.T) {
		mockJiraServer(t, issueMockHandlerWithEditFailure(t))
		stdout, _ := issueRunOK(t, "--json", "issue", "create", "--project", "PROJ", "--summary", "New", "--field", "Story Points=5")
		if !containsAny(stdout, "warning", "PROJ-2") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("custom field success", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "create", "--project", "PROJ", "--summary", "New", "--field", "Story Points=5")
		if !containsAny(stdout, "Created PROJ-2") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueEdit(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("no fields", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "edit", sampleIssueKey)
	})

	t.Run("dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "edit", sampleIssueKey, "--summary", "Updated")
		if !containsAny(stdout, "edit issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("success plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "edit", sampleIssueKey, "--summary", "Updated", "--description", "New desc", "--assignee", "me", "--priority", "Low", "--label", "x", "--component", "Backend", "--fix-version", "v2.0")
		if !containsAny(stdout, "Updated PROJ-1") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("success json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "edit", sampleIssueKey, "--summary", "Updated")
		if !containsAny(stdout, "updated", "PROJ-1") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("custom field invalid", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "edit", sampleIssueKey, "--field", "badfield")
		if !containsAny(stdout, "Updated PROJ-1", "Custom fields failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("custom field edit failure json", func(t *testing.T) {
		mockJiraServer(t, issueMockHandlerWithEditFailure(t))
		stdout, _ := issueRunOK(t, "--json", "issue", "edit", sampleIssueKey, "--summary", "Updated", "--field", "Story Points=3")
		if !containsAny(stdout, "warning", "updated") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueDelete(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "delete", sampleIssueKey)
		if !containsAny(stdout, "delete issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		_, _, err := runRootWithStdinIssue(t, "WRONG\n", "issue", "delete", sampleIssueKey)
		if err != ErrSilent {
			t.Fatalf("expected ErrSilent, got %v", err)
		}
		if LastExitCode() != ExitBadArgs {
			t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
		}
	})

	t.Run("force", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--force", "issue", "delete", sampleIssueKey)
		if !containsAny(stdout, "Deleted PROJ-1") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json confirmed", func(t *testing.T) {
		resetCLIState(t)
		token := dryRunConfirmToken(t, "--json", "issue", "delete", sampleIssueKey)
		resetCLIState(t)
		stdout, _ := runRootOK(t, "--confirm", token, "--json", "issue", "delete", sampleIssueKey)
		var result map[string]string
		decodeEnvelopeData(t, stdout, &result)
		if result["key"] != sampleIssueKey || result["status"] != "deleted" {
			t.Fatalf("result=%v stdout=%s", result, stdout)
		}
	})
}

func TestIssueAssignWatchVote(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("assign dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "assign", sampleIssueKey, "me")
		if !containsAny(stdout, "assign issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("assign plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "assign", sampleIssueKey, "bob")
		if !containsAny(stdout, "assigned to bob") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("assign json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "assign", sampleIssueKey, "me")
		if !containsAny(stdout, "alice", "assignee") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("unassign dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "unassign", sampleIssueKey)
	})

	t.Run("unassign", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "unassign", sampleIssueKey)
		if !containsAny(stdout, "unassigned") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("watch dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "watch", sampleIssueKey)
	})

	t.Run("watch plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "watch", sampleIssueKey)
		if !containsAny(stdout, "Now watching") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("watch json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "watch", sampleIssueKey)
		if !containsAny(stdout, "watching") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("unwatch dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "unwatch", sampleIssueKey)
		if !containsAny(stdout, "unwatch issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("unwatch json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "unwatch", sampleIssueKey)
		if !containsAny(stdout, "unwatched") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("watchers empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "watchers", "PROJ-NOWATCH")
		if !containsAny(stdout, "No watchers") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("watchers table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "watchers", sampleIssueKey)
		if !containsAny(stdout, "alice", "Alice") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("watchers json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "watchers", sampleIssueKey)
		if !containsAny(stdout, "Alice") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("vote dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "vote", sampleIssueKey)
	})

	t.Run("vote plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "vote", sampleIssueKey)
		if !containsAny(stdout, "Voted for") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("unvote json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "unvote", sampleIssueKey)
		if !containsAny(stdout, "unvoted") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueTransition(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("transitions table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "transitions", sampleIssueKey)
		if !containsAny(stdout, "Start Progress", "In Progress") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("transitions json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "transitions", sampleIssueKey)
		if !containsAny(stdout, "Start Progress") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("transition unavailable", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "transition", "PROJ-NOTRANS", "In Progress")
	})

	t.Run("transition dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "transition", sampleIssueKey, "In Progress")
		if !containsAny(stdout, "transition issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("transition plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "transition", sampleIssueKey, "In Progress")
		if !containsAny(stdout, "transitioned", "To Do", "In Progress") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("transition json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "transition", sampleIssueKey, "Done")
		if !containsAny(stdout, "toStatus", "Done", "fromStatus") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueBulkTransition(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("missing selector", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "bulk-transition", "In Progress")
	})

	t.Run("empty issues", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "bulk-transition", "In Progress", "--issues", " , ")
		if !containsAny(stdout, "No issues to transition") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("dry-run", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "bulk-transition", "In Progress", "--issues", "PROJ-1")
		if !containsAny(stdout, "bulk-transition") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("issues mixed results plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "bulk-transition", "In Progress", "--issues", "PROJ-1,PROJ-SKIP,PROJ-FAIL-T,PROJ-FAIL-D")
		if !containsAny(stdout, "Done:", "skipped", "failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("issues json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "bulk-transition", "In Progress", "--issues", "PROJ-1,PROJ-SKIP")
		if !containsAny(stdout, `"skipped"`, `"success"`, `"results"`) {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("jql", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "bulk-transition", "In Progress", "--jql", "project = BULKJQL")
		if !containsAny(stdout, `"total":2`, `"success"`) {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueComment(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("add missing body", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "comment", "add", sampleIssueKey)
	})

	t.Run("add dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "comment", "add", sampleIssueKey, "--body", "Hi")
	})

	t.Run("add plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "comment", "add", sampleIssueKey, "--body", "Hi")
		if !containsAny(stdout, "Comment added", "10011") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("add json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "comment", "add", sampleIssueKey, "--body", "Hi")
		if !containsAny(stdout, "10011") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "comment", "list", "PROJ-NOCOMMENT")
		if !containsAny(stdout, "No comments found") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "comment", "list", sampleIssueKey, "--limit", "5")
		if !containsAny(stdout, "10010", "Hello world", "Alice") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "comment", "list", sampleIssueKey)
		if !containsAny(stdout, "10010") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("delete missing id", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "comment", "delete", sampleIssueKey)
	})

	t.Run("delete dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "comment", "delete", sampleIssueKey, "--id", "10010")
	})

	t.Run("delete", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "comment", "delete", sampleIssueKey, "--id", "10010")
		if !containsAny(stdout, "Comment 10010 deleted") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueWorklog(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("add missing time", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "worklog", "add", sampleIssueKey)
	})

	t.Run("add dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "worklog", "add", sampleIssueKey, "--time", "1h")
	})

	t.Run("add plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "worklog", "add", sampleIssueKey, "--time", "30m", "--started", "2024-01-02T10:00:00.000+0000", "--comment", "Fix")
		if !containsAny(stdout, "Logged", "30m", "20011") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("add json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "worklog", "add", sampleIssueKey, "--time", "1h")
		if !containsAny(stdout, "20011") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "worklog", "list", "PROJ-NOWORK")
		if !containsAny(stdout, "No worklogs") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "worklog", "list", sampleIssueKey)
		if !containsAny(stdout, "20010", "1h", "Did work") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "worklog", "list", sampleIssueKey)
		if !containsAny(stdout, "20010") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueLink(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	t.Run("link missing to", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "link", sampleIssueKey, "--type", "Blocks")
	})

	t.Run("link missing type", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "link", sampleIssueKey, "--to", "PROJ-99")
	})

	t.Run("link dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "link", sampleIssueKey, "--to", "PROJ-99", "--type", "Blocks")
	})

	t.Run("link", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "link", sampleIssueKey, "--to", "PROJ-99", "--type", "Blocks")
		if !containsAny(stdout, "Linked", "Blocks") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("unlink dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "unlink", "10000")
	})

	t.Run("unlink", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "unlink", "10000")
		if !containsAny(stdout, "Link 10000 removed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("link-types table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "link-types")
		if !containsAny(stdout, "Blocks", "blocks") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("link-types json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "link-types")
		if !containsAny(stdout, "Blocks") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("remote-link missing flags", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "remote-link", sampleIssueKey, "--url", "https://x.com")
	})

	t.Run("remote-link dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "remote-link", sampleIssueKey, "--url", "https://x.com", "--title", "X")
	})

	t.Run("remote-link plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "remote-link", sampleIssueKey, "--url", "https://x.com", "--title", "X")
		if !containsAny(stdout, "Remote link added") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("remote-link json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "remote-link", sampleIssueKey, "--url", "https://x.com", "--title", "X")
		if !containsAny(stdout, `"id"`, "2") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("remote-links empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "remote-links", "PROJ-NOREMOTE")
		if !containsAny(stdout, "No remote links") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("remote-links table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "remote-links", sampleIssueKey)
		if !containsAny(stdout, "Docs", "https://docs.example.com") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("remote-links json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "remote-links", sampleIssueKey)
		if !containsAny(stdout, "Docs") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueClone(t *testing.T) {
	t.Run("dry-run", func(t *testing.T) {
		mockJiraServer(t, issueMockHandler(t))
		stdout, _ := issueRunOK(t, "--dry-run", "issue", "clone", "PROJ-CLONE")
		if !containsAny(stdout, "clone issue") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("plain", func(t *testing.T) {
		mockJiraServer(t, issueMockHandler(t))
		stdout, _ := issueRunOK(t, "issue", "clone", "PROJ-CLONE", "--summary", "Clone summary", "--project", "PROJ")
		if !containsAny(stdout, "Cloned PROJ-CLONE", "PROJ-2", "/browse/PROJ-2") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json with links", func(t *testing.T) {
		mockJiraServer(t, issueMockHandlerWithLinkFailure(t))
		stdout, _ := issueRunOK(t, "--json", "issue", "clone", "PROJ-CLONE", "--with-links")
		if !containsAny(stdout, "PROJ-CLONE", "linkErrors") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("plain with link warnings", func(t *testing.T) {
		mockJiraServer(t, issueMockHandlerWithLinkFailure(t))
		stdout, _ := issueRunOK(t, "issue", "clone", "PROJ-CLONE", "--with-links")
		if !containsAny(stdout, "Cloned", "link(s) failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueAttach(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))
	filePath := writeTempFile(t, "upload.txt", "hello attachment")

	t.Run("missing file", func(t *testing.T) {
		issueRunSilent(t, ExitBadArgs, "issue", "attach", sampleIssueKey)
	})

	t.Run("dry-run", func(t *testing.T) {
		issueRunOK(t, "--dry-run", "issue", "attach", sampleIssueKey, "--file", filePath)
	})

	t.Run("plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "attach", sampleIssueKey, "--file", filePath)
		if !containsAny(stdout, "Uploaded", "upload.txt") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "attach", sampleIssueKey, "--file", filePath)
		if !containsAny(stdout, "upload.txt") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("attachments empty", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "attachments", "PROJ-NOATTACH")
		if !containsAny(stdout, "No attachments") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("attachments table", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "attachments", sampleIssueKey)
		if !containsAny(stdout, "note.txt", "128") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("attachments json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "attachments", sampleIssueKey)
		if !containsAny(stdout, "note.txt") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func adfLongText(s string) string {
	doc := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": s},
				},
			},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func issueFailHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, `{"errorMessages":["request failed"]}`)
	}
}

func TestIssueCommands_NotConfigured_AllRunE(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"get", []string{"issue", "get", sampleIssueKey}},
		{"list", []string{"issue", "list", "--project", "PROJ"}},
		{"create", []string{"issue", "create", "--project", "PROJ", "--summary", "s"}},
		{"edit", []string{"issue", "edit", sampleIssueKey, "--summary", "s"}},
		{"delete", []string{"issue", "delete", sampleIssueKey, "--force"}},
		{"assign", []string{"issue", "assign", sampleIssueKey, "bob"}},
		{"unassign", []string{"issue", "unassign", sampleIssueKey}},
		{"watch", []string{"issue", "watch", sampleIssueKey}},
		{"unwatch", []string{"issue", "unwatch", sampleIssueKey}},
		{"watchers", []string{"issue", "watchers", sampleIssueKey}},
		{"vote", []string{"issue", "vote", sampleIssueKey}},
		{"unvote", []string{"issue", "unvote", sampleIssueKey}},
		{"transition", []string{"issue", "transition", sampleIssueKey, "Done"}},
		{"transitions", []string{"issue", "transitions", sampleIssueKey}},
		{"bulk-transition", []string{"issue", "bulk-transition", "Done", "--issues", "PROJ-1"}},
		{"comment add", []string{"issue", "comment", "add", sampleIssueKey, "--body", "x"}},
		{"comment list", []string{"issue", "comment", "list", sampleIssueKey}},
		{"comment delete", []string{"issue", "comment", "delete", sampleIssueKey, "--id", "1"}},
		{"worklog add", []string{"issue", "worklog", "add", sampleIssueKey, "--time", "1h"}},
		{"worklog list", []string{"issue", "worklog", "list", sampleIssueKey}},
		{"link", []string{"issue", "link", sampleIssueKey, "--to", "PROJ-2", "--type", "Blocks"}},
		{"unlink", []string{"issue", "unlink", "10000"}},
		{"link-types", []string{"issue", "link-types"}},
		{"remote-link", []string{"issue", "remote-link", sampleIssueKey, "--url", "https://x.com", "--title", "T"}},
		{"remote-links", []string{"issue", "remote-links", sampleIssueKey}},
		{"clone", []string{"issue", "clone", "PROJ-CLONE"}},
		{"attachments", []string{"issue", "attachments", sampleIssueKey}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearJiraEnv(t)
			tmpHome := t.TempDir()
			t.Setenv("HOME", tmpHome)
			t.Setenv("USERPROFILE", tmpHome)
			args := tc.args
			if tc.name == "attach" {
				args = []string{"issue", "attach", sampleIssueKey, "--file", writeTempFile(t, "x.txt", "x")}
			}
			_, _, err := issueRunRoot(t, args...)
			if err != ErrSilent {
				t.Fatalf("expected ErrSilent, got %v (exit=%d)", err, LastExitCode())
			}
			if LastExitCode() != ExitAuth {
				t.Fatalf("exit=%d, want %d", LastExitCode(), ExitAuth)
			}
		})
	}
}

func TestIssueCommands_APIErrors(t *testing.T) {
	filePath := writeTempFile(t, "up.txt", "data")

	cases := []struct {
		name string
		args []string
		path string
		code int
	}{
		{"get", []string{"issue", "get", sampleIssueKey}, "/rest/api/2/issue/PROJ-1", http.StatusNotFound},
		{"list", []string{"issue", "list", "--project", "PROJ"}, "/rest/api/2/search", http.StatusBadRequest},
		{"create", []string{"issue", "create", "--project", "PROJ", "--summary", "s"}, "/rest/api/2/issue", http.StatusBadRequest},
		{"edit", []string{"issue", "edit", sampleIssueKey, "--summary", "s"}, "/rest/api/2/issue/PROJ-1", http.StatusBadRequest},
		{"delete", []string{"--force", "issue", "delete", sampleIssueKey}, "/rest/api/2/issue/PROJ-1", http.StatusBadRequest},
		{"assign", []string{"issue", "assign", sampleIssueKey, "bob"}, "/rest/api/2/issue/PROJ-1/assignee", http.StatusBadRequest},
		{"unassign", []string{"issue", "unassign", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/assignee", http.StatusBadRequest},
		{"watch", []string{"issue", "watch", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/watchers", http.StatusBadRequest},
		{"unwatch", []string{"issue", "unwatch", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/watchers", http.StatusBadRequest},
		{"watchers", []string{"issue", "watchers", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/watchers", http.StatusNotFound},
		{"vote", []string{"issue", "vote", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/votes", http.StatusBadRequest},
		{"unvote", []string{"issue", "unvote", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/votes", http.StatusBadRequest},
		{"transition", []string{"issue", "transition", sampleIssueKey, "Done"}, "/rest/api/2/issue/PROJ-1/transitions", http.StatusBadRequest},
		{"transitions", []string{"issue", "transitions", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/transitions", http.StatusNotFound},
		{"bulk jql", []string{"issue", "bulk-transition", "Done", "--jql", "bad"}, "/rest/api/2/search", http.StatusBadRequest},
		{"comment add", []string{"issue", "comment", "add", sampleIssueKey, "--body", "x"}, "/rest/api/2/issue/PROJ-1/comment", http.StatusBadRequest},
		{"comment list", []string{"issue", "comment", "list", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/comment", http.StatusNotFound},
		{"comment delete", []string{"issue", "comment", "delete", sampleIssueKey, "--id", "1"}, "/rest/api/2/issue/PROJ-1/comment/1", http.StatusNotFound},
		{"worklog add", []string{"issue", "worklog", "add", sampleIssueKey, "--time", "1h"}, "/rest/api/2/issue/PROJ-1/worklog", http.StatusBadRequest},
		{"worklog list", []string{"issue", "worklog", "list", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/worklog", http.StatusNotFound},
		{"link", []string{"issue", "link", sampleIssueKey, "--to", "PROJ-2", "--type", "Blocks"}, "/rest/api/2/issueLink", http.StatusBadRequest},
		{"unlink", []string{"issue", "unlink", "10000"}, "/rest/api/2/issueLink/10000", http.StatusNotFound},
		{"link-types", []string{"issue", "link-types"}, "/rest/api/2/issueLinkType", http.StatusNotFound},
		{"remote-link", []string{"issue", "remote-link", sampleIssueKey, "--url", "https://x.com", "--title", "T"}, "/rest/api/2/issue/PROJ-1/remotelink", http.StatusBadRequest},
		{"remote-links", []string{"issue", "remote-links", sampleIssueKey}, "/rest/api/2/issue/PROJ-1/remotelink", http.StatusNotFound},
		{"clone get", []string{"issue", "clone", "PROJ-CLONE"}, "/rest/api/2/issue/PROJ-CLONE", http.StatusNotFound},
		{"attach", []string{"issue", "attach", sampleIssueKey, "--file", filePath}, "/rest/api/2/issue/PROJ-1/attachments", http.StatusBadRequest},
		{"attachments", []string{"issue", "attachments", sampleIssueKey}, "/rest/api/2/issue/PROJ-1", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/myself" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{"name":"alice","displayName":"Alice"}`)
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/search" && tc.name == "bulk jql" {
					issueFailHandler(http.StatusBadRequest)(w, r)
					return
				}
				if r.URL.Path == tc.path || (tc.name == "transition" && r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/transitions")) {
					issueFailHandler(tc.code)(w, r)
					return
				}
				if tc.name == "transition" && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/transitions") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{"transitions":[{"id":"21","name":"Done","to":{"name":"Done"}}]}`)
					return
				}
				if tc.name == "transition" && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issue/PROJ-1") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, sampleIssueJSON(sampleIssueKey))
					return
				}
				issueMockHandler(t)(w, r)
			})
			issueRunSilent(t, exitCodeForStatus(tc.code), tc.args...)
		})
	}
}

func TestIssueCommands_ResolveUsernameError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/myself" {
			issueFailHandler(http.StatusUnauthorized)(w, r)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunSilent(t, ExitAuth, "issue", "create", "--project", "PROJ", "--summary", "s", "--assignee", "me")
	issueRunSilent(t, ExitAuth, "issue", "edit", sampleIssueKey, "--assignee", "me")
	issueRunSilent(t, ExitAuth, "issue", "assign", sampleIssueKey, "me")
}

func TestIssueCommands_CreateEditCustomFieldPlainWarnings(t *testing.T) {
	t.Run("create editraw plain", func(t *testing.T) {
		mockJiraServer(t, issueMockHandlerWithEditFailure(t))
		stdout, _ := issueRunOK(t, "issue", "create", "--project", "PROJ", "--summary", "New", "--field", "Story Points=5")
		if !containsAny(stdout, "Created PROJ-2", "Custom fields failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("edit editraw plain", func(t *testing.T) {
		mockJiraServer(t, issueMockHandlerWithEditFailure(t))
		stdout, _ := issueRunOK(t, "issue", "edit", sampleIssueKey, "--summary", "Updated", "--field", "Story Points=3")
		if !containsAny(stdout, "Updated PROJ-1", "Custom fields failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueClone_DefaultProjectFromKey(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))
	issueRunOK(t, "issue", "clone", "PROJ-CLONE")
}

func TestIssueCommands_MiscBranches(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comment") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"comments":[{"id":"10010","author":{"displayName":"Alice"},"body":%s,"created":"2024-01-01T10:00:00.000+0000"}],"total":1}`, adfLongText(strings.Repeat("x", 210)))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/worklog") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"worklogs":[{"id":"20010","author":{"displayName":"Alice"},"timeSpent":"1h","started":"2024-01-01T10:00:00.000+0000","comment":%s}],"total":1,"startAt":0,"maxResults":100}`, adfLongText(strings.Repeat("y", 90)))
			return
		}
		issueMockHandler(t)(w, r)
	})

	t.Run("unwatch plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "unwatch", sampleIssueKey)
		if !containsAny(stdout, "Stopped watching") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("vote json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "vote", sampleIssueKey)
		if !containsAny(stdout, "voted") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("comment list truncate", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "comment", "list", sampleIssueKey)
		if !containsAny(stdout, "...", "…") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("worklog list truncate", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "worklog", "list", sampleIssueKey)
		if !containsAny(stdout, "...", "…") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("list assignee me", func(t *testing.T) {
		issueRunOK(t, "issue", "list", "--project", "PROJ", "--assignee", "me")
	})
}

func TestIssueTransition_DoTransitionError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/transitions") {
			issueFailHandler(http.StatusBadRequest)(w, r)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunSilent(t, ExitBadArgs, "issue", "transition", sampleIssueKey, "Done")
}

func TestIssueCreate_EditRawFailurePaths(t *testing.T) {
	mockJiraServer(t, issueMockHandlerWithEditFailure(t))

	t.Run("create json", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "--json", "issue", "create", "--project", "PROJ", "--summary", "New", "--field", "Story Points=8")
		if !containsAny(stdout, "warning", "PROJ-2") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("create plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "create", "--project", "PROJ", "--summary", "New", "--field", "Story Points=8")
		if !containsAny(stdout, "Created PROJ-2", "Custom fields failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})

	t.Run("edit plain", func(t *testing.T) {
		stdout, _ := issueRunOK(t, "issue", "edit", sampleIssueKey, "--field", "Story Points=8")
		if !containsAny(stdout, "Updated PROJ-1", "Custom fields failed") {
			t.Fatalf("stdout=%q", stdout)
		}
	})
}

func TestIssueUnwatch_APIError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/myself" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"name":"alice","displayName":"Alice"}`)
			return
		}
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/watchers") {
			issueFailHandler(http.StatusBadRequest)(w, r)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunSilent(t, ExitBadArgs, "issue", "unwatch", sampleIssueKey)
}

func TestIssueAttach_NotConfigured(t *testing.T) {
	clearJiraEnv(t)
	filePath := writeTempFile(t, "a.txt", "a")
	_, _, err := issueRunRoot(t, "issue", "attach", sampleIssueKey, "--file", filePath)
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
}

func TestIssueWatchUnwatch_MeError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/myself" {
			issueFailHandler(http.StatusUnauthorized)(w, r)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunSilent(t, ExitAuth, "issue", "watch", sampleIssueKey)
	issueRunSilent(t, ExitAuth, "issue", "unwatch", sampleIssueKey)
}

func TestIssueUnvote_Plain(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))
	stdout, _ := issueRunOK(t, "issue", "unvote", sampleIssueKey)
	if !containsAny(stdout, "Vote removed") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestIssueClone_ParseProjectFromSelfURL(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issue/PROJ-CLONE" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"id":"1","key":"PROJ-CLONE","self":"https://jira.test/rest/api/2/issue/PROJ-CLONE","fields":{"summary":"s","status":{"name":"Open"},"issuetype":{"name":"Task"}}}`)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunOK(t, "issue", "clone", "PROJ-CLONE")
}

func TestIssueClone_ParseProjectFallbackFromKey(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issue/PROJ-CLONE" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"id":"1","key":"PROJ-CLONE","self":"https://jira.test/rest/api/2/issue/99999","fields":{"summary":"s","status":{"name":"Open"},"issuetype":{"name":"Task"}}}`)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunOK(t, "issue", "clone", "PROJ-CLONE")
}

func TestIssueUnvote_DryRun(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))
	issueRunOK(t, "--dry-run", "issue", "unvote", sampleIssueKey)
}

func TestIssueClone_CreateError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issue" && r.Method == http.MethodPost {
			issueFailHandler(http.StatusBadRequest)(w, r)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunSilent(t, ExitBadArgs, "issue", "clone", "PROJ-CLONE")
}

func TestIssueTransition_GetIssueError(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/issue/PROJ-1") {
			issueFailHandler(http.StatusNotFound)(w, r)
			return
		}
		issueMockHandler(t)(w, r)
	})
	issueRunSilent(t, ExitNotFound, "issue", "transition", sampleIssueKey, "Done")
}

func TestIssueComment_Worklog_TruncatePlain(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/comment") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"comments":[{"id":"1","author":{"displayName":"A"},"body":%s,"created":"2024-01-01T10:00:00.000+0000"}],"total":1}`, adfLongText(strings.Repeat("z", 210)))
		case strings.HasSuffix(r.URL.Path, "/worklog") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"worklogs":[{"id":"1","author":{"displayName":"A"},"timeSpent":"1h","started":"2024-01-01T10:00:00.000+0000","comment":%s}],"total":1,"startAt":0,"maxResults":100}`, adfLongText(strings.Repeat("w", 90)))
		default:
			issueMockHandler(t)(w, r)
		}
	})
	stdout, _ := issueRunOK(t, "issue", "comment", "list", sampleIssueKey)
	if !containsAny(stdout, "...", "…") {
		t.Fatalf("comment stdout=%q", stdout)
	}
	stdout, _ = issueRunOK(t, "issue", "worklog", "list", sampleIssueKey)
	if !containsAny(stdout, "...", "…") {
		t.Fatalf("worklog stdout=%q", stdout)
	}
}
