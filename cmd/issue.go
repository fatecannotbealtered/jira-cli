package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage Jira issues",
}

func init() {
	rootCmd.AddCommand(issueCmd)

	// issue get
	issueGetCmd.Flags().StringSlice("expand", nil, "Expand fields (changelog,renderedFields,transitions)")
	issueGetCmd.Flags().Bool("raw", false, "Return raw Jira API response (default: flat JSON)")
	issueGetCmd.Flags().StringSlice("fields", nil, "Output only these fields (flat JSON only, e.g. key,summary,status,assignee)")
	issueCmd.AddCommand(issueGetCmd)

	// issue list
	issueListCmd.Flags().String("project", "", "Project key (required)")
	issueListCmd.Flags().String("status", "", "Filter by status")
	issueListCmd.Flags().String("assignee", "", "Filter by assignee (use 'me' for current user)")
	issueListCmd.Flags().String("type", "", "Filter by issue type")
	issueListCmd.Flags().String("label", "", "Filter by label")
	issueListCmd.Flags().String("priority", "", "Filter by priority")
	issueListCmd.Flags().Int("limit", 50, "Max results (1-100)")
	issueListCmd.Flags().String("order-by", "", "Order by field (created,updated,priority,status)")
	issueListCmd.Flags().Bool("raw", false, "Return raw Jira API response (default: flat JSON)")
	issueListCmd.Flags().StringSlice("fields", nil, "Output only these fields (flat JSON only, e.g. key,summary,status,assignee)")
	issueCmd.AddCommand(issueListCmd)

	// issue create
	issueCreateCmd.Flags().String("project", "", "Project key (required)")
	issueCreateCmd.Flags().String("summary", "", "Issue summary (required)")
	issueCreateCmd.Flags().String("type", "Task", "Issue type")
	issueCreateCmd.Flags().String("description", "", "Issue description")
	issueCreateCmd.Flags().String("assignee", "", "Assignee username or 'me'")
	issueCreateCmd.Flags().String("priority", "", "Priority (Highest,High,Medium,Low,Lowest)")
	issueCreateCmd.Flags().StringSlice("label", nil, "Labels (comma-separated)")
	issueCreateCmd.Flags().String("component", "", "Component name")
	issueCreateCmd.Flags().String("fix-version", "", "Fix version name")
	issueCreateCmd.Flags().String("parent", "", "Parent issue key (creates subtask)")
	issueCreateCmd.Flags().StringSlice("field", nil, "Custom field (format: 'FieldName=Value', repeatable)")
	issueCmd.AddCommand(issueCreateCmd)

	// issue edit
	issueEditCmd.Flags().String("summary", "", "New summary")
	issueEditCmd.Flags().String("description", "", "New description")
	issueEditCmd.Flags().String("assignee", "", "New assignee username or 'me'")
	issueEditCmd.Flags().String("priority", "", "New priority")
	issueEditCmd.Flags().StringSlice("label", nil, "New labels")
	issueEditCmd.Flags().String("component", "", "New component")
	issueEditCmd.Flags().String("fix-version", "", "New fix version")
	issueEditCmd.Flags().StringSlice("field", nil, "Custom field (format: 'FieldName=Value', repeatable)")
	issueCmd.AddCommand(issueEditCmd)

	// issue delete
	issueCmd.AddCommand(issueDeleteCmd)

	markWrite(issueCreateCmd)
	markWrite(issueEditCmd)
	markWrite(issueDeleteCmd)
}

var issueGetCmd = &cobra.Command{
	Use:   "get <ISSUE_KEY>",
	Short: "Get issue details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg, err := newClient()
		if err != nil {
			return err
		}
		expand, _ := cmd.Flags().GetStringSlice("expand")
		issue, err := client.Issues.Get(args[0], expand)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			raw, _ := cmd.Flags().GetBool("raw")
			fields, _ := cmd.Flags().GetStringSlice("fields")
			printIssueJSON(issue, raw, fields)
			return nil
		}
		printIssueDetail(issue, cfg.Host)
		return nil
	},
}

var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			output.Error("--project is required")
			return SilentErr(ExitBadArgs)
		}
		limit, _ := cmd.Flags().GetInt("limit")
		if limit < 1 || limit > 100 {
			output.Error("--limit must be between 1 and 100")
			return SilentErr(ExitBadArgs)
		}
		opts := api.IssueListOptions{
			Project:  project,
			Status:   mustGetString(cmd, "status"),
			Assignee: mustGetString(cmd, "assignee"),
			Type:     mustGetString(cmd, "type"),
			Label:    mustGetString(cmd, "label"),
			Priority: mustGetString(cmd, "priority"),
			Limit:    limit,
			OrderBy:  mustGetString(cmd, "order-by"),
		}
		issues, err := client.Issues.List(opts)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			raw, _ := cmd.Flags().GetBool("raw")
			fields, _ := cmd.Flags().GetStringSlice("fields")
			printIssuesJSON(issues, raw, fields)
			return nil
		}
		if len(issues) == 0 {
			output.Info("No issues found.")
			return nil
		}
		printIssueTable(issues)
		return nil
	},
}

var issueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg, err := newClient()
		if err != nil {
			return err
		}
		project, _ := cmd.Flags().GetString("project")
		summary, _ := cmd.Flags().GetString("summary")
		if project == "" || summary == "" {
			output.Error("--project and --summary are required")
			return SilentErr(ExitBadArgs)
		}

		issueType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")
		assignee, _ := cmd.Flags().GetString("assignee")
		priority, _ := cmd.Flags().GetString("priority")
		labels, _ := cmd.Flags().GetStringSlice("label")
		component, _ := cmd.Flags().GetString("component")
		fixVersion, _ := cmd.Flags().GetString("fix-version")
		parent, _ := cmd.Flags().GetString("parent")

		fields := api.CreateIssueFields{
			Project:   api.ProjectRef{Key: project},
			Summary:   summary,
			IssueType: api.IssueTypeRef{Name: issueType},
		}
		if description != "" {
			fields.Description = description
		}
		if assignee != "" {
			username, err := resolveUsername(client, assignee)
			if err != nil {
				return err
			}
			fields.Assignee = &api.NameRef{Name: username}
		}
		if priority != "" {
			fields.Priority = &api.NameRef{Name: priority}
		}
		if len(labels) > 0 {
			fields.Labels = labels
		}
		if component != "" {
			fields.Components = []api.NameRef{{Name: component}}
		}
		if fixVersion != "" {
			fields.FixVersions = []api.NameRef{{Name: fixVersion}}
		}
		if parent != "" {
			fields.Parent = &api.KeyRef{Key: parent}
		}

		if dryRunOutput("create issue", map[string]any{"project": project, "summary": summary, "type": issueType}) {
			return nil
		}

		ref, err := client.Issues.Create(api.CreateIssueRequest{Fields: fields})
		if err != nil {
			return handleAPIError(err, jsonMode)
		}

		// Handle custom fields
		customFields, _ := cmd.Flags().GetStringSlice("field")
		if len(customFields) > 0 {
			editBody := map[string]any{
				"fields": map[string]any{},
			}
			if err := client.Fields.SetCustomFields(editBody, customFields); err != nil {
				if jsonMode {
					output.PrintJSON(map[string]any{
						"key":     ref.Key,
						"id":      ref.ID,
						"warning": "created but custom fields failed: " + err.Error(),
					})
					return nil
				}
				output.Success(fmt.Sprintf("Created %s", ref.Key))
				output.Warn("Custom fields failed: " + err.Error())
				return nil
			}
			if err := client.Issues.EditRaw(ref.Key, editBody); err != nil {
				if jsonMode {
					output.PrintJSON(map[string]any{
						"key":     ref.Key,
						"id":      ref.ID,
						"warning": "created but custom fields failed: " + err.Error(),
					})
					return nil
				}
				output.Success(fmt.Sprintf("Created %s", ref.Key))
				output.Warn("Custom fields failed: " + err.Error())
				return nil
			}
		}

		url := cfg.Host + "/browse/" + ref.Key
		if jsonMode {
			output.PrintJSON(map[string]string{
				"key": ref.Key,
				"id":  ref.ID,
				"url": url,
			})
			return nil
		}
		output.Success(fmt.Sprintf("Created %s", ref.Key))
		output.Info(url)
		return nil
	},
}

var issueEditCmd = &cobra.Command{
	Use:   "edit <ISSUE_KEY>",
	Short: "Edit an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		var fields api.EditIssueFields
		hasUpdate := false
		if s, _ := cmd.Flags().GetString("summary"); s != "" {
			fields.Summary = s
			hasUpdate = true
		}
		if d, _ := cmd.Flags().GetString("description"); d != "" {
			fields.Description = d
			hasUpdate = true
		}
		if a, _ := cmd.Flags().GetString("assignee"); a != "" {
			username, err := resolveUsername(client, a)
			if err != nil {
				return err
			}
			fields.Assignee = &api.NameRef{Name: username}
			hasUpdate = true
		}
		if p, _ := cmd.Flags().GetString("priority"); p != "" {
			fields.Priority = &api.NameRef{Name: p}
			hasUpdate = true
		}
		if labels, _ := cmd.Flags().GetStringSlice("label"); len(labels) > 0 {
			fields.Labels = labels
			hasUpdate = true
		}
		if c, _ := cmd.Flags().GetString("component"); c != "" {
			fields.Components = []api.NameRef{{Name: c}}
			hasUpdate = true
		}
		if fv, _ := cmd.Flags().GetString("fix-version"); fv != "" {
			fields.FixVersions = []api.NameRef{{Name: fv}}
			hasUpdate = true
		}
		customFields, _ := cmd.Flags().GetStringSlice("field")
		if len(customFields) > 0 {
			hasUpdate = true
		}
		if !hasUpdate {
			output.Error("no fields to update")
			return SilentErr(ExitBadArgs)
		}

		if dryRunOutput("edit issue", map[string]any{"key": args[0]}) {
			return nil
		}

		if err := client.Issues.Edit(args[0], api.EditIssueRequest{Fields: fields}); err != nil {
			return handleAPIError(err, jsonMode)
		}

		if len(customFields) > 0 {
			editBody := map[string]any{
				"fields": map[string]any{},
			}
			if err := client.Fields.SetCustomFields(editBody, customFields); err != nil {
				if jsonMode {
					output.PrintJSON(map[string]string{"key": args[0], "status": "updated", "warning": "custom fields failed: " + err.Error()})
					return nil
				}
				output.Success(fmt.Sprintf("Updated %s", args[0]))
				output.Warn("Custom fields failed: " + err.Error())
				return nil
			}
			if err := client.Issues.EditRaw(args[0], editBody); err != nil {
				if jsonMode {
					output.PrintJSON(map[string]string{"key": args[0], "status": "updated", "warning": "custom fields failed: " + err.Error()})
					return nil
				}
				output.Success(fmt.Sprintf("Updated %s", args[0]))
				output.Warn("Custom fields failed: " + err.Error())
				return nil
			}
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"key": args[0], "status": "updated"})
			return nil
		}
		output.Success(fmt.Sprintf("Updated %s", args[0]))
		return nil
	},
}

var issueDeleteCmd = &cobra.Command{
	Use:   "delete <ISSUE_KEY>",
	Short: "Delete an issue (requires confirmation or --force)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		key := args[0]
		if dryRunOutput("delete issue", map[string]any{"key": key}) {
			return nil
		}
		if !confirmAction(fmt.Sprintf("Type the issue key to confirm deletion (%s)", key), key) {
			output.Warn("Deletion cancelled.")
			return SilentErr(ExitBadArgs)
		}
		if err := client.Issues.Delete(key); err != nil {
			return handleAPIError(err, jsonMode)
		}
		output.Success(fmt.Sprintf("Deleted %s", key))
		return nil
	},
}

// printIssueDetail prints issue details in a structured format.
func printIssueDetail(issue *api.Issue, host string) {
	f := issue.Fields
	fmt.Println()

	typeName := f.IssueType.Name
	priorityName := ""
	if f.Priority != nil {
		priorityName = f.Priority.Name
	}
	header := fmt.Sprintf("  %s · %s · %s",
		output.FormatCyanBold(issue.Key),
		output.FormatGray(typeName),
		output.PriorityBadge(priorityName))
	fmt.Println(header)
	output.Gray("  ────────────────────────────────────────────────")

	printField := func(label, value string) {
		if value != "" {
			fmt.Printf("  %-12s %s\n", output.FormatGray(label), value)
		}
	}

	printField("Summary", f.Summary)
	printField("Status", output.StatusBadge(f.Status.Name))
	if f.Assignee != nil {
		printField("Assignee", f.Assignee.DisplayName)
	}
	if f.Reporter != nil {
		printField("Reporter", f.Reporter.DisplayName)
	}
	printField("Created", formatTime(f.Created))
	printField("Updated", formatTime(f.Updated))
	if len(f.Labels) > 0 {
		printField("Labels", strings.Join(f.Labels, ", "))
	}
	if len(f.Components) > 0 {
		names := make([]string, len(f.Components))
		for i, c := range f.Components {
			names[i] = c.Name
		}
		printField("Components", strings.Join(names, ", "))
	}
	if len(f.FixVersions) > 0 {
		names := make([]string, len(f.FixVersions))
		for i, v := range f.FixVersions {
			names[i] = v.Name
		}
		printField("Fix Ver", strings.Join(names, ", "))
	}
	printField("URL", host+"/browse/"+issue.Key)

	desc := api.ADFToText(f.Description)
	if desc != "" {
		fmt.Println()
		output.Gray("  Description")
		output.Gray("  ────────────────────────────────────────────────")
		if len(desc) > 800 {
			desc = desc[:800] + "…"
		}
		for _, line := range strings.Split(desc, "\n") {
			fmt.Println("  " + line)
		}
	}
	fmt.Println()
}

// printIssueTable prints issues in a table format.
func printIssueTable(issues []api.Issue) {
	headers := []string{"KEY", "SUMMARY", "STATUS", "TYPE", "ASSIGNEE", "PRIORITY", "UPDATED"}
	rows := make([][]string, len(issues))
	for i, issue := range issues {
		f := issue.Fields
		assignee := ""
		if f.Assignee != nil {
			assignee = f.Assignee.DisplayName
		}
		priority := ""
		if f.Priority != nil {
			priority = output.PriorityBadge(f.Priority.Name)
		}
		summary := f.Summary
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}
		rows[i] = []string{
			issue.Key,
			summary,
			output.StatusBadge(f.Status.Name),
			f.IssueType.Name,
			assignee,
			priority,
			formatTime(f.Updated),
		}
	}
	output.Table(headers, rows)
}

func formatTime(t string) string {
	if t == "" {
		return ""
	}
	parsed, err := time.Parse("2006-01-02T15:04:05.000-0700", t)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, t)
		if err != nil {
			if len(t) >= 10 {
				return t[:10]
			}
			return t
		}
	}
	return parsed.Format("2006-01-02 15:04")
}

func mustGetString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}
