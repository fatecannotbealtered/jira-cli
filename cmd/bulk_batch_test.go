package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// batchEnvelope decodes the agent envelope plus the shared batch payload.
type batchEnvelope struct {
	OK   bool `json:"ok"`
	Data struct {
		Action string `json:"action"`
		Items  []struct {
			Target string `json:"target"`
			OK     bool   `json:"ok"`
			Key    string `json:"key"`
			Error  *struct {
				Code      string `json:"code"`
				Retryable bool   `json:"retryable"`
			} `json:"error"`
		} `json:"items"`
		Summary struct {
			Total     int `json:"total"`
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
			Skipped   int `json:"skipped"`
		} `json:"summary"`
		ConfirmToken string `json:"confirm_token"`
	} `json:"data"`
}

func decodeBatch(t *testing.T, out string) batchEnvelope {
	t.Helper()
	var env batchEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("invalid batch envelope: %v\n%s", err, out)
	}
	return env
}

// batchMockHandler answers the endpoints the batch commands touch. PROJ-FAIL
// returns 404 on PUT/POST so per-item failure aggregation can be exercised.
func batchMockHandler(t *testing.T, backlogCalls, rankCalls *int32) http.HandlerFunc {
	t.Helper()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		switch {
		case path == "/rest/api/2/myself" && method == http.MethodGet:
			writeJSON(w, http.StatusOK, `{"name":"alice","displayName":"Alice"}`)

		case path == "/rest/api/2/field" && method == http.MethodGet:
			writeJSON(w, http.StatusOK, `[{"id":"summary","name":"Summary","custom":false},{"id":"customfield_10001","name":"Story Points","custom":true}]`)

		case path == "/rest/api/2/search" && method == http.MethodPost:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			jql, _ := body["jql"].(string)
			if strings.Contains(jql, "EMPTY") {
				writeJSON(w, http.StatusOK, `{"startAt":0,"maxResults":50,"total":0,"issues":[]}`)
				return
			}
			writeJSON(w, http.StatusOK, `{"startAt":0,"maxResults":100,"total":2,"issues":[{"key":"PROJ-1"},{"key":"PROJ-2"}]}`)

		case path == "/rest/api/2/issue/bulk" && method == http.MethodPost:
			var body struct {
				IssueUpdates []json.RawMessage `json:"issueUpdates"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			// One created key per submitted spec, plus reject any spec whose
			// summary is "BAD" via failedElementNumber.
			var created []string
			var errs []string
			for i, raw := range body.IssueUpdates {
				if strings.Contains(string(raw), `"summary":"BAD"`) {
					errs = append(errs, fmt.Sprintf(`{"failedElementNumber":%d,"elementErrors":{"errors":{"summary":"bad"}}}`, i))
					continue
				}
				created = append(created, fmt.Sprintf(`{"id":"%d","key":"NEW-%d"}`, 1000+i, i))
			}
			writeJSON(w, http.StatusCreated, fmt.Sprintf(`{"issues":[%s],"errors":[%s]}`,
				strings.Join(created, ","), strings.Join(errs, ",")))

		case path == "/rest/agile/1.0/backlog/issue" && method == http.MethodPost:
			atomic.AddInt32(backlogCalls, 1)
			var body struct {
				Issues []string `json:"issues"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, k := range body.Issues {
				if k == "PROJ-FAIL" {
					writeJSON(w, http.StatusNotFound, `{"errorMessages":["not found"]}`)
					return
				}
			}
			w.WriteHeader(http.StatusNoContent)

		case path == "/rest/agile/1.0/issue/rank" && method == http.MethodPut:
			atomic.AddInt32(rankCalls, 1)
			w.WriteHeader(http.StatusNoContent)

		case strings.HasSuffix(path, "/assignee") && method == http.MethodPut:
			if strings.Contains(path, "PROJ-FAIL") {
				writeJSON(w, http.StatusNotFound, `{"errorMessages":["not found"]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case strings.HasPrefix(path, "/rest/api/2/issue/") && method == http.MethodPut:
			if strings.Contains(path, "PROJ-FAIL") {
				writeJSON(w, http.StatusNotFound, `{"errorMessages":["not found"]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Logf("unhandled: %s %s", method, path)
			writeJSON(w, http.StatusOK, `{}`)
		}
	}
}

func batchRunOK(t *testing.T, args ...string) string {
	t.Helper()
	resetCLIState(t)
	stdout, _ := runRootOK(t, append([]string{"--format", "json"}, args...)...)
	return stdout
}

func batchRunSilent(t *testing.T, code int, args ...string) string {
	t.Helper()
	resetCLIState(t)
	stdout, _ := runRootExpectSilent(t, code, append([]string{"--format", "json"}, args...)...)
	return stdout
}

// ===== backlog-move (class A) =====

func TestBatch_BacklogMove_Success(t *testing.T) {
	var backlogCalls int32
	mockJiraServer(t, batchMockHandler(t, &backlogCalls, nil))
	out := batchRunOK(t, "issue", "backlog-move", "--issues", "PROJ-1,PROJ-2")
	env := decodeBatch(t, out)
	if !env.OK || env.Data.Summary.Total != 2 || env.Data.Summary.Succeeded != 2 || env.Data.Summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", env.Data.Summary)
	}
	if env.Data.Items[0].Target != "PROJ-1" || env.Data.Items[1].Target != "PROJ-2" {
		t.Fatalf("input order not preserved: %+v", env.Data.Items)
	}
}

func TestBatch_BacklogMove_PartialFailure_TopLevelOK(t *testing.T) {
	var backlogCalls int32
	mockJiraServer(t, batchMockHandler(t, &backlogCalls, nil))
	// PROJ-FAIL chunks alone (each key is its own request only when chunked; here
	// all in one chunk, so the whole chunk 404s). Use continue-on-error semantics
	// with two separate single-key chunks by forcing the cap via many issues is
	// overkill; instead validate the chunk-level mapping: the failing chunk marks
	// every member failed but top-level ok stays true.
	out := batchRunOK(t, "issue", "backlog-move", "--issues", "PROJ-FAIL")
	env := decodeBatch(t, out)
	if !env.OK {
		t.Fatal("top-level ok must stay true on per-item failure")
	}
	if env.Data.Summary.Failed != 1 || env.Data.Items[0].OK {
		t.Fatalf("expected one failed item: %+v", env.Data)
	}
	if env.Data.Items[0].Error == nil || env.Data.Items[0].Error.Code != "E_NOT_FOUND" {
		t.Fatalf("expected E_NOT_FOUND item error: %+v", env.Data.Items[0])
	}
}

func TestBatch_BacklogMove_EmptyTargets_Validation(t *testing.T) {
	var backlogCalls int32
	mockJiraServer(t, batchMockHandler(t, &backlogCalls, nil))
	out := batchRunSilent(t, ExitBadArgs, "issue", "backlog-move", "--jql", "project = EMPTY")
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION, got %v", env["code"])
	}
}

func TestBatch_BacklogMove_ChunkBoundary(t *testing.T) {
	var backlogCalls int32
	mockJiraServer(t, batchMockHandler(t, &backlogCalls, nil))
	// 51 keys must split into two chunks (50 + 1) — invisible in the contract.
	keys := make([]string, 51)
	for i := range keys {
		keys[i] = fmt.Sprintf("PROJ-%d", i+1)
	}
	out := batchRunOK(t, "issue", "backlog-move", "--issues", strings.Join(keys, ","))
	env := decodeBatch(t, out)
	if env.Data.Summary.Total != 51 || env.Data.Summary.Succeeded != 51 {
		t.Fatalf("expected 51 succeeded: %+v", env.Data.Summary)
	}
	if got := atomic.LoadInt32(&backlogCalls); got != 2 {
		t.Fatalf("expected 2 upstream chunk calls, got %d", got)
	}
}

func TestBatch_BacklogMove_DryRunConfirmReplay(t *testing.T) {
	var backlogCalls int32
	mockJiraServer(t, batchMockHandler(t, &backlogCalls, nil))
	resetCLIState(t)
	dryOut, _, err := runRoot(t, "--format", "json", "--dry-run", "issue", "backlog-move", "--issues", "PROJ-1,PROJ-2")
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	env := decodeBatch(t, dryOut)
	if env.Data.ConfirmToken == "" {
		t.Fatal("dry-run produced no confirm token")
	}
	token := env.Data.ConfirmToken

	// First confirm consumes the token.
	resetCLIState(t)
	if _, _, err := runRoot(t, "--format", "json", "--confirm", token, "issue", "backlog-move", "--issues", "PROJ-1,PROJ-2"); err != nil {
		t.Fatalf("first confirm failed: %v", err)
	}
	// Replay must be rejected as E_CONFLICT (single-use token).
	resetCLIState(t)
	replayOut, _ := runRootExpectSilent(t, ExitConflict, "--format", "json", "--confirm", token, "issue", "backlog-move", "--issues", "PROJ-1,PROJ-2")
	if env := decodeEnvelopeError(t, replayOut); env["code"] != "E_CONFLICT" {
		t.Fatalf("expected E_CONFLICT on replay, got %v", env["code"])
	}
}

// ===== bulk-create (class A) =====

func TestBatch_BulkCreate_SuccessAndPartial(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	dir := t.TempDir()
	file := filepath.Join(dir, "issues.json")
	specs := `[{"project":"PROJ","summary":"one"},{"project":"PROJ","summary":"BAD"},{"project":"PROJ","summary":"three"}]`
	if err := os.WriteFile(file, []byte(specs), 0o600); err != nil {
		t.Fatal(err)
	}
	out := batchRunOK(t, "issue", "bulk-create", "--file", file)
	env := decodeBatch(t, out)
	if !env.OK || env.Data.Summary.Total != 3 || env.Data.Summary.Succeeded != 2 || env.Data.Summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", env.Data.Summary)
	}
	// Item target is the file index; created keys map positionally.
	if env.Data.Items[0].Target != "0" || env.Data.Items[0].Key != "NEW-0" {
		t.Fatalf("item 0 mismatch: %+v", env.Data.Items[0])
	}
	if env.Data.Items[1].OK {
		t.Fatalf("item 1 (BAD) should have failed: %+v", env.Data.Items[1])
	}
	if env.Data.Items[2].Target != "2" || env.Data.Items[2].Key != "NEW-2" {
		t.Fatalf("item 2 mismatch: %+v", env.Data.Items[2])
	}
}

func TestBatch_BulkCreate_MissingFile_Validation(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunSilent(t, ExitBadArgs, "issue", "bulk-create")
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION, got %v", env["code"])
	}
}

func TestBatch_BulkCreate_BadSpec_Validation(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	dir := t.TempDir()
	file := filepath.Join(dir, "issues.json")
	if err := os.WriteFile(file, []byte(`[{"summary":"no project"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := batchRunSilent(t, ExitBadArgs, "issue", "bulk-create", "--file", file)
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION, got %v", env["code"])
	}
}

// ===== bulk-assign / bulk-edit (class B) =====

func TestBatch_BulkAssign_Success(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunOK(t, "issue", "bulk-assign", "--issues", "PROJ-1,PROJ-2", "--assignee", "jdoe")
	env := decodeBatch(t, out)
	if env.Data.Summary.Succeeded != 2 {
		t.Fatalf("expected 2 succeeded: %+v", env.Data.Summary)
	}
}

func TestBatch_BulkAssign_PartialFailure(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunOK(t, "issue", "bulk-assign", "--issues", "PROJ-1,PROJ-FAIL", "--assignee", "jdoe")
	env := decodeBatch(t, out)
	if !env.OK || env.Data.Summary.Succeeded != 1 || env.Data.Summary.Failed != 1 {
		t.Fatalf("expected 1/1 split: %+v", env.Data.Summary)
	}
}

func TestBatch_BulkAssign_StopOnError_Skips(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunOK(t, "issue", "bulk-assign", "--issues", "PROJ-FAIL,PROJ-1,PROJ-2", "--assignee", "jdoe", "--continue-on-error=false")
	env := decodeBatch(t, out)
	if env.Data.Summary.Failed != 1 || env.Data.Summary.Skipped != 2 {
		t.Fatalf("expected stop-at-first with 2 skipped: %+v", env.Data.Summary)
	}
}

func TestBatch_BulkAssign_MissingAssignee_Validation(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunSilent(t, ExitBadArgs, "issue", "bulk-assign", "--issues", "PROJ-1")
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION, got %v", env["code"])
	}
}

func TestBatch_BulkEdit_Success_JQLSelection(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunOK(t, "issue", "bulk-edit", "--jql", "project = PROJ", "--priority", "High")
	env := decodeBatch(t, out)
	if env.Data.Summary.Total != 2 || env.Data.Summary.Succeeded != 2 {
		t.Fatalf("expected 2 from JQL: %+v", env.Data.Summary)
	}
}

func TestBatch_BulkEdit_NoFields_Validation(t *testing.T) {
	mockJiraServer(t, batchMockHandler(t, nil, nil))
	out := batchRunSilent(t, ExitBadArgs, "issue", "bulk-edit", "--issues", "PROJ-1")
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION, got %v", env["code"])
	}
}

// ===== rank (class A) =====

func TestBatch_Rank_Success(t *testing.T) {
	var rankCalls int32
	mockJiraServer(t, batchMockHandler(t, nil, &rankCalls))
	out := batchRunOK(t, "issue", "rank", "--issues", "PROJ-1,PROJ-2", "--after", "PROJ-9")
	env := decodeBatch(t, out)
	if env.Data.Summary.Succeeded != 2 {
		t.Fatalf("expected 2 ranked: %+v", env.Data.Summary)
	}
}

func TestBatch_Rank_RequiresExactlyOneAnchor(t *testing.T) {
	var rankCalls int32
	mockJiraServer(t, batchMockHandler(t, nil, &rankCalls))
	// Neither anchor.
	out := batchRunSilent(t, ExitBadArgs, "issue", "rank", "--issues", "PROJ-1")
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION (no anchor), got %v", env["code"])
	}
	// Both anchors.
	out = batchRunSilent(t, ExitBadArgs, "issue", "rank", "--issues", "PROJ-1", "--after", "X", "--before", "Y")
	if env := decodeEnvelopeError(t, out); env["code"] != "E_VALIDATION" {
		t.Fatalf("expected E_VALIDATION (both anchors), got %v", env["code"])
	}
}

func TestBatch_Rank_ChunkBoundary(t *testing.T) {
	var rankCalls int32
	mockJiraServer(t, batchMockHandler(t, nil, &rankCalls))
	keys := make([]string, 51)
	for i := range keys {
		keys[i] = fmt.Sprintf("PROJ-%d", i+1)
	}
	out := batchRunOK(t, "issue", "rank", "--issues", strings.Join(keys, ","), "--after", "PROJ-100")
	env := decodeBatch(t, out)
	if env.Data.Summary.Succeeded != 51 {
		t.Fatalf("expected 51 ranked: %+v", env.Data.Summary)
	}
	if got := atomic.LoadInt32(&rankCalls); got != 2 {
		t.Fatalf("expected 2 chunk calls, got %d", got)
	}
}
