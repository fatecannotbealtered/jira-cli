package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProjectList_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[
			{"id":"1","key":"PROJ","name":"My Project","projectTypeKey":"software"},
			{"id":"2","key":"OPS","name":"Operations","projectTypeKey":"business"}
		]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	projects, err := c.Projects.List("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestProjectGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"id":"1","key":"PROJ","name":"My Project","projectTypeKey":"software","lead":{"displayName":"John"}}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	project, err := c.Projects.Get("PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Key != "PROJ" {
		t.Errorf("Key = %q, want PROJ", project.Key)
	}
	if project.Lead == nil || project.Lead.DisplayName != "John" {
		t.Errorf("Lead = %v, want John", project.Lead)
	}
}

func TestProjectGetComponents_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[{"id":"1","name":"Backend"},{"id":"2","name":"Frontend"}]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	components, err := c.Projects.GetComponents("PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
}

func TestProjectGetVersions_FilterReleased(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[
			{"id":"1","name":"v1.0","released":true},
			{"id":"2","name":"v2.0","released":false}
		]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	// released only
	versions, err := c.Projects.GetVersions("PROJ", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0].Name != "v1.0" {
		t.Errorf("released filter: got %v", versions)
	}

	// unreleased only
	versions, err = c.Projects.GetVersions("PROJ", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0].Name != "v2.0" {
		t.Errorf("unreleased filter: got %v", versions)
	}

	// no filter
	versions, err = c.Projects.GetVersions("PROJ", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("no filter: expected 2, got %d", len(versions))
	}
}
