package cmd

import (
	"fmt"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "Manage Jira sprints",
}

func init() {
	rootCmd.AddCommand(sprintCmd)

	sprintListCmd.Flags().Int("board", 0, "Board ID (required)")
	sprintListCmd.Flags().String("state", "", "Filter by state (active,future,closed)")
	sprintListCmd.Flags().Bool("raw", false, "Return raw Jira API response (default: flat JSON)")
	sprintListCmd.Flags().StringSlice("fields", nil, "Output only these fields (flat JSON only)")
	sprintCmd.AddCommand(sprintListCmd)
	markRawFormat(sprintListCmd)

	sprintActiveCmd.Flags().Int("board", 0, "Board ID (required)")
	sprintActiveCmd.Flags().Bool("raw", false, "Return raw Jira API response (default: flat JSON)")
	sprintCmd.AddCommand(sprintActiveCmd)
	markRawFormat(sprintActiveCmd)

	sprintIssuesCmd.Flags().Int("sprint", 0, "Sprint ID (required)")
	sprintIssuesCmd.Flags().Bool("raw", false, "Return raw Jira API response (default: flat JSON)")
	sprintIssuesCmd.Flags().StringSlice("fields", nil, "Output only these fields (flat JSON only)")
	sprintCmd.AddCommand(sprintIssuesCmd)
	markRawFormat(sprintIssuesCmd)

	sprintCreateCmd.Flags().Int("board", 0, "Board ID (required)")
	sprintCreateCmd.Flags().String("name", "", "Sprint name (required)")
	sprintCreateCmd.Flags().String("start-date", "", "Start date (ISO 8601)")
	sprintCreateCmd.Flags().String("end-date", "", "End date (ISO 8601)")
	sprintCreateCmd.Flags().String("goal", "", "Sprint goal")
	sprintCmd.AddCommand(sprintCreateCmd)

	sprintUpdateCmd.Flags().Int("sprint", 0, "Sprint ID (required)")
	sprintUpdateCmd.Flags().String("name", "", "New name")
	sprintUpdateCmd.Flags().String("goal", "", "New goal")
	sprintUpdateCmd.Flags().String("start-date", "", "New start date")
	sprintUpdateCmd.Flags().String("end-date", "", "New end date")
	sprintCmd.AddCommand(sprintUpdateCmd)

	sprintCloseCmd.Flags().Int("sprint", 0, "Sprint ID (required)")
	sprintCmd.AddCommand(sprintCloseCmd)

	sprintMoveCmd.Flags().Int("sprint", 0, "Sprint ID (required)")
	sprintMoveCmd.Flags().String("issues", "", "Issue keys (comma-separated, required)")
	sprintCmd.AddCommand(sprintMoveCmd)

	markWrite(sprintCreateCmd)
	markWrite(sprintUpdateCmd)
	markWrite(sprintCloseCmd)
	markWrite(sprintMoveCmd)
}

var sprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sprints for a board",
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
		state, _ := cmd.Flags().GetString("state")
		sprints, err := client.Sprints.List(boardID, state)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			fields, _ := cmd.Flags().GetStringSlice("fields")
			printSprintsJSON(sprints, rawOutputRequested(cmd), fields)
			return nil
		}
		if len(sprints) == 0 {
			output.Info("No sprints found.")
			return nil
		}
		printSprintTable(sprints)
		return nil
	},
}

var sprintActiveCmd = &cobra.Command{
	Use:   "active",
	Short: "Show the active sprint(s) for a board",
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
		sprints, err := client.Sprints.List(boardID, "active")
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if len(sprints) == 0 {
			if jsonMode {
				output.PrintJSON([]map[string]any{})
				return nil
			}
			output.Info("No active sprint found.")
			return nil
		}

		type sprintWithIssues struct {
			Sprint api.Sprint  `json:"sprint"`
			Issues []api.Issue `json:"issues"`
		}
		var results []sprintWithIssues
		for _, sprint := range sprints {
			issues, err := client.Sprints.GetIssues(sprint.ID)
			if err != nil {
				return handleAPIError(err, jsonMode)
			}
			results = append(results, sprintWithIssues{Sprint: sprint, Issues: issues})
		}

		if jsonMode {
			if rawOutputRequested(cmd) {
				output.PrintJSON(results)
			} else {
				flat := make([]map[string]any, len(results))
				for i, r := range results {
					flat[i] = map[string]any{
						"sprint": toFlatSprint(&r.Sprint),
						"issues": toFlatIssues(r.Issues),
					}
				}
				output.PrintJSON(flat)
			}
			return nil
		}

		for i, r := range results {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println()
			output.Bold(fmt.Sprintf("  Sprint: %s (ID: %d)", r.Sprint.Name, r.Sprint.ID))
			output.Gray(fmt.Sprintf("  State: %s  |  %s \u2192 %s", r.Sprint.State, safeDate(r.Sprint.StartDate), safeDate(r.Sprint.EndDate)))
			if r.Sprint.Goal != "" {
				output.Gray(fmt.Sprintf("  Goal: %s", r.Sprint.Goal))
			}
			fmt.Println()
			if len(r.Issues) > 0 {
				output.Bold(fmt.Sprintf("  Issues (%d)", len(r.Issues)))
				printIssueTable(r.Issues)
			}
		}
		return nil
	},
}

var sprintIssuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List issues in a sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		sprintID, _ := cmd.Flags().GetInt("sprint")
		if sprintID == 0 {
			output.Error("--sprint is required")
			return SilentErr(ExitBadArgs)
		}
		issues, err := client.Sprints.GetIssues(sprintID)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			fields, _ := cmd.Flags().GetStringSlice("fields")
			printIssuesJSON(issues, rawOutputRequested(cmd), fields)
			return nil
		}
		if len(issues) == 0 {
			output.Info("No issues in this sprint.")
			return nil
		}
		printIssueTable(issues)
		return nil
	},
}

var sprintCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		boardID, _ := cmd.Flags().GetInt("board")
		name, _ := cmd.Flags().GetString("name")
		if boardID == 0 || name == "" {
			output.Error("--board and --name are required")
			return SilentErr(ExitBadArgs)
		}
		req := api.CreateSprintRequest{
			Name:          name,
			OriginBoardID: boardID,
		}
		if sd, _ := cmd.Flags().GetString("start-date"); sd != "" {
			req.StartDate = sd
		}
		if ed, _ := cmd.Flags().GetString("end-date"); ed != "" {
			req.EndDate = ed
		}
		if g, _ := cmd.Flags().GetString("goal"); g != "" {
			req.Goal = g
		}
		if dryRunOutput("create sprint", map[string]any{"name": name, "boardId": boardID, "request": req}) {
			return nil
		}
		sprint, err := client.Sprints.Create(req)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(sprint)
			return nil
		}
		output.Success(fmt.Sprintf("Created sprint: %s (ID: %d)", sprint.Name, sprint.ID))
		return nil
	},
}

var sprintUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		sprintID, _ := cmd.Flags().GetInt("sprint")
		if sprintID == 0 {
			output.Error("--sprint is required")
			return SilentErr(ExitBadArgs)
		}
		req := api.UpdateSprintRequest{}
		if n, _ := cmd.Flags().GetString("name"); n != "" {
			req.Name = n
		}
		if g, _ := cmd.Flags().GetString("goal"); g != "" {
			req.Goal = g
		}
		if sd, _ := cmd.Flags().GetString("start-date"); sd != "" {
			req.StartDate = sd
		}
		if ed, _ := cmd.Flags().GetString("end-date"); ed != "" {
			req.EndDate = ed
		}
		if dryRunOutput("update sprint", map[string]any{"sprintId": sprintID, "request": req}) {
			return nil
		}
		sprint, err := client.Sprints.Update(sprintID, req)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(sprint)
			return nil
		}
		output.Success(fmt.Sprintf("Updated sprint: %s (ID: %d)", sprint.Name, sprint.ID))
		return nil
	},
}

var sprintCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		sprintID, _ := cmd.Flags().GetInt("sprint")
		if sprintID == 0 {
			output.Error("--sprint is required")
			return SilentErr(ExitBadArgs)
		}
		sprint, err := client.Sprints.Get(sprintID)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if sprint.State == "closed" {
			if jsonMode {
				output.PrintJSON(map[string]any{"sprintId": sprintID, "state": "closed", "changed": false})
				return nil
			}
			output.Info("Sprint is already closed.")
			return nil
		}
		if dryRunOutput("close sprint", map[string]any{"sprintId": sprintID, "name": sprint.Name}) {
			return nil
		}
		if err := client.Sprints.Close(sprintID); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]any{"sprintId": sprintID, "state": "closed"})
			return nil
		}
		output.Success(fmt.Sprintf("Sprint %d closed", sprintID))
		return nil
	},
}

var sprintMoveCmd = &cobra.Command{
	Use:   "move",
	Short: "Move issues to a sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		sprintID, _ := cmd.Flags().GetInt("sprint")
		issuesStr, _ := cmd.Flags().GetString("issues")
		if sprintID == 0 || issuesStr == "" {
			output.Error("--sprint and --issues are required")
			return SilentErr(ExitBadArgs)
		}
		issueKeys := strings.Split(issuesStr, ",")
		for i, k := range issueKeys {
			issueKeys[i] = strings.TrimSpace(k)
		}
		if dryRunOutput("move issues to sprint", map[string]any{"sprintId": sprintID, "issues": issueKeys}) {
			return nil
		}
		if err := client.Sprints.MoveIssues(sprintID, issueKeys); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]any{"sprintId": sprintID, "issues": issueKeys})
			return nil
		}
		output.Success(fmt.Sprintf("Moved %d issue(s) to sprint %d", len(issueKeys), sprintID))
		return nil
	},
}

func safeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func printSprintTable(sprints []api.Sprint) {
	headers := []string{"ID", "NAME", "STATE", "START DATE", "END DATE", "GOAL"}
	rows := make([][]string, len(sprints))
	for i, s := range sprints {
		state := s.State
		switch state {
		case "active":
			state = output.FormatGreen(state)
		case "future":
			state = output.FormatCyan(state)
		default:
			state = output.FormatGray(state)
		}
		start := safeDate(s.StartDate)
		end := safeDate(s.EndDate)
		goal := s.Goal
		if len(goal) > 40 {
			goal = goal[:37] + "..."
		}
		rows[i] = []string{fmt.Sprintf("%d", s.ID), s.Name, state, start, end, goal}
	}
	output.Table(headers, rows)
}
