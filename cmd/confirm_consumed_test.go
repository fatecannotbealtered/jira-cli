package cmd

import (
	"strings"
	"testing"
)

// A confirm token may drive exactly one write; replaying it (e.g. an agent
// retrying a confirmed write that timed out) must be rejected with E_CONFLICT so
// the operation cannot be duplicated.
func TestConfirmTokenSingleUse(t *testing.T) {
	mockJiraServer(t, issueMockHandler(t))

	args := []string{"--json", "issue", "create", "--project", "PROJ", "--summary", "Single use", "--type", "Task"}
	token := dryRunConfirmToken(t, args...)

	resetCLIState(t)
	confirmArgs := append([]string{"--confirm", token}, args...)
	if _, _, err := runRoot(t, confirmArgs...); err != nil {
		t.Fatalf("first confirm should succeed, got %v (exit %d)", err, LastExitCode())
	}

	resetCLIState(t)
	stdout, _ := runRootExpectSilent(t, ExitConflict, confirmArgs...)
	if !strings.Contains(stdout, "E_CONFLICT") || !strings.Contains(stdout, "already used") {
		t.Fatalf("replayed token should be rejected as already-used E_CONFLICT, got: %s", stdout)
	}
}

func TestTokenFingerprintStable(t *testing.T) {
	a := tokenFingerprint("ct_123_abc")
	b := tokenFingerprint("ct_123_abc")
	c := tokenFingerprint("ct_123_abd")
	if a != b {
		t.Fatal("fingerprint not stable for the same token")
	}
	if a == c {
		t.Fatal("fingerprint collided for different tokens")
	}
	if len(a) != 16 {
		t.Fatalf("fingerprint length = %d, want 16", len(a))
	}
}
