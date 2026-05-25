package cmd

import (
	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Search and view Jira users",
}

func init() {
	rootCmd.AddCommand(userCmd)

	userSearchCmd.Flags().String("query", "", "Search query (required)")
	userSearchCmd.Flags().Bool("assignable", false, "Only show users assignable to a project")
	userSearchCmd.Flags().String("project", "", "Project key (used with --assignable)")
	userCmd.AddCommand(userSearchCmd)

	userCmd.AddCommand(userMeCmd)
}

var userSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for users",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		query, _ := cmd.Flags().GetString("query")
		if query == "" {
			output.Error("--query is required")
			return SilentErr(ExitBadArgs)
		}
		assignable, _ := cmd.Flags().GetBool("assignable")
		project, _ := cmd.Flags().GetString("project")

		var users []api.User
		if assignable && project != "" {
			users, err = client.Users.SearchAssignable(query, project)
		} else {
			users, err = client.Users.Search(query)
		}
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(users)
			return nil
		}
		if len(users) == 0 {
			output.Info("No users found.")
			return nil
		}
		headers := []string{"USERNAME", "DISPLAY NAME", "EMAIL", "ACTIVE"}
		rows := make([][]string, len(users))
		for i, u := range users {
			active := "No"
			if u.Active {
				active = output.FormatGreen("Yes")
			}
			rows[i] = []string{u.Name, u.DisplayName, u.EmailAddress, active}
		}
		output.Table(headers, rows)
		return nil
	},
}

var userMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current user info",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		myself, err := client.Users.Me()
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(myself)
			return nil
		}
		headers := []string{"USERNAME", "DISPLAY NAME", "EMAIL", "ACTIVE"}
		active := "No"
		if myself.Active {
			active = output.FormatGreen("Yes")
		}
		output.Table(headers, [][]string{
			{myself.Name, myself.DisplayName, myself.EmailAddress, active},
		})
		return nil
	},
}
