package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage Jira projects",
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectListCmd.Flags().String("type", "", "Filter by type (software,business,service_desk)")
	projectCmd.AddCommand(projectListCmd)

	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectComponentsCmd)

	projectVersionsCmd.Flags().Bool("released", false, "Show only released versions")
	projectVersionsCmd.Flags().Bool("unreleased", false, "Show only unreleased versions")
	projectCmd.AddCommand(projectVersionsCmd)

	projectCmd.AddCommand(projectIssueTypesCmd)

	projectFieldsCmd.Flags().Bool("custom", false, "Show only custom fields")
	projectCmd.AddCommand(projectFieldsCmd)
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		projectType, _ := cmd.Flags().GetString("type")
		projects, err := client.Projects.List(projectType)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(projects)
			return nil
		}
		if len(projects) == 0 {
			output.Info("No projects found.")
			return nil
		}
		headers := []string{"KEY", "NAME", "TYPE", "LEAD"}
		rows := make([][]string, len(projects))
		for i, p := range projects {
			lead := ""
			if p.Lead != nil {
				lead = p.Lead.DisplayName
			}
			rows[i] = []string{p.Key, p.Name, p.ProjectTypeKey, lead}
		}
		output.Table(headers, rows)
		return nil
	},
}

var projectGetCmd = &cobra.Command{
	Use:   "get <PROJECT_KEY>",
	Short: "Get project details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		project, err := client.Projects.Get(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(project)
			return nil
		}
		fmt.Println()
		output.Bold(fmt.Sprintf("  %s \u2014 %s", project.Key, project.Name))
		output.Gray(fmt.Sprintf("  Type: %s | Style: %s", project.ProjectTypeKey, project.Style))
		if project.Lead != nil {
			output.Gray(fmt.Sprintf("  Lead: %s", project.Lead.DisplayName))
		}
		if project.Description != "" {
			output.Gray(fmt.Sprintf("  Description: %s", project.Description))
		}
		fmt.Println()
		return nil
	},
}

var projectComponentsCmd = &cobra.Command{
	Use:   "components <PROJECT_KEY>",
	Short: "List project components",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		components, err := client.Projects.GetComponents(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(components)
			return nil
		}
		if len(components) == 0 {
			output.Info("No components found.")
			return nil
		}
		headers := []string{"ID", "NAME"}
		rows := make([][]string, len(components))
		for i, c := range components {
			rows[i] = []string{c.ID, c.Name}
		}
		output.Table(headers, rows)
		return nil
	},
}

var projectVersionsCmd = &cobra.Command{
	Use:   "versions <PROJECT_KEY>",
	Short: "List project versions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		released, _ := cmd.Flags().GetBool("released")
		unreleased, _ := cmd.Flags().GetBool("unreleased")
		versions, err := client.Projects.GetVersions(args[0], released, unreleased)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(versions)
			return nil
		}
		if len(versions) == 0 {
			output.Info("No versions found.")
			return nil
		}
		headers := []string{"ID", "NAME", "RELEASED", "RELEASE DATE", "DESCRIPTION"}
		rows := make([][]string, len(versions))
		for i, v := range versions {
			rel := "No"
			if v.Released {
				rel = output.FormatGreen("Yes")
			}
			rows[i] = []string{v.ID, v.Name, rel, v.ReleaseDate, v.Description}
		}
		output.Table(headers, rows)
		return nil
	},
}

var projectIssueTypesCmd = &cobra.Command{
	Use:   "issue-types <PROJECT_KEY>",
	Short: "List issue types for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		types, err := client.Projects.GetIssueTypes(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(types)
			return nil
		}
		headers := []string{"ID", "NAME", "SUBTASK"}
		rows := make([][]string, len(types))
		for i, t := range types {
			subtask := "No"
			if t.Subtask {
				subtask = "Yes"
			}
			rows[i] = []string{t.ID, t.Name, subtask}
		}
		output.Table(headers, rows)
		return nil
	},
}

var projectFieldsCmd = &cobra.Command{
	Use:   "fields [PROJECT_KEY]",
	Short: "List available fields (system and custom)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		customOnly, _ := cmd.Flags().GetBool("custom")

		var fields []api.FieldMeta
		if customOnly {
			fields, err = client.Fields.ListCustom()
		} else {
			fields, err = client.Fields.ListAll()
		}
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(fields)
			return nil
		}
		if len(fields) == 0 {
			output.Info("No fields found.")
			return nil
		}
		headers := []string{"ID", "NAME", "CUSTOM", "TYPE"}
		rows := make([][]string, len(fields))
		for i, f := range fields {
			custom := "No"
			if f.Custom {
				custom = output.FormatCyan("Yes")
			}
			fieldType := ""
			if f.Schema != nil {
				fieldType = f.Schema.Type
				if f.Schema.Items != "" {
					fieldType += "[" + f.Schema.Items + "]"
				}
			}
			rows[i] = []string{f.ID, f.Name, custom, fieldType}
		}
		output.Table(headers, rows)
		return nil
	},
}
