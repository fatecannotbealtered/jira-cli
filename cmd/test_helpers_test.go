package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

func init() {
	_ = os.Setenv("JIRA_CLI_RETRY_BASE_MS", "0")
}

// mockJiraServer starts an httptest TLS server and sets JIRA_HOST/JIRA_TOKEN env vars.
// Requires TestMain (search_auth_test.go) to set InsecureSkipVerify on DefaultTransport.
func mockJiraServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	t.Setenv("JIRA_HOST", ts.URL)
	t.Setenv("JIRA_TOKEN", "test-pat-token")
	// Prevent reading stale config file
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	return ts
}

// runRoot executes the CLI with args, capturing stdout/stderr separately.
func runRoot(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	lastExit = 0
	origOutputFormat := outputFormat
	origJSONCompatMode := jsonCompatMode
	origJSONMode := jsonMode
	origCompactMode := compactMode
	origQuietMode := quietMode
	origDryRun := dryRun
	origForceMode := forceMode
	origConfirmToken := confirmToken
	origCompactJSON := output.CompactJSON
	origErrorJSON := output.ErrorJSON
	origEnvelopeJSON := output.EnvelopeJSON
	origQuiet := output.Quiet
	defer func() {
		outputFormat = origOutputFormat
		jsonCompatMode = origJSONCompatMode
		jsonMode = origJSONMode
		compactMode = origCompactMode
		quietMode = origQuietMode
		dryRun = origDryRun
		forceMode = origForceMode
		confirmToken = origConfirmToken
		output.CompactJSON = origCompactJSON
		output.ErrorJSON = origErrorJSON
		output.EnvelopeJSON = origEnvelopeJSON
		output.Quiet = origQuiet
	}()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdoutBuf, stdoutR)
		close(stdoutDone)
	}()
	go func() {
		_, _ = io.Copy(&stderrBuf, stderrR)
		close(stderrDone)
	}()

	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(args)
	runErr := Execute()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout, os.Stderr = origOut, origErr
	<-stdoutDone
	<-stderrDone
	_ = stdoutR.Close()
	_ = stderrR.Close()

	rootCmd.SetOut(origOut)
	rootCmd.SetErr(origErr)
	rootCmd.SetArgs(nil)

	return stdoutBuf.String(), stderrBuf.String(), runErr
}

func textFormatUnlessSpecified(args []string) []string {
	for _, arg := range args {
		if arg == "--json" || arg == "--format" || strings.HasPrefix(arg, "--format=") {
			return args
		}
	}
	withFormat := make([]string, 0, len(args)+2)
	withFormat = append(withFormat, "--format", "text")
	withFormat = append(withFormat, args...)
	return withFormat
}

// runRootExpectSilent runs CLI and asserts ErrSilent with expected exit code.
func runRootExpectSilent(t *testing.T, code int, args ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, err := runRoot(t, args...)
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("args=%v: expected ErrSilent, got %v (exit=%d)", args, err, LastExitCode())
	}
	if LastExitCode() != code {
		t.Fatalf("args=%v: exit=%d, want %d", args, LastExitCode(), code)
	}
	return stdout, stderr
}

// runRootOK runs CLI expecting success (nil error, exit 0).
func runRootOK(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	if argsNeedAutoConfirm(args) {
		token := dryRunConfirmToken(t, args...)
		resetCLIState(t)
		if token != "" {
			args = append([]string{"--confirm", token}, args...)
		}
	}
	stdout, stderr, err := runRoot(t, args...)
	if err != nil {
		t.Fatalf("args=%v: unexpected error %v (exit=%d)", args, err, LastExitCode())
	}
	if LastExitCode() != ExitOK {
		t.Fatalf("args=%v: exit=%d, want 0", args, LastExitCode())
	}
	return stdout, stderr
}

func dryRunConfirmToken(t *testing.T, args ...string) string {
	t.Helper()
	stdout, _, err := runRoot(t, append([]string{"--dry-run"}, args...)...)
	if err != nil {
		t.Fatalf("dry-run args=%v: unexpected error %v (exit=%d)", args, err, LastExitCode())
	}
	var env struct {
		Data struct {
			ConfirmToken string `json:"confirm_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &env); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, stdout)
	}
	if env.Data.ConfirmToken == "" {
		return ""
	}
	return env.Data.ConfirmToken
}

func decodeEnvelopeData(t *testing.T, out string, v any) {
	t.Helper()
	trimmed := strings.TrimSpace(out)
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(env.Data) == 0 {
		if err := json.Unmarshal([]byte(trimmed), v); err != nil {
			t.Fatalf("invalid raw JSON: %v\n%s", err, out)
		}
		return
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatalf("invalid envelope data: %v\n%s", err, out)
	}
}

func decodeEnvelopeError(t *testing.T, out string) map[string]any {
	t.Helper()
	var env struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("invalid error envelope: %v\n%s", err, out)
	}
	if env.Error == nil {
		t.Fatalf("missing error envelope: %s", out)
	}
	return env.Error
}

func argsNeedAutoConfirm(args []string) bool {
	if !argsUseJSON(args) || hasFlag(args, "dry-run") || hasFlag(args, "confirm") || hasFlag(args, "force") {
		return false
	}
	tokens := commandTokens(args)
	return tokensAreWriteCommand(tokens)
}

func argsUseJSON(args []string) bool {
	for i, arg := range args {
		if arg == "--json" {
			return true
		}
		if arg == "--format" && i+1 < len(args) {
			return args[i+1] == "json"
		}
		if strings.HasPrefix(arg, "--format=") {
			return strings.TrimPrefix(arg, "--format=") == "json"
		}
	}
	return true
}

func hasFlag(args []string, name string) bool {
	long := "--" + name
	prefix := long + "="
	for _, arg := range args {
		if arg == long || strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func commandTokens(args []string) []string {
	var tokens []string
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--format" || arg == "--confirm" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		tokens = append(tokens, arg)
		if tokensAreWriteCommand(tokens) {
			return tokens
		}
	}
	return tokens
}

func tokensAreWriteCommand(tokens []string) bool {
	if len(tokens) == 1 {
		switch tokens[0] {
		case "login", "logout":
			return true
		}
	}
	if len(tokens) < 2 {
		return false
	}
	switch tokens[0] {
	case "filter":
		return tokens[1] == "create" || tokens[1] == "delete"
	case "sprint":
		switch tokens[1] {
		case "create", "update", "close", "move":
			return true
		}
	case "issue":
		switch tokens[1] {
		case "create", "edit", "delete", "assign", "unassign", "watch", "unwatch", "vote", "unvote",
			"transition", "bulk-transition", "unlink", "remote-link", "clone", "attach",
			"bulk-create", "backlog-move", "bulk-assign", "bulk-edit", "rank":
			return true
		case "link":
			// `issue link <KEY>` is a write, but `issue link list <KEY>` is a
			// read. Defer the decision until the 3rd token is seen so
			// commandTokens doesn't return early on the 2-token prefix.
			return len(tokens) >= 3 && tokens[2] != "list"
		case "comment":
			return len(tokens) >= 3 && (tokens[2] == "add" || tokens[2] == "delete")
		case "worklog":
			return len(tokens) >= 3 && tokens[2] == "add"
		}
	}
	return false
}

// containsAny reports whether s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
