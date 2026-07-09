package cmd

import (
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

func TestToFlatIssue_ReporterAndComponent(t *testing.T) {
	issue := &api.Issue{
		Key: "TEST-3",
		Fields: api.IssueFields{
			Summary:   "Full fields",
			Status:    api.Status{Name: "Open"},
			IssueType: api.IssueType{Name: "Story"},
			Reporter:  &api.User{Name: "reporter1"},
			Components: []api.Component{
				{Name: "Backend"},
				{Name: "Ignored"},
			},
			FixVersions: []api.Version{
				{Name: "v1.0"},
				{Name: "v1.1"},
			},
		},
	}
	fi := toFlatIssue(issue)
	if fi.Reporter != "reporter1" {
		t.Errorf("Reporter = %q, want reporter1", fi.Reporter)
	}
	if fi.Component != "Backend" {
		t.Errorf("Component = %q, want Backend", fi.Component)
	}
	if fi.FixVersions != "v1.0,v1.1" {
		t.Errorf("FixVersions = %q, want v1.0,v1.1", fi.FixVersions)
	}
}

func TestToFlatSprints(t *testing.T) {
	sprints := []api.Sprint{
		{ID: 1, Name: "S1", State: "active"},
		{ID: 2, Name: "S2", State: "closed"},
	}
	flat := toFlatSprints(sprints)
	if len(flat) != 2 {
		t.Fatalf("len = %d, want 2", len(flat))
	}
	if flat[0].ID != "1" || flat[1].Name != "S2" {
		t.Errorf("unexpected flat sprints: %+v", flat)
	}
}

func TestPrintSprintsJSON_Flat(t *testing.T) {
	sprints := []api.Sprint{
		{ID: 10, Name: "Sprint A", State: "future", Goal: "Goal A"},
	}
	out := captureStdout(t, func() {
		printSprintsJSON(sprints, false, nil)
	})
	var flat []output.FlatSprint
	decodeEnvelopeData(t, out, &flat)
	if len(flat) != 1 || flat[0].Name != "Sprint A" {
		t.Errorf("unexpected output: %+v", flat)
	}
}

func TestPrintSprintsJSON_Raw(t *testing.T) {
	sprints := []api.Sprint{{ID: 11, Name: "Raw Sprint", State: "active"}}
	out := captureStdout(t, func() {
		printSprintsJSON(sprints, true, nil)
	})
	var raw []api.Sprint
	decodeEnvelopeData(t, out, &raw)
	if len(raw) != 1 || raw[0].Name != "Raw Sprint" {
		t.Errorf("unexpected raw sprints: %+v", raw)
	}
}

func TestPrintSprintsJSON_Fields(t *testing.T) {
	sprints := []api.Sprint{
		{ID: 12, Name: "Filtered", State: "active", StartDate: "2024-03-01", EndDate: "2024-03-14"},
	}
	out := captureStdout(t, func() {
		printSprintsJSON(sprints, false, []string{"id", "name"})
	})
	var result []map[string]any
	decodeEnvelopeData(t, out, &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 sprint, got %d", len(result))
	}
	if result[0]["id"] != "12" {
		t.Errorf("id = %v", result[0]["id"])
	}
	if result[0]["name"] != "Filtered" {
		t.Errorf("name = %v", result[0]["name"])
	}
	if _, ok := result[0]["state"]; ok {
		t.Error("state should be filtered out")
	}
}

func TestPrintIssuesJSON_Raw(t *testing.T) {
	issues := []api.Issue{
		{Key: "R-1", Fields: api.IssueFields{Summary: "Raw", Status: api.Status{Name: "Open"}, IssueType: api.IssueType{Name: "Bug"}}},
	}
	out := captureStdout(t, func() {
		printIssuesJSON(issues, true, nil)
	})
	var raw []api.Issue
	decodeEnvelopeData(t, out, &raw)
	if len(raw) != 1 || raw[0].Key != "R-1" {
		t.Errorf("unexpected raw issues: %+v", raw)
	}
}

func TestPrintIssuesJSON_Fields(t *testing.T) {
	issues := []api.Issue{
		{
			Key: "F-1",
			Fields: api.IssueFields{
				Summary:   "Filtered list",
				Status:    api.Status{Name: "Done"},
				IssueType: api.IssueType{Name: "Task"},
				Assignee:  &api.User{Name: "bob"},
			},
		},
	}
	out := captureStdout(t, func() {
		printIssuesJSON(issues, false, []string{"key", "status"})
	})
	var result []map[string]any
	decodeEnvelopeData(t, out, &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if result[0]["key"] != "F-1" {
		t.Errorf("key = %v", result[0]["key"])
	}
	if result[0]["status"] != "Done" {
		t.Errorf("status = %v", result[0]["status"])
	}
	if _, ok := result[0]["summary"]; ok {
		t.Error("summary should be filtered out")
	}
}

func TestDescriptionText(t *testing.T) {
	adf := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": "ADF body"}},
			},
		},
	}
	cases := []struct {
		name string
		desc any
		want string
	}{
		{"string", "plain DC text", "plain DC text"},
		{"adf", adf, "ADF body"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := descriptionText(tc.desc); got != tc.want {
				t.Errorf("descriptionText = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPrintIssueJSON_Description verifies single-get exposes description in
// both default flat output and --fields, while bulk output stays lean.
func TestPrintIssueJSON_Description(t *testing.T) {
	issue := &api.Issue{Key: "DESC-1", Fields: api.IssueFields{Summary: "s", Description: "branch: feat-x"}}

	out := captureStdout(t, func() { printIssueJSON(issue, false, nil) })
	var m map[string]any
	decodeEnvelopeData(t, out, &m)
	if m["description"] != "branch: feat-x" {
		t.Errorf("get default description = %v, want branch: feat-x", m["description"])
	}

	out = captureStdout(t, func() { printIssueJSON(issue, false, []string{"key", "description"}) })
	m = nil
	decodeEnvelopeData(t, out, &m)
	if m["description"] != "branch: feat-x" {
		t.Errorf("get --fields description = %v, want branch: feat-x", m["description"])
	}

	// Bulk list stays lean: no description in default flat output.
	out = captureStdout(t, func() { printIssuesJSON([]api.Issue{*issue}, false, nil) })
	var arr []map[string]any
	decodeEnvelopeData(t, out, &arr)
	if _, ok := arr[0]["description"]; ok {
		t.Error("bulk flat output should not include description (token efficiency)")
	}
}
