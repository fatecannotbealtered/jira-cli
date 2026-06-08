package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var issueWorklogCmd = &cobra.Command{
	Use:   "worklog",
	Short: "Manage issue worklogs",
}

func init() {
	worklogAddCmd.Flags().String("time", "", "Time spent (e.g. 1h30m, 2h, 30m) (required)")
	worklogAddCmd.Flags().String("started", "", "Start time (ISO 8601, default: now)")
	worklogAddCmd.Flags().String("comment", "", "Worklog comment")
	issueWorklogCmd.AddCommand(worklogAddCmd)
	issueWorklogCmd.AddCommand(worklogListCmd)
	issueCmd.AddCommand(issueWorklogCmd)
	markWrite(worklogAddCmd)
}

var worklogAddCmd = &cobra.Command{
	Use:   "add <ISSUE_KEY>",
	Short: "Log time on an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		timeStr, _ := cmd.Flags().GetString("time")
		if timeStr == "" {
			output.Error("--time is required (e.g. 1h30m, 2h, 30m)")
			return SilentErr(ExitBadArgs)
		}
		started, _ := cmd.Flags().GetString("started")
		comment, _ := cmd.Flags().GetString("comment")

		if dryRunOutput("add worklog", map[string]any{"key": args[0], "time": timeStr, "started": started, "comment": comment}) {
			return nil
		}
		worklog, err := client.Issues.AddWorklog(args[0], timeStr, started, comment)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(toFlatWorklog(*worklog))
			return nil
		}
		output.Success(fmt.Sprintf("Logged %s on %s (ID: %s)", worklog.TimeSpent, args[0], worklog.ID))
		return nil
	},
}

var worklogListCmd = &cobra.Command{
	Use:   "list <ISSUE_KEY>",
	Short: "List worklogs for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		worklogs, err := client.Issues.ListWorklogs(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			flat := make([]flatWorklog, len(worklogs))
			for i := range worklogs {
				flat[i] = toFlatWorklog(worklogs[i])
			}
			output.PrintJSON(flat)
			return nil
		}
		if len(worklogs) == 0 {
			output.Info("No worklogs.")
			return nil
		}
		headers := []string{"ID", "AUTHOR", "TIME SPENT", "STARTED", "COMMENT"}
		rows := make([][]string, len(worklogs))
		for i, w := range worklogs {
			comment := api.ADFToText(w.Comment)
			if len(comment) > 80 {
				comment = comment[:77] + "..."
			}
			rows[i] = []string{w.ID, w.Author.DisplayName, w.TimeSpent, formatTime(w.Started), comment}
		}
		output.Table(headers, rows)
		return nil
	},
}

type flatWorklog struct {
	ID               string   `json:"id"`
	Author           string   `json:"author,omitempty"`
	Started          string   `json:"started,omitempty"`
	TimeSpent        string   `json:"timeSpent"`
	TimeSpentSeconds int      `json:"timeSpentSeconds"`
	Comment          string   `json:"comment,omitempty"`
	Untrusted        []string `json:"_untrusted,omitempty"`
}

func toFlatWorklog(worklog api.Worklog) flatWorklog {
	comment := api.ADFToText(worklog.Comment)
	if comment == "" {
		if s, ok := worklog.Comment.(string); ok {
			comment = s
		}
	}
	result := flatWorklog{
		ID:               worklog.ID,
		Author:           worklog.Author.Name,
		Started:          isoUTC(worklog.Started),
		TimeSpent:        worklog.TimeSpent,
		TimeSpentSeconds: worklog.TimeSpentSeconds,
		Comment:          comment,
	}
	if result.Comment != "" {
		result.Untrusted = []string{"comment"}
	}
	return result
}
