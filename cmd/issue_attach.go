package cmd

import (
	"fmt"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueAttachCmd.Flags().String("file", "", "File path to upload (required)")
	issueCmd.AddCommand(issueAttachCmd)
	issueCmd.AddCommand(issueAttachmentsCmd)
	markWrite(issueAttachCmd)
}

var issueAttachCmd = &cobra.Command{
	Use:   "attach <ISSUE_KEY>",
	Short: "Upload a file attachment to an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			output.Error("--file is required")
			return ErrSilent
		}
		if dryRunOutput("upload attachment", map[string]any{"key": args[0], "file": filePath}) {
			return nil
		}
		attachments, err := client.Issues.UploadAttachment(cmd.Context(), args[0], filePath)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(attachments)
			return nil
		}
		for _, a := range attachments {
			output.Success(fmt.Sprintf("Uploaded: %s (%d bytes)", a.Filename, a.Size))
		}
		return nil
	},
}

var issueAttachmentsCmd = &cobra.Command{
	Use:   "attachments <ISSUE_KEY>",
	Short: "List attachments of an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		attachments, err := client.Issues.ListAttachments(args[0])
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		if jsonMode {
			output.PrintJSON(attachments)
			return nil
		}
		if len(attachments) == 0 {
			output.Info("No attachments.")
			return nil
		}
		headers := []string{"ID", "FILENAME", "SIZE", "MIME TYPE", "AUTHOR", "CREATED"}
		rows := make([][]string, len(attachments))
		for i, a := range attachments {
			rows[i] = []string{
				a.ID,
				a.Filename,
				fmt.Sprintf("%d", a.Size),
				a.MimeType,
				a.Author.DisplayName,
				formatTime(a.Created),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}
