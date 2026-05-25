package cmd

import (
	"encoding/json"
	"strings"
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
		},
	}
	fi := toFlatIssue(issue)
	if fi.Reporter != "reporter1" {
		t.Errorf("Reporter = %q, want reporter1", fi.Reporter)
	}
	if fi.Component != "Backend" {
		t.Errorf("Component = %q, want Backend", fi.Component)
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
	if flat[0].ID != 1 || flat[1].Name != "S2" {
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &flat); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 sprint, got %d", len(result))
	}
	if result[0]["id"].(float64) != 12 {
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
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
