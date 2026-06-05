package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

func TestOutputFormat_DefaultJSONWithoutFlag(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))

	stdout, _ := runRootOKClean(t, "search", "project = P")

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["total"].(float64) != 2 {
		t.Fatalf("total=%v", result["total"])
	}
}

func TestOutputFormat_Text(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))

	stdout, _ := runRootOKClean(t, "--format", "text", "search", "project = P")

	if !containsAny(stdout, "P-1", "Showing 2 of 2") {
		t.Fatalf("expected text output, got: %s", stdout)
	}
}

func TestOutputFormat_RawSupportedCommand(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))

	stdout, _ := runRootOKClean(t, "--format", "raw", "search", "project = P")

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid raw JSON: %v\n%s", err, stdout)
	}
	issues := result["issues"].([]any)
	first := issues[0].(map[string]any)
	if _, ok := first["fields"]; !ok {
		t.Fatalf("expected raw issue shape with fields, got: %v", first)
	}
}

func TestOutputFormat_CompactJSON(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))

	stdout, _ := runRootOKClean(t, "--compact", "search", "project = P")

	if strings.Contains(stdout, "\n  ") {
		t.Fatalf("expected compact JSON, got: %s", stdout)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid compact JSON: %v\n%s", err, stdout)
	}
}

func TestOutputFormat_QuietDoesNotSuppressMachineResults(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))

	jsonOut, _ := runRootOKClean(t, "--quiet", "search", "project = P")
	rawOut, _ := runRootOKClean(t, "--quiet", "--format", "raw", "search", "project = P")

	for name, out := range map[string]string{"json": jsonOut, "raw": rawOut} {
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%s result was suppressed by --quiet", name)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
			t.Fatalf("%s output is not JSON: %v\n%s", name, err, out)
		}
	}
}

func TestOutputFormat_ArgumentErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "invalid format", args: []string{"--format", "xml", "reference"}},
		{name: "json conflicts text", args: []string{"--json", "--format", "text", "reference"}},
		{name: "json conflicts raw", args: []string{"--json", "--format", "raw", "search", "project = P"}},
		{name: "unsupported raw", args: []string{"--format", "raw", "reference"}},
		{name: "compact text", args: []string{"--compact", "--format", "text", "reference"}},
		{name: "compact raw", args: []string{"--compact", "--format", "raw", "search", "project = P"}},
		{name: "fields text", args: []string{"--format", "text", "search", "project = P", "--fields", "summary"}},
		{name: "fields raw", args: []string{"--format", "raw", "search", "project = P", "--fields", "summary"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runRootClean(t, tc.args...)
			if !errors.Is(err, ErrSilent) {
				t.Fatalf("expected ErrSilent, got %v", err)
			}
			if LastExitCode() != ExitBadArgs {
				t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
			}
		})
	}
}

func TestOutputFormat_CobraArgErrorDefaultJSON(t *testing.T) {
	stdout, stderr, err := runRootClean(t, "issue", "get")
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitBadArgs {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout should be empty for argument error, got %q", stdout)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr)
	}
	if payload["errorCode"] != string(output.ErrValidation) {
		t.Fatalf("payload=%v", payload)
	}
}

func TestOutputFormat_CobraArgErrorText(t *testing.T) {
	_, stderr, err := runRootClean(t, "--format", "text", "issue", "get")
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitBadArgs {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
	}
	if !strings.Contains(stderr, "accepts 1 arg") || strings.Contains(stderr, `"errorCode"`) {
		t.Fatalf("expected text argument error, got %q", stderr)
	}
}

func TestOutputFormat_CompactCobraArgError(t *testing.T) {
	_, stderr, err := runRootClean(t, "--compact", "issue", "get")
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if strings.Contains(stderr, "\n  ") {
		t.Fatalf("expected compact JSON error, got %q", stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr)
	}
}

func TestOutputFormat_DefaultJSONEmptySuccessBodies(t *testing.T) {
	t.Run("bulk transition no issues", func(t *testing.T) {
		setupMockJira(t, issueMockHandler(t))
		stdout, _ := runRootOKDefaultJSON(t, "issue", "bulk-transition", "In Progress", "--issues", " , ")
		var payload map[string]any
		decodeJSONOutput(t, stdout, &payload)
		if payload["total"].(float64) != 0 {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("attachment download empty", func(t *testing.T) {
		setupMockJira(t, issueMockHandler(t))
		stdout, _ := runRootOKDefaultJSON(t, "issue", "attachments", "PROJ-NOATTACH", "--out", t.TempDir())
		var saved []map[string]any
		decodeJSONOutput(t, stdout, &saved)
		if len(saved) != 0 {
			t.Fatalf("saved=%v", saved)
		}
	})

	t.Run("no active sprint", func(t *testing.T) {
		mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/board/") && strings.HasSuffix(r.URL.Path, "/sprint") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`))
				return
			}
			domainMockHandler(w, r)
		})
		stdout, _ := runRootOKDefaultJSON(t, "sprint", "active", "--board", "42")
		var payload []map[string]any
		decodeJSONOutput(t, stdout, &payload)
		if len(payload) != 0 {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("already closed sprint", func(t *testing.T) {
		mockJiraServer(t, domainMockHandler)
		stdout, _ := runRootOKDefaultJSON(t, "sprint", "close", "--sprint", "99")
		var payload map[string]any
		decodeJSONOutput(t, stdout, &payload)
		if payload["changed"] != false || payload["state"] != "closed" {
			t.Fatalf("payload=%v", payload)
		}
	})
}

func TestOutputFormat_DefaultJSONNonzeroPathsStayStructured(t *testing.T) {
	t.Run("transition unavailable", func(t *testing.T) {
		setupMockJira(t, issueMockHandler(t))
		stdout, stderr, err := runRootDefaultJSON(t, "issue", "transition", "PROJ-NOTRANS", "In Progress")
		if !errors.Is(err, ErrSilent) || LastExitCode() != ExitBadArgs {
			t.Fatalf("err=%v exit=%d", err, LastExitCode())
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("stdout should be empty, got %q", stdout)
		}
		var payload map[string]any
		decodeJSONOutput(t, stderr, &payload)
		if payload["errorCode"] != string(output.ErrValidation) {
			t.Fatalf("payload=%v", payload)
		}
	})

	t.Run("delete cancelled", func(t *testing.T) {
		setupMockJira(t, issueMockHandler(t))
		resetCLIState(t)
		stdout, stderr, err := runRootWithStdin(t, "WRONG\n", "issue", "delete", sampleIssueKey)
		if !errors.Is(err, ErrSilent) || LastExitCode() != ExitBadArgs {
			t.Fatalf("err=%v exit=%d", err, LastExitCode())
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("stdout should be empty, got %q", stdout)
		}
		var payload map[string]any
		decodeJSONSuffix(t, stderr, &payload)
		if payload["errorCode"] != string(output.ErrValidation) {
			t.Fatalf("payload=%v", payload)
		}
	})
}

func runRootOKDefaultJSON(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	resetCLIState(t)
	return runRootOK(t, args...)
}

func runRootDefaultJSON(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetCLIState(t)
	return runRoot(t, args...)
}

func decodeJSONOutput(t *testing.T, out string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
}

func decodeJSONSuffix(t *testing.T, out string, v any) {
	t.Helper()
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no JSON object in output: %q", out)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out[idx:])), v); err != nil {
		t.Fatalf("invalid JSON suffix: %v\n%s", err, out)
	}
}
