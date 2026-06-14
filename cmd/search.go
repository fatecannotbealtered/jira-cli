package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <JQL>",
	Short: "Search issues using JQL",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().Int("limit", 50, "Max results (1-500)")
	searchCmd.Flags().Int("start-at", 0, "0-based index of the first result (page forward with the nextStartAt from a prior response)")
	searchCmd.Flags().String("fields", "", "Jira fields to fetch (comma-separated, e.g. summary,status,assignee)")
	searchCmd.Flags().String("order-by", "", "Order by field")
	searchCmd.Flags().Bool("all", false, "Fetch all results (auto-paginate)")
	searchCmd.Flags().Bool("count", false, "Only show total count")
	searchCmd.Flags().Bool("raw", false, "Return raw Jira API response (default: flat JSON)")
	markRawFormat(searchCmd)
	rootCmd.AddCommand(searchCmd)
}

var jqlOrderByRe = regexp.MustCompile(`(?i)\border\s+by\b`)

func jqlHasOrderBy(jql string) bool {
	return jqlOrderByRe.MatchString(jql)
}

func runSearch(cmd *cobra.Command, args []string) error {
	client, _, err := newClient()
	if err != nil {
		return err
	}

	jql := args[0]
	if jql == "" {
		output.Error("JQL query cannot be empty")
		return SilentErr(ExitBadArgs)
	}

	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 || limit > 500 {
		output.Error("--limit must be between 1 and 500")
		return SilentErr(ExitBadArgs)
	}

	startAt, _ := cmd.Flags().GetInt("start-at")
	if startAt < 0 {
		output.Error("--start-at must be >= 0")
		return SilentErr(ExitBadArgs)
	}

	fieldsStr, _ := cmd.Flags().GetString("fields")
	var fields []string
	if fieldsStr != "" {
		for _, f := range strings.Split(fieldsStr, ",") {
			fields = append(fields, strings.TrimSpace(f))
		}
	}

	orderBy, _ := cmd.Flags().GetString("order-by")
	if orderBy != "" && !jqlHasOrderBy(jql) {
		jql += " ORDER BY " + orderBy
	}

	countOnly, _ := cmd.Flags().GetBool("count")
	fetchAll, _ := cmd.Flags().GetBool("all")

	if countOnly {
		result, err := client.Issues.Search(jql, api.SearchOptions{MaxResults: 0, Fields: []string{"id"}})
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]int{"total": result.Total})
			return nil
		}
		fmt.Printf("Total: %d\n", result.Total)
		return nil
	}

	if fetchAll {
		issues, truncated, err := client.Issues.SearchAll(jql, fields)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			payload := map[string]any{
				"total":     len(issues),
				"truncated": truncated,
			}
			if truncated {
				payload["cap"] = api.SearchAllCap
			}
			if rawOutputRequested(cmd) {
				payload["issues"] = issues
			} else {
				payload["issues"] = toFlatIssues(issues)
			}
			output.PrintJSON(payload)
			return nil
		}
		if truncated {
			output.Info(fmt.Sprintf("Note: results truncated at %d issues; narrow the JQL to see the rest.", api.SearchAllCap))
		}
		if len(issues) == 0 {
			output.Info("No issues found.")
			return nil
		}
		printIssueTable(issues)
		return nil
	}

	result, err := client.Issues.Search(jql, api.SearchOptions{
		StartAt:    startAt,
		MaxResults: limit,
		Fields:     fields,
	})
	if err != nil {
		return handleAPIError(err, jsonMode)
	}

	if jsonMode {
		if rawOutputRequested(cmd) {
			output.PrintJSON(result)
		} else {
			nextStartAt := result.StartAt + len(result.Issues)
			isLast := nextStartAt >= result.Total
			payload := map[string]any{
				"total":      result.Total,
				"startAt":    result.StartAt,
				"maxResults": result.MaxResults,
				"isLast":     isLast,
				"issues":     toFlatIssues(result.Issues),
			}
			// nextStartAt is the value to pass to page forward; omitted on the
			// last page so an agent stops cleanly.
			if !isLast {
				payload["nextStartAt"] = nextStartAt
			}
			output.PrintJSON(payload)
		}
		return nil
	}

	if len(result.Issues) == 0 {
		output.Info("No issues found.")
		return nil
	}

	printIssueTable(result.Issues)
	output.Gray(fmt.Sprintf("  Showing %d of %d results", len(result.Issues), result.Total))
	return nil
}
