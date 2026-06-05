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

# Login and verify
jira-cli login
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
4. Run `jira-cli doctor` to verify connectivity. Check that `authValid` is `true` (exit code 3 on auth/config failure).

**Important:** Credentials are saved locally at `~/.jira-cli/config.json`. Environment variables `JIRA_HOST` and `JIRA_TOKEN` override the config file — use them for CI or when the user prefers not to save credentials.

**Do not add output flags for programmatic parsing.** JSON is the default. Use `--format text` only when the user wants human-readable tables or summaries.

**Stdout vs stderr:** Success JSON is written to **stdout** by default; error JSON is written to **stderr**. Capture stdout for data; check exit code and stderr for failures.

## Safety

- **Do NOT use `--force` on destructive commands unless the user explicitly asks.** Commands like `issue delete` prompt for confirmation by default. Skipping confirmation with `--force` is irreversible.
- **`issue delete --dry-run` skips confirmation** — dry-run is evaluated before the confirmation prompt.
- **Use `--dry-run` before write operations** to preview what will happen without executing. Example: `issue create --project PROJ --summary "test" --type Task --dry-run`.
- **AI Agents can make mistakes.** Always confirm with the user before executing `issue delete`, `issue bulk-transition`, `sprint close`, or any operation that modifies multiple issues.
- All write operations are recorded in `~/.jira-cli/audit/` for traceability.

## Issue Management

```bash
# View issues (flat JSON by default — token-efficient)
jira-cli issue get PROJ-123
jira-cli issue get PROJ-123 --fields key,summary,status,assignee   # Output trimming (flat JSON)
jira-cli --format raw issue get PROJ-123                            # Full Jira API response
jira-cli issue list --project PROJ                                  # Returns bare array of flat issues
jira-cli issue list --project PROJ --fields key,summary,status      # Trimmed array

# Create and edit
jira-cli issue create --project PROJ --summary "Fix login bug" --type Bug
jira-cli issue create --project PROJ --summary "New feature" --type Story --assignee me --priority High
jira-cli issue create --project PROJ --summary "Sized story" --type Story --field "Story Points=5"
jira-cli issue edit PROJ-123 --summary "Updated summary" --priority Medium
jira-cli issue edit PROJ-123 --field "Story Points=8" --field "Team=Backend"
jira-cli issue delete PROJ-123 --force          # Skip confirmation prompt
jira-cli issue delete PROJ-123 --dry-run        # Preview delete (no confirmation prompt)

# Clone an issue
jira-cli issue clone PROJ-123                                # Clone with default summary
jira-cli issue clone PROJ-123 --summary "New title"          # Clone with custom summary
jira-cli issue clone PROJ-123 --with-links                   # Clone with issue links

# Status transitions
jira-cli issue transitions PROJ-123                 # List available transitions
jira-cli issue transition PROJ-123 "In Progress"    # Transition to status (name required)
jira-cli issue transition PROJ-123 "Done"

# Bulk transition
jira-cli issue bulk-transition "Done" --issues PROJ-1,PROJ-2,PROJ-3
jira-cli issue bulk-transition "In Progress" --jql "project = PROJ AND sprint = 10 AND status = 'To Do'"

# Assignment and watching
jira-cli issue assign PROJ-123 me                   # Assign to current user
jira-cli issue assign PROJ-123 johndoe              # Assign by username (DC uses name, not accountId)
jira-cli issue unassign PROJ-123
jira-cli issue watch PROJ-123
jira-cli issue unwatch PROJ-123
jira-cli issue watchers PROJ-123
jira-cli issue vote PROJ-123
jira-cli issue unvote PROJ-123

# Comments
jira-cli issue comment add PROJ-123 --body "Fixed in PR #42"
jira-cli issue comment list PROJ-123
jira-cli issue comment delete PROJ-123 --id <commentId>

# Worklogs
jira-cli issue worklog add PROJ-123 --time 2h --comment "Debugging session"
jira-cli issue worklog add PROJ-123 --time 1h30m --started "2024-01-15T10:00:00.000+0000"
jira-cli issue worklog list PROJ-123

# Links
jira-cli issue link-types                                           # List available link types
jira-cli issue link PROJ-123 --to PROJ-456 --type "blocks"
jira-cli issue unlink <linkId>
jira-cli issue remote-link PROJ-123 --url https://pr.url --title "PR #42"
jira-cli issue remote-links PROJ-123

# Attachments
jira-cli issue attach PROJ-123 --file ./screenshot.png
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
jira-cli sprint create --board 42 --name "Sprint 5" --start-date 2024-02-01 --end-date 2024-02-14
jira-cli sprint update --sprint 10 --goal "Complete payment refactor"
jira-cli sprint move --sprint 10 --issues PROJ-123,PROJ-124,PROJ-125
jira-cli sprint close --sprint 10
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
jira-cli filter create --name "My Bugs" --jql "assignee = me AND type = Bug"
jira-cli filter run <filterId>
jira-cli filter run <filterId> --fields key,summary,status   # Output trimming
jira-cli --format raw filter run <filterId>                  # Raw Jira search response
jira-cli filter delete <filterId>
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
jira-cli issue transition PROJ-123 "Done"

# 5. Add a comment
jira-cli issue comment add PROJ-123 --body "Completed and deployed to staging"
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
jira-cli sprint create --board 42 --name "Sprint 6" --start-date 2024-02-15 --end-date 2024-02-28

# 5. Move issues to sprint
jira-cli sprint move --sprint 11 --issues PROJ-200,PROJ-201,PROJ-202
```

## Guardrails

- Always run `jira-cli doctor` and verify **`authValid` is `true`** before bulk operations (exit code 3 on failure)
- `issue delete` requires typing the issue key to confirm. Use `--force` to skip confirmation in automated workflows; `--dry-run` skips confirmation
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
- `--force` — Skip interactive confirmation prompts (for CI/Agent automation)
- `--quiet` — Suppress auxiliary text output; does not suppress JSON/raw main results
- `--dry-run` — Show what would be done without executing (supported by write/update commands)

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
| `issue list` | Bare array: `[{key, summary, ...}, ...]` |
| `search`, `filter run` (default) | Object with pagination: `{total, startAt, maxResults, issues: [...]}` |
| `filter run --fields ...` | Bare trimmed array (like `issue list --fields`) |
| `issue get` | Single flat issue object |

**jq examples:**

```bash
jira-cli issue list --project PROJ | jq '.[].key'
jira-cli search "project = PROJ" | jq '.issues[].key'
jira-cli filter run 12345 | jq '.issues[].status'
```

### Flat Issue (default JSON)

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

### Flat Sprint

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

### Error Response (default JSON, written to stderr)

```json
{
  "error": "Jira API error 404: Issue does not exist",
  "statusCode": 404,
  "errorCode": "NOT_FOUND",
  "hint": "Verify the issue key exists and you have permission to view it"
}
```

### Error Codes

| Code | Status | Hint |
|------|--------|------|
| `AUTH_REQUIRED` | 401 | Run `jira-cli login` or set JIRA_HOST and JIRA_TOKEN |
| `FORBIDDEN` | 403 | Check your PAT scope and project permissions |
| `NOT_FOUND` | 404 | Verify the resource key/ID exists |
| `RATE_LIMITED` | 429 | Wait and retry; reduce request frequency |
| `SERVER_ERROR` | 5xx | Jira server error; retry later |
| `NETWORK_ERROR` | — | Check host URL and network connectivity |
| `CONFIG_ERROR` | — | Run `jira-cli login` or set env vars |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Bad arguments |
| 3 | Authentication error |
| 4 | Resource not found |
| 5 | Forbidden |
| 6 | Rate limited |
| 7 | Network/server error |

## Audit Logging

All write commands are automatically logged to `~/.jira-cli/audit/` in JSONL format (one file per month). Each entry records the command, arguments, exit code, and duration.

| Env var | Default | Description |
|---------|---------|-------------|
| `JIRA_NO_AUDIT` | (unset) | Set `1` to disable audit logging |
| `JIRA_AUDIT_RETENTION_MONTHS` | `3` | Auto-delete files older than N months (`0` = keep forever) |

## Self-Description

```bash
jira-cli reference   # Print all commands, subcommands, and flags in structured markdown
```
