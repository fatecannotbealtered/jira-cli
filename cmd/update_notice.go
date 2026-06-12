package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	updateNoticeCacheTTL       = 24 * time.Hour
	updateNoticeRefreshTimeout = 2 * time.Second
	updateNoticeEnvOptOut      = "JIRA_CLI_NO_UPDATE_CHECK"
)

type updateNotice struct {
	Type               string   `json:"type"`
	Severity           string   `json:"severity"`
	Message            string   `json:"message"`
	CurrentVersion     string   `json:"current_version"`
	LatestVersion      string   `json:"latest_version"`
	UpdateAvailable    bool     `json:"update_available"`
	InstallMethod      string   `json:"install_method,omitempty"`
	RecommendedCommand string   `json:"recommended_command"`
	ReleaseURL         string   `json:"release_url,omitempty"`
	CheckedAt          string   `json:"checked_at"`
	Source             string   `json:"source"`
	NextSteps          []string `json:"next_steps"`
}

type updateNoticeCache struct {
	CheckedAt string         `json:"checked_at"`
	Notices   []updateNotice `json:"notices,omitempty"`
}

func installUpdateNoticeHelp(root *cobra.Command) {
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		if cmd.Long != "" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), cmd.Long)
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		} else if cmd.Short != "" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), cmd.Short)
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), cmd.UsageString())
		printUpdateNoticeHint(cmd.OutOrStdout(), readCachedUpdateNotices())
	})
}

func refreshUpdateNotices(ctx context.Context, source string) []updateNotice {
	if updateNoticeAutoDisabled() {
		return nil
	}
	refreshCtx, cancel := context.WithTimeout(ctx, updateNoticeRefreshTimeout)
	defer cancel()

	release, err := fetchUpdateRelease(refreshCtx, "")
	if err != nil {
		return readCachedUpdateNotices()
	}
	latest := normalizeVersion(release.TagName)
	method := ""
	if exe, err := updateExecutable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		method = detectInstallMethod(exe)
	}
	notices := updateNoticesFromValues(version, latest, method, "", source)
	writeUpdateNoticeCache(notices)
	return notices
}

func updateNoticesFromResult(result updateResult, source string) []updateNotice {
	notices := updateNoticesFromValues(result.CurrentVersion, result.LatestVersion, result.InstallMethod, result.ManagerCommand, source)
	writeUpdateNoticeCache(notices)
	return notices
}

func updateNoticesFromValues(current, latest, installMethod, command, source string) []updateNotice {
	current = normalizeVersion(current)
	latest = normalizeVersion(latest)
	if !updateAvailable(current, latest) {
		return nil
	}
	if command == "" {
		command = updateNoticeRecommendedCommand(installMethod, latest)
	}
	notice := updateNotice{
		Type:               "update_available",
		Severity:           "info",
		CurrentVersion:     current,
		LatestVersion:      latest,
		UpdateAvailable:    true,
		InstallMethod:      installMethod,
		RecommendedCommand: command,
		ReleaseURL:         updateNoticeReleaseURL(latest),
		CheckedAt:          time.Now().UTC().Format(time.RFC3339),
		Source:             source,
		NextSteps: []string{
			"run the recommended command",
			"after update, run jira-cli changelog --since " + current + " --compact",
			"refresh jira-cli reference --compact before using new behavior",
		},
	}
	notice.Message = fmt.Sprintf("jira-cli %s is available (current %s)", latest, current)
	return []updateNotice{notice}
}

func updateNoticeRecommendedCommand(installMethod, latest string) string {
	switch strings.ToLower(strings.TrimSpace(installMethod)) {
	case "npm":
		return managerUpdateCommand("npm", latest)
	case "go":
		return managerUpdateCommand("go", latest)
	default:
		return "jira-cli update --dry-run --compact"
	}
}

func updateNoticeReleaseURL(latest string) string {
	if latest == "" {
		return ""
	}
	return "https://github.com/" + updateRepo + "/releases/tag/" + normalizeReleaseTag(latest)
}

func readCachedUpdateNotices() []updateNotice {
	if updateNoticeAutoDisabled() {
		return nil
	}
	path, err := updateNoticeCachePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache updateNoticeCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	checkedAt, err := time.Parse(time.RFC3339, cache.CheckedAt)
	if err != nil || time.Since(checkedAt) > updateNoticeCacheTTL {
		return nil
	}
	notices := make([]updateNotice, 0, len(cache.Notices))
	for _, notice := range cache.Notices {
		if notice.Type != "update_available" || !notice.UpdateAvailable {
			continue
		}
		notice.Source = "cache"
		notices = append(notices, notice)
	}
	return notices
}

func writeUpdateNoticeCache(notices []updateNotice) {
	if updateNoticeAutoDisabled() {
		return
	}
	path, err := updateNoticeCachePath()
	if err != nil {
		return
	}
	if len(notices) == 0 {
		_ = os.Remove(path)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	cache := updateNoticeCache{CheckedAt: checkedAt, Notices: notices}
	for i := range cache.Notices {
		cache.Notices[i].CheckedAt = checkedAt
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func updateNoticeCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", err
	}
	return filepath.Join(home, "."+updateBinaryName, "update-check.json"), nil
}

func updateNoticeDisabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(updateNoticeEnvOptOut)))
	return value == "1" || value == "true" || value == "yes"
}

func updateNoticeAutoDisabled() bool {
	return updateNoticeDisabled() || strings.HasSuffix(os.Args[0], ".test")
}

func printUpdateNoticeHint(w io.Writer, notices []updateNotice) {
	if len(notices) == 0 {
		return
	}
	notice := notices[0]
	_, _ = fmt.Fprintf(w, "\nUpdate available: jira-cli %s -> %s. Run: %s\n", notice.CurrentVersion, notice.LatestVersion, notice.RecommendedCommand)
}
