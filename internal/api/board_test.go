package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBoardList_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":2,"values":[
			{"id":1,"name":"Scrum Board","type":"scrum","location":{"projectKey":"PROJ"}},
			{"id":2,"name":"Kanban Board","type":"kanban","location":{"projectKey":"PROJ"}}
		]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	boards, err := c.Boards.List("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 2 {
		t.Errorf("expected 2 boards, got %d", len(boards))
	}
	if boards[0].Type != "scrum" {
		t.Errorf("first board type = %q", boards[0].Type)
	}
}

func TestBoardList_WithFilters(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"values":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, _ = c.Boards.List("PROJ", "scrum")
	if !strings.Contains(receivedPath, "projectKeyOrId=PROJ") {
		t.Errorf("expected project filter in path, got %q", receivedPath)
	}
	if !strings.Contains(receivedPath, "type=scrum") {
		t.Errorf("expected type filter in path, got %q", receivedPath)
	}
}

func TestBoardGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":42,"name":"My Board","type":"scrum","location":{"projectKey":"PROJ"}}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	board, err := c.Boards.Get(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != 42 {
		t.Errorf("ID = %d, want 42", board.ID)
	}
	if board.Name != "My Board" {
		t.Errorf("Name = %q, want 'My Board'", board.Name)
	}
}

func TestBoardGetBacklog_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"issues":[{"id":"1","key":"P-1","fields":{"summary":"backlog item","status":{"name":"To Do"},"issuetype":{"name":"Story"}}}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, err := c.Boards.GetBacklog(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

func TestBoardGetEpics_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"values":[{"id":"1","key":"P-10","fields":{"summary":"Epic 1","status":{"name":"In Progress"},"issuetype":{"name":"Epic"}}}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	epics, err := c.Boards.GetEpics(42, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(epics))
	}
}

func TestBoardGetSprints_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":1,"values":[{"id":10,"name":"Sprint 1","state":"active"}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	sprints, err := c.Boards.GetSprints(42, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) != 1 {
		t.Errorf("expected 1 sprint, got %d", len(sprints))
	}
}
