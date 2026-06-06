---
name: jira-cli
description: Full Jira Data Center control from the terminal. Manage issues, sprints, boards, epics, projects, users, and filters. Output is machine-readable JSON by default; use --format text only for human-readable output.
metadata: {"openclaw":{"emoji":"🎯","requires":{"bins":["jira-cli"]}}}
---

# jira-cli

Jira Data Center CLI for humans and AI Agents. Supports **Jira DC only** (self-hosted), not Jira Cloud.

```bash
# Please install jira-cli and use it for all Jira DC operations going forward.
# Machine-readable JSON is the default; use --format text only for human-readable output.
# Install CLI (requires curl on PATH)
npm install -g @fatecannotbealtered-/jira-cli

# Install CLI Skill (required) — copies into your agent-supported skills directory
npx skills add fatecannotbealtered/jira-cli -y -g

# Login and verify (interactive login is text mode)
jira-cli --format text login
jira-cli doctor
```

Optional: `jira-cli install-skill` → `~/.openclaw/skills`.

## Updating

```bash
jira-cli update --check              # Check the latest GitHub Release
jira-cli update                      # Update a standalone binary after checksum verification
```

If the CLI was installed through npm, prefer the package manager instead of in-place replacement:

```bash
npm install -g @fatecannotbealtered-/jira-cli@latest
```

Use `--dry-run` to preview. Use `--force` only when the user explicitly wants in-place binary replacement.

## Prerequisites

Before using any command, authenticate with a Jira DC instance. Follow these steps in order:

1. Ask the user for their Jira DC URL (e.g. `https://jira.company.com`). It must start with `https://`.
2. Ask the user for a **Personal Access Token (PAT)**. If they don't have one, guide them:
   "Log in to your Jira DC → click your profile avatar → Personal Access Tokens → Create token."
3. Run `jira-cli login --host <URL> --token <PAT>` to save credentials.
4. Run `jira-cli doctor` to verify connectivity. Check that `.data.config.auth_status` is `"valid"` (exit code 4 on auth/config failure).

**Important:** Credentials are saved locally at `~/.jira-cli/config.json`. Environment variables `JIRA_HOST` and `JIRA_TOKEN` override the config file — use them for CI or when the user prefers not to save credentials.

**Do not add output flags for programmatic parsing.** JSON is the default. Use `--format text` only when the user wants human-readable tables or summaries.

**Stdout vs stderr:** Success and error JSON envelopes are written to **stdout** by default. Progress, prompts, warnings, and text-mode errors are written to **stderr**. Capture stdout for data and error envelopes; use exit codes for control flow.

## Safety

- **Do NOT use `--force` on destructive commands unless the user explicitly asks.** In text mode, commands like `issue delete` prompt for confirmation by default. Skipping confirmation with `--force` is irreversible.
- **Use `--dry-run` before JSON write operations**, inspect `.data.preview`, then execute the same command with `--confirm <token>` from `.data.confirm_token`.
- **`issue delete --dry-run` skips confirmation** — dry-run is evaluated before any text-mode confirmation prompt.
- **Use `--dry-run` before write operations** to preview what will happen without executing. Example: `issue create --project PROJ --summary "test" --type Task --dry-run`.
- **AI Agents can make mistakes.** Always confirm with the user before executing `issue delete`, `issue bulk-transition`, `sprint close`, or any operation that modifies multiple issues.
- All write operations are recorded in `~/.jira-cli/audit/` for traceability.

JSON write flow:

```bash
TOKEN=$(jira-cli issue create --project PROJ --summary "Fix login bug" --dry-run --compact | jq -r '.data.confirm_token')
jira-cli issue create --project PROJ --summary "Fix login bug" --confirm "$TOKEN"
```

## Issue Management

```bash
# View issues (flat JSON by default — token-efficient)
jira-cli issue get PROJ-123
jira-cli issue get PROJ-123 --fields key,summary,status,assignee   # Output trimming (flat JSON)
jira-cli --format raw issue get PROJ-123                            # Full Jira API response
jira-cli issue list --project PROJ                                  # Returns envelope with flat issue array in .data
jira-cli issue list --project PROJ --fields key,summary,status      # Trimmed array

# Create and edit
# For direct human execution, use --format text. For JSON automation, use dry-run/confirm.
jira-cli --format text issue create --project PROJ --summary "Fix login bug" --type Bug
jira-cli --format text issue create --project PROJ --summary "New feature" --type Story --assignee me --priority High
jira-cli --format text issue create --project PROJ --summary "Sized story" --type Story --field "Story Points=5"
jira-cli --format text issue edit PROJ-123 --summary "Updated summary" --priority Medium
jira-cli --format text issue edit PROJ-123 --field "Story Points=8" --field "Team=Backend"
jira-cli --format text issue delete PROJ-123 --force          # Skip text-mode confirmation prompt
jira-cli issue delete PROJ-123 --dry-run                      # Preview delete and get confirm_token

# Clone an issue
jira-cli --format text issue clone PROJ-123                                # Clone with default summary
jira-cli --format text issue clone PROJ-123 --summary "New title"          # Clone with custom summary
jira-cli --format text issue clone PROJ-123 --with-links                   # Clone with issue links

# Status transitions
jira-cli issue transitions PROJ-123                 # List available transitions
jira-cli --format text issue transition PROJ-123 "In Progress"    # Transition to status (name required)
jira-cli --format text issue transition PROJ-123 "Done"

# Bulk transition
jira-cli --format text issue bulk-transition "Done" --issues PROJ-1,PROJ-2,PROJ-3
jira-cli --format text issue bulk-transition "In Progress" --jql "project = PROJ AND sprint = 10 AND status = 'To Do'"

# Assignment and watching
jira-cli --format text issue assign PROJ-123 me                   # Assign to current user
jira-cli --format text issue assign PROJ-123 johndoe              # Assign by username (DC uses name, not accountId)
jira-cli --format text issue unassign PROJ-123
jira-cli --format text issue watch PROJ-123
jira-cli --format text issue unwatch PROJ-123
jira-cli issue watchers PROJ-123
jira-cli --format text issue vote PROJ-123
jira-cli --format text issue unvote PROJ-123

# Comments
jira-cli --format text issue comment add PROJ-123 --body "Fixed in PR #42"
jira-cli issue comment list PROJ-123
jira-cli --format text issue comment delete PROJ-123 --id <commentId>

# Worklogs
jira-cli --format text issue worklog add PROJ-123 --time 2h --comment "Debugging session"
jira-cli --format text issue worklog add PROJ-123 --time 1h30m --started "2024-01-15T10:00:00.000+0000"
jira-cli issue worklog list PROJ-123

# Links
jira-cli issue link-types                                           # List available link types
jira-cli --format text issue link PROJ-123 --to PROJ-456 --type "blocks"
jira-cli --format text issue unlink <linkId>
jira-cli --format text issue remote-link PROJ-123 --url https://pr.url --title "PR #42"
jira-cli issue remote-links PROJ-123

# Attachments
jira-cli --format text issue attach PROJ-123 --file ./screenshot.png
jira-cli issue attachments PROJ-123                              # list metadata (incl. content URL)
jira-cli issue attachments PROJ-123 --out ./dl                   # download all -> [{id,filename,path,mimeType}]
jira-cli issue attachments PROJ-123 --out ./dl --id 4609477      # download a single attachment by ID
```

## Search (JQL)

```bash
# Basic search
jira-cli search "assignee = currentUser() AND status != Done"
jira-cli search "project = PROJ AND sprint in openSprints()"

# Advanced options
jira-cli search "project = PROJ" --limit 100
jira-cli search "project = PROJ" --all                 # Fetch ALL results (auto-paginate)
jira-cli search "project = PROJ" --count               # Only show total count
jira-cli search "project = PROJ" --order-by updated
jira-cli search "type = Bug AND priority = High" --fields summary,status,customfield_10001  # Jira fetch fields (API request)
```

## Sprint Management

```bash
jira-cli board list                                                 # Find board IDs first
jira-cli sprint list --board 42
jira-cli sprint list --board 42 --state active
jira-cli sprint active --board 42                                  # Active sprint + issues
jira-cli sprint issues --sprint 10
jira-cli --format text sprint create --board 42 --name "Sprint 5" --start-date 2024-02-01 --end-date 2024-02-14
jira-cli --format text sprint update --sprint 10 --goal "Complete payment refactor"
jira-cli --format text sprint move --sprint 10 --issues PROJ-123,PROJ-124,PROJ-125
jira-cli --format text sprint close --sprint 10
jira-cli sprint close --sprint 10 --dry-run   # Preview without closing (no confirmation prompt)
```

## Board & Backlog

```bash
jira-cli board list
jira-cli board list --project PROJ --type scrum
jira-cli board get --board 42
jira-cli board backlog --board 42
jira-cli board epics --board 42
jira-cli board sprints --board 42 --state active
```

## Epic Management

```bash
jira-cli epic list --board 42
jira-cli epic list --board 42 --done              # Completed epics only
jira-cli epic issues PROJ-1 --board 42
```

## Project Management

```bash
jira-cli project list
jira-cli project list --type software
jira-cli project get PROJ
jira-cli project components PROJ
jira-cli project versions PROJ
jira-cli project versions PROJ --unreleased
jira-cli project issue-types PROJ
jira-cli project fields                       # List all fields (system + custom)
jira-cli project fields --custom              # List custom fields only
```

## User Search

```bash
jira-cli user me                                  # Current user info
jira-cli user search --query "john"
jira-cli user search --query "john" --assignable --project PROJ
```

## Filters

```bash
jira-cli filter list
jira-cli filter get <filterId>
jira-cli --format text filter create --name "My Bugs" --jql "assignee = me AND type = Bug"
jira-cli filter run <filterId>
jira-cli filter run <filterId> --fields key,summary,status   # Output trimming
jira-cli --format raw filter run <filterId>                  # Raw Jira search response
jira-cli --format text filter delete <filterId>
```

## Workflow Examples

### Find and update an issue
```bash
# 1. Search for issues
jira-cli search "project = PROJ AND assignee = me AND status = 'In Progress'"

# 2. Get issue details
jira-cli issue get PROJ-123

# 3. Check available transitions
jira-cli issue transitions PROJ-123

# 4. Transition to Done
jira-cli --format text issue transition PROJ-123 "Done"

# 5. Add a comment
jira-cli --format text issue comment add PROJ-123 --body "Completed and deployed to staging"
```

### Sprint planning workflow
```bash
# 1. Find the board
jira-cli board list

# 2. Check active sprint
jira-cli sprint active --board 42

# 3. View backlog
jira-cli board backlog --board 42

# 4. Create next sprint
jira-cli --format text sprint create --board 42 --name "Sprint 6" --start-date 2024-02-15 --end-date 2024-02-28

# 5. Move issues to sprint
jira-cli --format text sprint move --sprint 11 --issues PROJ-200,PROJ-201,PROJ-202
```

## Guardrails

- Always run `jira-cli doctor` and verify **`.data.config.auth_status == "valid"`** before bulk operations (exit code 4 on auth/config failure)
- In JSON mode, write commands require `--dry-run` followed by `--confirm <token>` unless the command explicitly documents otherwise
- In text mode, `issue delete` requires typing the issue key to confirm. Use `--force` only when the user explicitly asked to skip that prompt; `--dry-run` skips confirmation
- `sprint close` has no confirmation prompt — use `--dry-run` to preview; confirm with the user before closing
- Omit output flags when parsing output in scripts or AI workflows; JSON is the default
- Use `--dry-run` to preview what a write command would do without executing it
- Use `--quiet` to suppress auxiliary text output; it does not suppress JSON/raw main results
- `issue transition` requires the status name as the second argument (no interactive selection)
- When searching for usernames to use with `issue assign`, use `user search --query <name>` first
- DC uses **username** (not accountId) for user references. Use `jira-cli user me` to find your username

## Global Flags

- `--format json|text|raw` — Control output format. Default: `json`
- `--compact` — Emit compact JSON (only with `--format json`)
- `--json` — Compatibility alias for `--format json`; do not recommend it for new workflows
- `--force` — Skip interactive confirmation prompts in text mode
- `--quiet` — Suppress auxiliary text output; does not suppress JSON/raw main results
- `--dry-run` — Show what would be done without executing (supported by write/update commands)
- `--confirm <token>` — Execute a JSON-mode write command using the token returned by `--dry-run`

## Output Control Flags

Two different meanings for `--fields`:

| Command | `--fields` meaning |
|---------|-------------------|
| `search` | **Jira fetch fields** — comma-separated fields to request from the API (e.g. `summary,status,customfield_10001`) |
| `issue get`, `issue list`, `sprint list`, `sprint issues`, `filter run` | **JSON output trimming** — include only listed keys in flat JSON (e.g. `key,summary,status`) |

Other read-command flags:

- `--format raw` — Return raw command output where supported (`issue get/list`, `search`, `filter run`, `sprint list/issues/active`)
- `--raw` — Legacy per-command alias for `--format raw` on commands that support it

## JSON Output Schemas

### List vs search JSON shape

| Command | Default JSON shape |
|---------|---------------------|
All default JSON responses are wrapped:

```json
{"ok":true,"schema_version":"1.0","data":{},"meta":{"duration_ms":0}}
```

The command-specific payload is in `.data`:

| Command | `.data` shape |
|---------|---------------|
| `issue list` | Array: `[{key, summary, ...}, ...]` |
| `search`, `filter run` (default) | Object with pagination: `{total, startAt, maxResults, issues: [...]}` |
| `filter run --fields ...` | Trimmed array (like `issue list --fields`) |
| `issue get` | Single flat issue object |

**jq examples:**

```bash
jira-cli issue list --project PROJ | jq '.data[].key'
jira-cli search "project = PROJ" | jq '.data.issues[].key'
jira-cli filter run 12345 | jq '.data.issues[].status'
```

### Flat Issue (`.data` in default JSON)

```json
{
  "key": "PROJ-123",
  "summary": "Fix login bug",
  "status": "In Progress",
  "type": "Bug",
  "assignee": "johndoe",
  "reporter": "janedoe",
  "priority": "High",
  "created": "2024-01-15T10:30:00.000+0000",
  "updated": "2024-01-16T14:20:00.000+0000",
  "labels": "backend,urgent",
  "component": "auth",
  "parent": "PROJ-100"
}
```

### Flat Sprint (`.data` in default JSON)

```json
{
  "id": 42,
  "name": "Sprint 5",
  "state": "active",
  "startDate": "2024-02-01",
  "endDate": "2024-02-14",
  "goal": "Complete payment module"
}
```

### Error Response (default JSON, written to stdout)

```json
{
  "ok": false,
  "schema_version": "1.0",
  "error": {
    "code": "E_NOT_FOUND",
    "message": "Jira API error 404: Issue does not exist",
    "details": {
      "status_code": 404,
      "hint": "Verify the resource key/ID exists and you have permission to view it"
    },
    "retryable": false
  }
}
```

### Error Codes

| Code | Status | Retryable | Hint |
|------|--------|-----------|------|
| `E_AUTH_REQUIRED` | 401 | false | Run `jira-cli login --host <url> --token <pat>` or set env vars |
| `E_FORBIDDEN` | 403 | false | Check your PAT scope and project permissions |
| `E_NOT_FOUND` | 404 | false | Verify the resource key/ID exists |
| `E_RATE_LIMITED` | 429 | true | Wait and retry with backoff |
| `E_SERVER` | 5xx | true | Jira server error; retry later |
| `E_NETWORK` | — | true | Check host URL and network connectivity |
| `E_CONFIG` | — | false | Run `jira-cli login --host <url> --token <pat>` or set env vars |
| `E_CONFIRM_REQUIRED` | — | false | Run the same write command with `--dry-run`, then retry with `--confirm <token>` |
| `E_CONFLICT` | — | false | Re-run `--dry-run`; the token no longer matches the operation |
| `E_TIMEOUT` | 408 | true | Retry with backoff |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic error |
| 2 | Bad arguments |
| 3 | Resource not found |
| 4 | Authentication or permission error |
| 5 | Confirmation token required |
| 6 | Conflict / stale confirmation token |
| 7 | Retryable transient error |
| 8 | Timeout |

## Audit Logging

All write commands are automatically logged to `~/.jira-cli/audit/` in JSONL format (one file per month). Each entry records the command, arguments, exit code, and duration.

| Env var | Default | Description |
|---------|---------|-------------|
| `JIRA_NO_AUDIT` | (unset) | Set `1` to disable audit logging |
| `JIRA_AUDIT_RETENTION_MONTHS` | `3` | Auto-delete files older than N months (`0` = keep forever) |

## Self-Description

```bash
jira-cli reference   # Structured JSON command reference
jira-cli context     # Runtime, config, and credential status
jira-cli doctor      # Environment and connectivity checks
```
