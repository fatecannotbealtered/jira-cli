package cmd

import (
	"fmt"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueCmd.AddCommand(issueTransitionCmd)
	issueCmd.AddCommand(issueTransitionsCmd)
	markWrite(issueTransitionCmd)
}

var issueTransitionCmd = &cobra.Command{
	Use:   "transition <ISSUE_KEY> <STATUS_NAME>",
	Short: "Transition an issue to a new status",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		key := args[0]
		statusName := args[1]

		// Get current issue status
		issue, err := client.Issues.Get(key, nil)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		fromStatus := issue.Fields.Status.Name

		// Get available transitions
		transitions, err := client.Issues.GetTransitions(key)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}

		// Match by name
		var targetTransition *api.Transition
		for i, t := range transitions {
			if strings.EqualFold(t.Name, statusName) || strings.EqualFold(t.To.Name, statusName) {
				targetTransition = &transitions[i]
				break
			}
		}
		if targetTransition == nil {
			if jsonMode {
				output.Error(fmt.Sprintf("Status %q not available.", statusName))
				return SilentErr(ExitBadArgs)
			}
			output.Error(fmt.Sprintf("Status %q not available. Available transitions:", statusName))
			for _, t := range transitions {
				fmt.Printf("  → %s → %s\n", t.Name, t.To.Name)
			}
			return SilentErr(ExitBadArgs)
		}

		if dryRunOutput("transition issue", map[string]any{"key": key, "from": fromStatus, "to": targetTransition.To.Name}) {
			return nil
		}

		if err := client.Issues.DoTransition(key, targetTransition.ID); err != nil {
			return handleAPIError(err, jsonMode)
		}

		if jsonMode {
			output.PrintJSON(map[string]string{
				"issueKey":   key,
				"fromStatus": fromStatus,
				"toStatus":   targetTransition.To.Name,
			})
			return nil
		}
		output.Success(fmt.Sprintf("%s transitioned: %s → %s", key, fromStatus, targetTransition.To.Name))
		return nil
	},
}

var issueTransitionsCmd = &cobra.Command{
	Use:   "transitions <ISSUE_KEY>",
	Short: "List available transitions for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		transitions, err := client.Issues.GetTransitions(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(transitions)
			return nil
		}
		headers := []string{"ID", "NAME", "TO STATUS"}
		rows := make([][]string, len(transitions))
		for i, t := range transitions {
			rows[i] = []string{t.ID, t.Name, output.StatusBadge(t.To.Name)}
		}
		output.Table(headers, rows)
		return nil
	},
}
