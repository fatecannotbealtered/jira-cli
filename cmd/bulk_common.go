package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

// batchItem is one per-target outcome in a batch result. target is the natural
// input key (issue key, list index for create) so an agent can zip results back
// to inputs. On failure, error mirrors the top-level envelope's {code,retryable}
// taxonomy.
type batchItem struct {
	Target string          `json:"target"`
	OK     bool            `json:"ok"`
	Key    string          `json:"key,omitempty"`
	URL    string          `json:"url,omitempty"`
	Error  *batchItemError `json:"error,omitempty"`
}

type batchItemError struct {
	Code      output.ErrorCode `json:"code"`
	Message   string           `json:"message"`
	Retryable bool             `json:"retryable"`
}

// batchSummary always reports total/succeeded/failed; skipped counts targets the
// batch never attempted (e.g. after --continue-on-error=false stopped early) so
// the agent can resume.
type batchSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

// batchResult is the aggregated payload every batch command emits. The shape is
// identical for class-A (native bulk endpoint) and class-B (client-side loop)
// commands so an agent cannot and need not tell them apart.
type batchResult struct {
	Action  string       `json:"action"`
	Items   []batchItem  `json:"items"`
	Summary batchSummary `json:"summary"`
}

// itemErrorFromAPI maps a Jira API error onto the per-item {code,retryable}
// taxonomy, identical to the top-level envelope mapping in handleAPIError.
func itemErrorFromAPI(err error) *batchItemError {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		code := output.ErrorCodeFromStatus(apiErr.StatusCode)
		return &batchItemError{Code: code, Message: apiErr.Error(), Retryable: output.RetryableForErrorCode(code)}
	}
	return &batchItemError{Code: output.ErrNetwork, Message: err.Error(), Retryable: true}
}

// parsePluralKeys collects batch targets from a repeatable + comma-separated
// StringSlice flag and any positional args, trims blanks, de-duplicates, and
// preserves first-seen order so result items zip back to inputs deterministically.
func parsePluralKeys(flagValues []string, positional []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			k := strings.TrimSpace(part)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, v := range flagValues {
		add(v)
	}
	for _, v := range positional {
		add(v)
	}
	return out
}

// resolveIssueTargets builds the batch target set from explicit --issues plus a
// --jql selection. An empty result is a usage error (E_VALIDATION, exit 2), not
// a silent no-op. A JQL match that exceeds the search cap is rejected rather than
// silently truncated, so the confirm token binds the complete set.
func resolveIssueTargets(client *api.Client, issues []string, jql string) ([]string, error) {
	keys := parsePluralKeys(issues, nil)
	if jql != "" {
		found, truncated, err := client.Issues.SearchAll(jql, []string{"key"})
		if err != nil {
			return nil, handleAPIError(err, jsonMode)
		}
		if truncated {
			output.Error(fmt.Sprintf("JQL matched more than the %d-issue cap; narrow the query so the bulk operation does not silently skip issues", api.SearchAllCap))
			return nil, SilentErr(ExitBadArgs)
		}
		jqlKeys := make([]string, len(found))
		for i, issue := range found {
			jqlKeys[i] = issue.Key
		}
		keys = parsePluralKeys(append(keys, jqlKeys...), nil)
	}
	if len(keys) == 0 {
		output.Error("no target issues; provide --issues or --jql")
		return nil, SilentErr(ExitBadArgs)
	}
	return keys, nil
}

// runBatchOverKeys applies fn to each key, aggregating per-item results and a
// summary. continueOnError=false stops at the first failure, leaving already
// applied items applied (no rollback) and reporting the unattempted remainder as
// skipped so the agent can resume. fn returns nil on success or an API/transport
// error on failure.
func runBatchOverKeys(action string, keys []string, continueOnError bool, fn func(key string) error) batchResult {
	result := batchResult{Action: action, Items: make([]batchItem, 0, len(keys))}
	result.Summary.Total = len(keys)
	stopped := false
	for i, key := range keys {
		if stopped {
			result.Summary.Skipped = len(keys) - i
			break
		}
		if err := fn(key); err != nil {
			result.Items = append(result.Items, batchItem{Target: key, OK: false, Error: itemErrorFromAPI(err)})
			result.Summary.Failed++
			if !continueOnError {
				stopped = true
			}
			continue
		}
		result.Items = append(result.Items, batchItem{Target: key, OK: true})
		result.Summary.Succeeded++
	}
	return result
}

// addBatchFlags registers the flags every issue batch command shares: target
// selection (--issues / --jql) and --continue-on-error (best-effort by default).
func addBatchFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("issues", nil, "Issue keys (comma-separated or repeatable)")
	cmd.Flags().String("jql", "", "JQL query selecting target issues (combined with --issues)")
	cmd.Flags().Bool("continue-on-error", true, "Keep going after a per-item failure; set false to stop at the first failure")
}

// printBatchResult emits the aggregated batch result. Top-level ok stays true
// when the batch ran and produced a result, even with per-item failures; per-item
// status lives in items[]. Text mode prints one line per item plus a summary.
func printBatchResult(result batchResult) {
	if jsonMode {
		output.PrintJSON(result)
		return
	}
	for _, item := range result.Items {
		if item.OK {
			output.Success(item.Target)
		} else {
			output.Warn(fmt.Sprintf("%s: %s", item.Target, item.Error.Message))
		}
	}
	fmt.Println()
	output.Info(fmt.Sprintf("Done: %d succeeded, %d failed, %d skipped (of %d total)",
		result.Summary.Succeeded, result.Summary.Failed, result.Summary.Skipped, result.Summary.Total))
}
