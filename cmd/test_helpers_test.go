package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

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
	origCompactJSON := output.CompactJSON
	origErrorJSON := output.ErrorJSON
	origQuiet := output.Quiet
	defer func() {
		outputFormat = origOutputFormat
		jsonCompatMode = origJSONCompatMode
		jsonMode = origJSONMode
		compactMode = origCompactMode
		quietMode = origQuietMode
		dryRun = origDryRun
		forceMode = origForceMode
		output.CompactJSON = origCompactJSON
		output.ErrorJSON = origErrorJSON
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
	stdout, stderr, err := runRoot(t, args...)
	if err != nil {
		t.Fatalf("args=%v: unexpected error %v (exit=%d)", args, err, LastExitCode())
	}
	if LastExitCode() != ExitOK {
		t.Fatalf("args=%v: exit=%d, want 0", args, LastExitCode())
	}
	return stdout, stderr
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
