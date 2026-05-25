package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Manage Jira boards",
}

func init() {
	rootCmd.AddCommand(boardCmd)

	boardListCmd.Flags().String("project", "", "Filter by project key")
	boardListCmd.Flags().String("type", "", "Filter by type (scrum,kanban)")
	boardCmd.AddCommand(boardListCmd)

	boardGetCmd.Flags().Int("board", 0, "Board ID (required)")
	boardCmd.AddCommand(boardGetCmd)

	boardBacklogCmd.Flags().Int("board", 0, "Board ID (required)")
	boardBacklogCmd.Flags().Int("limit", 50, "Max results (1-200)")
	boardCmd.AddCommand(boardBacklogCmd)

	boardEpicsCmd.Flags().Int("board", 0, "Board ID (required)")
	boardEpicsCmd.Flags().Bool("done", false, "Show only completed epics")
	boardEpicsCmd.Flags().Int("limit", 50, "Max results (1-200)")
	boardCmd.AddCommand(boardEpicsCmd)

	boardSprintsCmd.Flags().Int("board", 0, "Board ID (required)")
	boardSprintsCmd.Flags().String("state", "", "Filter by state (active,future,closed)")
	boardCmd.AddCommand(boardSprintsCmd)
}

var boardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all boards",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		project, _ := cmd.Flags().GetString("project")
		boardType, _ := cmd.Flags().GetString("type")
		boards, err := client.Boards.List(project, boardType)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(boards)
			return nil
		}
		if len(boards) == 0 {
			output.Info("No boards found.")
			return nil
		}
		headers := []string{"ID", "NAME", "TYPE", "PROJECT"}
		rows := make([][]string, len(boards))
		for i, b := range boards {
			rows[i] = []string{
				fmt.Sprintf("%d", b.ID),
				b.Name,
				b.Type,
				b.Location.ProjectKey,
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var boardGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get board details",
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
		board, err := client.Boards.Get(boardID)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(board)
			return nil
		}
		fmt.Println()
		output.Bold(fmt.Sprintf("  Board: %s (ID: %d)", board.Name, board.ID))
		output.Gray(fmt.Sprintf("  Type: %s | Project: %s", board.Type, board.Location.ProjectKey))
		fmt.Println()
		return nil
	},
}

var boardBacklogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "List issues in the board backlog",
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
		issues, err := client.Boards.GetBacklog(boardID)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		limit, _ := cmd.Flags().GetInt("limit")
		if limit > 0 && limit < len(issues) {
			issues = issues[:limit]
		}
		if jsonMode {
			output.PrintJSON(issues)
			return nil
		}
		if len(issues) == 0 {
			output.Info("Backlog is empty.")
			return nil
		}
		printIssueTable(issues)
		return nil
	},
}

var boardEpicsCmd = &cobra.Command{
	Use:   "epics",
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
		limit, _ := cmd.Flags().GetInt("limit")
		if limit > 0 && limit < len(epics) {
			epics = epics[:limit]
		}
		if jsonMode {
			output.PrintJSON(epics)
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

var boardSprintsCmd = &cobra.Command{
	Use:   "sprints",
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
		sprints, err := client.Boards.GetSprints(boardID, state)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(sprints)
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
