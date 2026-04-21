package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Sprint.List ──────────────────────────────────────────────────────────────

func TestSprintList_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":2,"values":[
			{"id":1,"name":"Sprint 1","state":"active"},
			{"id":2,"name":"Sprint 2","state":"future"}
		]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	sprints, err := c.Sprints.List(42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) != 2 {
		t.Errorf("expected 2 sprints, got %d", len(sprints))
	}
	if sprints[0].Name != "Sprint 1" {
		t.Errorf("first sprint name = %q", sprints[0].Name)
	}
}

func TestSprintList_WithStateFilter(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"values":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, _ = c.Sprints.List(42, "active")
	if receivedPath == "" || !contains(receivedPath, "state=active") {
		t.Errorf("expected state=active in path, got %q", receivedPath)
	}
}

// ─── Sprint.Get ───────────────────────────────────────────────────────────────

func TestSprintGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":10,"name":"Sprint 10","state":"active","goal":"Ship v2"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	sprint, err := c.Sprints.Get(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sprint.Name != "Sprint 10" {
		t.Errorf("Name = %q, want 'Sprint 10'", sprint.Name)
	}
	if sprint.Goal != "Ship v2" {
		t.Errorf("Goal = %q, want 'Ship v2'", sprint.Goal)
	}
}

// ─── Sprint.Create ────────────────────────────────────────────────────────────

func TestSprintCreate_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":20,"name":"New Sprint","state":"future"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	sprint, err := c.Sprints.Create(CreateSprintRequest{
		Name:          "New Sprint",
		OriginBoardID: 42,
		Goal:          "Test goal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sprint.ID != 20 {
		t.Errorf("ID = %d, want 20", sprint.ID)
	}
	if receivedBody["name"] != "New Sprint" {
		t.Errorf("sent name = %v", receivedBody["name"])
	}
}

// ─── Sprint.Close ─────────────────────────────────────────────────────────────

func TestSprintClose_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":10,"state":"closed"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Sprints.Close(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["state"] != "closed" {
		t.Errorf("expected state=closed, got %v", receivedBody["state"])
	}
}

// ─── Sprint.MoveIssues ───────────────────────────────────────────────────────

func TestSprintMoveIssues_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Sprints.MoveIssues(10, []string{"PROJ-1", "PROJ-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	issues, _ := receivedBody["issues"].([]any)
	if len(issues) != 2 {
		t.Errorf("expected 2 issues in body, got %d", len(issues))
	}
}

// ─── Sprint.GetIssues ─────────────────────────────────────────────────────────

func TestSprintGetIssues_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"issues":[
			{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Open"},"issuetype":{"name":"Task"}}},
			{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"Done"},"issuetype":{"name":"Bug"}}}
		]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, err := c.Sprints.GetIssues(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
