package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Manage Jira epics",
}

func init() {
	rootCmd.AddCommand(epicCmd)

	epicListCmd.Flags().Int("board", 0, "Board ID (required)")
	epicListCmd.Flags().Bool("done", false, "Show only completed epics")
	epicCmd.AddCommand(epicListCmd)

	epicIssuesCmd.Flags().Int("board", 0, "Board ID (required)")
	epicCmd.AddCommand(epicIssuesCmd)
}

var epicListCmd = &cobra.Command{
	Use:   "list",
	Short: "List epics on a board",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		boardID, _ := cmd.Flags().GetInt("board")
		if boardID == 0 {
			output.Error("--board is required")
			return SilentErr(ExitBadArgs)
		}
		done, _ := cmd.Flags().GetBool("done")
		epics, err := client.Boards.GetEpics(boardID, done)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			printIssuesJSON(epics, false, nil)
			return nil
		}
		if len(epics) == 0 {
			output.Info("No epics found.")
			return nil
		}
		printIssueTable(epics)
		return nil
	},
}

var epicIssuesCmd = &cobra.Command{
	Use:   "issues <EPIC_KEY>",
	Short: "List issues in an epic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		epicKey := args[0]
		boardID, _ := cmd.Flags().GetInt("board")
		if boardID == 0 {
			output.Error("--board is required")
			return SilentErr(ExitBadArgs)
		}

		// Try Agile API first
		epics, listErr := client.Boards.GetEpics(boardID, false)
		if listErr == nil {
			for _, epic := range epics {
				if epic.Key == epicKey && epic.ID != "" {
					issues, err := client.Boards.GetEpicIssues(epic.ID)
					if err == nil {
						if jsonMode {
							printIssuesJSON(issues, false, nil)
							return nil
						}
						if len(issues) == 0 {
							output.Info("No issues found in this epic.")
							return nil
						}
						printIssueTable(issues)
						return nil
					}
					break
				}
			}
		}

		// Fallback: use JQL with proper quoting to prevent injection
		jql := fmt.Sprintf(`"Epic Link" = %q`, epicKey)
		result, err := client.Issues.Search(jql, api.SearchOptions{MaxResults: 100})
		if err != nil {
			// Try the newer Epic field name
			jql = fmt.Sprintf(`parentEpic = %q`, epicKey)
			result, err = client.Issues.Search(jql, api.SearchOptions{MaxResults: 100})
			if err != nil {
				return handleAPIError(err, jsonMode)
			}
		}
		if jsonMode {
			printIssuesJSON(result.Issues, false, nil)
			return nil
		}
		if len(result.Issues) == 0 {
			output.Info("No issues found in this epic.")
			return nil
		}
		printIssueTable(result.Issues)
		return nil
	},
}
