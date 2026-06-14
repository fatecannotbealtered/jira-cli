package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// Jira Data Center / Server REST API v2 takes a plain-string description, not
// Cloud ADF. Live smoke against a real DC instance rejected the ADF object with
// "description: must be a string"; this guards the v2 plain-string contract.
func TestIssueCreate_DescriptionIsPlainString(t *testing.T) {
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
	if desc, ok := fields["description"].(string); !ok || desc != "plain text description" {
		t.Errorf("expected description to be the plain string, got %#v", fields["description"])
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

func TestIssueEdit_DescriptionIsPlainString(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.Edit("PROJ-1", EditIssueRequest{
		Fields: EditIssueFields{Description: "plain text description"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// API v2 (Data Center/Server) takes a plain-string description, not ADF.
	fields, _ := receivedBody["fields"].(map[string]any)
	if desc, ok := fields["description"].(string); !ok || desc != "plain text description" {
		t.Errorf("expected description to be the plain string, got %#v", fields["description"])
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
	issues, truncated, err := c.Issues.SearchAll("project = P", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated {
		t.Errorf("expected truncated=false for a small result set")
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

// ─── Issue.ListWorklogs ───────────────────────────────────────────────────────

func TestIssueListWorklogs_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		startAt := r.URL.Query().Get("startAt")
		callCount++

		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":3,"worklogs":[
				{"id":"1","timeSpent":"1h","timeSpentSeconds":3600},
				{"id":"2","timeSpent":"2h","timeSpentSeconds":7200}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":100,"total":3,"worklogs":[
				{"id":"3","timeSpent":"30m","timeSpentSeconds":1800}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	worklogs, err := c.Issues.ListWorklogs("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(worklogs) != 3 {
		t.Errorf("expected 3 worklogs, got %d", len(worklogs))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
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

func TestParseTimeSpent_InvalidUnitParts(t *testing.T) {
	cases := []string{"abcw", "abcd", "1dabc h", "1habc m"}
	for _, input := range cases {
		if _, err := parseTimeSpent(input); err == nil {
			t.Errorf("expected error for %q", input)
		}
	}
}

// ─── Issue.Get (error paths) ──────────────────────────────────────────────────

func TestIssueGet_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Get("PROJ-1", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing issue") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Issue.List (full coverage) ───────────────────────────────────────────────

func TestIssueList_AllFiltersAndDefaultLimit(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"issues":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.List(IssueListOptions{
		Project:  "PROJ",
		Assignee: "alice",
		Label:    "backend",
		Priority: "High",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jql, _ := receivedBody["jql"].(string)
	if !strings.Contains(jql, `assignee = "alice"`) {
		t.Errorf("JQL missing assignee: %s", jql)
	}
	if !strings.Contains(jql, `labels = "backend"`) {
		t.Errorf("JQL missing label: %s", jql)
	}
	if !strings.Contains(jql, `priority = "High"`) {
		t.Errorf("JQL missing priority: %s", jql)
	}
	maxResults, _ := receivedBody["maxResults"].(float64)
	if maxResults != 50 {
		t.Errorf("maxResults = %v, want 50 (default limit)", maxResults)
	}
}

func TestIssueList_SearchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.List(IssueListOptions{Project: "PROJ"})
	if err == nil {
		t.Fatal("expected search error")
	}
}

// ─── Issue.Create (error paths) ───────────────────────────────────────────────

func TestIssueCreate_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Create(CreateIssueRequest{
		Fields: CreateIssueFields{
			Project:   ProjectRef{Key: "P"},
			Summary:   "x",
			IssueType: IssueTypeRef{Name: "Task"},
		},
	})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueCreate_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{bad json`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Create(CreateIssueRequest{
		Fields: CreateIssueFields{
			Project:   ProjectRef{Key: "P"},
			Summary:   "x",
			IssueType: IssueTypeRef{Name: "Task"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "parsing create response") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestIssueCreate_MissingKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"100"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Create(CreateIssueRequest{
		Fields: CreateIssueFields{
			Project:   ProjectRef{Key: "P"},
			Summary:   "x",
			IssueType: IssueTypeRef{Name: "Task"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "missing issue key") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}

// ─── Issue.EditRaw ────────────────────────────────────────────────────────────

func TestIssueEditRaw_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.EditRaw("PROJ-1", map[string]any{
		"fields": map[string]any{"customfield_10001": "value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields, _ := receivedBody["fields"].(map[string]any)
	if fields["customfield_10001"] != "value" {
		t.Errorf("unexpected body: %v", receivedBody)
	}
}

func TestIssueEditRaw_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.EditRaw("PROJ-1", map[string]any{"fields": map[string]any{}})
	if err == nil {
		t.Fatal("expected API error")
	}
}

// ─── Issue.GetTransitions (error paths) ───────────────────────────────────────

func TestIssueGetTransitions_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.GetTransitions("PROJ-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIssueGetTransitions_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.GetTransitions("PROJ-1")
	if err == nil || !strings.Contains(err.Error(), "parsing transitions") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue.DoTransition ───────────────────────────────────────────────────────

func TestIssueDoTransition_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.DoTransition("PROJ-1", "21")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	transition, _ := receivedBody["transition"].(map[string]any)
	if transition["id"] != "21" {
		t.Errorf("transition id = %v, want 21", transition["id"])
	}
}

func TestIssueDoTransition_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.DoTransition("PROJ-1", "21")
	if err == nil {
		t.Fatal("expected API error")
	}
}

// ─── Issue.Assign ─────────────────────────────────────────────────────────────

func TestIssueAssign_WithUsername(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.Assign("PROJ-1", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["name"] != "alice" {
		t.Errorf("name = %v, want alice", receivedBody["name"])
	}
}

func TestIssueAssign_ClearAssignee(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.Assign("PROJ-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["name"] != nil {
		t.Errorf("name = %v, want nil", receivedBody["name"])
	}
}

func TestIssueAssign_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.Assign("PROJ-1", "alice")
	if err == nil {
		t.Fatal("expected API error")
	}
}

// ─── Issue watchers ───────────────────────────────────────────────────────────

func TestIssueAddWatcher_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.AddWatcher("PROJ-1", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIssueAddWatcher_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.AddWatcher("PROJ-1", "bob")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueRemoveWatcher_Success(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.RemoveWatcher("PROJ-1", "bob@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedQuery, "username=") {
		t.Errorf("expected username query param, got %q", receivedQuery)
	}
}

func TestIssueRemoveWatcher_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.RemoveWatcher("PROJ-1", "bob")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueGetWatchers_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"watchers":[{"name":"alice","displayName":"Alice"}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	watchers, err := c.Issues.GetWatchers("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(watchers) != 1 || watchers[0].Name != "alice" {
		t.Errorf("unexpected watchers: %+v", watchers)
	}
}

func TestIssueGetWatchers_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.GetWatchers("PROJ-1")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueGetWatchers_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.GetWatchers("PROJ-1")
	if err == nil || !strings.Contains(err.Error(), "parsing watchers") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue votes ──────────────────────────────────────────────────────────────

func TestIssueAddVote_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.AddVote("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIssueAddVote_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.AddVote("PROJ-1")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueRemoveVote_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.RemoveVote("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIssueRemoveVote_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.RemoveVote("PROJ-1")
	if err == nil {
		t.Fatal("expected API error")
	}
}

// ─── Issue.AddComment (error paths) ─────────────────────────────────────────────

func TestIssueAddComment_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.AddComment("PROJ-1", "comment")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueAddComment_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.AddComment("PROJ-1", "comment")
	if err == nil || !strings.Contains(err.Error(), "parsing comment") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue.ListComments ─────────────────────────────────────────────────────────

func TestIssueListComments_Success(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"comments":[{"id":"1","created":"2024-01-01T00:00:00.000+0000"}],"total":1}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	comments, err := c.Issues.ListComments("PROJ-1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(comments))
	}
	if !strings.Contains(receivedQuery, "maxResults=10") {
		t.Errorf("expected maxResults=10 in query, got %q", receivedQuery)
	}
}

func TestIssueListComments_DefaultLimit(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"comments":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListComments("PROJ-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedQuery, "maxResults=50") {
		t.Errorf("expected default maxResults=50, got %q", receivedQuery)
	}
}

func TestIssueListComments_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListComments("PROJ-1", 10)
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueListComments_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListComments("PROJ-1", 10)
	if err == nil || !strings.Contains(err.Error(), "parsing comments") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue.DeleteComment ──────────────────────────────────────────────────────

func TestIssueDeleteComment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/comment/10100") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.DeleteComment("PROJ-1", "10100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIssueDeleteComment_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.DeleteComment("PROJ-1", "10100")
	if err == nil {
		t.Fatal("expected API error")
	}
}

// ─── Issue.AddWorklog (full coverage) ─────────────────────────────────────────

func TestIssueAddWorklog_WithCommentAndStarted(t *testing.T) {
	var receivedBody WorklogRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"10200","timeSpent":"1h","timeSpentSeconds":3600}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	worklog, err := c.Issues.AddWorklog("PROJ-1", "1h", "2024-06-01T10:00:00.000+0000", "done work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worklog.ID != "10200" {
		t.Errorf("ID = %q, want 10200", worklog.ID)
	}
	if receivedBody.Started != "2024-06-01T10:00:00.000+0000" {
		t.Errorf("Started = %q", receivedBody.Started)
	}
	if receivedBody.Comment != "done work" {
		t.Errorf("Comment = %v, want done work", receivedBody.Comment)
	}
}

func TestIssueAddWorklog_PostError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.AddWorklog("PROJ-1", "1h", "", "")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueAddWorklog_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.AddWorklog("PROJ-1", "1h", "", "")
	if err == nil || !strings.Contains(err.Error(), "parsing worklog") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue.ListWorklogs (error paths) ───────────────────────────────────────────

func TestIssueListWorklogs_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListWorklogs("PROJ-1")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueListWorklogs_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListWorklogs("PROJ-1")
	if err == nil || !strings.Contains(err.Error(), "parsing worklogs") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue links ──────────────────────────────────────────────────────────────

func TestIssueCreateLink_Success(t *testing.T) {
	var receivedBody IssueLinkRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.CreateLink(IssueLinkRequest{
		Type:         LinkTypeRef{Name: "Blocks"},
		InwardIssue:  KeyRef{Key: "PROJ-1"},
		OutwardIssue: KeyRef{Key: "PROJ-2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody.Type.Name != "Blocks" {
		t.Errorf("link type = %q", receivedBody.Type.Name)
	}
}

func TestIssueCreateLink_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.CreateLink(IssueLinkRequest{})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueDeleteLink_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/issueLink/10001") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.DeleteLink("10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIssueDeleteLink_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Issues.DeleteLink("10001")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueGetLinkTypes_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	types, err := c.Issues.GetLinkTypes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0].Name != "Blocks" {
		t.Errorf("unexpected link types: %+v", types)
	}
}

func TestIssueGetLinkTypes_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.GetLinkTypes()
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueGetLinkTypes_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.GetLinkTypes()
	if err == nil || !strings.Contains(err.Error(), "parsing link types") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue remote links ─────────────────────────────────────────────────────────

func TestIssueAddRemoteLink_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":1,"object":{"url":"https://example.com","title":"Doc"}}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	link, err := c.Issues.AddRemoteLink("PROJ-1", RemoteLinkRequest{
		Object: RemoteLinkObject{URL: "https://example.com", Title: "Doc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.ID != 1 || link.Object.Title != "Doc" {
		t.Errorf("unexpected link: %+v", link)
	}
}

func TestIssueAddRemoteLink_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.AddRemoteLink("PROJ-1", RemoteLinkRequest{})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueAddRemoteLink_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.AddRemoteLink("PROJ-1", RemoteLinkRequest{})
	if err == nil || !strings.Contains(err.Error(), "parsing remote link") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestIssueListRemoteLinks_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[{"id":1,"object":{"url":"https://example.com","title":"Doc"}}]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	links, err := c.Issues.ListRemoteLinks("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 1 || links[0].Object.URL != "https://example.com" {
		t.Errorf("unexpected links: %+v", links)
	}
}

func TestIssueListRemoteLinks_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListRemoteLinks("PROJ-1")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueListRemoteLinks_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListRemoteLinks("PROJ-1")
	if err == nil || !strings.Contains(err.Error(), "parsing remote links") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue attachments ──────────────────────────────────────────────────────────

func TestIssueUploadAttachment_Success(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(filePath, []byte("attachment content"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			t.Errorf("missing X-Atlassian-Token header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[{"id":"10001","filename":"note.txt","size":18}]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	attachments, err := c.Issues.UploadAttachment(context.Background(), "PROJ-1", filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "note.txt" {
		t.Errorf("unexpected attachments: %+v", attachments)
	}
}

func TestIssueUploadAttachment_FileNotFound(t *testing.T) {
	c := newTestClient("http://unused")
	_, err := c.Issues.UploadAttachment(context.Background(), "PROJ-1", filepath.Join(t.TempDir(), "missing.txt"))
	if err == nil {
		t.Fatal("expected file open error")
	}
}

func TestIssueUploadAttachment_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.UploadAttachment(context.Background(), "PROJ-1", filePath)
	if err == nil || !strings.Contains(err.Error(), "parsing attachments") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestIssueListAttachments_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"id":"10001",
			"key":"PROJ-1",
			"fields":{
				"summary":"Test",
				"status":{"name":"Open"},
				"issuetype":{"name":"Task"},
				"attachment":[{"id":"20001","filename":"a.png","size":1024}]
			}
		}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	attachments, err := c.Issues.ListAttachments("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "a.png" {
		t.Errorf("unexpected attachments: %+v", attachments)
	}
}

func TestIssueListAttachments_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.ListAttachments("PROJ-1")
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueDownloadAttachment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bytes"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	dir := t.TempDir()
	att := Attachment{ID: "20001", Filename: "a.png", Content: ts.URL + "/secure/attachment/20001/a.png"}
	path, err := c.Issues.DownloadAttachment(context.Background(), att, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != filepath.Join(dir, "a.png") {
		t.Errorf("path = %q", path)
	}
	if b, _ := os.ReadFile(path); string(b) != "bytes" {
		t.Errorf("content = %q", b)
	}
}

func TestIssueDownloadAttachment_NoContentURL(t *testing.T) {
	c := newTestClient("https://jira.example.com")
	_, err := c.Issues.DownloadAttachment(context.Background(), Attachment{ID: "1", Filename: "a.png"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no content URL") {
		t.Fatalf("expected no content URL error, got %v", err)
	}
}

func TestIssueDownloadAttachment_PathTraversalSanitised(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	dir := t.TempDir()
	att := Attachment{ID: "1", Filename: "../../evil.txt", Content: ts.URL + "/x"}
	path, err := c.Issues.DownloadAttachment(context.Background(), att, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != filepath.Join(dir, "evil.txt") {
		t.Errorf("filename not sanitised: %q", path)
	}
}

// ─── Issue.Search (full coverage) ─────────────────────────────────────────────

func TestIssueSearch_WithFieldsAndDefaultMax(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"issues":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Search("project = P", SearchOptions{Fields: []string{"summary", "status"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields, ok := receivedBody["fields"].([]any)
	if !ok || len(fields) != 2 {
		t.Errorf("expected fields in body, got %v", receivedBody["fields"])
	}
	maxResults, _ := receivedBody["maxResults"].(float64)
	if maxResults != 50 {
		t.Errorf("maxResults = %v, want 50", maxResults)
	}
}

func TestIssueSearch_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Search("bad jql", SearchOptions{})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueSearch_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{bad`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Issues.Search("project = P", SearchOptions{MaxResults: 10})
	if err == nil || !strings.Contains(err.Error(), "parsing search result") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// ─── Issue.SearchAll (full coverage) ──────────────────────────────────────────

func TestIssueSearchAll_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, _, err := c.Issues.SearchAll("project = P", []string{"summary"})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestIssueSearchAll_WithFields(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"issues":[
			{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Open"},"issuetype":{"name":"Task"}}}
		]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, _, err := c.Issues.SearchAll("project = P", []string{"summary"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
	fields, ok := receivedBody["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Errorf("expected fields in search body, got %v", receivedBody["fields"])
	}
}

func TestIssueSearchAll_MaxTotalCap(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		issues := make([]string, 10000)
		for i := range issues {
			issues[i] = fmt.Sprintf(
				`{"id":"%d","key":"P-%d","fields":{"summary":"s","status":{"name":"Open"},"issuetype":{"name":"Task"}}}`,
				i, i,
			)
		}
		_, _ = fmt.Fprintf(w, `{"startAt":0,"maxResults":100,"total":50000,"issues":[%s]}`, strings.Join(issues, ","))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, _, err := c.Issues.SearchAll("project = P", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 10000 {
		t.Errorf("expected 10000 issues (cap), got %d", len(issues))
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call before cap, got %d", callCount)
	}
}
