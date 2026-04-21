package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var sampleFieldsJSON = `[
	{"id":"summary","key":"summary","name":"Summary","custom":false,"navigable":true,"searchable":true,"schema":{"type":"string","system":"summary"}},
	{"id":"status","key":"status","name":"Status","custom":false,"navigable":true,"searchable":true,"schema":{"type":"status","system":"status"}},
	{"id":"customfield_10001","key":"customfield_10001","name":"Story Points","custom":true,"navigable":true,"searchable":true,"schema":{"type":"number","custom":"com.atlassian.jira.plugin.system.customfieldtypes:float","customId":10001}},
	{"id":"customfield_10002","key":"customfield_10002","name":"Team","custom":true,"navigable":true,"searchable":true,"schema":{"type":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:select","customId":10002}},
	{"id":"customfield_10003","key":"customfield_10003","name":"Sprint Team","custom":true,"navigable":true,"searchable":true,"schema":{"type":"string","customId":10003}}
]`

func newFieldTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, sampleFieldsJSON)
	}))
}

func TestFieldListAll_Success(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	fields, err := c.Fields.ListAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 5 {
		t.Errorf("expected 5 fields, got %d", len(fields))
	}
}

func TestFieldListCustom_Success(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	fields, err := c.Fields.ListCustom()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 3 {
		t.Errorf("expected 3 custom fields, got %d", len(fields))
	}
	for _, f := range fields {
		if !f.Custom {
			t.Errorf("field %q should be custom", f.Name)
		}
	}
}

func TestFieldResolveFieldID_ExactMatch(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	id, err := c.Fields.ResolveFieldID("Story Points")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "customfield_10001" {
		t.Errorf("ID = %q, want customfield_10001", id)
	}
}

func TestFieldResolveFieldID_CaseInsensitive(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	id, err := c.Fields.ResolveFieldID("story points")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "customfield_10001" {
		t.Errorf("ID = %q, want customfield_10001", id)
	}
}

func TestFieldResolveFieldID_DirectID(t *testing.T) {
	c := newTestClient("http://unused")
	id, err := c.Fields.ResolveFieldID("customfield_10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "customfield_10001" {
		t.Errorf("ID = %q, want customfield_10001", id)
	}
}

func TestFieldResolveFieldID_NotFound(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Fields.ResolveFieldID("Nonexistent Field")
	if err == nil {
		t.Fatal("expected error for nonexistent field")
	}
}

func TestFieldResolveFieldID_Ambiguous(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	// "Team" matches both "Team" (exact) and "Sprint Team" (partial)
	// But exact match should win
	id, err := c.Fields.ResolveFieldID("Team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "customfield_10002" {
		t.Errorf("ID = %q, want customfield_10002", id)
	}
}

func TestFieldSetCustomFields_SimpleValue(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	body := map[string]any{
		"fields": map[string]any{},
	}
	err := c.Fields.SetCustomFields(body, []string{"Story Points=5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := body["fields"].(map[string]any)
	val, ok := fields["customfield_10001"]
	if !ok {
		t.Fatal("customfield_10001 not set")
	}
	// "5" is valid JSON number, should be parsed as float64
	if val != float64(5) {
		t.Errorf("value = %v (%T), want 5", val, val)
	}
}

func TestFieldSetCustomFields_JSONValue(t *testing.T) {
	ts := newFieldTestServer()
	defer ts.Close()

	c := newTestClient(ts.URL)
	body := map[string]any{
		"fields": map[string]any{},
	}
	err := c.Fields.SetCustomFields(body, []string{`Team={"value":"Backend"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := body["fields"].(map[string]any)
	val, ok := fields["customfield_10002"]
	if !ok {
		t.Fatal("customfield_10002 not set")
	}
	m, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if m["value"] != "Backend" {
		t.Errorf("value = %v", m["value"])
	}
}

func TestFieldSetCustomFields_InvalidFormat(t *testing.T) {
	c := newTestClient("http://unused")
	body := map[string]any{
		"fields": map[string]any{},
	}
	err := c.Fields.SetCustomFields(body, []string{"no-equals-sign"})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
