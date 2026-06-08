package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var issueCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage issue comments",
}

func init() {
	commentAddCmd.Flags().String("body", "", "Comment body (required)")
	issueCommentCmd.AddCommand(commentAddCmd)

	commentListCmd.Flags().Int("limit", 20, "Max comments to return")
	issueCommentCmd.AddCommand(commentListCmd)

	commentDeleteCmd.Flags().String("id", "", "Comment ID (required)")
	issueCommentCmd.AddCommand(commentDeleteCmd)

	issueCmd.AddCommand(issueCommentCmd)

	markWrite(commentAddCmd)
	markWrite(commentDeleteCmd)
}

var commentAddCmd = &cobra.Command{
	Use:   "add <ISSUE_KEY>",
	Short: "Add a comment to an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		body, _ := cmd.Flags().GetString("body")
		if body == "" {
			output.Error("--body cannot be empty")
			return SilentErr(ExitBadArgs)
		}
		if dryRunOutput("add comment", map[string]any{"key": args[0], "body": body}) {
			return nil
		}
		comment, err := client.Issues.AddComment(args[0], body)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(toFlatComment(*comment))
			return nil
		}
		output.Success(fmt.Sprintf("Comment added (ID: %s) at %s", comment.ID, formatTime(comment.Created)))
		return nil
	},
}

var commentListCmd = &cobra.Command{
	Use:   "list <ISSUE_KEY>",
	Short: "List comments on an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		comments, err := client.Issues.ListComments(args[0], limit)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			flat := make([]flatComment, len(comments))
			for i := range comments {
				flat[i] = toFlatComment(comments[i])
			}
			output.PrintJSON(flat)
			return nil
		}
		if len(comments) == 0 {
			output.Info("No comments found.")
			return nil
		}
		headers := []string{"ID", "AUTHOR", "CREATED", "BODY"}
		rows := make([][]string, len(comments))
		for i, c := range comments {
			body := api.ADFToText(c.Body)
			if len(body) > 200 {
				body = body[:197] + "..."
			}
			rows[i] = []string{c.ID, c.Author.DisplayName, formatTime(c.Created), body}
		}
		output.Table(headers, rows)
		return nil
	},
}

type flatComment struct {
	ID        string   `json:"id"`
	Author    string   `json:"author,omitempty"`
	Created   string   `json:"created,omitempty"`
	Updated   string   `json:"updated,omitempty"`
	Body      string   `json:"body,omitempty"`
	Untrusted []string `json:"_untrusted,omitempty"`
}

func toFlatComment(comment api.Comment) flatComment {
	body := api.ADFToText(comment.Body)
	if body == "" {
		if s, ok := comment.Body.(string); ok {
			body = s
		}
	}
	result := flatComment{
		ID:      comment.ID,
		Author:  comment.Author.Name,
		Created: isoUTC(comment.Created),
		Updated: isoUTC(comment.Updated),
		Body:    body,
	}
	if result.Body != "" {
		result.Untrusted = []string{"body"}
	}
	return result
}

var commentDeleteCmd = &cobra.Command{
	Use:   "delete <ISSUE_KEY>",
	Short: "Delete a comment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			output.Error("--id is required")
			return SilentErr(ExitBadArgs)
		}
		if dryRunOutput("delete comment", map[string]any{"key": args[0], "commentId": id}) {
			return nil
		}
		if err := client.Issues.DeleteComment(args[0], id); err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(map[string]string{"issueKey": args[0], "commentId": id, "status": "deleted"})
			return nil
		}
		output.Success(fmt.Sprintf("Comment %s deleted", id))
		return nil
	},
}
