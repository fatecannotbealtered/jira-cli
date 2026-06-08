package cmd

import (
	"regexp"
	"sort"
	"strings"

	jiracli "github.com/fatecannotbealtered/jira-cli"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var changelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Print version changes from CHANGELOG.md",
	Args:  cobra.NoArgs,
	RunE:  runChangelog,
}

func init() {
	changelogCmd.Flags().String("since", "", "Return entries newer than this version")
	rootCmd.AddCommand(changelogCmd)
}

type changelogEntry struct {
	Version string              `json:"version"`
	Date    string              `json:"date,omitempty"`
	Changes map[string][]string `json:"changes"`
}

type changelogResult struct {
	CurrentVersion string           `json:"current_version"`
	Since          string           `json:"since,omitempty"`
	Entries        []changelogEntry `json:"entries"`
}

var changelogHeadingRe = regexp.MustCompile(`^## \[([^\]]+)\](?: - ([0-9]{4}-[0-9]{2}-[0-9]{2}))?`)

func runChangelog(cmd *cobra.Command, _ []string) error {
	since, _ := cmd.Flags().GetString("since")
	entries := parseChangelog(jiracli.ChangelogMarkdown)
	if since != "" {
		normalizedSince := normalizeVersion(since)
		filtered := entries[:0]
		for _, entry := range entries {
			if entry.Version == "Unreleased" || compareVersions(entry.Version, normalizedSince) > 0 {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	result := changelogResult{
		CurrentVersion: normalizeVersion(version),
		Since:          normalizeVersion(since),
		Entries:        entries,
	}
	if jsonMode {
		output.PrintJSON(result)
		return nil
	}
	printChangelogText(result)
	return nil
}

func parseChangelog(markdown string) []changelogEntry {
	var entries []changelogEntry
	var current *changelogEntry
	currentCategory := ""

	flush := func() {
		if current == nil {
			return
		}
		for key, values := range current.Changes {
			if len(values) == 0 {
				delete(current.Changes, key)
			}
		}
		if len(current.Changes) > 0 || current.Version == "Unreleased" {
			entries = append(entries, *current)
		}
		current = nil
		currentCategory = ""
	}

	for _, rawLine := range strings.Split(markdown, "\n") {
		line := strings.TrimSpace(rawLine)
		if match := changelogHeadingRe.FindStringSubmatch(line); match != nil {
			flush()
			current = &changelogEntry{
				Version: normalizeVersion(match[1]),
				Changes: map[string][]string{
					"added":      {},
					"changed":    {},
					"fixed":      {},
					"deprecated": {},
					"removed":    {},
					"security":   {},
				},
			}
			if len(match) > 2 {
				current.Date = match[2]
			}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			currentCategory = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
			continue
		}
		if strings.HasPrefix(line, "- ") && isChangelogCategory(currentCategory) {
			current.Changes[currentCategory] = append(current.Changes[currentCategory], strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		}
	}
	flush()

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Version == "Unreleased" {
			return true
		}
		if entries[j].Version == "Unreleased" {
			return false
		}
		return compareVersions(entries[i].Version, entries[j].Version) > 0
	})
	return entries
}

func isChangelogCategory(category string) bool {
	switch category {
	case "added", "changed", "fixed", "deprecated", "removed", "security":
		return true
	default:
		return false
	}
}

func printChangelogText(result changelogResult) {
	output.Bold("jira-cli changelog")
	for _, entry := range result.Entries {
		title := entry.Version
		if entry.Date != "" {
			title += " - " + entry.Date
		}
		output.Gray("")
		output.Bold(title)
		for _, category := range []string{"added", "changed", "fixed", "deprecated", "removed", "security"} {
			items := entry.Changes[category]
			if len(items) == 0 {
				continue
			}
			output.Gray("  " + category)
			for _, item := range items {
				output.Gray("    - " + item)
			}
		}
	}
}
