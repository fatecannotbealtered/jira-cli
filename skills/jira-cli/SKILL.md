---
name: jira-cli
description: Full Jira Data Center control from the terminal. Manage issues, sprints, boards, epics, projects, users, and filters. Use --json flag for machine-readable output when parsing results programmatically.
metadata: {"openclaw":{"emoji":"🎯","requires":{"bins":["jira-cli"]}}}
---

# jira-cli

Jira Data Center CLI for humans and AI Agents. Supports **Jira DC only** (self-hosted), not Jira Cloud.

> Install CLI: `npm install -g @fatecannotbealtered-/jira-cli` (requires `curl` on PATH)
>
> Install Skill (pick one):
> - `npx skills add fatecannotbealtered/jira-cli -y -g`
> - `jira-cli install-skill` (bundled skill → `~/.openclaw/skills`)

## Prerequisites

Before using any command, authenticate with a Jira DC instance. Follow these steps in order:

1. Ask the user for their Jira DC URL (e.g. `https://jira.company.com`). It must start with `https://`.
2. Ask the user for a **Personal Access Token (PAT)**. If they don't have one, guide them:
   "Log in to your Jira DC → click your profile avatar → Personal Access Tokens → Create token."
3. Run `jira-cli login --host <URL> --token <PAT>` to save credentials.
4. Run `jira-cli doctor --json` to verify connectivity. Check that `authValid` is `true` (exit code 3 on auth/config failure).

**Important:** Credentials are saved locally at `~/.jira-cli/config.json`. Environment variables `JIRA_HOST` and `JIRA_TOKEN` override the config file — use them for CI or when the user prefers not to save credentials.

**Always use `--json` flag when parsing output.** Without it, output is human-formatted and not suitable for programmatic use.

**Stdout vs stderr:** Success JSON is written to **stdout**; error JSON is written to **stderr**. Capture stdout for data; check exit code and stderr for failures.

## Safety

- **Do NOT use `--force` on destructive commands unless the user explicitly asks.** Commands like `issue delete` prompt for confirmation by default. Skipping confirmation with `--force` is irreversible.
- **`issue delete --dry-run` skips confirmation** — dry-run is evaluated before the confirmation prompt.
- **Use `--dry-run` before write operations** to preview what will happen without executing. Example: `issue create --project PROJ --summary "test" --type Task --dry-run --json`.
- **AI Agents can make mistakes.** Always confirm with the user before executing `issue delete`, `issue bulk-transition`, `sprint close`, or any operation that modifies multiple issues.
- All write operations are recorded in `~/.jira-cli/audit/` for traceability.

## Issue Management

```bash
# View issues (flat JSON by default — token-efficient)
jira-cli issue get PROJ-123 --json
jira-cli issue get PROJ-123 --json --fields key,summary,status,assignee   # Output trimming (flat JSON)
jira-cli issue get PROJ-123 --json --raw                                  # Full Jira API response
jira-cli issue list --project PROJ --json                                 # Returns bare array of flat issues
jira-cli issue list --project PROJ --json --fields key,summary,status     # Trimmed array

# Create and edit
jira-cli issue create --project PROJ --summary "Fix login bug" --type Bug --json
jira-cli issue create --project PROJ --summary "New feature" --type Story --assignee me --priority High --json
jira-cli issue create --project PROJ --summary "Sized story" --type Story --field "Story Points=5" --json
jira-cli issue edit PROJ-123 --summary "Updated summary" --priority Medium
jira-cli issue edit PROJ-123 --field "Story Points=8" --field "Team=Backend"
jira-cli issue delete PROJ-123 --force          # Skip confirmation prompt
jira-cli issue delete PROJ-123 --dry-run --json   # Preview delete (no confirmation prompt)

# Clone an issue
jira-cli issue clone PROJ-123 --json                                # Clone with default summary
jira-cli issue clone PROJ-123 --summary "New title" --json          # Clone with custom summary
jira-cli issue clone PROJ-123 --with-links --json                   # Clone with issue links

# Status transitions
jira-cli issue transitions PROJ-123 --json          # List available transitions
jira-cli issue transition PROJ-123 "In Progress"    # Transition to status (name required)
jira-cli issue transition PROJ-123 "Done" --json

# Bulk transition
jira-cli issue bulk-transition "Done" --issues PROJ-1,PROJ-2,PROJ-3 --json
jira-cli issue bulk-transition "In Progress" --jql "project = PROJ AND sprint = 10 AND status = 'To Do'" --json

# Assignment and watching
jira-cli issue assign PROJ-123 me                   # Assign to current user
jira-cli issue assign PROJ-123 johndoe              # Assign by username (DC uses name, not accountId)
jira-cli issue unassign PROJ-123
jira-cli issue watch PROJ-123
jira-cli issue unwatch PROJ-123
jira-cli issue watchers PROJ-123 --json
jira-cli issue vote PROJ-123
jira-cli issue unvote PROJ-123

# Comments
jira-cli issue comment add PROJ-123 --body "Fixed in PR #42" --json
jira-cli issue comment list PROJ-123 --json
jira-cli issue comment delete PROJ-123 --id <commentId>

# Worklogs
jira-cli issue worklog add PROJ-123 --time 2h --comment "Debugging session" --json
jira-cli issue worklog add PROJ-123 --time 1h30m --started "2024-01-15T10:00:00.000+0000"
jira-cli issue worklog list PROJ-123 --json

# Links
jira-cli issue link-types --json                                    # List available link types
jira-cli issue link PROJ-123 --to PROJ-456 --type "blocks"
jira-cli issue unlink <linkId>
jira-cli issue remote-link PROJ-123 --url https://pr.url --title "PR #42"
jira-cli issue remote-links PROJ-123 --json

# Attachments
jira-cli issue attach PROJ-123 --file ./screenshot.png
jira-cli issue attachments PROJ-123 --json
```

## Search (JQL)

```bash
# Basic search
jira-cli search "assignee = currentUser() AND status != Done" --json
jira-cli search "project = PROJ AND sprint in openSprints()" --json

# Advanced options
jira-cli search "project = PROJ" --limit 100 --json
jira-cli search "project = PROJ" --all --json          # Fetch ALL results (auto-paginate)
jira-cli search "project = PROJ" --count               # Only show total count
jira-cli search "project = PROJ" --order-by updated --json
jira-cli search "type = Bug AND priority = High" --fields summary,status,customfield_10001 --json  # Jira fetch fields (API request)
```

## Sprint Management

```bash
jira-cli board list --json                                          # Find board IDs first
jira-cli sprint list --board 42 --json
jira-cli sprint list --board 42 --state active --json
jira-cli sprint active --board 42 --json                           # Active sprint + issues
jira-cli sprint issues --sprint 10 --json
jira-cli sprint create --board 42 --name "Sprint 5" --start-date 2024-02-01 --end-date 2024-02-14 --json
jira-cli sprint update --sprint 10 --goal "Complete payment refactor"
jira-cli sprint move --sprint 10 --issues PROJ-123,PROJ-124,PROJ-125
jira-cli sprint close --sprint 10 --json
jira-cli sprint close --sprint 10 --dry-run --json   # Preview without closing (no confirmation prompt)
```

## Board & Backlog

```bash
jira-cli board list --json
jira-cli board list --project PROJ --type scrum --json
jira-cli board get --board 42 --json
jira-cli board backlog --board 42 --json
jira-cli board epics --board 42 --json
jira-cli board sprints --board 42 --state active --json
```

## Epic Management

```bash
jira-cli epic list --board 42 --json
jira-cli epic list --board 42 --done --json              # Completed epics only
jira-cli epic issues PROJ-1 --board 42 --json
```

## Project Management

```bash
jira-cli project list --json
jira-cli project list --type software --json
jira-cli project get PROJ --json
jira-cli project components PROJ --json
jira-cli project versions PROJ --json
jira-cli project versions PROJ --unreleased --json
jira-cli project issue-types PROJ --json
jira-cli project fields --json                       # List all fields (system + custom)
jira-cli project fields --custom --json              # List custom fields only
```

## User Search

```bash
jira-cli user me --json                                  # Current user info
jira-cli user search --query "john" --json
jira-cli user search --query "john" --assignable --project PROJ --json
```

## Filters

```bash
jira-cli filter list --json
jira-cli filter get <filterId> --json
jira-cli filter create --name "My Bugs" --jql "assignee = me AND type = Bug" --json
jira-cli filter run <filterId> --json
jira-cli filter run <filterId> --json --fields key,summary,status   # Output trimming
jira-cli filter run <filterId> --json --raw                         # Raw Jira search response
jira-cli filter delete <filterId>
```

## Workflow Examples

### Find and update an issue
```bash
# 1. Search for issues
jira-cli search "project = PROJ AND assignee = me AND status = 'In Progress'" --json

# 2. Get issue details
jira-cli issue get PROJ-123 --json

# 3. Check available transitions
jira-cli issue transitions PROJ-123 --json

# 4. Transition to Done
jira-cli issue transition PROJ-123 "Done"

# 5. Add a comment
jira-cli issue comment add PROJ-123 --body "Completed and deployed to staging"
```

### Sprint planning workflow
```bash
# 1. Find the board
jira-cli board list --json

# 2. Check active sprint
jira-cli sprint active --board 42 --json

# 3. View backlog
jira-cli board backlog --board 42 --json

# 4. Create next sprint
jira-cli sprint create --board 42 --name "Sprint 6" --start-date 2024-02-15 --end-date 2024-02-28 --json

# 5. Move issues to sprint
jira-cli sprint move --sprint 11 --issues PROJ-200,PROJ-201,PROJ-202
```

## Guardrails

- Always run `jira-cli doctor --json` and verify **`authValid` is `true`** before bulk operations (exit code 3 on failure)
- `issue delete` requires typing the issue key to confirm. Use `--force` to skip confirmation in automated workflows; `--dry-run` skips confirmation
- `sprint close` has no confirmation prompt — use `--dry-run` to preview; confirm with the user before closing
- Use `--json` flag when parsing output in scripts or AI workflows
- Use `--dry-run` to preview what a write command would do without executing it
- Use `--quiet` with `--json` to suppress all non-JSON output (clean pipe-friendly output)
- `issue transition` requires the status name as the second argument (no interactive selection)
- When searching for usernames to use with `issue assign`, use `user search --query <name> --json` first
- DC uses **username** (not accountId) for user references. Use `jira-cli user me --json` to find your username

## Global Flags

- `--json` — Output as JSON (machine-readable, flat format by default)
- `--force` — Skip interactive confirmation prompts (for CI/Agent automation)
- `--quiet` — Suppress non-JSON stdout output (ideal for scripts and AI Agents)
- `--dry-run` — Show what would be done without executing (write commands only)

## Output Control Flags

Two different meanings for `--fields`:

| Command | `--fields` meaning |
|---------|-------------------|
| `search` | **Jira fetch fields** — comma-separated fields to request from the API (e.g. `summary,status,customfield_10001`) |
| `issue get`, `issue list`, `sprint list`, `sprint issues`, `filter run` | **Output trimming** — include only listed keys in flat JSON (e.g. `key,summary,status`) |

Other read-command flags:

- `--raw` — Return the full raw Jira API response instead of the token-efficient flat format (on `issue get/list`, `search`, `filter run`, `sprint list/issues/active`)

## JSON Output Schemas

### List vs search JSON shape

| Command | Flat `--json` shape |
|---------|---------------------|
| `issue list` | Bare array: `[{key, summary, ...}, ...]` |
| `search`, `filter run` (default) | Object with pagination: `{total, startAt, maxResults, issues: [...]}` |
| `filter run --fields ...` | Bare trimmed array (like `issue list --fields`) |
| `issue get` | Single flat issue object |

**jq examples:**

```bash
jira-cli issue list --project PROJ --json | jq '.[].key'
jira-cli search "project = PROJ" --json | jq '.issues[].key'
jira-cli filter run 12345 --json | jq '.issues[].status'
```

### Flat Issue (default with --json)

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

### Error Response (with --json, written to stderr)

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
