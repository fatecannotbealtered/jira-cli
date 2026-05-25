package api

import (
	"encoding/json"
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

func TestBoardList_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		callCount++
		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":3,"isLast":false,"values":[
				{"id":1,"name":"Board 1","type":"scrum"},
				{"id":2,"name":"Board 2","type":"kanban"}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":100,"total":3,"isLast":true,"values":[
				{"id":3,"name":"Board 3","type":"scrum"}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	boards, err := c.Boards.List("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 3 {
		t.Errorf("expected 3 boards, got %d", len(boards))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestBoardGetBacklog_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		callCount++
		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":2,"isLast":false,"issues":[
				{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"To Do"},"issuetype":{"name":"Story"}}}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":1,"maxResults":100,"total":2,"isLast":true,"issues":[
				{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"To Do"},"issuetype":{"name":"Story"}}}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, err := c.Boards.GetBacklog(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestBoardGetEpics_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		callCount++
		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":2,"isLast":false,"values":[
				{"id":"1","key":"P-10","fields":{"summary":"Epic 1","status":{"name":"In Progress"},"issuetype":{"name":"Epic"}}}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":1,"maxResults":100,"total":2,"isLast":true,"values":[
				{"id":"2","key":"P-11","fields":{"summary":"Epic 2","status":{"name":"To Do"},"issuetype":{"name":"Epic"}}}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	epics, err := c.Boards.GetEpics(42, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(epics) != 2 {
		t.Errorf("expected 2 epics, got %d", len(epics))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestBoardGetEpicIssues_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		callCount++
		w.WriteHeader(http.StatusOK)
		if startAt == "0" || startAt == "" {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":2,"isLast":false,"issues":[
				{"id":"1","key":"P-1","fields":{"summary":"story 1","status":{"name":"Open"},"issuetype":{"name":"Story"}}}
			]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"startAt":1,"maxResults":100,"total":2,"isLast":true,"issues":[
				{"id":"2","key":"P-2","fields":{"summary":"story 2","status":{"name":"Open"},"issuetype":{"name":"Story"}}}
			]}`)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issues, err := c.Boards.GetEpicIssues("100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", ""); got != "" {
		t.Errorf("all empty = %q, want empty", got)
	}
	if got := firstNonEmpty("", "first", "second"); got != "first" {
		t.Errorf("skip empty = %q, want first", got)
	}
	if got := firstNonEmpty("only"); got != "only" {
		t.Errorf("single value = %q, want only", got)
	}
}

func TestParseEpicValues_IssueFormat(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"1","key":"P-1","fields":{"summary":"Direct Epic","issuetype":{"name":"Epic"}}}`),
	}
	epics, err := parseEpicValues(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(epics) != 1 || epics[0].Key != "P-1" {
		t.Fatalf("got %+v", epics)
	}
}

func TestParseEpicValues_AgileFormat(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":10,"key":"P-10","self":"http://jira/epic/10","summary":"From Summary","name":"From Name","done":false}`),
		json.RawMessage(`{"id":11,"key":"P-11","self":"http://jira/epic/11","name":"Done Epic","done":true}`),
		json.RawMessage(`{"id":12,"key":"P-12","self":"http://jira/epic/12","summary":"","name":"Name Fallback","done":false}`),
	}
	epics, err := parseEpicValues(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(epics) != 3 {
		t.Fatalf("expected 3 epics, got %d", len(epics))
	}
	if epics[0].Fields.Summary != "From Summary" {
		t.Errorf("summary[0] = %q, want From Summary", epics[0].Fields.Summary)
	}
	if epics[0].Fields.Status.Name != "To Do" || epics[0].Fields.Status.StatusCategory.Key != "new" {
		t.Errorf("open epic status = %+v", epics[0].Fields.Status)
	}
	if epics[1].Fields.Status.Name != "Done" || epics[1].Fields.Status.StatusCategory.Key != "done" {
		t.Errorf("done epic status = %+v", epics[1].Fields.Status)
	}
	if epics[2].Fields.Summary != "Name Fallback" {
		t.Errorf("summary[2] = %q, want Name Fallback", epics[2].Fields.Summary)
	}
}

func TestParseEpicValues_InvalidJSON(t *testing.T) {
	raw := []json.RawMessage{json.RawMessage(`not-valid-json`)}
	_, err := parseEpicValues(raw)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing epics") {
		t.Errorf("error = %v", err)
	}
}

func TestBoardGet_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Boards.Get(99)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != 404 {
		t.Fatalf("expected *APIError 404, got %T %v", err, err)
	}
}

func TestBoardGet_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Boards.Get(42)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing board") {
		t.Errorf("error = %v", err)
	}
}

func TestBoardGetEpics_AgileFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"values":[{"id":1,"key":"P-10","name":"Agile Epic","done":false}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	epics, err := c.Boards.GetEpics(42, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(epics) != 1 || epics[0].Fields.Summary != "Agile Epic" {
		t.Fatalf("got %+v", epics)
	}
}

func TestBoardGetEpics_DoneFilter(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"values":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Boards.GetEpics(42, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedQuery, "done=true") {
		t.Errorf("expected done=true in query, got %q", receivedQuery)
	}
}

func TestBoardGetEpics_FetchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Boards.GetEpics(42, false)
	if err == nil {
		t.Fatal("expected fetch error")
	}
}

func TestBoardGetEpics_InvalidEpicJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"values":["not-an-object"]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Boards.GetEpics(42, false)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
