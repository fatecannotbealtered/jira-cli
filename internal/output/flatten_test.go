package output

import (
	"reflect"
	"testing"
)

func TestFilterSprintFields_StartDateEndDate(t *testing.T) {
	s := FlatSprint{
		ID:        "1",
		Name:      "Sprint 1",
		State:     "active",
		StartDate: "2026-01-01",
		EndDate:   "2026-01-14",
		Goal:      "Ship feature",
	}

	result := FilterSprintFields(s, []string{"startDate", "endDate"})

	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d: %v", len(result), result)
	}
	if result["startDate"] != "2026-01-01" {
		t.Errorf("startDate = %v, want 2026-01-01", result["startDate"])
	}
	if result["endDate"] != "2026-01-14" {
		t.Errorf("endDate = %v, want 2026-01-14", result["endDate"])
	}
	if _, ok := result["name"]; ok {
		t.Error("name should not be in filtered result")
	}
}

func TestFilterSprintFields_CaseInsensitive(t *testing.T) {
	s := FlatSprint{
		ID:        "2",
		Name:      "Sprint 2",
		State:     "closed",
		StartDate: "2026-02-01",
		EndDate:   "2026-02-14",
		Goal:      "Bug fixes",
	}

	t.Run("camelCase fields", func(t *testing.T) {
		for _, fields := range [][]string{
			{"startdate", "enddate", "goal"},
			{"STARTDATE", "ENDDATE", "GOAL"},
			{"StartDate", "EndDate", "Goal"},
		} {
			result := FilterSprintFields(s, fields)
			if len(result) != 3 {
				t.Fatalf("fields %v: expected 3 fields, got %d: %v", fields, len(result), result)
			}
			if result["startDate"] != "2026-02-01" {
				t.Errorf("fields %v: startDate = %v, want 2026-02-01", fields, result["startDate"])
			}
			if result["endDate"] != "2026-02-14" {
				t.Errorf("fields %v: endDate = %v, want 2026-02-14", fields, result["endDate"])
			}
			if result["goal"] != "Bug fixes" {
				t.Errorf("fields %v: goal = %v, want Bug fixes", fields, result["goal"])
			}
		}
	})

	t.Run("simple fields", func(t *testing.T) {
		result := FilterSprintFields(s, []string{"ID", "Name", "STATE"})
		if len(result) != 3 {
			t.Fatalf("expected 3 fields, got %d: %v", len(result), result)
		}
		if result["id"] != "2" {
			t.Errorf("id = %v, want 2", result["id"])
		}
		if result["name"] != "Sprint 2" {
			t.Errorf("name = %v, want Sprint 2", result["name"])
		}
		if result["state"] != "closed" {
			t.Errorf("state = %v, want closed", result["state"])
		}
	})
}

func TestIssueToMap_AllFields(t *testing.T) {
	issue := FlatIssue{
		Key:         "PROJ-1",
		Summary:     "Summary",
		Status:      "Open",
		Type:        "Task",
		Assignee:    "alice",
		Reporter:    "bob",
		Priority:    "High",
		Created:     "2026-01-01",
		Updated:     "2026-01-02",
		Labels:      "bug",
		Component:   "backend",
		FixVersions: "v1.0,v1.1",
		Parent:      "PROJ-0",
	}

	got := IssueToMap(issue)
	want := map[string]any{
		"key":         "PROJ-1",
		"summary":     "Summary",
		"status":      "Open",
		"type":        "Task",
		"assignee":    "alice",
		"reporter":    "bob",
		"priority":    "High",
		"created":     "2026-01-01",
		"updated":     "2026-01-02",
		"labels":      "bug",
		"component":   "backend",
		"fixVersions": "v1.0,v1.1",
		"parent":      "PROJ-0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IssueToMap() = %v, want %v", got, want)
	}
}

func TestIssueToMap_RequiredOnly(t *testing.T) {
	issue := FlatIssue{Key: "K", Summary: "S", Status: "Open", Type: "Bug"}
	got := IssueToMap(issue)
	if len(got) != 4 {
		t.Fatalf("expected 4 fields, got %d: %v", len(got), got)
	}
}

func TestFilterFields(t *testing.T) {
	issue := FlatIssue{
		Key:      "PROJ-1",
		Summary:  "Test",
		Status:   "Open",
		Type:     "Task",
		Assignee: "alice",
	}

	t.Run("empty field list returns all", func(t *testing.T) {
		got := FilterFields(issue, nil)
		if len(got) != 5 {
			t.Fatalf("expected all 5 fields, got %d: %v", len(got), got)
		}
	})

	t.Run("filters requested fields case-insensitively", func(t *testing.T) {
		got := FilterFields(issue, []string{" KEY ", "Summary", "missing", "ASSIGNEE"})
		if len(got) != 3 {
			t.Fatalf("expected 3 fields, got %d: %v", len(got), got)
		}
		if got["key"] != "PROJ-1" || got["summary"] != "Test" || got["assignee"] != "alice" {
			t.Errorf("unexpected filtered map: %v", got)
		}
	})
}

func TestLookupSprintField(t *testing.T) {
	m := SprintToMap(FlatSprint{ID: "1", Name: "S1", State: "active", StartDate: "2026-01-01"})

	t.Run("empty name", func(t *testing.T) {
		key, val, ok := lookupSprintField(m, "  ")
		if ok || key != "" || val != nil {
			t.Errorf("empty name: key=%q val=%v ok=%v", key, val, ok)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		key, val, ok := lookupSprintField(m, "nope")
		if ok || key != "" || val != nil {
			t.Errorf("unknown field: key=%q val=%v ok=%v", key, val, ok)
		}
	})

	t.Run("found field", func(t *testing.T) {
		key, val, ok := lookupSprintField(m, "startdate")
		if !ok || key != "startDate" || val != "2026-01-01" {
			t.Errorf("startdate lookup: key=%q val=%v ok=%v", key, val, ok)
		}
	})
}

func TestFilterSprintFields_EmptyList(t *testing.T) {
	s := FlatSprint{ID: "3", Name: "All", State: "future"}
	got := FilterSprintFields(s, nil)
	if len(got) != 3 {
		t.Fatalf("empty filter should return full map, got %d: %v", len(got), got)
	}
}

func TestFilterSprintFields_SkipsEmptyAndUnknown(t *testing.T) {
	s := FlatSprint{ID: "4", Name: "S4", State: "active"}
	got := FilterSprintFields(s, []string{"", "  ", "name", "missing"})
	if len(got) != 1 || got["name"] != "S4" {
		t.Errorf("expected only name, got %v", got)
	}
}

func TestSprintToMap_AllFields(t *testing.T) {
	s := FlatSprint{
		ID:        "10",
		Name:      "Sprint",
		State:     "closed",
		StartDate: "2026-03-01",
		EndDate:   "2026-03-14",
		Goal:      "Release",
	}
	got := SprintToMap(s)
	if len(got) != 6 {
		t.Fatalf("expected 6 fields, got %d: %v", len(got), got)
	}
}

func TestSprintCanonicalKey(t *testing.T) {
	if sprintCanonicalKey("startdate") != "startDate" {
		t.Error("startdate should map to startDate")
	}
	if sprintCanonicalKey("enddate") != "endDate" {
		t.Error("enddate should map to endDate")
	}
	if sprintCanonicalKey("name") != "name" {
		t.Error("name should pass through unchanged")
	}
}
