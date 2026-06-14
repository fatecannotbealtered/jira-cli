package cmd

import (
	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	addBatchFlags(issueBulkAssignCmd)
	issueBulkAssignCmd.Flags().String("assignee", "", "Assignee username or 'me' (empty unassigns)")
	issueBulkAssignCmd.Flags().Bool("unassign", false, "Clear the assignee on every target issue")
	issueCmd.AddCommand(issueBulkAssignCmd)
	markWrite(issueBulkAssignCmd)

	addBatchFlags(issueBulkEditCmd)
	issueBulkEditCmd.Flags().String("priority", "", "New priority")
	issueBulkEditCmd.Flags().StringSlice("label", nil, "New labels (replaces existing)")
	issueBulkEditCmd.Flags().String("component", "", "New component")
	issueBulkEditCmd.Flags().String("fix-version", "", "New fix version")
	issueBulkEditCmd.Flags().StringSlice("field", nil, "Custom field (format: 'FieldName=Value', repeatable)")
	issueCmd.AddCommand(issueBulkEditCmd)
	markWrite(issueBulkEditCmd)
}

// issueBulkAssignCmd assigns (or unassigns) many issues in one command. Class B:
// no native bulk assign exists, so it loops client-side PUT /issue/{key}/assignee,
// aggregating per-item results. The external contract is identical to a class-A
// batch — the agent cannot tell the difference.
var issueBulkAssignCmd = &cobra.Command{
	Use:   "bulk-assign",
	Short: "Assign or unassign multiple issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		issues, _ := cmd.Flags().GetStringSlice("issues")
		jql, _ := cmd.Flags().GetString("jql")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
		assignee, _ := cmd.Flags().GetString("assignee")
		unassign, _ := cmd.Flags().GetBool("unassign")

		if unassign {
			assignee = ""
		} else {
			if assignee == "" {
				output.Error("--assignee is required (or pass --unassign to clear)")
				return SilentErr(ExitBadArgs)
			}
			assignee, err = resolveUsername(client, assignee)
			if err != nil {
				return err
			}
		}

		keys, err := resolveIssueTargets(client, issues, jql)
		if err != nil {
			return err
		}

		if dryRunOutput("bulk assign issues", map[string]any{"issues": keys, "assignee": assignee, "unassign": unassign}) {
			return nil
		}

		result := runBatchOverKeys("bulk assign issues", keys, continueOnError, func(key string) error {
			return client.Issues.Assign(key, assignee)
		})
		printBatchResult(result)
		return nil
	},
}

// issueBulkEditCmd applies the same field changes to many issues. Class B: loops
// client-side PUT /issue/{key}, aggregating per-item results. At least one field
// flag must be set, so an empty edit is a usage error rather than a no-op batch.
var issueBulkEditCmd = &cobra.Command{
	Use:   "bulk-edit",
	Short: "Apply the same field changes to multiple issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		issues, _ := cmd.Flags().GetStringSlice("issues")
		jql, _ := cmd.Flags().GetString("jql")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

		// Assemble the shared field set once; the same body is applied to every
		// target issue.
		var fields api.EditIssueFields
		editPreview := map[string]any{}
		hasUpdate := false
		if p, _ := cmd.Flags().GetString("priority"); p != "" {
			fields.Priority = &api.NameRef{Name: p}
			editPreview["priority"] = p
			hasUpdate = true
		}
		if labels, _ := cmd.Flags().GetStringSlice("label"); len(labels) > 0 {
			fields.Labels = labels
			editPreview["labels"] = stableStringSlice(labels)
			hasUpdate = true
		}
		if c, _ := cmd.Flags().GetString("component"); c != "" {
			fields.Components = []api.NameRef{{Name: c}}
			editPreview["component"] = c
			hasUpdate = true
		}
		if fv, _ := cmd.Flags().GetString("fix-version"); fv != "" {
			fields.FixVersions = []api.NameRef{{Name: fv}}
			editPreview["fixVersion"] = fv
			hasUpdate = true
		}
		customFields, _ := cmd.Flags().GetStringSlice("field")
		if len(customFields) > 0 {
			editPreview["customFields"] = stableStringSlice(customFields)
			hasUpdate = true
		}
		if !hasUpdate {
			output.Error("no fields to update; set --priority/--label/--component/--fix-version/--field")
			return SilentErr(ExitBadArgs)
		}

		keys, err := resolveIssueTargets(client, issues, jql)
		if err != nil {
			return err
		}

		// Resolve custom-field IDs once up front so a bad field name fails as a
		// usage error rather than a per-item failure on every issue.
		customBody := map[string]any{"fields": map[string]any{}}
		if len(customFields) > 0 {
			if err := client.Fields.SetCustomFields(customBody, customFields); err != nil {
				output.Error(err.Error())
				return SilentErr(ExitBadArgs)
			}
		}

		if dryRunOutput("bulk edit issues", map[string]any{"issues": keys, "fields": editPreview}) {
			return nil
		}

		result := runBatchOverKeys("bulk edit issues", keys, continueOnError, func(key string) error {
			if err := client.Issues.Edit(key, api.EditIssueRequest{Fields: fields}); err != nil {
				return err
			}
			if len(customFields) > 0 {
				return client.Issues.EditRaw(key, cloneCustomBody(customBody))
			}
			return nil
		})
		printBatchResult(result)
		return nil
	},
}

// cloneCustomBody returns a fresh copy of the resolved custom-field edit body so
// each per-issue PUT carries its own map (the client may mutate it).
func cloneCustomBody(body map[string]any) map[string]any {
	src, _ := body["fields"].(map[string]any)
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return map[string]any{"fields": dst}
}
