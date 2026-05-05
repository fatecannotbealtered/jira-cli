package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Issue.Get ────────────────────────────────────────────────────────────────

func TestIssueGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"id": "10001",
			"key": "PROJ-123",
			"fields": {
				"summary": "Test issue",
				"status": {"name": "To Do"},
				"issuetype": {"name": "Task"},
				"priority": {"name": "High"}
			}
		}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issue, err := c.Issues.Get("PROJ-123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Key != "PROJ-123" {
		t.Errorf("Key = %q, want PROJ-123", issue.Key)
	}
	if issue.Fields.Summary != "Test issue" {
		t.Errorf("Summary = %q, want 'Test issue'", issue.Fields.Summary)
	}
	if issue.Fields.Status.Name != "To Do" {
		t.Errorf("Status = %q, want 'To Do'", issue.Fields.Status.Name)
	}
}

func TestIssueGet_WithExpand(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":"1","key":"X-1","fields":{"summary":"s","status":{"name":"Open"},"issuetype":{"name":"Task"}}}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Get("X-1", []string{"changelog", "renderedFields"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedQuery, "expand=") {
		t.Errorf("expected expand query param, got %q", receivedQuery)
	}
}

func TestIssueGet_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Get("NOTEXIST-1", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
}

// ─── Issue.List ───────────────────────────────────────────────────────────────

func TestIssueList_BuildsJQL(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":1,"issues":[{"id":"1","key":"P-1","fields":{"summary":"s","status":{"name":"Open"},"issuetype":{"name":"Task"}}}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	opts := IssueListOptions{
		Project:  "PROJ",
		Status:   "In Progress",
		Assignee: "me",
		Type:     "Bug",
		Limit:    25,
		OrderBy:  "updated",
	}
	issues, err := c.Issues.List(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}

	jql, _ := receivedBody["jql"].(string)
	if !strings.Contains(jql, `project = "PROJ"`) {
		t.Errorf("JQL missing project clause: %s", jql)
	}
	if !strings.Contains(jql, `status = "In Progress"`) {
		t.Errorf("JQL missing status clause: %s", jql)
	}
	if !strings.Contains(jql, "assignee = currentUser()") {
		t.Errorf("JQL missing assignee clause: %s", jql)
	}
	if !strings.Contains(jql, `issuetype = "Bug"`) {
		t.Errorf("JQL missing type clause: %s", jql)
	}
	if !strings.Contains(jql, "ORDER BY updated") {
		t.Errorf("JQL missing ORDER BY: %s", jql)
	}
}

// ─── Issue.Create ─────────────────────────────────────────────────────────────

func TestIssueCreate_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"10002","key":"PROJ-124"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	req := CreateIssueRequest{
		Fields: CreateIssueFields{
			Project:   ProjectRef{Key: "PROJ"},
			Summary:   "New issue",
			IssueType: IssueTypeRef{Name: "Task"},
		},
	}
	ref, err := c.Issues.Create(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Key != "PROJ-124" {
		t.Errorf("Key = %q, want PROJ-124", ref.Key)
	}
}

func TestIssueCreate_ConvertsDescriptionToADF(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"1","key":"P-1"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	req := CreateIssueRequest{
		Fields: CreateIssueFields{
			Project:     ProjectRef{Key: "P"},
			Summary:     "Test",
			IssueType:   IssueTypeRef{Name: "Task"},
			Description: "plain text description",
		},
	}
	_, err := c.Issues.Create(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields, _ := receivedBody["fields"].(map[string]any)
	desc, _ := fields["description"].(map[string]any)
	if desc["type"] != "doc" {
		t.Errorf("expected description to be ADF doc, got %v", desc)
	}
}

// ─── Issue.Edit ───────────────────────────────────────────────────────────────

func TestIssueEdit_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.Edit("PROJ-1", EditIssueRequest{
		Fields: EditIssueFields{Summary: "Updated"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Issue.Delete ─────────────────────────────────────────────────────────────

func TestIssueDelete_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.Delete("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Issue.GetTransitions ─────────────────────────────────────────────────────

func TestIssueGetTransitions_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"transitions":[{"id":"11","name":"Start Progress","to":{"name":"In Progress"}},{"id":"21","name":"Done","to":{"name":"Done"}}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	transitions, err := c.Issues.GetTransitions("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(transitions))
	}
	if transitions[0].Name != "Start Progress" {
		t.Errorf("first transition name = %q", transitions[0].Name)
	}
}

// ─── Issue.AddComment ─────────────────────────────────────────────────────────

func TestIssueAddComment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"10100","author":{"displayName":"Test"},"created":"2024-01-01T00:00:00.000+0000"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	comment, err := c.Issues.AddComment("PROJ-1", "test comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comment.ID != "10100" {
		t.Errorf("comment ID = %q, want 10100", comment.ID)
	}
}

// ─── Issue.Search ─────────────────────────────────────────────────────────────

func TestIssueSearch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":2,"issues":[
			{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Open"},"issuetype":{"name":"Task"}}},
			{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"Done"},"issuetype":{"name":"Bug"}}}
		]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	result, err := c.Issues.Search("project = P", SearchOptions{MaxResults: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if len(result.Issues) != 2 {
		t.Errorf("Issues count = %d, want 2", len(result.Issues))
	}
}

// ─── Issue.SearchAll (pagination) ─────────────────────────────────────────────

func TestIssueSearchAll_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		startAt := int(body["startAt"].(float64))
		callCount++

		w.WriteHeader(http.StatusOK)
		if startAt == 0 {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":3,"issues":[
				{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Open"},"issuetype":{"name":"Task"}}},
				{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"Open"},"issuetype":{"name":"Task"}}}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":100,"total":3,"issues":[
				{"id":"3","key":"P-3","fields":{"summary":"c","status":{"name":"Open"},"issuetype":{"name":"Task"}}}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, err := c.Issues.SearchAll("project = P", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(issues))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

// ─── Issue.AddWorklog ─────────────────────────────────────────────────────────

func TestIssueAddWorklog_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"10200","timeSpent":"2h","timeSpentSeconds":7200}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	worklog, err := c.Issues.AddWorklog("PROJ-1", "2h", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worklog.TimeSpent != "2h" {
		t.Errorf("TimeSpent = %q, want 2h", worklog.TimeSpent)
	}
}

func TestIssueAddWorklog_InvalidTime(t *testing.T) {
	c := newTestClient("http://unused")
	_, err := c.Issues.AddWorklog("PROJ-1", "invalid", "", "")
	if err == nil {
		t.Fatal("expected error for invalid time format")
	}
}

// ─── parseTimeSpent ───────────────────────────────────────────────────────────

func TestParseTimeSpent(t *testing.T) {
	cases := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"1h", 3600, false},
		{"30m", 1800, false},
		{"1h30m", 5400, false},
		{"2h15m", 8100, false},
		{"1d", 8 * 3600, false},
		{"2d", 16 * 3600, false},
		{"1d4h", 12 * 3600, false},
		{"1w", 5 * 8 * 3600, false},
		{"1w2d3h30m", 1*5*8*3600 + 2*8*3600 + 3*3600 + 30*60, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"0h0m", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseTimeSpent(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseTimeSpent(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}
