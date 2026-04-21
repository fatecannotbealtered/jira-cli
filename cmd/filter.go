package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var filterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Manage Jira filters",
}

func init() {
	rootCmd.AddCommand(filterCmd)

	filterCmd.AddCommand(filterListCmd)
	filterCmd.AddCommand(filterGetCmd)

	filterCreateCmd.Flags().String("name", "", "Filter name (required)")
	filterCreateCmd.Flags().String("jql", "", "JQL query (required)")
	filterCreateCmd.Flags().String("description", "", "Filter description")
	filterCmd.AddCommand(filterCreateCmd)

	filterCmd.AddCommand(filterDeleteCmd)

	filterRunCmd.Flags().Int("limit", 50, "Max results")
	filterCmd.AddCommand(filterRunCmd)

	markWrite(filterCreateCmd)
	markWrite(filterDeleteCmd)
}

var filterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		filters, err := client.Filters.ListMy()
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(filters)
			return nil
		}
		if len(filters) == 0 {
			output.Info("No filters found.")
			return nil
		}
		headers := []string{"ID", "NAME", "JQL", "FAVOURITE"}
		rows := make([][]string, len(filters))
		for i, f := range filters {
			fav := "No"
			if f.Favourite {
				fav = output.FormatGreen("Yes")
			}
			jql := f.JQL
			if len(jql) > 60 {
				jql = jql[:57] + "..."
			}
			rows[i] = []string{f.ID, f.Name, jql, fav}
		}
		output.Table(headers, rows)
		return nil
	},
}

var filterGetCmd = &cobra.Command{
	Use:   "get <FILTER_ID>",
	Short: "Get filter details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		filter, err := client.Filters.Get(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(filter)
			return nil
		}
		fmt.Println()
		output.Bold(fmt.Sprintf("  Filter: %s (ID: %s)", filter.Name, filter.ID))
		output.Gray(fmt.Sprintf("  JQL: %s", filter.JQL))
		if filter.Description != "" {
			output.Gray(fmt.Sprintf("  Description: %s", filter.Description))
		}
		fmt.Println()
		return nil
	},
}

var filterCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new filter",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		jql, _ := cmd.Flags().GetString("jql")
		if name == "" || jql == "" {
			output.Error("--name and --jql are required")
			return ErrSilent
		}
		description, _ := cmd.Flags().GetString("description")
		filter, err := client.Filters.Create(name, jql, description)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(filter)
			return nil
		}
		output.Success(fmt.Sprintf("Created filter: %s (ID: %s)", filter.Name, filter.ID))
		return nil
	},
}

var filterDeleteCmd = &cobra.Command{
	Use:   "delete <FILTER_ID>",
	Short: "Delete a filter",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		if err := client.Filters.Delete(args[0]); err != nil {
			return handleAPIError(err, jsonMode)
		}
		output.Success(fmt.Sprintf("Filter %s deleted", args[0]))
		return nil
	},
}

var filterRunCmd = &cobra.Command{
	Use:   "run <FILTER_ID>",
	Short: "Run a filter and show results",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		filter, err := client.Filters.Get(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		result, err := client.Issues.Search(filter.JQL, api.SearchOptions{MaxResults: limit})
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(result)
			return nil
		}
		if len(result.Issues) == 0 {
			output.Info("No issues found.")
			return nil
		}
		printIssueTable(result.Issues)
		output.Gray(fmt.Sprintf("  Showing %d of %d results", len(result.Issues), result.Total))
		return nil
	},
}
