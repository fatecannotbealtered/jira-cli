package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

// captureStdout captures stdout during fn().
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	// Drain pipe concurrently to avoid deadlock on Windows when buffer fills.
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	os.Stdout = orig
	<-done
	_ = r.Close()
	return buf.String()
}

// ─── jqlHasOrderBy ──────────────────────────────────────────────────────────

func TestJqlHasOrderBy(t *testing.T) {
	cases := []struct {
		jql  string
		want bool
	}{
		{"project = PROJ ORDER BY updated", true},
		{"project = PROJ order by updated", true},
		{"ORDER BY created DESC", true},
		{"project = PROJ", false},
		{"project = PROJ AND summary ~ 'reorder items'", false},
	}
	for _, tc := range cases {
		got := jqlHasOrderBy(tc.jql)
		if got != tc.want {
			t.Errorf("jqlHasOrderBy(%q) = %v, want %v", tc.jql, got, tc.want)
		}
	}
}

// ─── exitCodeForStatus ──────────────────────────────────────────────────────

func TestExitCodeForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{401, ExitAuth},
		{403, ExitForbidden},
		{404, ExitNotFound},
		{429, ExitRateLimit},
		{500, ExitNetwork},
		{503, ExitNetwork},
		{400, ExitBadArgs},
		{422, ExitBadArgs},
	}
	for _, tc := range cases {
		t.Run(strings.TrimSpace(strings.ReplaceAll(
			strings.Join([]string{string(rune(tc.status/100 + '0')), string(rune(tc.status%100/10 + '0')), string(rune(tc.status%10 + '0'))}, ""),
			"\x00", "")),
			func(t *testing.T) {
				got := exitCodeForStatus(tc.status)
				if got != tc.want {
					t.Errorf("exitCodeForStatus(%d) = %d, want %d", tc.status, got, tc.want)
				}
			})
	}
}

func TestExitCodeForStatus_Named(t *testing.T) {
	tests := map[int]int{
		401: ExitAuth,
		403: ExitForbidden,
		404: ExitNotFound,
		429: ExitRateLimit,
		500: ExitNetwork,
		502: ExitNetwork,
		503: ExitNetwork,
		400: ExitBadArgs,
		422: ExitBadArgs,
	}
	for status, want := range tests {
		if got := exitCodeForStatus(status); got != want {
			t.Errorf("exitCodeForStatus(%d) = %d, want %d", status, got, want)
		}
	}
}

// ─── setExitCode / LastExitCode ─────────────────────────────────────────────

func TestSetExitCode_Monotonic(t *testing.T) {
	// Reset
	lastExit = 0
	setExitCode(ExitAuth) // 3
	if LastExitCode() != ExitAuth {
		t.Fatalf("expected %d, got %d", ExitAuth, LastExitCode())
	}
	setExitCode(ExitOK) // 0 — should not decrease
	if LastExitCode() != ExitAuth {
		t.Fatalf("exit code should not decrease: expected %d, got %d", ExitAuth, LastExitCode())
	}
	setExitCode(ExitNetwork) // 7 — should increase
	if LastExitCode() != ExitNetwork {
		t.Fatalf("expected %d, got %d", ExitNetwork, LastExitCode())
	}
	lastExit = 0 // cleanup
}

func TestSilentErr_SetsExitCode(t *testing.T) {
	lastExit = 0
	err := SilentErr(ExitAuth)
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Errorf("LastExitCode() = %d, want %d", LastExitCode(), ExitAuth)
	}
	lastExit = 0
}

func TestDoctor_NotConfigured_ExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")

	lastExit = 0
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"doctor"})
	err := Execute()
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)

	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Errorf("LastExitCode() = %d, want %d", LastExitCode(), ExitAuth)
	}
	lastExit = 0
}

// ─── Root command help ──────────────────────────────────────────────────────

func TestRootHelp_ContainsUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})
	_ = rootCmd.Execute()
	rootCmd.SetOut(os.Stdout)
	out := buf.String()
	if !strings.Contains(out, "jira-cli") {
		t.Error("help should contain 'jira-cli'")
	}
	if !strings.Contains(out, "--json") {
		t.Error("help should document --json flag")
	}
	if !strings.Contains(out, "--format") {
		t.Error("help should document --format flag")
	}
	if !strings.Contains(out, "--quiet") {
		t.Error("help should document --quiet flag")
	}
	if !strings.Contains(out, "--dry-run") {
		t.Error("help should document --dry-run flag")
	}
	if !strings.Contains(out, "--force") {
		t.Error("help should document --force flag")
	}
}

func TestRootVersion_ContainsValue(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--version"})
	_ = rootCmd.Execute()
	rootCmd.SetOut(os.Stdout)
	out := buf.String()
	if !strings.Contains(out, "jira-cli") {
		t.Error("version output should contain 'jira-cli'")
	}
}

// ─── Reference command ──────────────────────────────────────────────────────

func TestReference_ContainsCommands(t *testing.T) {
	out, _ := runRootOKClean(t, "--format", "text", "reference")
	if !strings.Contains(out, "# jira-cli Command Reference") {
		t.Error("reference should start with header")
	}
	if !strings.Contains(out, "## jira-cli issue") {
		t.Error("reference should document 'issue' command")
	}
	if !strings.Contains(out, "## jira-cli search") {
		t.Error("reference should document 'search' command")
	}
	if !strings.Contains(out, "--json") {
		t.Error("reference should list --json flag")
	}
}

// ─── Quiet mode ─────────────────────────────────────────────────────────────

func TestQuietMode_SuppressesOutput(t *testing.T) {
	orig := output.Quiet
	defer func() { output.Quiet = orig }()

	output.Quiet = true
	out := captureStdout(t, func() {
		output.Success("should be suppressed")
		output.Info("should be suppressed")
		output.Bold("should be suppressed")
		output.Gray("should be suppressed")
	})
	if out != "" {
		t.Errorf("quiet mode should suppress output, got: %q", out)
	}
}

func TestQuietMode_JSONStillPrints(t *testing.T) {
	orig := output.Quiet
	defer func() { output.Quiet = orig }()

	output.Quiet = true
	out := captureStdout(t, func() {
		output.PrintJSON(map[string]string{"key": "value"})
	})
	if !strings.Contains(out, `"key"`) {
		t.Error("JSON output should not be suppressed by quiet mode")
	}
}

// ─── Flatten functions ──────────────────────────────────────────────────────

func TestToFlatIssue_BasicFields(t *testing.T) {
	issue := &api.Issue{
		Key: "TEST-1",
		Fields: api.IssueFields{
			Summary: "Test issue",
			Status:  api.Status{Name: "Open"},
			IssueType: api.IssueType{
				Name: "Bug",
			},
			Created: "2024-01-01T00:00:00.000+0000",
			Updated: "2024-01-02T00:00:00.000+0000",
		},
	}
	fi := toFlatIssue(issue)
	if fi.Key != "TEST-1" {
		t.Errorf("Key = %q, want TEST-1", fi.Key)
	}
	if fi.Summary != "Test issue" {
		t.Errorf("Summary = %q, want 'Test issue'", fi.Summary)
	}
	if fi.Status != "Open" {
		t.Errorf("Status = %q, want Open", fi.Status)
	}
	if fi.Type != "Bug" {
		t.Errorf("Type = %q, want Bug", fi.Type)
	}
}

func TestToFlatIssue_WithOptionalFields(t *testing.T) {
	issue := &api.Issue{
		Key: "TEST-2",
		Fields: api.IssueFields{
			Summary: "With assignee",
			Status:  api.Status{Name: "In Progress"},
			IssueType: api.IssueType{
				Name: "Task",
			},
			Assignee: &api.User{Name: "jdoe", DisplayName: "John Doe"},
			Priority: &api.Priority{Name: "High"},
			Labels:   []string{"backend", "urgent"},
			Parent:   &api.IssueRef{Key: "TEST-0"},
		},
	}
	fi := toFlatIssue(issue)
	if fi.Assignee != "jdoe" {
		t.Errorf("Assignee = %q, want jdoe", fi.Assignee)
	}
	if fi.Priority != "High" {
		t.Errorf("Priority = %q, want High", fi.Priority)
	}
	if fi.Labels != "backend,urgent" {
		t.Errorf("Labels = %q, want 'backend,urgent'", fi.Labels)
	}
	if fi.Parent != "TEST-0" {
		t.Errorf("Parent = %q, want TEST-0", fi.Parent)
	}
}

func TestFilterFields_ReturnsOnlyRequested(t *testing.T) {
	fi := output.FlatIssue{
		Key:      "TEST-1",
		Summary:  "Test",
		Status:   "Open",
		Type:     "Bug",
		Assignee: "jdoe",
	}
	result := output.FilterFields(fi, []string{"key", "summary"})
	if len(result) != 2 {
		t.Errorf("expected 2 fields, got %d", len(result))
	}
	if result["key"] != "TEST-1" {
		t.Errorf("key = %v, want TEST-1", result["key"])
	}
	if result["summary"] != "Test" {
		t.Errorf("summary = %v, want Test", result["summary"])
	}
	if _, ok := result["status"]; ok {
		t.Error("status should not be in filtered result")
	}
}

func TestToFlatSprint(t *testing.T) {
	sprint := &api.Sprint{
		ID:        42,
		Name:      "Sprint 1",
		State:     "active",
		StartDate: "2024-01-01",
		EndDate:   "2024-01-14",
		Goal:      "Ship v1",
	}
	fs := toFlatSprint(sprint)
	if fs.ID != 42 || fs.Name != "Sprint 1" || fs.State != "active" {
		t.Errorf("unexpected flat sprint: %+v", fs)
	}
	if fs.Goal != "Ship v1" {
		t.Errorf("Goal = %q, want 'Ship v1'", fs.Goal)
	}
}

// ─── printIssueJSON / printIssuesJSON ───────────────────────────────────────

func TestPrintIssueJSON_Flat(t *testing.T) {
	issue := &api.Issue{
		Key: "TEST-1",
		Fields: api.IssueFields{
			Summary:   "Test",
			Status:    api.Status{Name: "Open"},
			IssueType: api.IssueType{Name: "Bug"},
		},
	}
	out := captureStdout(t, func() {
		printIssueJSON(issue, false, nil)
	})
	var fi output.FlatIssue
	decodeEnvelopeData(t, out, &fi)
	if fi.Key != "TEST-1" || fi.Status != "Open" {
		t.Errorf("unexpected flat issue: %+v", fi)
	}
}

func TestPrintIssueJSON_Raw(t *testing.T) {
	issue := &api.Issue{
		Key: "TEST-1",
		Fields: api.IssueFields{
			Summary:   "Test",
			Status:    api.Status{Name: "Open"},
			IssueType: api.IssueType{Name: "Bug"},
		},
	}
	out := captureStdout(t, func() {
		printIssueJSON(issue, true, nil)
	})
	var raw api.Issue
	decodeEnvelopeData(t, out, &raw)
	if raw.Key != "TEST-1" {
		t.Errorf("raw key = %q, want TEST-1", raw.Key)
	}
}

func TestPrintIssueJSON_Fields(t *testing.T) {
	issue := &api.Issue{
		Key: "TEST-1",
		Fields: api.IssueFields{
			Summary:   "Test",
			Status:    api.Status{Name: "Open"},
			IssueType: api.IssueType{Name: "Bug"},
			Assignee:  &api.User{Name: "jdoe"},
		},
	}
	out := captureStdout(t, func() {
		printIssueJSON(issue, false, []string{"key", "assignee"})
	})
	var m map[string]any
	decodeEnvelopeData(t, out, &m)
	if m["key"] != "TEST-1" {
		t.Errorf("key = %v", m["key"])
	}
	if m["assignee"] != "jdoe" {
		t.Errorf("assignee = %v", m["assignee"])
	}
	if _, ok := m["summary"]; ok {
		t.Error("summary should not be present with --fields key,assignee")
	}
}

func TestPrintIssuesJSON_Flat(t *testing.T) {
	issues := []api.Issue{
		{Key: "A-1", Fields: api.IssueFields{Summary: "First", Status: api.Status{Name: "Open"}, IssueType: api.IssueType{Name: "Bug"}}},
		{Key: "A-2", Fields: api.IssueFields{Summary: "Second", Status: api.Status{Name: "Done"}, IssueType: api.IssueType{Name: "Task"}}},
	}
	out := captureStdout(t, func() {
		printIssuesJSON(issues, false, nil)
	})
	var flat []output.FlatIssue
	decodeEnvelopeData(t, out, &flat)
	if len(flat) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(flat))
	}
	if flat[0].Key != "A-1" || flat[1].Key != "A-2" {
		t.Errorf("unexpected keys: %s, %s", flat[0].Key, flat[1].Key)
	}
}

// ─── dryRunOutput ───────────────────────────────────────────────────────────

func TestDryRunOutput_Disabled(t *testing.T) {
	orig := dryRun
	defer func() { dryRun = orig }()
	dryRun = false
	if dryRunOutput("test", nil) {
		t.Error("dryRunOutput should return false when dryRun is false")
	}
}

func TestDryRunOutput_PlainText(t *testing.T) {
	origDR := dryRun
	origJM := jsonMode
	defer func() { dryRun = origDR; jsonMode = origJM }()
	dryRun = true
	jsonMode = false
	out := captureStdout(t, func() {
		if !dryRunOutput("create issue", map[string]any{"key": "TEST-1"}) {
			t.Error("dryRunOutput should return true when dryRun is true")
		}
	})
	if !strings.Contains(out, "[dry-run] create issue") {
		t.Errorf("expected dry-run message, got: %s", out)
	}
}

func TestDryRunOutput_JSON(t *testing.T) {
	origDR := dryRun
	origJM := jsonMode
	defer func() { dryRun = origDR; jsonMode = origJM }()
	dryRun = true
	jsonMode = true
	out := captureStdout(t, func() {
		dryRunOutput("edit issue", map[string]any{"key": "TEST-1"})
	})
	var m map[string]any
	decodeEnvelopeData(t, out, &m)
	preview := m["preview"].(map[string]any)
	if preview["action"] != "edit issue" {
		t.Errorf("action = %v", preview["action"])
	}
	if m["confirm_token"] == "" {
		t.Errorf("confirm_token = %v", m["confirm_token"])
	}
	changes := preview["changes"].([]any)
	target := changes[0].(map[string]any)["target"].(map[string]any)
	if target["key"] != "TEST-1" {
		t.Errorf("key = %v", target["key"])
	}
}

// ─── ErrorCodeFromStatus ────────────────────────────────────────────────────

func TestErrorCodeFromStatus(t *testing.T) {
	cases := []struct {
		status int
		want   output.ErrorCode
	}{
		{401, output.ErrAuth},
		{403, output.ErrForbidden},
		{404, output.ErrNotFound},
		{429, output.ErrRateLimit},
		{500, output.ErrServer},
		{503, output.ErrServer},
		{400, output.ErrValidation},
		{422, output.ErrValidation},
		{0, output.ErrUnknown},
	}
	for _, tc := range cases {
		got := output.ErrorCodeFromStatus(tc.status)
		if got != tc.want {
			t.Errorf("ErrorCodeFromStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}
