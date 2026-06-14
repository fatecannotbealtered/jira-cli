package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	issueBulkCreateCmd.Flags().String("file", "", "Path to a JSON file: array of issue specs (required)")
	issueBulkCreateCmd.Flags().Bool("continue-on-error", true, "Keep going after a per-item failure; set false to stop at the first failure")
	issueCmd.AddCommand(issueBulkCreateCmd)
	markWrite(issueBulkCreateCmd)
}

// bulkCreateSpec is one issue to create from the --file input. Mirrors the
// `issue create` flags so the file format is predictable; project, summary, and
// type are the required minimum (type defaults to Task).
type bulkCreateSpec struct {
	Project     string   `json:"project"`
	Summary     string   `json:"summary"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Assignee    string   `json:"assignee"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	Component   string   `json:"component"`
	FixVersion  string   `json:"fixVersion"`
	Parent      string   `json:"parent"`
}

// issueBulkCreateCmd creates many issues from one file via the native platform
// /issue/bulk endpoint, auto-chunking at AgileMoveCap. One command, one confirm
// token, one aggregated items[]/summary; the target key for each item is its
// 0-based position in the file so the agent can zip created keys back to inputs.
var issueBulkCreateCmd = &cobra.Command{
	Use:   "bulk-create --file <FILE>",
	Short: "Create multiple issues from a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg, err := newClient()
		if err != nil {
			return err
		}
		file, _ := cmd.Flags().GetString("file")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
		if file == "" {
			output.Error("--file is required")
			return SilentErr(ExitBadArgs)
		}
		specs, err := loadBulkCreateSpecs(file)
		if err != nil {
			output.Error(err.Error())
			return SilentErr(ExitBadArgs)
		}
		if len(specs) == 0 {
			output.Error("file contains no issue specs")
			return SilentErr(ExitBadArgs)
		}

		// Validate every spec up front so a bad row is a usage error, not a
		// half-applied batch. Build the per-index detail the token binds to.
		previews := make([]map[string]any, len(specs))
		for i, spec := range specs {
			if spec.Project == "" || spec.Summary == "" {
				output.Error(fmt.Sprintf("spec %d: project and summary are required", i))
				return SilentErr(ExitBadArgs)
			}
			previews[i] = map[string]any{
				"index": i, "project": spec.Project, "summary": spec.Summary, "type": specType(spec),
			}
		}

		if dryRunOutput("bulk create issues", map[string]any{"total": len(specs), "specs": previews}) {
			return nil
		}

		result := bulkCreateOverChunks(client, cfg.Host, specs, continueOnError)
		printBatchResult(result)
		return nil
	},
}

func specType(spec bulkCreateSpec) string {
	if spec.Type == "" {
		return "Task"
	}
	return spec.Type
}

func loadBulkCreateSpecs(path string) ([]bulkCreateSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading --file: %w", err)
	}
	var specs []bulkCreateSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, fmt.Errorf("parsing --file as a JSON array of issue specs: %w", err)
	}
	return specs, nil
}

func (s bulkCreateSpec) toRequest() api.CreateIssueRequest {
	fields := api.CreateIssueFields{
		Project:   api.ProjectRef{Key: s.Project},
		Summary:   s.Summary,
		IssueType: api.IssueTypeRef{Name: specType(s)},
	}
	if s.Description != "" {
		fields.Description = s.Description
	}
	if s.Assignee != "" {
		fields.Assignee = &api.NameRef{Name: s.Assignee}
	}
	if s.Priority != "" {
		fields.Priority = &api.NameRef{Name: s.Priority}
	}
	if len(s.Labels) > 0 {
		fields.Labels = s.Labels
	}
	if s.Component != "" {
		fields.Components = []api.NameRef{{Name: s.Component}}
	}
	if s.FixVersion != "" {
		fields.FixVersions = []api.NameRef{{Name: s.FixVersion}}
	}
	if s.Parent != "" {
		fields.Parent = &api.KeyRef{Key: s.Parent}
	}
	return api.CreateIssueRequest{Fields: fields}
}

// bulkCreateOverChunks submits the specs in AgileMoveCap-sized chunks to
// /issue/bulk and maps the native per-element response back onto item targets
// (the spec's global index). A whole-chunk transport failure marks every spec in
// the chunk failed; the upstream's own per-element errors mark only the offending
// specs. Created issues land in order, so chunk created keys map positionally to
// the chunk's non-failed specs.
func bulkCreateOverChunks(client *api.Client, host string, specs []bulkCreateSpec, continueOnError bool) batchResult {
	result := batchResult{Action: "bulk create issues", Items: make([]batchItem, 0, len(specs))}
	result.Summary.Total = len(specs)

	indices := make([]int, len(specs))
	for i := range specs {
		indices[i] = i
	}
	chunks := api.ChunkKeys(indices, api.AgileMoveCap)
	stopped := false
	processed := 0
	for _, chunk := range chunks {
		if stopped {
			break
		}
		reqs := make([]api.CreateIssueRequest, len(chunk))
		for j, idx := range chunk {
			reqs[j] = specs[idx].toRequest()
		}
		resp, err := client.Issues.BulkCreate(reqs)
		chunkFailed := applyBulkCreateChunk(&result, host, chunk, resp, err)
		processed += len(chunk)
		if chunkFailed && !continueOnError {
			stopped = true
		}
	}
	if processed < len(specs) {
		result.Summary.Skipped = len(specs) - processed
	}
	return result
}

// applyBulkCreateChunk records one chunk's outcome onto result.Items, keyed by
// each spec's global index, and reports whether any spec in the chunk failed.
func applyBulkCreateChunk(result *batchResult, host string, chunk []int, resp *api.BulkCreateResult, err error) bool {
	if err != nil {
		for _, idx := range chunk {
			result.Items = append(result.Items, batchItem{Target: strconv.Itoa(idx), OK: false, Error: itemErrorFromAPI(err)})
			result.Summary.Failed++
		}
		return true
	}
	// Map upstream failedElementNumber (position within the chunk) to failures.
	failedLocal := map[int]bool{}
	for _, e := range resp.Errors {
		failedLocal[e.FailedElementNumber] = true
	}
	created := resp.Issues
	createdAt := 0
	anyFailed := false
	for local, idx := range chunk {
		if failedLocal[local] {
			result.Items = append(result.Items, batchItem{
				Target: strconv.Itoa(idx), OK: false,
				Error: &batchItemError{Code: output.ErrValidation, Message: "create rejected by Jira (see element errors)", Retryable: false},
			})
			result.Summary.Failed++
			anyFailed = true
			continue
		}
		item := batchItem{Target: strconv.Itoa(idx), OK: true}
		if createdAt < len(created) {
			item.Key = created[createdAt].Key
			item.URL = host + "/browse/" + created[createdAt].Key
			createdAt++
		}
		result.Items = append(result.Items, item)
		result.Summary.Succeeded++
	}
	return anyFailed
}
