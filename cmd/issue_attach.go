package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueAttachCmd.Flags().String("file", "", "File path to upload (required)")
	issueAttachmentsCmd.Flags().String("out", "", "Download attachments into this directory instead of listing")
	issueAttachmentsCmd.Flags().String("id", "", "With --out, download only the attachment with this ID")
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
			return SilentErr(ExitBadArgs)
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
	Short: "List or download attachments of an issue",
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
		if outDir, _ := cmd.Flags().GetString("out"); outDir != "" {
			id, _ := cmd.Flags().GetString("id")
			return downloadAttachments(cmd.Context(), client, attachments, outDir, id)
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

type savedAttachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	MimeType string `json:"mimeType"`
}

func downloadAttachments(ctx context.Context, client *api.Client, attachments []api.Attachment, outDir, id string) error {
	if id != "" {
		var filtered []api.Attachment
		for _, a := range attachments {
			if a.ID == id {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) == 0 {
			output.Error(fmt.Sprintf("no attachment with id %s", id))
			return SilentErr(ExitNotFound)
		}
		attachments = filtered
	}
	if len(attachments) == 0 {
		if jsonMode {
			output.PrintJSON([]savedAttachment{})
			return nil
		}
		output.Info("No attachments.")
		return nil
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	saved := make([]savedAttachment, 0, len(attachments))
	for _, a := range attachments {
		path, err := client.Issues.DownloadAttachment(ctx, a, outDir)
		if err != nil {
			return handleAPIError(err, jsonMode)
		}
		saved = append(saved, savedAttachment{ID: a.ID, Filename: a.Filename, Path: path, MimeType: a.MimeType})
		if !jsonMode {
			output.Success(fmt.Sprintf("Downloaded: %s (%d bytes) -> %s", a.Filename, a.Size, path))
		}
	}
	if jsonMode {
		output.PrintJSON(saved)
	}
	return nil
}
