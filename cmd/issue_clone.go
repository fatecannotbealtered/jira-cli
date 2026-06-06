package cmd

import (
	"fmt"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueCloneCmd.Flags().String("summary", "", "Override summary (default: 'Clone of <original>')")
	issueCloneCmd.Flags().String("project", "", "Target project (default: same project)")
	issueCloneCmd.Flags().Bool("with-links", false, "Copy issue links")
	issueCmd.AddCommand(issueCloneCmd)
	markWrite(issueCloneCmd)
}

var issueCloneCmd = &cobra.Command{
	Use:   "clone <ISSUE_KEY>",
	Short: "Clone an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg, err := newClient()
		if err != nil {
			return err
		}
		sourceKey := args[0]

		source, err := client.Issues.Get(sourceKey, nil)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}

		summary, _ := cmd.Flags().GetString("summary")
		if summary == "" {
			summary = "Clone of " + source.Fields.Summary
		}

		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			// Try to get project from the source issue's self URL or key
			if source.Self != "" {
				// Extract from self URL: .../rest/api/2/issue/PROJECT-123
				for i := len(source.Self) - 1; i >= 0; i-- {
					if source.Self[i] == '/' {
						rest := source.Self[i+1:]
						if dashIdx := strings.Index(rest, "-"); dashIdx > 0 {
							project = rest[:dashIdx]
						}
						break
					}
				}
			}
			if project == "" {
				// Fallback: parse from issue key
				if dashIdx := strings.Index(sourceKey, "-"); dashIdx > 0 {
					project = sourceKey[:dashIdx]
				}
			}
		}

		fields := api.CreateIssueFields{
			Project:   api.ProjectRef{Key: project},
			Summary:   summary,
			IssueType: api.IssueTypeRef{Name: source.Fields.IssueType.Name},
		}

		if source.Fields.Priority != nil {
			fields.Priority = &api.NameRef{Name: source.Fields.Priority.Name}
		}
		if len(source.Fields.Labels) > 0 {
			fields.Labels = source.Fields.Labels
		}
		if len(source.Fields.Components) > 0 {
			components := make([]api.NameRef, len(source.Fields.Components))
			for i, c := range source.Fields.Components {
				components[i] = api.NameRef{Name: c.Name}
			}
			fields.Components = components
		}
		if len(source.Fields.FixVersions) > 0 {
			versions := make([]api.NameRef, len(source.Fields.FixVersions))
			for i, v := range source.Fields.FixVersions {
				versions[i] = api.NameRef{Name: v.Name}
			}
			fields.FixVersions = versions
		}
		if source.Fields.Description != nil {
			fields.Description = api.ADFToText(source.Fields.Description)
		}
		if source.Fields.Assignee != nil && source.Fields.Assignee.Name != "" {
			fields.Assignee = &api.NameRef{Name: source.Fields.Assignee.Name}
		}

		withLinks, _ := cmd.Flags().GetBool("with-links")
		if dryRunOutput("clone issue", map[string]any{"source": sourceKey, "project": project, "summary": summary, "withLinks": withLinks, "fields": fields}) {
			return nil
		}

		ref, err := client.Issues.Create(api.CreateIssueRequest{Fields: fields})
		if err != nil {
			return handleAPIError(err, jsonMode)
		}

		var linkErrors []string
		if withLinks && len(source.Fields.IssueLinks) > 0 {
			for _, link := range source.Fields.IssueLinks {
				var req api.IssueLinkRequest
				req.Type = api.LinkTypeRef{Name: link.Type.Name}
				if link.OutwardIssue != nil {
					req.InwardIssue = api.KeyRef{Key: ref.Key}
					req.OutwardIssue = api.KeyRef{Key: link.OutwardIssue.Key}
				} else if link.InwardIssue != nil {
					req.InwardIssue = api.KeyRef{Key: link.InwardIssue.Key}
					req.OutwardIssue = api.KeyRef{Key: ref.Key}
				} else {
					continue
				}
				if err := client.Issues.CreateLink(req); err != nil {
					linkErrors = append(linkErrors, err.Error())
				}
			}
		}

		url := cfg.Host + "/browse/" + ref.Key
		if jsonMode {
			result := map[string]any{
				"key":        ref.Key,
				"id":         ref.ID,
				"url":        url,
				"clonedFrom": sourceKey,
			}
			if len(linkErrors) > 0 {
				result["linkErrors"] = linkErrors
			}
			output.PrintJSON(result)
			return nil
		}

		output.Success(fmt.Sprintf("Cloned %s → %s", sourceKey, ref.Key))
		output.Info(url)
		if len(linkErrors) > 0 {
			output.Warn(fmt.Sprintf("%d link(s) failed to copy", len(linkErrors)))
		}
		return nil
	},
}
