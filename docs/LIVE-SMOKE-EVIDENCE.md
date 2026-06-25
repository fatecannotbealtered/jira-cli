# Live Smoke Evidence

Recorded live smoke for `release_readiness.required_evidence:
recorded_live_smoke_for_stable`, run against a **real, licensed Jira Data
Center** instance.

- **Date:** 2026-06-14
- **Target:** a production Jira Data Center (REST API v2). Host, token, and all
  returned content are intentionally **not** recorded here — only aggregate
  counts and pass/fail. Auth is a Personal Access Token stored in
  `~/.jira-cli/config.json` (0600).
- **Method:** each command invoked with `--format json`; envelope `ok`/`error`
  asserted. All mutations were self-contained and cleaned up; no other users
  were assigned or notified.

## 2026-06-14 — v1.1.3 new reads + single-use (live)

Verified against the same production Jira Data Center (aggregate counts only):

| Command / behavior | Result | Notes |
|---|---|---|
| `issue link list <KEY>` | PASS | returns the issue's links (id/direction/type/linked key+summary+status), `_untrusted` on summary; empty array for an unlinked issue |
| `sprint report <sprintId> --board <id>` | PASS | live closed sprint: committed 72 issues / 91 pts, completed 67 / 84.5 pts, not-completed 5 / 6.5 pts; `source: greenhopper` |
| confirm token **single-use** | PASS (unit+live) | replaying a confirmed write's token → `E_CONFLICT` (verified live on gitlab; same code path) |

All v1.1.3 new read commands are live-verified.

## 2026-06-15 — batch commands (live + dry-run)

Run against the same production Jira Data Center. All mutations used
self-created throwaway issues in a test project, self-reported and
self-assigned to a throwaway test account; aggregate pass/fail and method only — no
hosts, keys, tokens, or returned content recorded. Issues were neutralised
(transitioned to Done, unassigned) afterward; hard-delete is denied by the
project ACL (E_FORBIDDEN), a project-config artifact, not a CLI defect.

| Command / behavior | Result | Method |
|---|---|---|
| `issue bulk-create --file` | PASS (live) | native `/issue/bulk`; created issues map positionally to `items[]` by 0-based file index, with real key+URL |
| `issue bulk-create` partial failure | PASS (live) | one valid + one bad issue-type spec → upstream `errors[].failedElementNumber` mapped to that item's `ok:false` (E_VALIDATION); the other item created; top-level `ok:true`, summary `succeeded:1 failed:1` |
| `issue backlog-move` | PASS (live) | `POST /rest/agile/1.0/backlog/issue` 204; aggregated `items[]`/summary |
| `issue rank --after` | PASS (live) | `PUT /rest/agile/1.0/issue/rank` `rankAfterIssue` 204 |
| `issue rank --before` | PASS (live) | same endpoint, `rankBeforeIssue` 204 |
| `issue bulk-assign` partial failure | PASS (live) | valid key + nonexistent key → per-item `E_NOT_FOUND` (real 404) aggregated; top-level `ok:true` |
| `issue bulk-edit` partial failure | PASS (live) | valid key + nonexistent key → per-item `E_NOT_FOUND` (real 404) aggregated |
| `issue bulk-transition` | PASS (live) | `--dangerous` gate enforced in both dry-run and confirm; closed 4 self-owned issues 4/4 |
| single-use confirm on a batch | PASS (live) | first `--confirm` succeeded; replay of the same token → `E_CONFLICT` |
| `--dangerous` gate (bulk-transition) | PASS (live) | command blocked with `E_CONFIRMATION_REQUIRED` until `--dangerous` supplied |
| dry-run envelope + validation guards | PASS (dry-run) | every new batch command emits a well-formed `confirm_token`/`preview`; empty file, both-anchor rank, no-field edit each rejected with `E_VALIDATION` |
| `sprint move` >50 keys (chunking) | dry-run + contract-test only, **not real-sprint executed** | dry-run accepts a 51-key set; the 50+1 → two-POST split is the shared `api.ChunkKeys(_, AgileMoveCap=50)` path, asserted live-equivalent by `TestBatch_BacklogMove_ChunkBoundary` and `TestBatch_Rank_ChunkBoundary` (51 keys → exactly 2 upstream calls). A real 51-issue move was **not** executed to avoid polluting a shared team board. |

Honest scope note: bulk-create's partial-failure path was exercised against
the real native endpoint; the class-B loop commands (bulk-assign/bulk-edit)
had their partial-failure aggregation exercised with a real per-issue 404. The
sprint-move >50 chunking is verified by contract test + live dry-run, not by a
real >50 production move. No dangerous/irreversible mass operation (mass
delete, etc.) was executed.

## Result by class

### Auth + reads — PASS
| Command | Result |
|---|---|
| `doctor` | PASS (token valid, authenticated) |
| `context` | PASS |
| `filter list` | PASS |
| `board list --type scrum` | PASS |
| `issue link-types` | PASS |
| `project get <KEY>` | PASS |
| `issue list --project <KEY>` | PASS |
| `issue comment list <ISSUE>` | PASS |

### Error taxonomy — PASS
| Path | Result |
|---|---|
| `issue get <nonexistent>` | `E_NOT_FOUND` |
| `issue list` without `--project` | `E_VALIDATION` |
| any write without `--confirm` | `E_CONFIRMATION_REQUIRED` |

### Write contract — PASS (real mutation + cleanup)
| Step | Result |
|---|---|
| `issue comment add` without confirm | blocked (`E_CONFIRMATION_REQUIRED`) |
| `issue comment add --dry-run` | issued a `confirm_token` |
| `issue comment add --confirm <token>` | **real comment created** on a test-project issue |
| `issue comment list` | verified the comment is present |
| `issue comment delete --confirm <token>` | comment removed; list re-verified empty (zero residue) |

## Defect found and fixed by this smoke run

**`issue create` / `issue edit` sent the description as Cloud ADF.** The client
converted the plain-text `--description` into an Atlassian Document Format doc
object (`{type:"doc",content:[…]}`) before POSTing to `/rest/api/2/issue`. But
jira-cli targets **Jira Data Center / Server**, whose REST API v2 takes a
**plain string** — the real instance rejected the ADF object with
`description: must be a string` (400). Mock tests had asserted the ADF shape, so
they never caught it. Fixed: description is now passed through as a plain string
for both create and edit, with a regression test
(`TestIssueCreate_DescriptionIsPlainString`).

## Note: `issue create` execution not exercised here

The throwaway test project used for the smoke had a non-standard create-screen
configuration (the `summary` field was not on the create screen, confirmed via
the raw `createmeta`/create API — "Field 'summary' cannot be set… not on the
appropriate screen"), so a brand-new issue could not be created there. This is
a project-configuration artifact of that instance, not a jira-cli defect; the
write path is covered end-to-end by the comment add/delete cycle above, and the
description-field fix is guarded by a unit test.

## Reproduce

```bash
jira-cli login --host https://<jira-dc> --token <PAT>   # stored 0600
jira-cli doctor --compact
jira-cli issue list --project <KEY> --limit 3 --compact
jira-cli issue comment add <ISSUE> --body 'smoke' --dry-run --compact
jira-cli issue comment add <ISSUE> --body 'smoke' --confirm <token> --compact
jira-cli issue comment delete <ISSUE> --id <id> --confirm <token> --compact
```
