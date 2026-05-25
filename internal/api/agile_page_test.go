package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBuildAgilePagePath_ValidURL(t *testing.T) {
	params := url.Values{"state": {"active"}}
	got := buildAgilePagePath("http://jira.example.com/rest/agile/1.0/board", params, 50, 100)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if u.Query().Get("startAt") != "50" {
		t.Errorf("startAt = %q, want 50", u.Query().Get("startAt"))
	}
	if u.Query().Get("maxResults") != "100" {
		t.Errorf("maxResults = %q, want 100", u.Query().Get("maxResults"))
	}
	if u.Query().Get("state") != "active" {
		t.Errorf("state = %q, want active", u.Query().Get("state"))
	}
}

func TestBuildAgilePagePath_ParseError(t *testing.T) {
	params := url.Values{"state": {"active"}}

	got := buildAgilePagePath(":", params, 0, 100)
	if !strings.Contains(got, "startAt=0") {
		t.Errorf("expected startAt in %q", got)
	}
	if !strings.Contains(got, "maxResults=100") {
		t.Errorf("expected maxResults in %q", got)
	}
	if !strings.HasPrefix(got, ":") {
		t.Errorf("expected base path preserved, got %q", got)
	}
	if !strings.Contains(got, "?") {
		t.Errorf("expected ? query separator, got %q", got)
	}

	gotWithQuery := buildAgilePagePath(":?existing=1", params, 10, 25)
	if !strings.Contains(gotWithQuery, "&startAt=10") {
		t.Errorf("expected & separator when base already has query, got %q", gotWithQuery)
	}
	if !strings.Contains(gotWithQuery, "existing=1") {
		t.Errorf("expected existing query preserved, got %q", gotWithQuery)
	}
}

func TestParseAgilePageMeta_Success(t *testing.T) {
	meta, err := parseAgilePageMeta([]byte(`{"startAt":0,"maxResults":100,"total":1,"isLast":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Total != 1 || !meta.IsLast {
		t.Errorf("meta = %+v", meta)
	}
}

func TestParseAgilePageMeta_InvalidJSON(t *testing.T) {
	_, err := parseAgilePageMeta([]byte(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing agile page metadata") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetAgilePageAll_GetError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.getAgilePageAll("/rest/agile/1.0/board", nil, func([]byte) (int, error) {
		return 0, nil
	})
	if err == nil {
		t.Fatal("expected error from Get")
	}
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != 500 {
		t.Fatalf("expected *APIError 500, got %T %v", err, err)
	}
}

func TestGetAgilePageAll_ParseMetaError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.getAgilePageAll("/rest/agile/1.0/board", nil, func([]byte) (int, error) {
		return 0, nil
	})
	if err == nil {
		t.Fatal("expected parse meta error")
	}
	if !strings.Contains(err.Error(), "parsing agile page metadata") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetAgilePageAll_CollectError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"values":[]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	collectErr := fmt.Errorf("collect failed")
	err := c.getAgilePageAll("/rest/agile/1.0/board", nil, func([]byte) (int, error) {
		return 0, collectErr
	})
	if err != collectErr {
		t.Fatalf("expected collect error, got %v", err)
	}
}

func TestGetAgilePageAll_MultiPage(t *testing.T) {
	var page int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&page, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":150,"isLast":false,"values":[1,2]}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":100,"total":150,"isLast":true,"values":[3]}`)
	}))
	defer ts.Close()

	var collected int
	c := newTestClient(ts.URL)
	err := c.getAgilePageAll("/rest/agile/1.0/board", nil, func(data []byte) (int, error) {
		collected++
		return 2, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if collected != 2 {
		t.Errorf("expected 2 pages collected, got %d", collected)
	}
}

func TestFetchAllSprintPages_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"values":"not-an-array"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.fetchAllSprintPages("/rest/agile/1.0/board/1/sprint", nil)
	if err == nil || !strings.Contains(err.Error(), "parsing sprints") {
		t.Fatalf("expected parsing error, got %v", err)
	}
}

func TestFetchAllBoardPages_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"values":"not-an-array"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.fetchAllBoardPages("/rest/agile/1.0/board", nil)
	if err == nil || !strings.Contains(err.Error(), "parsing boards") {
		t.Fatalf("expected parsing error, got %v", err)
	}
}

func TestFetchAllIssuePages_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"issues":"not-an-array"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.fetchAllIssuePages("/rest/agile/1.0/board/1/issue", nil)
	if err == nil || !strings.Contains(err.Error(), "parsing issues") {
		t.Fatalf("expected parsing error, got %v", err)
	}
}

func TestFetchAllEpicValuePages_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"isLast":true,"values":"not-an-array"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.fetchAllEpicValuePages("/rest/agile/1.0/board/1/epic", nil)
	if err == nil || !strings.Contains(err.Error(), "parsing epics") {
		t.Fatalf("expected parsing error, got %v", err)
	}
}
