package cmd

import (
	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueRankCmd.Flags().StringSlice("issues", nil, "Issue keys to rank, in target order (comma-separated or repeatable)")
	issueRankCmd.Flags().String("after", "", "Rank the issues immediately after this anchor issue")
	issueRankCmd.Flags().String("before", "", "Rank the issues immediately before this anchor issue")
	issueRankCmd.Flags().Bool("continue-on-error", true, "Keep going after a per-chunk failure; set false to stop at the first failure")
	issueCmd.AddCommand(issueRankCmd)
	markWrite(issueRankCmd)
}

// issueRankCmd reorders many issues relative to an anchor via the native agile
// /issue/rank endpoint, auto-chunking at AgileMoveCap. To keep the requested
// order stable across chunks, the first chunk ranks against the anchor and each
// later chunk ranks after the previous chunk's last issue.
var issueRankCmd = &cobra.Command{
	Use:   "rank",
	Short: "Reorder multiple issues relative to an anchor issue",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		issues, _ := cmd.Flags().GetStringSlice("issues")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

		if (after == "") == (before == "") {
			output.Error("exactly one of --after or --before is required")
			return SilentErr(ExitBadArgs)
		}

		keys := parsePluralKeys(issues, nil)
		if len(keys) == 0 {
			output.Error("no target issues; provide --issues")
			return SilentErr(ExitBadArgs)
		}

		anchorMode := "after"
		anchor := after
		if before != "" {
			anchorMode = "before"
			anchor = before
		}

		if dryRunOutput("rank issues", map[string]any{"issues": keys, "mode": anchorMode, "anchor": anchor}) {
			return nil
		}

		result := rankOverChunks(client, keys, anchorMode, anchor, continueOnError)
		printBatchResult(result)
		return nil
	},
}

// rankOverChunks ranks keys in AgileMoveCap-sized chunks. The first chunk uses
// the caller's anchor; each later chunk ranks after the previous chunk's last
// issue so the overall requested order holds across chunk boundaries. A chunk
// error marks its members failed; continueOnError stops after the first failure
// and reports the remainder as skipped.
func rankOverChunks(client *api.Client, keys []string, anchorMode, anchor string, continueOnError bool) batchResult {
	result := batchResult{Action: "rank issues", Items: make([]batchItem, 0, len(keys))}
	result.Summary.Total = len(keys)
	chunks := api.ChunkKeys(keys, api.AgileMoveCap)
	stopped := false
	processed := 0
	for _, chunk := range chunks {
		if stopped {
			break
		}
		req := api.RankRequest{Issues: chunk}
		if anchorMode == "before" {
			req.RankBeforeIssue = anchor
		} else {
			req.RankAfterIssue = anchor
		}
		err := client.Issues.Rank(req)
		for _, key := range chunk {
			if err != nil {
				result.Items = append(result.Items, batchItem{Target: key, OK: false, Error: itemErrorFromAPI(err)})
				result.Summary.Failed++
			} else {
				result.Items = append(result.Items, batchItem{Target: key, OK: true})
				result.Summary.Succeeded++
			}
		}
		processed += len(chunk)
		if err != nil {
			if !continueOnError {
				stopped = true
			}
			continue
		}
		// Chain the next chunk after this chunk's last issue so order is stable
		// even when before-mode was requested (subsequent chunks must still land
		// after the issues already placed).
		anchorMode = "after"
		anchor = chunk[len(chunk)-1]
	}
	if processed < len(keys) {
		result.Summary.Skipped = len(keys) - processed
	}
	return result
}
