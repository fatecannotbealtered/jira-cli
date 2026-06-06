package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/config"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	os.Stderr = orig
	<-done
	_ = r.Close()
	return buf.String()
}

func withStdin(t *testing.T, input string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	go func() {
		_, _ = io.WriteString(w, input)
		_ = w.Close()
	}()
	return func() {
		os.Stdin = orig
		_ = r.Close()
	}
}

func setupAuditDir(t *testing.T) string {
	t.Helper()
	t.Setenv("JIRA_NO_AUDIT", "")
	t.Setenv("JIRA_AUDIT_RETENTION_MONTHS", "0")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	return filepath.Join(tmpDir, ".jira-cli", "audit")
}

func auditDirFromEnv() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".jira-cli", "audit")
}

func readLastAuditEntry(t *testing.T, auditDir string) map[string]any {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(auditDir, "audit-*.jsonl"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no audit files in %s: %v", auditDir, err)
	}
	data, err := os.ReadFile(matches[len(matches)-1])
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("unmarshal audit entry: %v", err)
	}
	return entry
}

func TestHandleAPIError_APIError_Text(t *testing.T) {
	lastExit = 0
	apiErr := &api.APIError{StatusCode: 404, ErrorMessages: []string{"not found"}}
	err := captureStderr(t, func() {
		got := handleAPIError(apiErr, false)
		if !errors.Is(got, ErrSilent) {
			t.Fatalf("expected ErrSilent, got %v", got)
		}
	})
	if !strings.Contains(err, "not found") {
		t.Errorf("stderr = %q, want API error message", err)
	}
	if LastExitCode() != ExitNotFound {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitNotFound)
	}
	lastExit = 0
}

func TestHandleAPIError_APIError_JSON(t *testing.T) {
	lastExit = 0
	apiErr := &api.APIError{StatusCode: 401, ErrorMessages: []string{"unauthorized"}}
	errOut := captureStdout(t, func() {
		got := handleAPIError(apiErr, true)
		if !errors.Is(got, ErrSilent) {
			t.Fatalf("expected ErrSilent, got %v", got)
		}
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(errOut)), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, errOut)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(output.ErrAuth) {
		t.Errorf("code = %v, want %q", errPayload["code"], output.ErrAuth)
	}
	if LastExitCode() != ExitAuth {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitAuth)
	}
	lastExit = 0
}

func TestHandleAPIError_NonAPIError_Text(t *testing.T) {
	lastExit = 0
	errOut := captureStderr(t, func() {
		got := handleAPIError(errors.New("connection refused"), false)
		if !errors.Is(got, ErrSilent) {
			t.Fatalf("expected ErrSilent, got %v", got)
		}
	})
	if !strings.Contains(errOut, "connection refused") {
		t.Errorf("stderr = %q", errOut)
	}
	if LastExitCode() != ExitNetwork {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitNetwork)
	}
	lastExit = 0
}

func TestHandleAPIError_NonAPIError_JSON(t *testing.T) {
	lastExit = 0
	errOut := captureStdout(t, func() {
		got := handleAPIError(errors.New("timeout"), true)
		if !errors.Is(got, ErrSilent) {
			t.Fatalf("expected ErrSilent, got %v", got)
		}
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(errOut)), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, errOut)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(output.ErrNetwork) {
		t.Errorf("code = %v, want %q", errPayload["code"], output.ErrNetwork)
	}
	if LastExitCode() != ExitNetwork {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitNetwork)
	}
	lastExit = 0
}

func TestConfirmAction_ForceMode(t *testing.T) {
	orig := forceMode
	defer func() { forceMode = orig }()
	forceMode = true
	if !confirmAction("Type YES", "YES") {
		t.Fatal("confirmAction should return true when --force is set")
	}
}

func TestConfirmAction_CorrectInput(t *testing.T) {
	orig := forceMode
	defer func() { forceMode = orig }()
	forceMode = false
	restore := withStdin(t, "DELETE\n")
	defer restore()
	if !confirmAction("Type DELETE to confirm", "DELETE") {
		t.Fatal("confirmAction should accept matching input")
	}
}

func TestConfirmAction_WrongInput(t *testing.T) {
	orig := forceMode
	defer func() { forceMode = orig }()
	forceMode = false
	restore := withStdin(t, "no\n")
	defer restore()
	if confirmAction("Type DELETE to confirm", "DELETE") {
		t.Fatal("confirmAction should reject non-matching input")
	}
}

func TestConfirmAction_ScanError(t *testing.T) {
	orig := forceMode
	defer func() { forceMode = orig }()
	forceMode = false
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	_ = w.Close()
	defer func() {
		os.Stdin = origStdin
		_ = r.Close()
	}()
	if confirmAction("Type YES", "YES") {
		t.Fatal("confirmAction should return false on stdin read error")
	}
}

func TestNewClient_ConfigError_Text(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")

	orig := jsonMode
	defer func() { jsonMode = orig; lastExit = 0 }()
	jsonMode = false
	lastExit = 0

	errOut := captureStderr(t, func() {
		client, cfg, err := newClient()
		if client != nil || cfg != nil {
			t.Fatal("expected nil client and config")
		}
		if !errors.Is(err, ErrSilent) {
			t.Fatalf("expected ErrSilent, got %v", err)
		}
	})
	if errOut == "" {
		t.Error("expected config error on stderr")
	}
	if LastExitCode() != ExitAuth {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitAuth)
	}
}

func TestNewClient_ConfigError_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")

	orig := jsonMode
	defer func() { jsonMode = orig; lastExit = 0 }()
	jsonMode = true
	lastExit = 0

	errOut := captureStdout(t, func() {
		_, _, err := newClient()
		if !errors.Is(err, ErrSilent) {
			t.Fatalf("expected ErrSilent, got %v", err)
		}
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(errOut)), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, errOut)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(output.ErrConfig) {
		t.Errorf("code = %v, want %q", errPayload["code"], output.ErrConfig)
	}
}

func TestNewClient_Success(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client, cfg, err := newClient()
	if err != nil {
		t.Fatalf("newClient() error: %v", err)
	}
	if client == nil || cfg == nil {
		t.Fatal("expected client and config")
	}
	if cfg.Host == "" || cfg.Token == "" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestResolveUsername_Passthrough(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client, _, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveUsername(client, "jdoe")
	if err != nil {
		t.Fatalf("resolveUsername error: %v", err)
	}
	if got != "jdoe" {
		t.Errorf("got %q, want jdoe", got)
	}
}

func TestResolveUsername_Me(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/myself" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"alice","displayName":"Alice"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client, _, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveUsername(client, "me")
	if err != nil {
		t.Fatalf("resolveUsername(me) error: %v", err)
	}
	if got != "alice" {
		t.Errorf("got %q, want alice", got)
	}
}

func TestResolveUsername_Me_APIError(t *testing.T) {
	orig := jsonMode
	defer func() { jsonMode = orig; lastExit = 0 }()
	jsonMode = false
	lastExit = 0

	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["bad token"]}`))
	})
	client, _, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveUsername(client, "me")
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitAuth)
	}
}

func TestExecute_AuditWriteCommand_Success(t *testing.T) {
	setupAuditDir(t)
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	origArgs := os.Args
	os.Args = []string{"jira-cli", "issue", "create", "--dry-run", "--project", "TEST", "--summary", "hello"}
	t.Cleanup(func() { os.Args = origArgs })

	_, _, err := runRoot(t, "issue", "create", "--dry-run", "--project", "TEST", "--summary", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v (exit=%d)", err, LastExitCode())
	}

	entry := readLastAuditEntry(t, auditDirFromEnv())
	if entry["cmd"] != "jira-cli issue create" {
		t.Errorf("cmd = %v, want jira-cli issue create", entry["cmd"])
	}
	if int(entry["exit"].(float64)) != ExitOK {
		t.Errorf("exit = %v, want 0", entry["exit"])
	}
}

func TestExecute_AuditWriteCommand_Failure(t *testing.T) {
	setupAuditDir(t)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")

	origArgs := os.Args
	os.Args = []string{"jira-cli", "issue", "create", "--project", "TEST", "--summary", "hello"}
	t.Cleanup(func() { os.Args = origArgs })

	_, _, err := runRoot(t, "issue", "create", "--project", "TEST", "--summary", "hello")
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v (exit=%d)", err, LastExitCode())
	}
	if LastExitCode() != ExitAuth {
		t.Errorf("exit = %d, want %d", LastExitCode(), ExitAuth)
	}

	entry := readLastAuditEntry(t, auditDirFromEnv())
	if entry["cmd"] != "jira-cli issue create" {
		t.Errorf("cmd = %v, want jira-cli issue create", entry["cmd"])
	}
	if int(entry["exit"].(float64)) != ExitAuth {
		t.Errorf("exit = %v, want %d", entry["exit"], ExitAuth)
	}
}

func TestExecute_AuditSkippedForReadCommand(t *testing.T) {
	auditDir := setupAuditDir(t)

	origArgs := os.Args
	os.Args = []string{"jira-cli", "reference"}
	t.Cleanup(func() { os.Args = origArgs })

	runRootOK(t, "reference")

	matches, _ := filepath.Glob(filepath.Join(auditDir, "audit-*.jsonl"))
	if len(matches) > 0 {
		t.Errorf("read command should not write audit log, found %v", matches)
	}
}

func TestExecute_AuditDisabled(t *testing.T) {
	setupAuditDir(t)
	t.Setenv("JIRA_NO_AUDIT", "1")
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	origArgs := os.Args
	os.Args = []string{"jira-cli", "issue", "create", "--dry-run", "--project", "TEST", "--summary", "hello"}
	t.Cleanup(func() { os.Args = origArgs })

	runRootOK(t, "issue", "create", "--dry-run", "--project", "TEST", "--summary", "hello")

	matches, _ := filepath.Glob(filepath.Join(auditDirFromEnv(), "audit-*.jsonl"))
	if len(matches) > 0 {
		t.Errorf("JIRA_NO_AUDIT=1 should skip audit, found %v", matches)
	}
}

func TestMustGetString(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("project", "PROJ", "")
	if got := mustGetString(cmd, "project"); got != "PROJ" {
		t.Errorf("mustGetString = %q, want PROJ", got)
	}
}

func TestMarkWrite_NilAnnotations(t *testing.T) {
	cmd := &cobra.Command{}
	markWrite(cmd)
	if cmd.Annotations["write"] != "true" {
		t.Errorf("write annotation = %q, want true", cmd.Annotations["write"])
	}
}

func TestIsWriteCommand(t *testing.T) {
	read := &cobra.Command{}
	write := &cobra.Command{}
	markWrite(write)
	if isWriteCommand(read) {
		t.Error("read command should not be a write command")
	}
	if !isWriteCommand(write) {
		t.Error("marked command should be a write command")
	}
}

func TestExecute_SetsCmdStartTime(t *testing.T) {
	mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	before := time.Now()
	runRootOK(t, "reference")
	if cmdStartTime.Before(before) {
		t.Error("cmdStartTime should be updated during Execute")
	}
}

func TestNewClient_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")

	cfg := &config.Config{Host: "https://jira.example.com", Token: "file-token"}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	client, loaded, err := newClient()
	if err != nil {
		t.Fatalf("newClient() error: %v", err)
	}
	if client == nil || loaded.Host != "https://jira.example.com" {
		t.Fatalf("unexpected client/config: %+v", loaded)
	}
}
