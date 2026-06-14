package cmd

import (
	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/spf13/cobra"
)

func init() {
	addBatchFlags(issueBacklogMoveCmd)
	issueCmd.AddCommand(issueBacklogMoveCmd)
	markWrite(issueBacklogMoveCmd)
}

// issueBacklogMoveCmd is the symmetric inverse of `sprint move`: it pulls issues
// out of any sprint back to the backlog via the native agile backlog endpoint,
// auto-chunking at AgileMoveCap so a >50 batch never 400s upstream.
var issueBacklogMoveCmd = &cobra.Command{
	Use:   "backlog-move",
	Short: "Move issues out of their sprint back to the backlog",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := newClient()
		if err != nil {
			return err
		}
		issues, _ := cmd.Flags().GetStringSlice("issues")
		jql, _ := cmd.Flags().GetString("jql")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

		keys, err := resolveIssueTargets(client, issues, jql)
		if err != nil {
			return err
		}

		// Token binds the whole resolved target set: adding/removing a key
		// changes detail and invalidates the dry-run token with E_CONFLICT.
		if dryRunOutput("move issues to backlog", map[string]any{"issues": keys}) {
			return nil
		}

		// Class A: chunk to the agile cap and submit each chunk natively, then
		// map a chunk-level failure back onto the affected items so one bad chunk
		// does not fail the whole command (subject to --continue-on-error).
		result := runBatchOverChunks("move issues to backlog", keys, continueOnError,
			func(chunk []string) error {
				return client.Sprints.MoveToBacklog(chunk)
			})
		printBatchResult(result)
		return nil
	},
}

// runBatchOverChunks applies fn to consecutive chunks of keys (sized at
// AgileMoveCap) and maps each chunk's outcome onto its member items. A chunk-level
// error marks every key in that chunk failed; chunking stays invisible in the
// contract — one aggregated items[]/summary across all chunks. continueOnError
// stops after the first failed chunk, reporting the rest as skipped.
func runBatchOverChunks(action string, keys []string, continueOnError bool, fn func(chunk []string) error) batchResult {
	result := batchResult{Action: action, Items: make([]batchItem, 0, len(keys))}
	result.Summary.Total = len(keys)
	chunks := api.ChunkKeys(keys, api.AgileMoveCap)
	stopped := false
	processed := 0
	for _, chunk := range chunks {
		if stopped {
			break
		}
		err := fn(chunk)
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
		if err != nil && !continueOnError {
			stopped = true
		}
	}
	if processed < len(keys) {
		result.Summary.Skipped = len(keys) - processed
	}
	return result
}
