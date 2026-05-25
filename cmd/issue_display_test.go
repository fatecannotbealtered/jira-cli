package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
)

func TestFormatTime_Empty(t *testing.T) {
	if got := formatTime(""); got != "" {
		t.Errorf("formatTime(\"\") = %q, want empty", got)
	}
}

func TestFormatTime_JiraFormat(t *testing.T) {
	got := formatTime("2024-06-15T10:30:45.000+0000")
	want := "2024-06-15 10:30"
	if got != want {
		t.Errorf("formatTime(jira) = %q, want %q", got, want)
	}
}

func TestFormatTime_RFC3339(t *testing.T) {
	ts := time.Date(2024, 7, 20, 14, 5, 0, 0, time.FixedZone("CST", 8*3600))
	got := formatTime(ts.Format(time.RFC3339))
	if !strings.HasPrefix(got, "2024-07-20") {
		t.Errorf("formatTime(RFC3339) = %q", got)
	}
}

func TestFormatTime_FallbackLong(t *testing.T) {
	got := formatTime("2024-08-01-extra")
	if got != "2024-08-01" {
		t.Errorf("formatTime(fallback long) = %q, want 2024-08-01", got)
	}
}

func TestFormatTime_FallbackShort(t *testing.T) {
	got := formatTime("short")
	if got != "short" {
		t.Errorf("formatTime(fallback short) = %q, want short", got)
	}
}

func TestSafeDate(t *testing.T) {
	if got := safeDate("2024-09-10T00:00:00.000Z"); got != "2024-09-10" {
		t.Errorf("safeDate(long) = %q", got)
	}
	if got := safeDate("2024"); got != "2024" {
		t.Errorf("safeDate(short) = %q", got)
	}
}

func adfParagraph(text string) map[string]any {
	return map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": text},
				},
			},
		},
	}
}

func TestPrintIssueDetail_Full(t *testing.T) {
	longDesc := strings.Repeat("x", 900)
	issue := &api.Issue{
		Key: "DISP-1",
		Fields: api.IssueFields{
			Summary:     "Display test",
			Status:      api.Status{Name: "In Progress"},
			IssueType:   api.IssueType{Name: "Bug"},
			Priority:    &api.Priority{Name: "High"},
			Assignee:    &api.User{DisplayName: "Assignee User"},
			Reporter:    &api.User{DisplayName: "Reporter User"},
			Created:     "2024-01-01T08:00:00.000+0000",
			Updated:     "2024-01-02T09:00:00.000+0000",
			Labels:      []string{"a", "b"},
			Components:  []api.Component{{Name: "Core"}},
			FixVersions: []api.Version{{Name: "v1.0"}},
			Description: adfParagraph(longDesc),
		},
	}
	out := captureStdout(t, func() {
		printIssueDetail(issue, "https://jira.example.com")
	})
	for _, want := range []string{"DISP-1", "Display test", "Assignee User", "Reporter User", "a, b", "Core", "v1.0", "https://jira.example.com/browse/DISP-1", "Description"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if !strings.Contains(out, "…") {
		t.Error("long description should be truncated with ellipsis")
	}
}

func TestPrintIssueDetail_Minimal(t *testing.T) {
	issue := &api.Issue{
		Key: "DISP-2",
		Fields: api.IssueFields{
			Summary:   "Minimal",
			Status:    api.Status{Name: "Open"},
			IssueType: api.IssueType{Name: "Task"},
		},
	}
	out := captureStdout(t, func() {
		printIssueDetail(issue, "https://jira.example.com")
	})
	if !strings.Contains(out, "DISP-2") || !strings.Contains(out, "Minimal") {
		t.Errorf("unexpected minimal output: %s", out)
	}
	if strings.Contains(out, "Assignee") {
		t.Error("assignee should be omitted when nil")
	}
}

func TestPrintIssueTable(t *testing.T) {
	issues := []api.Issue{
		{
			Key: "TBL-1",
			Fields: api.IssueFields{
				Summary:   strings.Repeat("s", 70),
				Status:    api.Status{Name: "Open"},
				IssueType: api.IssueType{Name: "Bug"},
				Assignee:  &api.User{DisplayName: "Alice"},
				Priority:  &api.Priority{Name: "Medium"},
				Updated:   "2024-02-01T12:00:00.000+0000",
			},
		},
		{
			Key: "TBL-2",
			Fields: api.IssueFields{
				Summary:   "Second",
				Status:    api.Status{Name: "Done"},
				IssueType: api.IssueType{Name: "Task"},
				Updated:   "bad-date",
			},
		},
	}
	out := captureStdout(t, func() {
		printIssueTable(issues)
	})
	if !strings.Contains(out, "TBL-1") || !strings.Contains(out, "TBL-2") {
		t.Errorf("table missing issue keys:\n%s", out)
	}
	if !strings.Contains(out, "...") {
		t.Error("long summary should be truncated in table")
	}
	if !strings.Contains(out, "Alice") {
		t.Error("table should include assignee display name")
	}
}

func TestPrintSprintTable(t *testing.T) {
	sprints := []api.Sprint{
		{ID: 1, Name: "Active Sprint", State: "active", StartDate: "2024-03-01T00:00:00.000Z", EndDate: "2024-03-14T00:00:00.000Z", Goal: strings.Repeat("g", 50)},
		{ID: 2, Name: "Future Sprint", State: "future", StartDate: "2024", EndDate: ""},
		{ID: 3, Name: "Closed Sprint", State: "closed", Goal: "Done"},
	}
	out := captureStdout(t, func() {
		printSprintTable(sprints)
	})
	for _, want := range []string{"Active Sprint", "Future Sprint", "Closed Sprint", "2024-03-01", "2024", "..."} {
		if !strings.Contains(out, want) {
			t.Errorf("sprint table missing %q\n%s", want, out)
		}
	}
}
