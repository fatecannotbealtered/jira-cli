package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueCmd.AddCommand(issueAssignCmd)
	issueCmd.AddCommand(issueUnassignCmd)
	issueCmd.AddCommand(issueWatchCmd)
	issueCmd.AddCommand(issueUnwatchCmd)
	issueCmd.AddCommand(issueWatchersCmd)
	issueCmd.AddCommand(issueVoteCmd)
	issueCmd.AddCommand(issueUnvoteCmd)

	markWrite(issueAssignCmd)
	markWrite(issueUnassignCmd)
	markWrite(issueWatchCmd)
	markWrite(issueUnwatchCmd)
	markWrite(issueVoteCmd)
	markWrite(issueUnvoteCmd)
}

var issueAssignCmd = &cobra.Command{
	Use:   "assign <ISSUE_KEY> <USERNAME|me>",
	Short: "Assign an issue to a user",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		key, username := args[0], args[1]
		username, err = resolveUsername(client, username)
		if err != nil {
			return err
		}
		if dryRunOutput("assign issue", map[string]any{"key": key, "assignee": username}) {
			return nil
		}
		if err := client.Issues.Assign(key, username); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"issueKey": key, "assignee": username})
			return nil
		}
		output.Success(fmt.Sprintf("%s assigned to %s", key, username))
		return nil
	},
}

var issueUnassignCmd = &cobra.Command{
	Use:   "unassign <ISSUE_KEY>",
	Short: "Remove assignee from an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		if dryRunOutput("unassign issue", map[string]any{"key": args[0]}) {
			return nil
		}
		if err := client.Issues.Assign(args[0], ""); err != nil {
			return handleAPIError(err, jsonMode)
		}
		output.Success(fmt.Sprintf("%s unassigned", args[0]))
		return nil
	},
}

var issueWatchCmd = &cobra.Command{
	Use:   "watch <ISSUE_KEY>",
	Short: "Watch an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		myself, err := client.Users.Me()
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if dryRunOutput("watch issue", map[string]any{"key": args[0]}) {
			return nil
		}
		if err := client.Issues.AddWatcher(args[0], myself.Name); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"issueKey": args[0], "action": "watching"})
			return nil
		}
		output.Success(fmt.Sprintf("Now watching %s", args[0]))
		return nil
	},
}

var issueUnwatchCmd = &cobra.Command{
	Use:   "unwatch <ISSUE_KEY>",
	Short: "Stop watching an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		myself, err := client.Users.Me()
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if dryRunOutput("unwatch issue", map[string]any{"key": args[0]}) {
			return nil
		}
		if err := client.Issues.RemoveWatcher(args[0], myself.Name); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"issueKey": args[0], "action": "unwatched"})
			return nil
		}
		output.Success(fmt.Sprintf("Stopped watching %s", args[0]))
		return nil
	},
}

var issueWatchersCmd = &cobra.Command{
	Use:   "watchers <ISSUE_KEY>",
	Short: "List watchers of an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		watchers, err := client.Issues.GetWatchers(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(watchers)
			return nil
		}
		if len(watchers) == 0 {
			output.Info("No watchers.")
			return nil
		}
		headers := []string{"USERNAME", "DISPLAY NAME", "EMAIL"}
		rows := make([][]string, len(watchers))
		for i, w := range watchers {
			rows[i] = []string{w.Name, w.DisplayName, w.EmailAddress}
		}
		output.Table(headers, rows)
		return nil
	},
}

var issueVoteCmd = &cobra.Command{
	Use:   "vote <ISSUE_KEY>",
	Short: "Vote for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		if dryRunOutput("vote for issue", map[string]any{"key": args[0]}) {
			return nil
		}
		if err := client.Issues.AddVote(args[0]); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"issueKey": args[0], "action": "voted"})
			return nil
		}
		output.Success(fmt.Sprintf("Voted for %s", args[0]))
		return nil
	},
}

var issueUnvoteCmd = &cobra.Command{
	Use:   "unvote <ISSUE_KEY>",
	Short: "Remove vote from an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		if dryRunOutput("remove vote from issue", map[string]any{"key": args[0]}) {
			return nil
		}
		if err := client.Issues.RemoveVote(args[0]); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"issueKey": args[0], "action": "unvoted"})
			return nil
		}
		output.Success(fmt.Sprintf("Vote removed from %s", args[0]))
		return nil
	},
}
