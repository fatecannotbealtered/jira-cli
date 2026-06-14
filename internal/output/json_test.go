package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

type marshalFail struct{}

func (marshalFail) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failed")
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w

	fn()

	_ = w.Close()
	os.Stderr = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestErrorCodeFromStatus(t *testing.T) {
	cases := []struct {
		code int
		want ErrorCode
	}{
		{401, ErrAuth},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{409, ErrConflict},
		{429, ErrRateLimit},
		{500, ErrServer},
		{503, ErrServer},
		{400, ErrValidation},
		{422, ErrValidation},
		{200, ErrUnknown},
		{0, ErrUnknown},
	}
	for _, tc := range cases {
		t.Run(string(rune(tc.code)), func(t *testing.T) {
			if got := ErrorCodeFromStatus(tc.code); got != tc.want {
				t.Errorf("ErrorCodeFromStatus(%d) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestHintForErrorCode(t *testing.T) {
	cases := []struct {
		code ErrorCode
		want string
	}{
		{ErrConfig, "Run 'jira-cli login' or set JIRA_HOST and JIRA_TOKEN environment variables"},
		{ErrAuth, "Check your PAT; regenerate in Jira DC Profile > Personal Access Tokens"},
		{ErrForbidden, "Check your PAT scope and project permissions"},
		{ErrNotFound, "Verify the resource key/ID exists and you have permission to view it"},
		{ErrRateLimit, "Wait and retry; reduce request frequency"},
		{ErrServer, "Jira server error; try again later"},
		{ErrValidation, "Check command arguments and flags"},
		{ErrNetwork, "Check host URL and network connectivity"},
		{ErrUnknown, ""},
		{ErrorCode("OTHER"), ""},
	}
	for _, tc := range cases {
		if got := HintForErrorCode(tc.code); got != tc.want {
			t.Errorf("HintForErrorCode(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestPrintJSON_MarshalError(t *testing.T) {
	out := captureStdout(t, func() {
		PrintJSON(marshalFail{})
	})
	if !strings.Contains(out, `"ok": false`) || !strings.Contains(out, "marshal failed") {
		t.Errorf("PrintJSON marshal error: got %q", out)
	}
}

func TestPrintErrorJSON(t *testing.T) {
	t.Run("with status", func(t *testing.T) {
		out := captureStdout(t, func() {
			PrintErrorJSON("not found", 404)
		})
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
			t.Fatalf("invalid JSON: %v\nout: %s", err, out)
		}
		errPayload := payload["error"].(map[string]any)
		if errPayload["message"] != "not found" {
			t.Errorf("message = %v", errPayload["message"])
		}
		if errPayload["code"] != string(ErrNotFound) {
			t.Errorf("code = %v, want %s", errPayload["code"], ErrNotFound)
		}
		details := errPayload["details"].(map[string]any)
		if details["hint"] == "" {
			t.Error("expected non-empty hint for 404")
		}
	})

	t.Run("status zero uses unknown", func(t *testing.T) {
		out := captureStdout(t, func() {
			PrintErrorJSON("oops", 0)
		})
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		errPayload := payload["error"].(map[string]any)
		if errPayload["code"] != string(ErrUnknown) {
			t.Errorf("code = %v, want %s", errPayload["code"], ErrUnknown)
		}
	})
}

func TestPrintErrorJSONWithCode(t *testing.T) {
	out := captureStdout(t, func() {
		PrintErrorJSONWithCode("network down", 0, ErrNetwork)
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(ErrNetwork) {
		t.Errorf("code = %v, want %s", errPayload["code"], ErrNetwork)
	}
	if errPayload["retryable"] != true {
		t.Errorf("retryable = %v", errPayload["retryable"])
	}
	details := errPayload["details"].(map[string]any)
	if details["hint"] != HintForErrorCode(ErrNetwork) {
		t.Errorf("hint = %v", details["hint"])
	}
}

func TestPrintErrorJSON_MarshalFallback(t *testing.T) {
	old := jsonMarshalIndent
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("indent failed")
	}
	t.Cleanup(func() { jsonMarshalIndent = old })

	out := captureStdout(t, func() {
		PrintErrorJSON("boom", 500)
	})
	if !strings.Contains(out, `"message":"boom"`) || !strings.Contains(out, string(ErrServer)) {
		t.Errorf("PrintErrorJSON fallback: got %q", out)
	}
}

func TestPrintErrorJSONWithCode_MarshalFallback(t *testing.T) {
	old := jsonMarshalIndent
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("indent failed")
	}
	t.Cleanup(func() { jsonMarshalIndent = old })

	out := captureStdout(t, func() {
		PrintErrorJSONWithCode("bad", 0, ErrConfig)
	})
	if !strings.Contains(out, `"message":"bad"`) || !strings.Contains(out, string(ErrConfig)) {
		t.Errorf("PrintErrorJSONWithCode fallback: got %q", out)
	}
}
