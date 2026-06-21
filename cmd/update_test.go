package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

func resetUpdateState(t *testing.T) {
	t.Helper()
	oldVersion := version
	oldClient := updateHTTPClient
	oldBaseURL := updateBaseURL
	oldExecutable := updateExecutable
	oldGOOS := updateGOOS
	oldGOARCH := updateGOARCH
	oldGetenv := updateGetenv
	oldSkillSync := updateSkillSync
	oldVerifySig := updateVerifySignature
	t.Cleanup(func() {
		version = oldVersion
		updateHTTPClient = oldClient
		updateBaseURL = oldBaseURL
		updateExecutable = oldExecutable
		updateGOOS = oldGOOS
		updateGOARCH = oldGOARCH
		updateGetenv = oldGetenv
		updateSkillSync = oldSkillSync
		updateVerifySignature = oldVerifySig
	})
	version = "1.0.0"
	updateGOOS = func() string { return "windows" }
	updateGOARCH = func() string { return "amd64" }
	updateGetenv = func(string) string { return "" }
	updateSkillSync = func(context.Context, string) error { return nil }
	// In-process Sigstore verification is stubbed in tests; a live OIDC-signed
	// bundle cannot be produced in a unit test. Fail-closed control flow is
	// covered by overriding this with an error-returning stub.
	updateVerifySignature = func(_, _, _ string) error { return nil }
}

func newUpdateTestServer(t *testing.T, releaseVersion string, archive []byte) *httptest.Server {
	t.Helper()
	archiveName := updateArchiveName(releaseVersion, "windows", "amd64")
	sum := sha256.Sum256(archive)
	checksums := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest", "/releases/tags/v" + releaseVersion:
			_, _ = fmt.Fprintf(w, `{"tag_name":"v%s","assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q},{"name":"checksums.txt.sigstore.json","browser_download_url":%q}]}`,
				releaseVersion,
				archiveName,
				serverURL+"/assets/"+archiveName,
				serverURL+"/assets/checksums.txt",
				serverURL+"/assets/checksums.txt.sigstore.json",
			)
		case "/assets/" + archiveName:
			_, _ = w.Write(archive)
		case "/assets/checksums.txt":
			_, _ = fmt.Fprint(w, checksums)
		case "/assets/checksums.txt.sigstore.json":
			// Opaque bundle bytes; in-process verification is stubbed in tests.
			_, _ = fmt.Fprint(w, `{"bundle":"stub"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = ts.URL
	t.Cleanup(ts.Close)
	updateHTTPClient = ts.Client()
	updateBaseURL = ts.URL
	return ts
}

func makeUpdateZip(t *testing.T, binaryName string, contents []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(binaryName)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := f.Write(contents); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestUpdateCheckJSON(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("new-binary"))
	newUpdateTestServer(t, "1.2.3", archive)
	updateExecutable = func() (string, error) {
		return filepath.Join(t.TempDir(), "jira-cli.exe"), nil
	}

	stdout, _ := runRootOK(t, "--json", "update", "--check")
	var result updateResult
	decodeEnvelopeData(t, stdout, &result)
	if !result.UpdateAvailable || result.CurrentVersion != "1.0.0" || result.LatestVersion != "1.2.3" {
		t.Fatalf("result=%+v", result)
	}
	if !result.CheckOnly {
		t.Fatalf("checkOnly=false in %+v", result)
	}
}

func TestUpdatePackageManagerInstallRequiresForce(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("new-binary"))
	newUpdateTestServer(t, "1.2.3", archive)
	updateExecutable = func() (string, error) {
		return filepath.Join(t.TempDir(), "jira-cli.exe"), nil
	}
	updateGetenv = func(key string) string {
		if key == "JIRA_CLI_INSTALL_METHOD" {
			return "npm"
		}
		return ""
	}

	stdout, _ := runRootExpectSilent(t, ExitBadArgs, "update")
	if !strings.Contains(stdout, "npm install -g "+updatePackageName+"@latest") {
		t.Fatalf("expected npm update command in stdout, got %q", stdout)
	}
}

func TestUpdateInstallsRelease(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	want := []byte("new-binary")
	archive := makeUpdateZip(t, "jira-cli.exe", want)
	newUpdateTestServer(t, "1.2.3", archive)

	dir := t.TempDir()
	exePath := filepath.Join(dir, "jira-cli.exe")
	if err := os.WriteFile(exePath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write old binary: %v", err)
	}
	updateExecutable = func() (string, error) { return exePath, nil }

	// Bare `update` (no --confirm token, no --force) performs the whole update in
	// one call: the confirm gate has been removed from update.
	stdout, _ := runRootOK(t, "--json", "update")
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("binary=%q, want %q", got, want)
	}
	var result updateResult
	decodeEnvelopeData(t, stdout, &result)
	if !result.Installed || !result.ChecksumVerified || result.LatestVersion != "1.2.3" {
		t.Fatalf("result=%+v", result)
	}
	if result.SignatureStatus != "verified" || !result.SignatureVerified {
		t.Fatalf("signature not verified: %+v", result)
	}
	if result.SkillSyncStatus != "synced" || result.PreviousVersion != "1.0.0" {
		t.Fatalf("result=%+v", result)
	}
}

func TestUpdateDryRunDoesNotInstall(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("new-binary"))
	newUpdateTestServer(t, "1.2.3", archive)

	dir := t.TempDir()
	exePath := filepath.Join(dir, "jira-cli.exe")
	if err := os.WriteFile(exePath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write old binary: %v", err)
	}
	updateExecutable = func() (string, error) { return exePath, nil }

	stdout, _ := runRootOK(t, "--json", "--dry-run", "update")
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("dry-run changed binary to %q", got)
	}
	result := decodeUpdateDryRunResult(t, stdout)
	if !result.DryRun || result.Installed {
		t.Fatalf("result=%+v", result)
	}
}

func TestUpdateSpecificVersionCanDryRunDowngrade(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	version = "2.0.0"
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("older-binary"))
	newUpdateTestServer(t, "1.2.3", archive)

	dir := t.TempDir()
	exePath := filepath.Join(dir, "jira-cli.exe")
	if err := os.WriteFile(exePath, []byte("current-binary"), 0o755); err != nil {
		t.Fatalf("write current binary: %v", err)
	}
	updateExecutable = func() (string, error) { return exePath, nil }

	stdout, _ := runRootOK(t, "--json", "--dry-run", "update", "--version", "v1.2.3")
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != "current-binary" {
		t.Fatalf("dry-run changed binary to %q", got)
	}
	result := decodeUpdateDryRunResult(t, stdout)
	if !result.DryRun || result.UpdateAvailable || result.RequestedVersion != "1.2.3" {
		t.Fatalf("result=%+v", result)
	}
}

func decodeUpdateDryRunResult(t *testing.T, stdout string) updateResult {
	t.Helper()
	var data struct {
		Preview struct {
			Result updateResult `json:"result"`
		} `json:"preview"`
		ConfirmToken string `json:"confirm_token"`
		ExpiresAt    string `json:"expires_at"`
	}
	decodeEnvelopeData(t, stdout, &data)
	// update --dry-run is a read-only preview, not a confirm gate: it must issue
	// NO confirm_token and NO expires_at.
	if data.ConfirmToken != "" {
		t.Fatalf("dry-run must not issue a confirm_token, got %q in %s", data.ConfirmToken, stdout)
	}
	if data.ExpiresAt != "" {
		t.Fatalf("dry-run must not issue expires_at, got %q in %s", data.ExpiresAt, stdout)
	}
	return data.Preview.Result
}

func TestUpdatePlatformWindowsARM64Fallback(t *testing.T) {
	platform, arch, err := updatePlatform("windows", "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if platform != "windows" || arch != "amd64" {
		t.Fatalf("platform=%s arch=%s", platform, arch)
	}
}

func TestUpdateVersionCompare(t *testing.T) {
	if compareVersions("1.2.3", "1.2.4") >= 0 {
		t.Fatal("expected 1.2.3 < 1.2.4")
	}
	if compareVersions("v1.10.0", "1.9.9") <= 0 {
		t.Fatal("expected 1.10.0 > 1.9.9")
	}
	if compareVersions("1.2.0", "1.2") != 0 {
		t.Fatal("expected 1.2.0 == 1.2")
	}
}

func TestChecksumForArchive(t *testing.T) {
	sum, ok := checksumForArchive([]byte("abc123  jira-cli-1.0.0-linux-amd64.tar.gz\n"), "jira-cli-1.0.0-linux-amd64.tar.gz")
	if !ok || sum != "abc123" {
		t.Fatalf("sum=%q ok=%v", sum, ok)
	}
}

func TestManagerUpdateCommandVersionFormats(t *testing.T) {
	if got := managerUpdateCommand("npm", "v1.2.3"); got != "npm install -g "+updatePackageName+"@1.2.3" {
		t.Fatalf("npm command = %q", got)
	}
	if got := managerUpdateCommand("go", "1.2.3"); got != "go install github.com/"+updateRepo+"/cmd/jira-cli@v1.2.3" {
		t.Fatalf("go command = %q", got)
	}
}

func TestUpdateIdempotentNoOp(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	version = "1.2.3"
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("same-binary"))
	newUpdateTestServer(t, "1.2.3", archive)

	dir := t.TempDir()
	exePath := filepath.Join(dir, "jira-cli.exe")
	if err := os.WriteFile(exePath, []byte("current-binary"), 0o755); err != nil {
		t.Fatalf("write current binary: %v", err)
	}
	updateExecutable = func() (string, error) { return exePath, nil }

	// Already at latest: bare update is a no-op success, binary untouched.
	stdout, _ := runRootOK(t, "--json", "update")
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != "current-binary" {
		t.Fatalf("no-op update changed binary to %q", got)
	}
	var result updateResult
	decodeEnvelopeData(t, stdout, &result)
	if result.Installed || result.UpdateAvailable {
		t.Fatalf("expected no-op, got %+v", result)
	}
}

func TestUpdateIntegrityFailureNonRetryable(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("new-binary"))
	newUpdateTestServer(t, "1.2.3", archive)

	dir := t.TempDir()
	exePath := filepath.Join(dir, "jira-cli.exe")
	if err := os.WriteFile(exePath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write old binary: %v", err)
	}
	updateExecutable = func() (string, error) { return exePath, nil }
	updateVerifySignature = func(_, _, _ string) error { return errors.New("certificate identity mismatch") }

	stdout, _ := runRootExpectSilent(t, ExitGeneric, "--json", "update")
	errObj := decodeEnvelopeError(t, stdout)
	if errObj["code"] != string(output.ErrIntegrity) {
		t.Fatalf("code=%v want E_INTEGRITY", errObj["code"])
	}
	if errObj["retryable"] != false {
		t.Fatalf("integrity failure must be non-retryable, got %v", errObj["retryable"])
	}
	details, _ := errObj["details"].(map[string]any)
	if details["stage"] != updateStageVerifySignature {
		t.Fatalf("stage=%v want verify_signature", details["stage"])
	}
	if details["binary_replaced"] != false {
		t.Fatalf("binary_replaced=%v want false", details["binary_replaced"])
	}
	// Binary must be untouched after an integrity failure.
	if got, _ := os.ReadFile(exePath); string(got) != "old-binary" {
		t.Fatalf("integrity failure changed binary to %q", got)
	}
}

func TestUpdateSkillSyncFailureIsPartialSuccess(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)
	archive := makeUpdateZip(t, "jira-cli.exe", []byte("new-binary"))
	newUpdateTestServer(t, "1.2.3", archive)

	dir := t.TempDir()
	exePath := filepath.Join(dir, "jira-cli.exe")
	if err := os.WriteFile(exePath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write old binary: %v", err)
	}
	updateExecutable = func() (string, error) { return exePath, nil }
	updateSkillSync = func(context.Context, string) error { return errors.New("npx not found") }

	stdout, _ := runRootExpectSilent(t, ExitNetwork, "--json", "update")
	errObj := decodeEnvelopeError(t, stdout)
	if errObj["retryable"] != true {
		t.Fatalf("skill_sync failure must be retryable, got %v", errObj["retryable"])
	}
	details, _ := errObj["details"].(map[string]any)
	if details["stage"] != updateStageSkillSync {
		t.Fatalf("stage=%v want skill_sync", details["stage"])
	}
	if details["binary_replaced"] != true {
		t.Fatalf("binary_replaced=%v want true (binary already swapped)", details["binary_replaced"])
	}
	if details["skill_sync_status"] != "failed" {
		t.Fatalf("skill_sync_status=%v want failed", details["skill_sync_status"])
	}
	if details["skill_sync_command"] != updateSkillSyncCommand() {
		t.Fatalf("missing skill_sync_command, got %v", details["skill_sync_command"])
	}
	// Binary WAS replaced.
	if got, _ := os.ReadFile(exePath); string(got) != "new-binary" {
		t.Fatalf("binary=%q want new-binary", got)
	}
}

func TestUpdateErrorCodeExitMapping(t *testing.T) {
	cases := []struct {
		code output.ErrorCode
		exit int
	}{
		{output.ErrNetwork, ExitNetwork},
		{output.ErrTimeout, ExitTimeout},
		{output.ErrForbidden, ExitForbidden},
		{output.ErrIO, ExitGeneric},
		{output.ErrIntegrity, ExitGeneric},
		{output.ErrInterrupted, ExitInterrupted},
		{output.ErrValidation, ExitBadArgs},
	}
	for _, tc := range cases {
		if got := exitForUpdateCode(tc.code); got != tc.exit {
			t.Fatalf("exitForUpdateCode(%s)=%d want %d", tc.code, got, tc.exit)
		}
	}
	if ExitInterrupted != 130 {
		t.Fatalf("ExitInterrupted=%d want 130", ExitInterrupted)
	}
	if !output.RetryableForErrorCode(output.ErrInterrupted) {
		t.Fatal("E_INTERRUPTED must be retryable")
	}
	if output.RetryableForErrorCode(output.ErrIO) {
		t.Fatal("E_IO must be non-retryable")
	}
}

func TestClassifyReplaceError(t *testing.T) {
	if got := classifyReplaceError(os.ErrPermission); got != output.ErrForbidden {
		t.Fatalf("permission -> %s want E_FORBIDDEN", got)
	}
	if got := classifyReplaceError(errors.New("disk full")); got != output.ErrIO {
		t.Fatalf("io -> %s want E_IO", got)
	}
}

func TestVerifyChecksumSignature_FailClosed(t *testing.T) {
	resetCLIState(t)
	resetUpdateState(t)

	// No bundle in the release: refused, not skipped.
	if _, err := verifyChecksumSignature(context.Background(), []byte("sums"), githubReleaseAsset{}, false); err == nil {
		t.Fatal("missing signature bundle must be refused")
	} else if !strings.Contains(err.Error(), "unsigned release") {
		t.Fatalf("unexpected error for missing bundle: %v", err)
	}

	// Bundle present but verification fails: aborts.
	srv := newUpdateTestServer(t, "1.2.3", makeUpdateZip(t, "jira-cli.exe", []byte("b")))
	asset := githubReleaseAsset{Name: "checksums.txt.sigstore.json", BrowserDownloadURL: srv.URL + "/assets/checksums.txt.sigstore.json"}
	updateVerifySignature = func(_, _, _ string) error { return errors.New("certificate identity mismatch") }
	if _, err := verifyChecksumSignature(context.Background(), []byte("sums"), asset, true); err == nil {
		t.Fatal("signature verification failure must abort")
	}
}
