package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testDir = filepath.Join(dir, "audit")
	t.Cleanup(func() { testDir = "" })
	return testDir
}

func TestLog_CreatesFile(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	Log("issue create", []string{"--project", "TEST", "--summary", "hello"}, 0, 1234)

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 audit file, got %d", len(files))
	}

	// Verify filename format
	name := filepath.Base(files[0])
	expected := "audit-" + time.Now().Format("2006-01") + ".jsonl"
	if name != expected {
		t.Errorf("filename = %q, want %q", name, expected)
	}

	// Verify content
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected at least one line in audit file")
	}

	var e entry
	if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.Cmd != "issue create" {
		t.Errorf("cmd = %q, want %q", e.Cmd, "issue create")
	}
	if e.Exit != 0 {
		t.Errorf("exit = %d, want 0", e.Exit)
	}
	if e.Ms != 1234 {
		t.Errorf("ms = %d, want 1234", e.Ms)
	}
	if len(e.Args) != 4 {
		t.Errorf("args len = %d, want 4", len(e.Args))
	}
}

func TestLog_Disabled(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "1")

	Log("issue delete", []string{"TEST-1"}, 0, 100)

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 audit files when disabled, got %d", len(files))
	}
}

func TestLog_AppendMultipleEntries(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	Log("issue create", []string{"TEST-1"}, 0, 100)
	Log("issue edit", []string{"TEST-1", "--summary", "new"}, 0, 200)

	files, _ := Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f, _ := os.Open(files[0])
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if lines != 2 {
		t.Errorf("expected 2 lines, got %d", lines)
	}
}

func TestSanitizeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"no token", []string{"--project", "TEST"}, []string{"--project", "TEST"}},
		{"with --token", []string{"--token", "secret123", "--project", "TEST"}, []string{"--project", "TEST"}},
		{"with -t", []string{"-t", "secret123", "arg1"}, []string{"arg1"}},
		{"with --token=value", []string{"--token=secret123", "--project", "TEST"}, []string{"--project", "TEST"}},
		{"with --TOKEN=value", []string{"--TOKEN=secret123", "arg1"}, []string{"arg1"}},
		{"with --Token=value", []string{"--Token=mysecret", "arg1"}, []string{"arg1"}},
		{"empty", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeArgs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "1")

	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create a file with an old date
	oldFile := filepath.Join(auditDir, "audit-2020-01.jsonl")
	if err := os.WriteFile(oldFile, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a current file
	currentFile := filepath.Join(auditDir, "audit-"+time.Now().Format("2006-01")+".jsonl")
	if err := os.WriteFile(currentFile, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Trigger cleanup via Log
	Log("test", []string{"arg"}, 0, 10)

	// Old file should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old audit file should have been deleted")
	}

	// Current file should still exist
	if _, err := os.Stat(currentFile); os.IsNotExist(err) {
		t.Error("current audit file should still exist")
	}
}

func TestCleanup_Disabled(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(auditDir, "audit-2020-01.jsonl")
	if err := os.WriteFile(oldFile, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	Log("test", []string{"arg"}, 0, 10)

	// Old file should NOT be deleted when retention is 0
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Error("old audit file should NOT be deleted when retention is 0")
	}
}

func TestFiles_NoDirectory(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "1")

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if files != nil {
		t.Errorf("expected nil files when dir doesn't exist, got %v", files)
	}
}

func TestEntry_Fields(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	Log("issue edit", []string{"PROJ-1", "--summary", "updated"}, 2, 500)

	files, _ := Files()
	if len(files) == 0 {
		t.Fatal("expected at least 1 audit file")
	}
	data, _ := os.ReadFile(files[0])
	line := strings.TrimSpace(string(data))

	var e entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify timestamp is recent (within last minute)
	ts, err := time.Parse(time.RFC3339Nano, e.Ts)
	if err != nil {
		t.Fatalf("parse timestamp: %v", err)
	}
	if time.Since(ts) > time.Minute {
		t.Errorf("timestamp %v is too old", ts)
	}

	if e.Cmd != "issue edit" {
		t.Errorf("cmd = %q", e.Cmd)
	}
	if e.Exit != 2 {
		t.Errorf("exit = %d", e.Exit)
	}
	if e.Ms != 500 {
		t.Errorf("ms = %d", e.Ms)
	}
}
