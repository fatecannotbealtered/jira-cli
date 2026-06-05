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
	"fmt"
	"io"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	updateRepo            = "fatecannotbealtered/jira-cli"
	updateAPIBase         = "https://api.github.com/repos/" + updateRepo
	updatePackageName     = "@fatecannotbealtered-/jira-cli"
	updateBinaryName      = "jira-cli"
	maxReleaseJSONBytes   = 5 << 20
	maxChecksumFileBytes  = 1 << 20
	maxArchiveBytes       = 100 << 20
	maxExtractedBinaryLen = 100 << 20
)

var (
	updateHTTPClient = &http.Client{Timeout: 30 * time.Second}
	updateBaseURL    = updateAPIBase
	updateExecutable = os.Executable
	updateGOOS       = func() string { return runtime.GOOS }
	updateGOARCH     = func() string { return runtime.GOARCH }
	updateGetenv     = os.Getenv
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update jira-cli to a GitHub release",
	Long: `Update jira-cli by downloading the matching GitHub Release asset,
verifying checksums.txt, and replacing the current standalone binary.

Package-manager installs are detected where possible. For npm installs, use
npm install -g @fatecannotbealtered-/jira-cli@latest unless --force is set.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().Bool("check", false, "Check whether an update is available without installing")
	updateCmd.Flags().String("version", "", "Install a specific release version (for example v1.2.3)")
	rootCmd.AddCommand(updateCmd)
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
	CurrentVersion   string `json:"currentVersion"`
	LatestVersion    string `json:"latestVersion"`
	RequestedVersion string `json:"requestedVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	Installed        bool   `json:"installed,omitempty"`
	CheckOnly        bool   `json:"checkOnly,omitempty"`
	DryRun           bool   `json:"dryRun,omitempty"`
	InstallMethod    string `json:"installMethod,omitempty"`
	ManagerCommand   string `json:"managerCommand,omitempty"`
	Asset            string `json:"asset,omitempty"`
	Path             string `json:"path,omitempty"`
	ChecksumVerified bool   `json:"checksumVerified,omitempty"`
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")
	targetVersion, _ := cmd.Flags().GetString("version")
	requestedSpecific := strings.TrimSpace(targetVersion) != ""

	release, err := fetchUpdateRelease(cmd.Context(), targetVersion)
	if err != nil {
		return handleUpdateError(err, ExitNetwork)
	}
	latestVersion := normalizeVersion(release.TagName)
	if latestVersion == "" {
		return handleUpdateError(fmt.Errorf("release is missing tag_name"), ExitNetwork)
	}

	platform, arch, err := updatePlatform(updateGOOS(), updateGOARCH())
	if err != nil {
		return handleUpdateError(err, ExitBadArgs)
	}
	archiveName := updateArchiveName(latestVersion, platform, arch)
	archiveAsset, ok := release.assetByName(archiveName)
	if !ok {
		return handleUpdateError(fmt.Errorf("release %s has no asset for %s-%s (%s)", release.TagName, platform, arch, archiveName), ExitBadArgs)
	}
	checksumAsset, ok := release.assetByName("checksums.txt")
	if !ok {
		return handleUpdateError(fmt.Errorf("release %s has no checksums.txt asset", release.TagName), ExitNetwork)
	}

	currentVersion := version
	available := updateAvailable(currentVersion, latestVersion)
	exePath, err := updateExecutable()
	if err != nil {
		return handleUpdateError(fmt.Errorf("locating current executable: %w", err), ExitBadArgs)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	installMethod := detectInstallMethod(exePath)
	result := updateResult{
		CurrentVersion:   currentVersion,
		LatestVersion:    latestVersion,
		RequestedVersion: requestedVersionField(targetVersion),
		UpdateAvailable:  available,
		CheckOnly:        checkOnly,
		InstallMethod:    installMethod,
		Asset:            archiveName,
		Path:             exePath,
	}
	if installMethod != "" {
		result.ManagerCommand = managerUpdateCommand(installMethod, targetVersion)
	}

	if checkOnly {
		printUpdateResult(result)
		return nil
	}
	if installMethod != "" && !forceMode {
		printPackageManagerUpdate(result)
		return SilentErr(ExitBadArgs)
	}
	if !available && !requestedSpecific && !forceMode {
		printUpdateResult(result)
		return nil
	}
	if dryRun {
		result.DryRun = true
		printUpdateResult(result)
		return nil
	}
	if !forceMode && !confirmAction(
		fmt.Sprintf("Update jira-cli from %s to %s? Type %s to confirm", currentVersion, latestVersion, latestVersion),
		latestVersion,
	) {
		if jsonMode {
			output.PrintErrorJSONWithCode("Update cancelled.", 0, output.ErrValidation)
		} else {
			output.Warn("Update cancelled.")
		}
		return SilentErr(ExitBadArgs)
	}

	archiveData, err := downloadUpdateURL(cmd.Context(), archiveAsset.BrowserDownloadURL, maxArchiveBytes)
	if err != nil {
		return handleUpdateError(err, ExitNetwork)
	}
	checksumData, err := downloadUpdateURL(cmd.Context(), checksumAsset.BrowserDownloadURL, maxChecksumFileBytes)
	if err != nil {
		return handleUpdateError(err, ExitNetwork)
	}
	if err := verifyArchiveChecksum(archiveName, archiveData, checksumData); err != nil {
		return handleUpdateError(err, ExitNetwork)
	}
	binaryData, err := extractBinaryFromArchive(archiveName, archiveData, binaryNameForPlatform(platform))
	if err != nil {
		return handleUpdateError(err, ExitNetwork)
	}
	if err := replaceExecutable(exePath, binaryData); err != nil {
		return handleUpdateError(err, ExitBadArgs)
	}

	result.Installed = true
	result.ChecksumVerified = true
	printUpdateResult(result)
	return nil
}

func handleUpdateError(err error, code int) error {
	if jsonMode {
		output.PrintErrorJSONWithCode(err.Error(), 0, updateErrorCode(code))
	} else {
		output.Error(err.Error())
	}
	return SilentErr(code)
}

func updateErrorCode(exitCode int) output.ErrorCode {
	switch exitCode {
	case ExitBadArgs:
		return output.ErrValidation
	case ExitNetwork:
		return output.ErrNetwork
	default:
		return output.ErrUnknown
	}
}

func printPackageManagerUpdate(result updateResult) {
	if jsonMode {
		output.PrintJSON(result)
		return
	}
	output.Warn("This jira-cli installation appears to be managed by " + result.InstallMethod + ".")
	if result.ManagerCommand != "" {
		output.Info("Update with: " + result.ManagerCommand)
	}
	output.Info("Use --force only if you intentionally want to replace the binary in place.")
}

func printUpdateResult(result updateResult) {
	if jsonMode {
		output.PrintJSON(result)
		return
	}
	if result.Installed {
		if result.UpdateAvailable {
			output.Success(fmt.Sprintf("Updated jira-cli from %s to %s", result.CurrentVersion, result.LatestVersion))
		} else {
			output.Success(fmt.Sprintf("Installed jira-cli %s over %s", result.LatestVersion, result.CurrentVersion))
		}
		return
	}
	if result.DryRun {
		if result.UpdateAvailable || result.RequestedVersion != "" {
			output.Info(fmt.Sprintf("[dry-run] would install jira-cli %s over %s", result.LatestVersion, result.CurrentVersion))
		} else {
			output.Info(fmt.Sprintf("[dry-run] jira-cli is already at %s", result.CurrentVersion))
		}
		return
	}
	if result.CheckOnly {
		if result.UpdateAvailable {
			output.Info(fmt.Sprintf("Update available: %s -> %s", result.CurrentVersion, result.LatestVersion))
		} else {
			output.Success(fmt.Sprintf("jira-cli is up to date (%s)", result.CurrentVersion))
		}
		return
	}
	if result.UpdateAvailable {
		output.Info(fmt.Sprintf("Update available: %s -> %s", result.CurrentVersion, result.LatestVersion))
	} else {
		output.Success(fmt.Sprintf("jira-cli is already up to date (%s)", result.CurrentVersion))
	}
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
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("GitHub returned %s: %s", resp.Status, msg)
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
