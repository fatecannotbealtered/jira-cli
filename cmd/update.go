package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	updateRepo              = "fatecannotbealtered/jira-cli"
	updateAPIBase           = "https://api.github.com/repos/" + updateRepo
	updatePackageName       = "@fateforge/jira-cli"
	updateBinaryName        = "jira-cli"
	updateSkillRepo         = updateRepo
	maxReleaseJSONBytes     = 5 << 20
	maxChecksumFileBytes    = 1 << 20
	maxSignatureBundleBytes = 1 << 20
	maxArchiveBytes         = 100 << 20
	maxExtractedBinaryLen   = 100 << 20
)

var (
	updateHTTPClient        = &http.Client{Timeout: 30 * time.Second}
	updateBaseURL           = updateAPIBase
	updateExecutable        = os.Executable
	updateGOOS              = func() string { return runtime.GOOS }
	updateGOARCH            = func() string { return runtime.GOARCH }
	updateGetenv            = os.Getenv
	updateSkillSync         = runUpdateSkillSync
	updateRunPackageManager = runPackageManagerInstall
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update jira-cli to a GitHub release",
	Long: `Update jira-cli by downloading the matching GitHub Release asset,
verifying the Sigstore signature on checksums.txt in-process against this repo's
tagged release workflow identity, verifying the archive checksum, and replacing
the current standalone binary. An unsigned or unverifiable release is refused;
there is no skip path.

Package-manager installs are detected where possible. For npm installs, use
npm install -g @fateforge/jira-cli@latest unless --force is set.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().Bool("check", false, "Check whether an update is available without installing")
	updateCmd.Flags().String("target-version", "", "Install a specific release version (for example v1.2.3)")
	// --version is kept as a hidden, deprecated alias for --target-version. The
	// canonical flag is --target-version so the target-release selector is never
	// confused with the root command's --version (which prints the tool version).
	updateCmd.Flags().String("version", "", "Deprecated alias for --target-version")
	_ = updateCmd.Flags().MarkHidden("version")
	rootCmd.AddCommand(updateCmd)
}

// updateTargetVersion resolves the requested target release, preferring the
// canonical --target-version and falling back to the deprecated --version alias.
func updateTargetVersion(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("target-version"); strings.TrimSpace(v) != "" {
		return v
	}
	v, _ := cmd.Flags().GetString("version")
	return v
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type updateResult struct {
	// current_version is the running version before an install and the newly
	// installed version afterwards; previous_version is set on install.
	Status            string         `json:"status,omitempty"`
	CurrentVersion    string         `json:"current_version"`
	TargetVersion     string         `json:"target_version"`
	RequestedVersion  string         `json:"requested_version,omitempty"`
	PreviousVersion   string         `json:"previous_version,omitempty"`
	KnowledgeRefresh  string         `json:"knowledge_refresh,omitempty"`
	UpdateAvailable   bool           `json:"update_available"`
	Installed         bool           `json:"installed,omitempty"`
	CheckOnly         bool           `json:"check_only,omitempty"`
	DryRun            bool           `json:"dry_run,omitempty"`
	InstallMethod     string         `json:"install_method,omitempty"`
	Command           string         `json:"command,omitempty"`
	Asset             string         `json:"asset,omitempty"`
	Path              string         `json:"path,omitempty"`
	ChecksumVerified  bool           `json:"checksum_verified,omitempty"`
	SignatureStatus   string         `json:"signature_status,omitempty"`
	SignatureVerified bool           `json:"signature_verified,omitempty"`
	SkillSyncCommand  string         `json:"skill_sync_command,omitempty"`
	SkillSyncStatus   string         `json:"skill_sync_status,omitempty"`
	Notices           []updateNotice `json:"notices,omitempty"`
}

// Update stages, in execution order. Every update failure/interruption envelope
// names the stage it failed in so an agent can reason about the post-state.
const (
	updateStageDiscover        = "discover"
	updateStageDownload        = "download"
	updateStageVerifySignature = "verify_signature"
	updateStageVerifyChecksum  = "verify_checksum"
	updateStageReplace         = "replace"
	updateStageSkillSync       = "skill_sync"
)

// updateState carries the post-failure invariant: the version actually running
// now, whether the atomic binary swap committed, and where Skill sync stands.
// It is threaded through every update failure path so messages can never lie
// about the tool's real state.
type updateState struct {
	stage           string
	currentVersion  string
	binaryReplaced  bool
	skillSyncStatus string
	skillSyncCmd    string
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")
	targetVersion := updateTargetVersion(cmd)
	requestedSpecific := strings.TrimSpace(targetVersion) != ""

	currentVersion := version
	// st tracks the honest post-state at all times; it starts before the swap.
	st := &updateState{
		stage:           updateStageDiscover,
		currentVersion:  normalizeVersion(currentVersion),
		binaryReplaced:  false,
		skillSyncStatus: "not_run",
		skillSyncCmd:    updateSkillSyncCommand(),
	}

	// SIGINT/SIGTERM trap: an interrupted self-update must still hand the agent a
	// parseable terminal envelope (E_INTERRUPTED, exit 130), never die as a bare
	// killed process. The staged-work invariant makes the message honest. Temp
	// dirs are always cleaned by their own defers as the stack unwinds.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	release, err := fetchUpdateRelease(ctx, targetVersion)
	if err != nil {
		return handleUpdateError(ctx, err, classifyDiscoverError(err), st)
	}
	latestVersion := normalizeVersion(release.TagName)
	if latestVersion == "" {
		return handleUpdateError(ctx, fmt.Errorf("release is missing tag_name"), output.ErrNetwork, st)
	}

	platform, arch, err := updatePlatform(updateGOOS(), updateGOARCH())
	if err != nil {
		return handleUpdateError(ctx, err, output.ErrValidation, st)
	}
	archiveName := updateArchiveName(latestVersion, platform, arch)
	archiveAsset, ok := release.assetByName(archiveName)
	if !ok {
		return handleUpdateError(ctx, fmt.Errorf("release %s has no asset for %s-%s (%s)", release.TagName, platform, arch, archiveName), output.ErrValidation, st)
	}
	checksumAsset, ok := release.assetByName("checksums.txt")
	if !ok {
		return handleUpdateError(ctx, fmt.Errorf("release %s has no checksums.txt asset", release.TagName), output.ErrNetwork, st)
	}
	signatureBundleAsset, signatureBundleFound := release.assetByName("checksums.txt.sigstore.json")

	available := updateAvailable(currentVersion, latestVersion)
	exePath, err := updateExecutable()
	if err != nil {
		return handleUpdateError(ctx, fmt.Errorf("locating current executable: %w", err), output.ErrValidation, st)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	installMethod := detectInstallMethod(exePath)
	result := updateResult{
		CurrentVersion:   currentVersion,
		TargetVersion:    latestVersion,
		RequestedVersion: requestedVersionField(targetVersion),
		UpdateAvailable:  available,
		CheckOnly:        checkOnly,
		InstallMethod:    installMethod,
		Asset:            archiveName,
		Path:             exePath,
		SignatureStatus:  "not_checked",
		SkillSyncCommand: updateSkillSyncCommand(),
		SkillSyncStatus:  "not_run",
	}
	if installMethod != "" {
		result.Command = managerUpdateCommand(installMethod, latestVersion)
	}
	if checkOnly {
		result.Status = "checked"
		result.Notices = updateNoticesFromResult(result, "update_check")
	}

	if checkOnly {
		printUpdateResult(result)
		return nil
	}
	if dryRun {
		// Read-only preview only: no confirm token, no expires_at — update is a
		// single command, not a confirm-gated write. Placed BEFORE the npm gate so
		// the preview is always reachable: a package-managed install must still be
		// able to show what `update` would do without being short-circuited into
		// the "use your package manager" error.
		result.DryRun = true
		result.Status = "dry_run"
		printUpdateDryRunResult(result)
		return nil
	}
	if installMethod != "" && !forceMode {
		return runPackageManagerUpdate(ctx, result, installMethod, latestVersion)
	}
	if !available && !requestedSpecific && !forceMode {
		// Idempotent no-op: already on the latest (or requested) version.
		result.Status = "up_to_date"
		printUpdateResult(result)
		return nil
	}

	st.stage = updateStageDownload
	archiveData, err := downloadUpdateURL(ctx, archiveAsset.BrowserDownloadURL, maxArchiveBytes)
	if err != nil {
		return handleUpdateError(ctx, err, classifyDiscoverError(err), st)
	}
	checksumData, err := downloadUpdateURL(ctx, checksumAsset.BrowserDownloadURL, maxChecksumFileBytes)
	if err != nil {
		return handleUpdateError(ctx, err, classifyDiscoverError(err), st)
	}

	st.stage = updateStageVerifySignature
	signatureStatus, err := verifyChecksumSignature(ctx, checksumData, signatureBundleAsset, signatureBundleFound)
	if err != nil {
		// Distinguish "could not fetch the signature bundle" from "the signature is
		// bad". A failed bundle download (5xx / reset / DNS / rate-limit) is a
		// transient network problem the agent SHOULD retry — not a supply-chain
		// red flag. Only a present-but-invalid signature (or a deliberately missing
		// bundle) is the non-retryable E_INTEGRITY case. An interrupt mid-download
		// still surfaces as E_INTERRUPTED via handleUpdateError's ctx check.
		var dlErr *signatureDownloadError
		if errors.As(err, &dlErr) {
			return handleUpdateError(ctx, fmt.Errorf("downloading release signature: %w", dlErr.err), classifyDiscoverError(dlErr.err), st)
		}
		// Integrity failure is non-retryable: a missing or invalid signature is
		// a supply-chain red flag, not a transient blip an agent should retry.
		return handleUpdateError(ctx, fmt.Errorf("verifying release signature: %w", err), output.ErrIntegrity, st)
	}

	st.stage = updateStageVerifyChecksum
	if err := verifyArchiveChecksum(archiveName, archiveData, checksumData); err != nil {
		return handleUpdateError(ctx, err, output.ErrIntegrity, st)
	}
	binaryData, err := extractBinaryFromArchive(archiveName, archiveData, binaryNameForPlatform(platform))
	if err != nil {
		return handleUpdateError(ctx, err, output.ErrIntegrity, st)
	}

	st.stage = updateStageReplace
	if err := replaceExecutable(exePath, binaryData); err != nil {
		// Local filesystem/permission failure — the atomic swap did not commit,
		// so the old binary is intact. Classify by next action (permission vs IO),
		// NOT as a network blip.
		return handleUpdateError(ctx, err, classifyReplaceError(err), st)
	}
	// Atomic swap committed: the tool is now on the new version.
	st.binaryReplaced = true
	st.currentVersion = latestVersion

	st.stage = updateStageSkillSync
	if err := updateSkillSync(ctx, updateSkillRepo); err != nil {
		// Post-swap: the binary already updated. This is partial success, not a
		// hard failure — tell the agent it is on the new binary and just needs to
		// run skill_sync_command.
		st.skillSyncStatus = "failed"
		return handleSkillSyncPartial(ctx, currentVersion, latestVersion, signatureStatus, err, st)
	}

	result.Installed = true
	result.ChecksumVerified = true
	result.SignatureStatus = signatureStatus
	result.SignatureVerified = signatureStatus == "verified"
	result.PreviousVersion = currentVersion
	result.CurrentVersion = latestVersion
	result.SkillSyncStatus = "synced"
	result.KnowledgeRefresh = fmt.Sprintf("run \"jira-cli changelog --since %s\" before continuing", normalizeVersion(currentVersion))
	printUpdateResult(result)
	return nil
}

// updateFailureDetails renders the staged-failure envelope fields every update
// error (and the interrupt path) must carry.
func (st *updateState) details() map[string]any {
	return map[string]any{
		"stage":             st.stage,
		"current_version":   st.currentVersion,
		"binary_replaced":   st.binaryReplaced,
		"skill_sync_status": st.skillSyncStatus,
	}
}

// handleUpdateError emits the staged-failure envelope. If the context was
// cancelled by a trapped signal, it is reclassified as E_INTERRUPTED (exit 130)
// so the agent receives a parseable terminal state rather than a killed process.
func handleUpdateError(ctx context.Context, err error, code output.ErrorCode, st *updateState) error {
	if interrupted(ctx) {
		return emitInterrupted(st)
	}
	if jsonMode {
		output.PrintErrorJSONWithDetails(err.Error(), 0, code, st.details())
	} else {
		output.Error(err.Error())
	}
	return SilentErr(exitForUpdateCode(code))
}

// handleSkillSyncPartial reports the post-swap Skill-sync failure as partial
// success: ok:false but binary_replaced:true, retryable, with the command the
// agent must run to finish.
func handleSkillSyncPartial(ctx context.Context, prev, latest, signatureStatus string, syncErr error, st *updateState) error {
	if interrupted(ctx) {
		return emitInterrupted(st)
	}
	msg := fmt.Sprintf("binary updated to %s but skill sync failed: %v; run %q, then \"jira-cli changelog --since %s\"",
		latest, syncErr, st.skillSyncCmd, normalizeVersion(prev))
	if jsonMode {
		details := st.details()
		details["binary_replaced"] = true
		details["skill_sync_command"] = st.skillSyncCmd
		details["previous_version"] = normalizeVersion(prev)
		details["signature_status"] = signatureStatus
		output.PrintErrorJSONWithDetails(msg, 0, output.ErrNetwork, details)
	} else {
		output.Error(msg)
	}
	return SilentErr(ExitNetwork)
}

func interrupted(ctx context.Context) bool {
	return ctx != nil && errors.Is(ctx.Err(), context.Canceled)
}

// emitInterrupted writes the terminal E_INTERRUPTED envelope (exit 130). The
// message states the real post-state per the stage invariant.
func emitInterrupted(st *updateState) error {
	var msg string
	switch {
	case st.binaryReplaced:
		msg = fmt.Sprintf("update interrupted during skill sync; binary is at %s, run %q to finish", st.currentVersion, st.skillSyncCmd)
	default:
		msg = fmt.Sprintf("update cancelled; no change, still on %s", st.currentVersion)
	}
	if jsonMode {
		output.PrintErrorJSONWithDetails(msg, 0, output.ErrInterrupted, st.details())
	} else {
		output.Error(msg)
	}
	return SilentErr(ExitInterrupted)
}

// classifyDiscoverError maps a discover/download transport failure onto the
// retryable network taxonomy. When the upstream returned an HTTP status, it is
// mapped by status TYPE through the single §6 ErrorCodeFromStatus function
// (404 -> E_NOT_FOUND, 429 -> E_RATE_LIMITED, 5xx -> E_SERVER, …) rather than
// collapsing every non-2xx into E_NETWORK. Transport-level failures (DNS,
// connection reset/refused) have no status and stay E_NETWORK.
func classifyDiscoverError(err error) output.ErrorCode {
	if err == nil {
		return output.ErrNetwork
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return output.ErrTimeout
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return output.ErrorCodeFromStatus(statusErr.statusCode)
	}
	return output.ErrNetwork
}

// classifyReplaceError maps a local replace-stage failure to E_FORBIDDEN
// (permission) or E_IO (disk/io). These were historically misclassified as
// E_NETWORK; the binary swap is a local filesystem operation.
func classifyReplaceError(err error) output.ErrorCode {
	if err != nil && errors.Is(err, os.ErrPermission) {
		return output.ErrForbidden
	}
	return output.ErrIO
}

func exitForUpdateCode(code output.ErrorCode) int {
	return output.ExitCodeForErrorCode(code)
}

// runPackageManagerUpdate handles `update` for a package-manager-managed install
// (npm or Go). The tool DRIVES the package manager — it runs the install command
// on the user's behalf, then syncs the Skill. Integrity on this path is the
// package manager's own, so signature_status stays "not_checked". The new version
// takes effect on the next invocation (this process is still the old image).
func runPackageManagerUpdate(ctx context.Context, result updateResult, method, targetVersion string) error {
	if err := updateRunPackageManager(ctx, method, targetVersion); err != nil {
		// The package manager owns download/integrity/replace; a failure here
		// leaves the installed binary unchanged (binary_replaced:false).
		msg := fmt.Sprintf("package-manager update failed: %s — run %q manually", strings.TrimSpace(err.Error()), result.Command)
		details := map[string]any{
			"stage":             updateStageReplace,
			"current_version":   result.CurrentVersion,
			"binary_replaced":   false,
			"skill_sync_status": "not_run",
			"install_method":    method,
			"command":           result.Command,
		}
		if jsonMode {
			output.PrintErrorJSONWithDetails(msg, 0, output.ErrIO, details)
			setExitCode(exitForUpdateCode(output.ErrIO))
			return ErrSilent
		}
		output.Error(msg)
		return ErrSilent
	}

	result.PreviousVersion = result.CurrentVersion
	result.CurrentVersion = result.TargetVersion
	result.Installed = true
	result.SignatureStatus = "not_checked"

	if err := updateSkillSync(ctx, updateSkillRepo); err != nil {
		result.SkillSyncStatus = "failed"
		if jsonMode {
			output.PrintJSON(result)
			return nil
		}
		output.Warn(fmt.Sprintf("updated jira-cli to %s via %s, but Skill sync failed: %s — run %q", result.CurrentVersion, method, err.Error(), result.SkillSyncCommand))
		return nil
	}
	result.SkillSyncStatus = "synced"
	printUpdateResult(result)
	return nil
}

func printUpdateResult(result updateResult) {
	if jsonMode {
		output.PrintJSON(result)
		return
	}
	if result.Installed {
		if result.UpdateAvailable {
			output.Success(fmt.Sprintf("Updated jira-cli from %s to %s", result.PreviousVersion, result.CurrentVersion))
		} else {
			output.Success(fmt.Sprintf("Installed jira-cli %s over %s", result.CurrentVersion, result.PreviousVersion))
		}
		return
	}
	if result.DryRun {
		if result.UpdateAvailable || result.RequestedVersion != "" {
			output.Info(fmt.Sprintf("[dry-run] would install jira-cli %s over %s", result.TargetVersion, result.CurrentVersion))
		} else {
			output.Info(fmt.Sprintf("[dry-run] jira-cli is already at %s", result.CurrentVersion))
		}
		return
	}
	if result.CheckOnly {
		if result.UpdateAvailable {
			output.Info(fmt.Sprintf("Update available: %s -> %s", result.CurrentVersion, result.TargetVersion))
		} else {
			output.Success(fmt.Sprintf("jira-cli is up to date (%s)", result.CurrentVersion))
		}
		return
	}
	if result.UpdateAvailable {
		output.Info(fmt.Sprintf("Update available: %s -> %s", result.CurrentVersion, result.TargetVersion))
	} else {
		output.Success(fmt.Sprintf("jira-cli is already up to date (%s)", result.CurrentVersion))
	}
}

func printUpdateDryRunResult(result updateResult) {
	if jsonMode {
		// Read-only preview of the plan. Issues NO confirm_token and NO
		// expires_at: update is a single command, never a confirm-gated write.
		output.PrintJSON(map[string]any{
			"preview": map[string]any{
				"action": "update jira-cli",
				"changes": []map[string]any{
					{"operation": "replace executable", "target": result.Path},
					{"operation": "sync skill directory", "command": result.SkillSyncCommand},
				},
				"result": result,
			},
		})
		return
	}
	printUpdateResult(result)
}

func updateSkillSyncCommand() string {
	return "npx skills add " + updateSkillRepo + " -y -g"
}

func runUpdateSkillSync(ctx context.Context, repo string) error {
	command := exec.CommandContext(ctx, "npx", "skills", "add", repo, "-y", "-g")
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(outputBytes))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, truncateForUpdateError(msg, 300))
		}
		return err
	}
	return nil
}

func truncateForUpdateError(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// verifyChecksumSignature enforces a mandatory, in-process Sigstore signature
// check on checksums.txt before the release is trusted. There is no skip path: a
// release without a signature bundle, or one whose signature does not verify
// against this repo's release-workflow identity, is refused. The returned status
// is always "verified" on the nil-error path.
func verifyChecksumSignature(ctx context.Context, checksumData []byte, bundleAsset githubReleaseAsset, bundleFound bool) (string, error) {
	if !bundleFound {
		return "missing", errors.New("release does not include checksums.txt.sigstore.json; refusing to install an unsigned release")
	}

	bundleData, err := downloadUpdateURL(ctx, bundleAsset.BrowserDownloadURL, maxSignatureBundleBytes)
	if err != nil {
		// A bundle that cannot be fetched is a transient network failure, NOT an
		// integrity verdict on the release. Tag it so the caller classifies it on
		// the retryable network taxonomy instead of E_INTEGRITY.
		return "download_failed", &signatureDownloadError{err: err}
	}
	tmpDir, err := os.MkdirTemp("", "jira-cli-signature-*")
	if err != nil {
		return "failed", fmt.Errorf("creating signature temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	bundlePath := filepath.Join(tmpDir, "checksums.txt.sigstore.json")
	if err := os.WriteFile(checksumPath, checksumData, 0o600); err != nil {
		return "failed", fmt.Errorf("writing checksum temp file: %w", err)
	}
	if err := os.WriteFile(bundlePath, bundleData, 0o600); err != nil {
		return "failed", fmt.Errorf("writing checksum signature bundle: %w", err)
	}

	if err := updateVerifySignature(ctx, checksumPath, bundlePath, updateSignerIdentityRegexp()); err != nil {
		if errors.Is(err, errTrustRootUnavailable) {
			// Refreshing the TUF trust metadata is a network step, not a signature
			// verdict: surface it as a retryable network failure, not E_INTEGRITY.
			return "trust_root_unavailable", &signatureDownloadError{err: err}
		}
		return "failed", err
	}
	return "verified", nil
}

func fetchUpdateRelease(ctx context.Context, targetVersion string) (*githubRelease, error) {
	path := "/releases/latest"
	if strings.TrimSpace(targetVersion) != "" {
		path = "/releases/tags/" + normalizeReleaseTag(targetVersion)
	}
	data, err := downloadUpdateURL(ctx, strings.TrimRight(updateBaseURL, "/")+path, maxReleaseJSONBytes)
	if err != nil {
		return nil, err
	}
	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("parsing GitHub release response: %w", err)
	}
	return &release, nil
}

// httpStatusError carries the upstream HTTP status so the failure can be mapped
// onto the §6 status->code taxonomy (404 != 5xx != 429) instead of collapsing
// every non-2xx into a single class. Transport-level failures (DNS/reset/refused)
// have no status and surface as plain errors classified by classifyDiscoverError.
type httpStatusError struct {
	statusCode int
	status     string
	url        string
	body       string
}

func (e *httpStatusError) Error() string {
	msg := e.body
	if msg == "" {
		msg = e.status
	}
	return fmt.Sprintf("GitHub returned %s: %s", e.status, msg)
}

// signatureDownloadError marks a failure to FETCH the signature bundle (network
// transport / HTTP status), as opposed to a signature that was fetched but did
// not verify. The former is retryable network; the latter is non-retryable
// E_INTEGRITY. Keeping them distinct stops an agent from looping on E_INTEGRITY
// when the real problem was a 5xx on the bundle URL.
type signatureDownloadError struct{ err error }

func (e *signatureDownloadError) Error() string { return e.err.Error() }
func (e *signatureDownloadError) Unwrap() error { return e.err }

func downloadUpdateURL(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", defaultUpdateUserAgent())
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if readErr != nil {
		return nil, fmt.Errorf("reading %s: %w", rawURL, readErr)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("download from %s exceeded %d bytes", rawURL, limit)
	}
	if resp.StatusCode >= 400 {
		return nil, &httpStatusError{
			statusCode: resp.StatusCode,
			status:     resp.Status,
			url:        rawURL,
			body:       strings.TrimSpace(string(data)),
		}
	}
	return data, nil
}

func defaultUpdateUserAgent() string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	return "jira-cli/" + v
}

func (r *githubRelease) assetByName(name string) (githubReleaseAsset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return githubReleaseAsset{}, false
}

func updatePlatform(goos, goarch string) (string, string, error) {
	switch goos {
	case "darwin", "linux", "windows":
	default:
		return "", "", fmt.Errorf("unsupported platform %s-%s", goos, goarch)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", "", fmt.Errorf("unsupported platform %s-%s", goos, goarch)
	}
	if goos == "windows" && goarch == "arm64" {
		goarch = "amd64"
	}
	return goos, goarch, nil
}

func updateArchiveName(version, platform, arch string) string {
	ext := ".tar.gz"
	if platform == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s-%s-%s-%s%s", updateBinaryName, normalizeVersion(version), platform, arch, ext)
}

func binaryNameForPlatform(platform string) string {
	if platform == "windows" {
		return updateBinaryName + ".exe"
	}
	return updateBinaryName
}

func verifyArchiveChecksum(archiveName string, archiveData, checksumData []byte) error {
	expected, ok := checksumForArchive(checksumData, archiveName)
	if !ok {
		return fmt.Errorf("checksums.txt does not include %s", archiveName)
	}
	sum := sha256.Sum256(archiveData)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	return nil
}

func checksumForArchive(data []byte, archiveName string) (string, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] == archiveName {
			return fields[0], true
		}
	}
	return "", false
}

func extractBinaryFromArchive(archiveName string, data []byte, binaryName string) ([]byte, error) {
	switch {
	case strings.HasSuffix(archiveName, ".zip"):
		return extractBinaryFromZip(data, binaryName)
	case strings.HasSuffix(archiveName, ".tar.gz"):
		return extractBinaryFromTarGz(data, binaryName)
	default:
		return nil, fmt.Errorf("unsupported archive format %s", archiveName)
	}
}

func extractBinaryFromZip(data []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening zip archive: %w", err)
	}
	for _, f := range zr.File {
		if pathpkg.Base(f.Name) != binaryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s in zip archive: %w", binaryName, err)
		}
		defer func() { _ = rc.Close() }()
		return readLimitedBinary(rc, binaryName)
	}
	return nil, fmt.Errorf("%s not found in zip archive", binaryName)
}

func extractBinaryFromTarGz(data []byte, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("opening tar.gz archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar.gz archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || pathpkg.Base(header.Name) != binaryName {
			continue
		}
		return readLimitedBinary(tr, binaryName)
	}
	return nil, fmt.Errorf("%s not found in tar.gz archive", binaryName)
}

func readLimitedBinary(r io.Reader, name string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxExtractedBinaryLen+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	if len(data) > maxExtractedBinaryLen {
		return nil, fmt.Errorf("%s exceeds %d bytes", name, maxExtractedBinaryLen)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", name)
	}
	return data, nil
}

func replaceExecutable(exePath string, binaryData []byte) error {
	target := exePath
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		target = resolved
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat executable %s: %w", target, err)
	}
	mode := info.Mode()
	if mode.Perm() == 0 {
		mode = 0o755
	}
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	newPath := filepath.Join(dir, "."+base+".new")
	backupPath := filepath.Join(dir, "."+base+".old")

	_ = os.Remove(newPath)
	if err := os.WriteFile(newPath, binaryData, mode.Perm()); err != nil {
		return fmt.Errorf("writing replacement binary %s: %w", newPath, err)
	}
	if err := os.Chmod(newPath, mode.Perm()); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("setting executable mode on %s: %w", newPath, err)
	}

	_ = os.Remove(backupPath)
	if err := os.Rename(target, backupPath); err != nil {
		return fmt.Errorf("preparing to replace %s: %w; replacement left at %s", target, err, newPath)
	}
	if err := os.Rename(newPath, target); err != nil {
		_ = os.Rename(backupPath, target)
		return fmt.Errorf("replacing %s: %w; original restored", target, err)
	}
	_ = os.Remove(backupPath)
	return nil
}

func detectInstallMethod(exePath string) string {
	if method := strings.TrimSpace(updateGetenv("JIRA_CLI_INSTALL_METHOD")); method != "" {
		return strings.ToLower(method)
	}
	normalized := filepath.ToSlash(strings.ToLower(exePath))
	if strings.Contains(normalized, "/node_modules/") && strings.Contains(normalized, "jira-cli") {
		return "npm"
	}
	return ""
}

func managerUpdateCommand(method, targetVersion string) string {
	npmVersion := "latest"
	goVersion := "latest"
	if strings.TrimSpace(targetVersion) != "" {
		npmVersion = normalizeVersion(targetVersion)
		goVersion = normalizeReleaseTag(targetVersion)
	}
	switch strings.ToLower(method) {
	case "npm":
		return "npm install -g " + updatePackageName + "@" + npmVersion
	case "go":
		return "go install github.com/" + updateRepo + "/cmd/jira-cli@" + goVersion
	default:
		return ""
	}
}

func updateAvailable(current, latest string) bool {
	current = normalizeVersion(current)
	latest = normalizeVersion(latest)
	if latest == "" {
		return false
	}
	if current == "" || current == "dev" {
		return true
	}
	return compareVersions(current, latest) < 0
}

func normalizeReleaseTag(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

func requestedVersionField(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return normalizeVersion(v)
}

func compareVersions(a, b string) int {
	aa := parseVersionParts(a)
	bb := parseVersionParts(b)
	for i := 0; i < len(aa) || i < len(bb); i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func parseVersionParts(v string) []int {
	base := strings.SplitN(normalizeVersion(v), "-", 2)[0]
	fields := strings.Split(base, ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, n)
	}
	return parts
}

// runPackageManagerInstall drives the package manager to install the target
// version. argv is built directly (no shell) so the version string cannot be
// reinterpreted by a shell.
func runPackageManagerInstall(ctx context.Context, method, targetVersion string) error {
	var name string
	var args []string
	switch strings.ToLower(method) {
	case "npm":
		ver := normalizeVersion(targetVersion)
		if ver == "" {
			ver = "latest"
		}
		name = "npm"
		args = []string{"install", "-g", updatePackageName + "@" + ver}
	case "go":
		name = "go"
		args = []string{"install", "github.com/" + updateRepo + "/cmd/jira-cli@" + normalizeReleaseTag(targetVersion)}
	default:
		return fmt.Errorf("unsupported package manager: %s", method)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
