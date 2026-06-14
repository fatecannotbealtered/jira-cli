package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueLinkCmd.Flags().String("to", "", "Target issue key (required)")
	issueLinkCmd.Flags().String("type", "", "Link type name (required)")
	issueLinkCmd.AddCommand(issueLinkListCmd)
	issueCmd.AddCommand(issueLinkCmd)

	issueCmd.AddCommand(issueUnlinkCmd)
	issueCmd.AddCommand(issueLinkTypesCmd)

	issueRemoteLinkCmd.Flags().String("url", "", "Remote URL (required)")
	issueRemoteLinkCmd.Flags().String("title", "", "Link title (required)")
	issueCmd.AddCommand(issueRemoteLinkCmd)

	issueCmd.AddCommand(issueRemoteLinksCmd)

	markWrite(issueLinkCmd)
	markWrite(issueUnlinkCmd)
	markWrite(issueRemoteLinkCmd)
}

var issueLinkCmd = &cobra.Command{
	Use:   "link <ISSUE_KEY>",
	Short: "Link two issues",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		to, _ := cmd.Flags().GetString("to")
		if to == "" {
			output.Error("--to is required")
			return SilentErr(ExitBadArgs)
		}
		linkType, _ := cmd.Flags().GetString("type")
		if linkType == "" {
			output.Error("--type is required (use 'jira-cli issue link-types' to see available types)")
			return SilentErr(ExitBadArgs)
		}

		req := api.IssueLinkRequest{
			Type:         api.LinkTypeRef{Name: linkType},
			InwardIssue:  api.KeyRef{Key: args[0]},
			OutwardIssue: api.KeyRef{Key: to},
		}
		if dryRunOutput("link issues", map[string]any{"from": args[0], "to": to, "type": linkType}) {
			return nil
		}
		if err := client.Issues.CreateLink(req); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"from": args[0], "to": to, "type": linkType, "status": "linked"})
			return nil
		}
		output.Success(fmt.Sprintf("Linked %s → %s (%s)", args[0], to, linkType))
		return nil
	},
}

var issueLinkListCmd = &cobra.Command{
	Use:   "list <ISSUE_KEY>",
	Short: "List an issue's links",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		links, err := client.Issues.GetLinks(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			printIssueLinksJSON(links)
			return nil
		}
		if len(links) == 0 {
			output.Info("No links.")
			return nil
		}
		headers := []string{"LINK ID", "DIRECTION", "TYPE", "LINKED KEY", "SUMMARY", "STATUS"}
		rows := make([][]string, len(links))
		for i := range links {
			row := toFlatIssueLink(&links[i])
			summary := row.LinkedSummary
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}
			rows[i] = []string{row.ID, row.Direction, row.Type, row.LinkedKey, summary, row.LinkedStatus}
		}
		output.Table(headers, rows)
		return nil
	},
}

var issueUnlinkCmd = &cobra.Command{
	Use:   "unlink <LINK_ID>",
	Short: "Remove an issue link",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		if dryRunOutput("unlink issues", map[string]any{"linkId": args[0]}) {
			return nil
		}
		if err := client.Issues.DeleteLink(args[0]); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"linkId": args[0], "status": "removed"})
			return nil
		}
		output.Success(fmt.Sprintf("Link %s removed", args[0]))
		return nil
	},
}

var issueLinkTypesCmd = &cobra.Command{
	Use:   "link-types",
	Short: "List available issue link types",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		types, err := client.Issues.GetLinkTypes()
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(types)
			return nil
		}
		headers := []string{"ID", "NAME", "INWARD", "OUTWARD"}
		rows := make([][]string, len(types))
		for i, t := range types {
			rows[i] = []string{t.ID, t.Name, t.Inward, t.Outward}
		}
		output.Table(headers, rows)
		return nil
	},
}

var issueRemoteLinkCmd = &cobra.Command{
	Use:   "remote-link <ISSUE_KEY>",
	Short: "Add a remote link to an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		u, _ := cmd.Flags().GetString("url")
		title, _ := cmd.Flags().GetString("title")
		if u == "" || title == "" {
			output.Error("--url and --title are required")
			return SilentErr(ExitBadArgs)
		}
		req := api.RemoteLinkRequest{Object: api.RemoteLinkObject{URL: u, Title: title}}
		if dryRunOutput("add remote link", map[string]any{"key": args[0], "url": u, "title": title}) {
			return nil
		}
		link, err := client.Issues.AddRemoteLink(args[0], req)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(link)
			return nil
		}
		output.Success(fmt.Sprintf("Remote link added (ID: %d)", link.ID))
		return nil
	},
}

var issueRemoteLinksCmd = &cobra.Command{
	Use:   "remote-links <ISSUE_KEY>",
	Short: "List remote links of an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		links, err := client.Issues.ListRemoteLinks(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(links)
			return nil
		}
		if len(links) == 0 {
			output.Info("No remote links.")
			return nil
		}
		headers := []string{"ID", "TITLE", "URL"}
		rows := make([][]string, len(links))
		for i, l := range links {
			rows[i] = []string{fmt.Sprintf("%d", l.ID), l.Object.Title, l.Object.URL}
		}
		output.Table(headers, rows)
		return nil
	},
}
