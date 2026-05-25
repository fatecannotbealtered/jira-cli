package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilterListMy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[
			{"id":"100","name":"My Bugs","jql":"type = Bug","favourite":true},
			{"id":"101","name":"Sprint Issues","jql":"sprint in openSprints()","favourite":false}
		]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	filters, err := c.Filters.ListMy()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(filters))
	}
	if filters[0].Name != "My Bugs" {
		t.Errorf("first filter name = %q", filters[0].Name)
	}
	if !filters[0].Favourite {
		t.Error("first filter should be favourite")
	}
}

func TestFilterGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":"100","name":"My Bugs","jql":"type = Bug","description":"All bugs","favourite":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	filter, err := c.Filters.Get("100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.ID != "100" {
		t.Errorf("ID = %q, want 100", filter.ID)
	}
	if filter.JQL != "type = Bug" {
		t.Errorf("JQL = %q", filter.JQL)
	}
}

func TestFilterCreate_Success(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":"200","name":"New Filter","jql":"project = PROJ"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	filter, err := c.Filters.Create("New Filter", "project = PROJ", "test desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.ID != "200" {
		t.Errorf("ID = %q, want 200", filter.ID)
	}
	if receivedBody["name"] != "New Filter" {
		t.Errorf("sent name = %v", receivedBody["name"])
	}
	if receivedBody["description"] != "test desc" {
		t.Errorf("sent description = %v", receivedBody["description"])
	}
}

func TestFilterDelete_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Filters.Delete("100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFilterListMy_GetError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Filters.ListMy()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterListMy_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Filters.ListMy()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing filters") {
		t.Errorf("error = %v", err)
	}
}

func TestFilterGet_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Filters.Get("999")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterGet_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Filters.Get("100")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing filter") {
		t.Errorf("error = %v", err)
	}
}

func TestFilterCreate_PostError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Filters.Create("X", "project = X", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterCreate_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Filters.Create("New", "project = PROJ", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing created filter") {
		t.Errorf("error = %v", err)
	}
}
