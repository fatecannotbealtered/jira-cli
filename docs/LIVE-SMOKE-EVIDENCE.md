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
