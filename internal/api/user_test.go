package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserSearch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "username=john") {
			t.Errorf("expected username=john in URL, got %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[{"name":"johndoe","displayName":"John Doe","emailAddress":"john@example.com","active":true}]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	users, err := c.Users.Search("john")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
	if users[0].DisplayName != "John Doe" {
		t.Errorf("DisplayName = %q", users[0].DisplayName)
	}
}

func TestUserSearchAssignable_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "project=PROJ") {
			t.Errorf("expected project=PROJ in URL, got %s", r.URL.String())
		}
		if !strings.Contains(r.URL.String(), "username=jane") {
			t.Errorf("expected username=jane in URL, got %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[{"name":"janedoe","displayName":"Jane","active":true}]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	users, err := c.Users.SearchAssignable("jane", "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestUserSearchAssignable_GetError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Users.SearchAssignable("jane", "PROJ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserSearchAssignable_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Users.SearchAssignable("jane", "PROJ")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing assignable users") {
		t.Errorf("error = %v", err)
	}
}

func TestUserMe_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/myself") {
			t.Errorf("expected /myself path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"name":"currentuser","displayName":"Current User","emailAddress":"me@example.com","active":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	myself, err := c.Users.Me()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if myself.Name != "currentuser" {
		t.Errorf("Name = %q, want currentuser", myself.Name)
	}
	if myself.DisplayName != "Current User" {
		t.Errorf("DisplayName = %q", myself.DisplayName)
	}
}

func TestUserSearch_GetError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Users.Search("john")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserSearch_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Users.Search("john")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing users") {
		t.Errorf("error = %v", err)
	}
}

func TestUserMe_GetError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Users.Me()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserMe_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{invalid`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Users.Me()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parsing myself") {
		t.Errorf("error = %v", err)
	}
}
