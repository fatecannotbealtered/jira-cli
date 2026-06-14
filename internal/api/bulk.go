package api

import (
	"encoding/json"
	"fmt"
)

// AgileMoveCap is the per-request issue limit shared by the agile move
// endpoints (/sprint/{id}/issue, /backlog/issue) and the platform
// /issue/bulk create endpoint. Jira Data Center rejects larger batches with a
// 400, so every command touching these endpoints must split its input into
// chunks of at most this size. Keeping the cap in one place stops it drifting
// between the commands that share an endpoint.
const AgileMoveCap = 50

// ChunkKeys splits keys into consecutive slices of at most size, preserving
// order. size <= 0 degrades to a single chunk so a misconfigured caller never
// silently drops keys. An empty input yields no chunks.
func ChunkKeys[T any](keys []T, size int) [][]T {
	if len(keys) == 0 {
		return nil
	}
	if size <= 0 {
		return [][]T{keys}
	}
	var chunks [][]T
	for i := 0; i < len(keys); i += size {
		end := i + size
		if end > len(keys) {
			end = len(keys)
		}
		chunks = append(chunks, keys[i:end])
	}
	return chunks
}

// BulkCreateResult is the platform /issue/bulk response: the issues that were
// created and per-input errors keyed by their position in the submitted list.
type BulkCreateResult struct {
	Issues []IssueRef          `json:"issues"`
	Errors []BulkCreateElement `json:"errors"`
}

// BulkCreateElement carries one failed create from /issue/bulk. FailedElementNumber
// is the index of the offending entry within the chunk submitted (0-based).
type BulkCreateElement struct {
	FailedElementNumber int             `json:"failedElementNumber"`
	ElementErrors       json.RawMessage `json:"elementErrors"`
}

// BulkCreate submits one chunk of issue create requests to the native platform
// /rest/api/2/issue/bulk endpoint. The chunk must be <= AgileMoveCap; callers
// split larger input with ChunkKeys. The result reports created issues and any
// per-element errors so the caller can map them back onto its inputs.
func (a *IssueAPI) BulkCreate(reqs []CreateIssueRequest) (*BulkCreateResult, error) {
	body := map[string]any{"issueUpdates": reqs}
	data, err := a.client.Post("/rest/api/2/issue/bulk", body)
	if err != nil {
		return nil, err
	}
	var result BulkCreateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing bulk create response: %w", err)
	}
	return &result, nil
}

// Rank moves the given issues so they sit after (rankAfter) or before
// (rankBefore) the anchor issue, via the agile /issue/rank endpoint. Exactly one
// anchor mode is set by the caller. The chunk must be <= AgileMoveCap; split
// larger input with ChunkKeys.
func (a *IssueAPI) Rank(req RankRequest) error {
	_, err := a.client.Put("/rest/agile/1.0/issue/rank", req)
	return err
}
