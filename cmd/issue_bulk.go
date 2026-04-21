package cmd

import (
	"fmt"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueBulkTransitionCmd.Flags().String("issues", "", "Issue keys (comma-separated, required)")
	issueBulkTransitionCmd.Flags().String("jql", "", "JQL query to select issues (alternative to --issues)")
	issueCmd.AddCommand(issueBulkTransitionCmd)
	markWrite(issueBulkTransitionCmd)
}

var issueBulkTransitionCmd = &cobra.Command{
	Use:   "bulk-transition <STATUS_NAME>",
	Short: "Transition multiple issues to a new status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		targetStatus := args[0]

		// Collect issue keys
		var issueKeys []string
		issuesStr, _ := cmd.Flags().GetString("issues")
		jql, _ := cmd.Flags().GetString("jql")

		if issuesStr != "" {
			for _, k := range strings.Split(issuesStr, ",") {
				k = strings.TrimSpace(k)
				if k != "" {
					issueKeys = append(issueKeys, k)
				}
			}
		} else if jql != "" {
			issues, err := client.Issues.SearchAll(jql, []string{"key"})
			if err != nil {
				return handleAPIError(err, jsonMode)
			}
			for _, issue := range issues {
				issueKeys = append(issueKeys, issue.Key)
			}
		} else {
			output.Error("--issues or --jql is required")
			return ErrSilent
		}

		if len(issueKeys) == 0 {
			output.Info("No issues to transition.")
			return nil
		}

		if dryRunOutput("bulk-transition issues", map[string]any{"issues": issueKeys, "targetStatus": targetStatus}) {
			return nil
		}

		type result struct {
			Key    string `json:"key"`
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}
		var results []result
		successCount := 0
		failCount := 0

		for _, key := range issueKeys {
			// Get available transitions
			transitions, err := client.Issues.GetTransitions(key)
			if err != nil {
				results = append(results, result{Key: key, Status: "error", Error: err.Error()})
				failCount++
				continue
			}

			// Find matching transition
			var transitionID string
			for _, t := range transitions {
				if strings.EqualFold(t.Name, targetStatus) || strings.EqualFold(t.To.Name, targetStatus) {
					transitionID = t.ID
					break
				}
			}

			if transitionID == "" {
				results = append(results, result{Key: key, Status: "skipped", Error: fmt.Sprintf("status %q not available", targetStatus)})
				failCount++
				continue
			}

			if err := client.Issues.DoTransition(key, transitionID); err != nil {
				results = append(results, result{Key: key, Status: "error", Error: err.Error()})
				failCount++
				continue
			}

			results = append(results, result{Key: key, Status: "transitioned"})
			successCount++
		}

		if jsonMode {
			output.PrintJSON(map[string]any{
				"targetStatus": targetStatus,
				"total":        len(issueKeys),
				"success":      successCount,
				"failed":       failCount,
				"results":      results,
			})
			return nil
		}

		for _, r := range results {
			if r.Error != "" {
				output.Warn(fmt.Sprintf("%s: %s", r.Key, r.Error))
			} else {
				output.Success(fmt.Sprintf("%s \u2192 %s", r.Key, targetStatus))
			}
		}
		fmt.Println()
		output.Info(fmt.Sprintf("Done: %d succeeded, %d failed (of %d total)", successCount, failCount, len(issueKeys)))
		return nil
	},
}
