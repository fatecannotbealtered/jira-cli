package audit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func overrideConfigHome(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	_ = os.Setenv("HOME", tmpDir)
	_ = os.Setenv("USERPROFILE", tmpDir)
	return tmpDir, func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	}
}

func TestDir_Default(t *testing.T) {
	testDir = ""
	t.Cleanup(func() { testDir = "" })

	tmpDir, restore := overrideConfigHome(t)
	defer restore()

	want := filepath.Join(tmpDir, ".jira-cli", "audit")
	if got := Dir(); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestRetentionMonths(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{"default empty", "", 3},
		{"invalid string", "abc", 3},
		{"negative", "-1", 3},
		{"zero disables cleanup", "0", 0},
		{"custom retention", "6", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == "" {
				t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "")
			} else {
				t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", tt.env)
			}
			if got := retentionMonths(); got != tt.want {
				t.Errorf("retentionMonths() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCleanup_SkipsNonAuditFiles(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "1")

	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(auditDir, "audit-2020-01.jsonl")
	if err := os.WriteFile(oldFile, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(auditDir, "notes.txt")
	if err := os.WriteFile(otherFile, []byte("keep\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cleanup(auditDir)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old audit file should have been deleted")
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Errorf("non-audit file should remain: %v", err)
	}
}

func TestCleanup_ReadDirError(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "1")

	orig := auditReadDir
	auditReadDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("readdir failed")
	}
	t.Cleanup(func() { auditReadDir = orig })

	cleanup(auditDir)
}

func TestLog_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	testDir = filepath.Join(blocker, "audit")
	t.Cleanup(func() { testDir = "" })
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	Log("issue create", []string{"arg"}, 0, 10)

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no audit files, got %d", len(files))
	}
}

func TestLog_OpenFileError(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatal(err)
	}
	monthFile := filepath.Join(auditDir, "audit-"+time.Now().Format("2006-01")+".jsonl")
	if err := os.Mkdir(monthFile, 0700); err != nil {
		t.Fatal(err)
	}

	Log("issue create", []string{"arg"}, 0, 10)

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no audit files after OpenFile failure, got %d", len(files))
	}
}

func TestLog_MarshalError(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	orig := auditJSONMarshal
	auditJSONMarshal = func(v any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() { auditJSONMarshal = orig })

	Log("issue create", []string{"arg"}, 0, 10)

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no audit files after marshal failure, got %d", len(files))
	}
}

func TestFiles_ReadDirError(t *testing.T) {
	setupTest(t)

	orig := auditReadDir
	auditReadDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("readdir failed")
	}
	t.Cleanup(func() { auditReadDir = orig })

	_, err := Files()
	if err == nil {
		t.Fatal("Files() should return error when ReadDir fails")
	}
}

func TestCleanup_RemovesOldAuditFilesDirectly(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "2")

	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(auditDir, "audit-2019-06.jsonl")
	currentFile := filepath.Join(auditDir, "audit-"+time.Now().Format("2006-01")+".jsonl")
	for _, path := range []string{oldFile, currentFile} {
		if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cleanup(auditDir)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old audit file should have been deleted by cleanup")
	}
	if _, err := os.Stat(currentFile); err != nil {
		t.Errorf("current audit file should remain: %v", err)
	}
}

func TestCleanup_SkipsMalformedAuditNames(t *testing.T) {
	auditDir := setupTest(t)
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "1")

	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatal(err)
	}

	malformed := filepath.Join(auditDir, "audit-legacy.jsonl")
	if err := os.WriteFile(malformed, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cleanup(auditDir)

	if _, err := os.Stat(malformed); err != nil {
		t.Errorf("malformed audit filename should be kept: %v", err)
	}
}

func TestLog_WritesEntryWhenOpenSucceeds(t *testing.T) {
	setupTest(t)
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")

	Log("issue create", []string{"--token", "secret", "--project", "TEST"}, 1, 42)

	files, err := Files()
	if err != nil {
		t.Fatalf("Files() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 audit file, got %d", len(files))
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.Contains(line, `"cmd":"issue create"`) {
		t.Errorf("audit line = %q, want cmd field", line)
	}
	if !strings.Contains(line, `"exit":1`) {
		t.Errorf("audit line = %q, want exit code", line)
	}
}
