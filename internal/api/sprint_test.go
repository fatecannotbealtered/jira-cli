package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSprintList_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		callCount++
		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":3,"isLast":false,"values":[
				{"id":1,"name":"Sprint 1","state":"active"},
				{"id":2,"name":"Sprint 2","state":"future"}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":100,"total":3,"isLast":true,"values":[
				{"id":3,"name":"Sprint 3","state":"closed"}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	sprints, err := c.Sprints.List(42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) != 3 {
		t.Errorf("expected 3 sprints, got %d", len(sprints))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestSprintGetIssues_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		callCount++
		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":3,"isLast":false,"issues":[
				{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Open"},"issuetype":{"name":"Task"}}},
				{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"Open"},"issuetype":{"name":"Task"}}}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":100,"total":3,"isLast":true,"issues":[
				{"id":"3","key":"P-3","fields":{"summary":"c","status":{"name":"Open"},"issuetype":{"name":"Task"}}}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, err := c.Sprints.GetIssues(10)
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

func TestSprintGet_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Sprints.Get(99)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSprintGet_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Sprints.Get(10)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing sprint") {
		t.Errorf("error = %v", err)
	}
}

func TestSprintCreate_PostError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Sprints.Create(CreateSprintRequest{Name: "X", OriginBoardID: 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSprintCreate_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Sprints.Create(CreateSprintRequest{Name: "X", OriginBoardID: 1})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing created sprint") {
		t.Errorf("error = %v", err)
	}
}

func TestSprintUpdate_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":10,"name":"Updated Sprint","state":"active","goal":"New goal"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	sprint, err := c.Sprints.Update(10, UpdateSprintRequest{Name: "Updated Sprint", Goal: "New goal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sprint.Name != "Updated Sprint" {
		t.Errorf("Name = %q", sprint.Name)
	}
	if receivedBody["goal"] != "New goal" {
		t.Errorf("sent goal = %v", receivedBody["goal"])
	}
}

func TestSprintUpdate_PutError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Sprints.Update(10, UpdateSprintRequest{Name: "X"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSprintUpdate_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Sprints.Update(10, UpdateSprintRequest{Name: "X"})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing updated sprint") {
		t.Errorf("error = %v", err)
	}
}

func TestSprintClose_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Sprints.Close(10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSprintMoveIssues_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Sprints.MoveIssues(10, []string{"PROJ-1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ─── Sprint.GetReport ──────────────────────────────────────────────────────────

func TestSprintGetReport_Greenhopper(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"contents":{
			"completedIssues":[{"key":"P-1"},{"key":"P-2"}],
			"issuesNotCompletedInCurrentSprint":[{"key":"P-3"}],
			"puntedIssues":[{"key":"P-4"}],
			"completedIssuesEstimateSum":{"value":8,"text":"8.0"},
			"issuesNotCompletedEstimateSum":{"value":3,"text":"3.0"}
		},"sprint":{"id":10,"name":"Sprint 10","state":"closed"}}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	report, err := c.Sprints.GetReport(10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(receivedPath, "sprintreport") || !contains(receivedPath, "rapidViewId=1") || !contains(receivedPath, "sprintId=10") {
		t.Errorf("unexpected chart path %q", receivedPath)
	}
	if report.Source != "greenhopper" {
		t.Errorf("source = %q, want greenhopper", report.Source)
	}
	if report.CompletedIssues != 2 || report.CommittedIssues != 3 || report.PunctedIssues != 1 {
		t.Errorf("counts = %+v", report)
	}
	if report.CompletedPoints != 8 || report.CommittedPoints != 11 {
		t.Errorf("points = %+v", report)
	}
}

func TestSprintGetReport_FallbackWhenChartMissing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case contains(r.URL.Path, "sprintreport"):
			w.WriteHeader(http.StatusNotFound)
		case contains(r.URL.Path, "/sprint/10/issue"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"issues":[
				{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Done","statusCategory":{"key":"done"}},"issuetype":{"name":"Task"}}},
				{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"Open","statusCategory":{"key":"new"}},"issuetype":{"name":"Task"}}}
			]}`)
		case contains(r.URL.Path, "/sprint/10"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"id":10,"name":"Sprint 10","state":"active"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	report, err := c.Sprints.GetReport(10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Source != "computed" {
		t.Errorf("source = %q, want computed", report.Source)
	}
	if report.CommittedIssues != 2 || report.CompletedIssues != 1 || report.NotCompletedIssues != 1 {
		t.Errorf("counts = %+v", report)
	}
}

func TestSprintGetReport_ComputedWhenNoBoard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contains(r.URL.Path, "sprintreport") {
			t.Errorf("chart endpoint should not be called without board id")
		}
		switch {
		case contains(r.URL.Path, "/sprint/10/issue"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"issues":[{"id":"1","key":"P-1","fields":{"status":{"statusCategory":{"key":"done"}},"issuetype":{"name":"Task"}}}]}`)
		case contains(r.URL.Path, "/sprint/10"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"id":10,"name":"Sprint 10","state":"closed"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	report, err := c.Sprints.GetReport(10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Source != "computed" || report.CompletedIssues != 1 {
		t.Errorf("report = %+v", report)
	}
}

func TestSprintGetReport_FallbackError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	if _, err := c.Sprints.GetReport(10, 0); err == nil {
		t.Fatal("expected error from fallback")
	}
}
