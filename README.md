# jira-cli

[![CI](https://github.com/fatecannotbealtered/jira-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/fatecannotbealtered/jira-cli/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fatecannotbealtered/jira-cli)](https://goreportcard.com/report/github.com/fatecannotbealtered/jira-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![npm version](https://img.shields.io/npm/v/@fatecannotbealtered-/jira-cli.svg)](https://www.npmjs.com/package/@fatecannotbealtered-/jira-cli)

English | [中文](README_zh.md)

Full-featured Jira Data Center CLI for humans and AI Agents. Manage issues, sprints, boards, epics, projects, users, and filters — all from your terminal.

Built with Go. Single static binary (small dependency footprint via `go.mod`). No separate runtime to install.

[Installation](#installation) · [Authentication](#authentication) · [Commands](#commands) · [JSON Output](#json-output) · [Security](#security) · [Contributing](#contributing) · [Disclaimer](#disclaimer)

## Disclaimer

This project is shared for **personal learning, research, and everyday productivity**—not as a product with SLAs. The maintainers do not offer commercial support or make any **fitness-for-production** promise. If you use it at work, that is your call: follow your org's rules. Software is provided "as is," without warranty of any kind.

## Features

| Capability | Description |
|---|---|
| 🎯 **Complete Coverage** | Issues, Sprints, Boards, Epics, Projects, Users, Filters |
| 🤖 **AI Friendly** | `--json` flat output (token-efficient), `--quiet`, `--dry-run`, `--force`, `--fields`, `--raw` |
| ⚡ **Single Binary** | Download and run; no JVM/Node required for the CLI itself |
| 🔄 **Smart Retry** | Auto-retry on 429 rate limits and 5xx errors with exponential backoff |
| 🌈 **Beautiful Output** | Colored tables with CJK character support |
| 🔍 **Powerful Search** | Full JQL support with auto-pagination |
| 🔧 **Custom Fields** | Set custom fields during create and edit |
| 🔐 **PAT Auth** | Bearer token authentication (Personal Access Token) |
| 🌐 **Env Vars** | `JIRA_HOST` and `JIRA_TOKEN` override config file for CI/Agent use |
| 📋 **Audit Log** | Automatic JSONL audit trail for all write commands with auto-rotation |

## Installation

### Quick start

Copy the block below to your agent (or run it yourself):

```bash
# Please install jira-cli and use it for all Jira DC operations going forward (always pass --json).
# Install CLI (requires curl on PATH — used to download the binary)
npm install -g @fatecannotbealtered-/jira-cli

# Install CLI Skill (required) — copies into your agent-supported skills directory
npx skills add fatecannotbealtered/jira-cli -y -g

# Login and verify
jira-cli login
jira-cli doctor --json
```

Optional: `jira-cli install-skill` copies the bundled skill to `~/.openclaw/skills` for OpenClaw-compatible agents.

### Alternative: Go install

```bash
go install github.com/fatecannotbealtered/jira-cli/cmd/jira-cli@latest
```

### Alternative: Download binary

Download from [GitHub Releases](https://github.com/fatecannotbealtered/jira-cli/releases) and add to your PATH.

## Authentication

jira-cli supports **Jira Data Center** (private deployments) with **Personal Access Token (PAT)** authentication.

### Interactive login

```bash
jira-cli login
# Jira host: https://jira.company.com
# Personal Access Token (PAT): ****
# ✔ Logged in as John Doe (johndoe)

jira-cli doctor    # Verify connectivity
jira-cli logout    # Remove credentials
```

### Non-interactive login (CI / AI Agent)

```bash
jira-cli login --host https://jira.company.com --token <PAT>
```

### Environment variables

Environment variables take precedence over the config file. This is the recommended approach for CI pipelines and AI Agents:

```bash
export JIRA_HOST=https://jira.company.com
export JIRA_TOKEN=<your-personal-access-token>
jira-cli doctor --json   # verify authValid is true
```

### Generating a PAT

1. Log into your Jira Data Center instance
2. Go to **Profile** → **Personal Access Tokens**
3. Create a new token with appropriate permissions

## Commands

### Issue Management

```bash
# View
jira-cli issue get PROJ-123
jira-cli issue list --project PROJ
jira-cli issue list --project PROJ --status "In Progress" --assignee me

# Create & Edit
jira-cli issue create --project PROJ --summary "Fix login bug" --type Bug
jira-cli issue create --project PROJ --summary "Sized story" --field "Story Points=5"
jira-cli issue edit PROJ-123 --priority High --assignee me
jira-cli issue edit PROJ-123 --field "Story Points=8" --field "Team=Backend"
jira-cli issue delete PROJ-123 --force          # --force skips confirmation prompt
jira-cli issue delete PROJ-123 --dry-run --json   # preview delete (no confirmation prompt)

# Clone
jira-cli issue clone PROJ-123
jira-cli issue clone PROJ-123 --summary "New title" --with-links

# Status
jira-cli issue transitions PROJ-123          # List available transitions
jira-cli issue transition PROJ-123 "Done"    # Status name required as argument

# Bulk Transition
jira-cli issue bulk-transition "Done" --issues PROJ-1,PROJ-2,PROJ-3
jira-cli issue bulk-transition "In Progress" --jql "sprint = 10 AND status = 'To Do'"

# Collaboration
jira-cli issue assign PROJ-123 me               # Assign to current user
jira-cli issue assign PROJ-123 johndoe          # Assign by username (DC uses name, not accountId)
jira-cli issue watch PROJ-123
jira-cli issue vote PROJ-123

# Comments
jira-cli issue comment add PROJ-123 --body "Fixed in PR #42"
jira-cli issue comment list PROJ-123

# Worklogs
jira-cli issue worklog add PROJ-123 --time 2h --comment "Debugging"
jira-cli issue worklog list PROJ-123

# Links & Attachments
jira-cli issue link PROJ-123 --to PROJ-456 --type "blocks"
jira-cli issue attach PROJ-123 --file ./screenshot.png
jira-cli issue attachments PROJ-123                       # List attachments
jira-cli issue attachments PROJ-123 --out ./downloads     # Download all attachments
jira-cli issue attachments PROJ-123 --out ./downloads --id 4609477   # Download one by ID
jira-cli issue remote-link PROJ-123 --url https://pr.url --title "PR #42"
```

### Search (JQL)

```bash
jira-cli search "assignee = currentUser() AND status != Done"
jira-cli search "project = PROJ AND sprint in openSprints()" --all
jira-cli search "type = Bug AND priority = High" --count
jira-cli search "project = PROJ" --limit 100 --order-by updated
```

### Sprint Management

```bash
jira-cli sprint list --board 42
jira-cli sprint active --board 42
jira-cli sprint create --board 42 --name "Sprint 5" --start-date 2024-02-01 --end-date 2024-02-14
jira-cli sprint move --sprint 10 --issues PROJ-123,PROJ-124
jira-cli sprint close --sprint 10
jira-cli sprint close --sprint 10 --dry-run       # preview without closing
```

### Epic Management

```bash
jira-cli epic list --board 42
jira-cli epic list --board 42 --done             # completed epics only
jira-cli epic issues PROJ-1 --board 42
```

### Board & Backlog

```bash
jira-cli board list
jira-cli board backlog --board 42
jira-cli board epics --board 42
```

### Projects, Users & Filters

```bash
jira-cli project list
jira-cli project versions PROJ --unreleased
jira-cli project fields --custom              # List custom fields
jira-cli user search --query "john"
jira-cli user me
jira-cli filter list
jira-cli filter run <filterId>
jira-cli filter run <filterId> --json --fields key,summary,status
jira-cli filter run <filterId> --json --raw
```

## JSON Output

All commands support `--json` for machine-readable output. **Success JSON goes to stdout; error JSON goes to stderr** — pipe or capture stdout for data, check `$?` and stderr for failures.

By default, issue and sprint data is returned in a **flat, token-efficient format** (ideal for AI Agents):

```bash
# Flat JSON (default) — minimal fields, low token cost
jira-cli issue get PROJ-123 --json
jira-cli search "project = PROJ" --json | jq '.issues[].key'

# issue list returns a bare array; search wraps issues in pagination metadata
# filter run with --fields also returns a bare trimmed array
jira-cli issue list --project PROJ --json | jq '.[].key'
jira-cli search "project = PROJ" --json | jq '.issues[].key'
jira-cli filter run 12345 --json --fields key,summary | jq '.[].key'

# Trim flat JSON output (issue get / issue list / sprint / filter run)
jira-cli issue get PROJ-123 --json --fields key,summary,status,assignee

# search --fields controls which fields Jira fetches (API request), not output trimming
jira-cli search "project = PROJ" --fields summary,status,customfield_10001 --json

# Raw Jira API response (full nested structure)
jira-cli issue get PROJ-123 --json --raw

# Clean output for scripts (suppress all non-JSON noise on stdout)
jira-cli issue get PROJ-123 --json --quiet

# Preview destructive operations without executing
jira-cli issue delete PROJ-123 --dry-run --json
```

### Verify connectivity (`doctor`)

When using `--json`, check the `authValid` field (exit code 3 on auth/config failure):

```bash
jira-cli doctor --json | jq '.authValid'   # must be true
```

Error responses (stderr) include machine-readable error codes and actionable hints:

```json
{
  "error": "Jira API error 404: Issue does not exist",
  "statusCode": 404,
  "errorCode": "NOT_FOUND",
  "hint": "Verify the issue key exists and you have permission to view it"
}
```

Set `NO_COLOR=1` to disable colored output (useful in CI/CD).

Run `jira-cli reference` to get a complete listing of all commands and flags in structured markdown.

## Environment Variables

| Variable | Description |
|---|---|
| `JIRA_HOST` | Jira Data Center host URL (overrides config file) |
| `JIRA_TOKEN` | Personal Access Token (overrides config file) |
| `NO_COLOR` | Set to any value to disable colored output ([no-color.org](https://no-color.org)) |
| `JIRA_CLI_USER_AGENT` | Custom User-Agent string for HTTP requests |
| `JIRA_NO_AUDIT` | Set to `1` to disable audit logging |
| `JIRA_AUDIT_RETENTION_MONTHS` | Auto-delete audit files older than N months (default: `3`, `0` = keep forever) |

## Config File

Credentials stored at `~/.jira-cli/config.json` (permissions: 0600):

```json
{
  "host": "https://jira.company.com",
  "token": "your-personal-access-token"
}
```

## Global Flags

| Flag | Description |
|---|---|
| `--json` | Output as JSON (flat format by default; use `--raw` for full Jira response) |
| `--force` | Skip interactive confirmation prompts |
| `--quiet` | Suppress non-JSON stdout output (for scripts and AI Agents) |
| `--dry-run` | Show what would be done without executing (write commands only) |

### Per-command flags

| Flag | Commands | Description |
|---|---|---|
| `--raw` | `issue get`, `issue list`, `search`, `filter run`, `sprint list`, `sprint issues`, `sprint active` | Return raw Jira API response instead of flat format |
| `--fields` | `issue get`, `issue list`, `sprint list`, `sprint issues`, `filter run` | **Output trimming** — include only listed fields in flat JSON (e.g. `--fields key,summary,status`) |
| `--fields` | `search` only | **Jira fetch fields** — comma-separated fields to request from the API (e.g. `--fields summary,status,customfield_10001`); does not trim flat output |
| `--out` | `issue attachments` | Download attachments into this directory instead of listing (default cwd via the dir you pass); with `--json` prints `{id, filename, path, mimeType}` |
| `--id` | `issue attachments` | With `--out`, download only the attachment with this ID (exit code 4 if not found) |

## Troubleshooting

| Issue | Solution |
|---|---|
| npm install fails / curl not found | Ensure `curl` is on PATH (required by npm postinstall to download the binary) |
| Config not found | Run `jira-cli login` or set `JIRA_HOST` and `JIRA_TOKEN` env vars |
| Authentication failed | Regenerate PAT in your Jira DC profile settings |
| Permission denied | Check your PAT scope and project permissions |
| Resource not found | Verify the issue key or project key exists |
| Rate limited (429) | The CLI auto-retries; reduce request frequency if persistent |
| Host must start with https:// | Ensure your host URL includes the `https://` protocol |

## Security

- Credentials are stored locally at `~/.jira-cli/config.json` with `0600` file permissions (user-only readable)
- Config directory is created with `0700` permissions
- PAT input is hidden during `jira-cli login` (uses terminal secure input)
- All communication uses HTTPS (host must start with `https://`)
- No credentials are logged or transmitted to third parties
- Environment variables `JIRA_HOST` and `JIRA_TOKEN` take precedence over config file

> **AI Agent Note:** This tool can be invoked by AI Agents to automate Jira operations. Use `--force` to skip interactive prompts and `--json` for structured output. Set `JIRA_HOST` and `JIRA_TOKEN` environment variables for non-interactive authentication.

For vulnerability reports, see [SECURITY.md](SECURITY.md).

## Audit Logging

Every write command (create, edit, delete, transition, assign, comment, etc.) is automatically logged to `~/.jira-cli/audit/` in JSONL format — one JSON object per line, one file per month.

```bash
# Example: view today's audit log
cat ~/.jira-cli/audit/audit-2026-05.jsonl

# Each entry looks like:
# {"ts":"2026-05-03T14:22:01+08:00","cmd":"issue edit","args":["issue","edit","PROJ-123","--summary","new"],"exit":0,"ms":2031}
```

### Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `JIRA_NO_AUDIT` | (unset) | Set to `1` to disable audit logging entirely |
| `JIRA_AUDIT_RETENTION_MONTHS` | `3` | Auto-delete audit files older than N months. Set to `0` to keep forever. |

Cleanup runs lazily on each write command — no background process or cron needed.

## E2E Integration Tests

A comprehensive E2E test script covers **every jira-cli command** (55+ operations) against a real Jira Data Center instance.

### Quick start

```bash
# Read-only mode (safe — no data is modified)
export JIRA_HOST=https://jira.company.com
export JIRA_TOKEN=<your-pat>
export JIRA_E2E_MUTATE=0
pwsh ./scripts/e2e-full.ps1
```

### Full test (creates and deletes test issues, filters)

```bash
pwsh ./scripts/e2e-full.ps1
```

### With sprint write tests

```bash
export JIRA_E2E_SPRINT=1
pwsh ./scripts/e2e-full.ps1
```

The script produces:
- Terminal summary with PASS / FAIL / SKIP counts
- `scripts/e2e-report.csv` — machine-readable results for CI dashboards

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `JIRA_HOST` | required | Jira DC host URL |
| `JIRA_TOKEN` | required | Personal Access Token |
| `JIRA_CLI_BIN` | `jira-cli` | Path to binary |
| `JIRA_E2E_PROJECT` | auto-detect | Force a project key |
| `JIRA_E2E_MUTATE` | `1` | Set `0` for read-only tests |
| `JIRA_E2E_SPRINT` | `0` | Set `1` for sprint write tests |
| `JIRA_E2E_CLEANUP` | `1` | Set `0` to keep test resources |

## Project Structure

```
jira-cli/
├── cmd/
│   ├── jira-cli/
│   │   └── main.go          # Entry point (semantic exit codes)
│   ├── root.go              # Root command, global flags, error handling
│   ├── login.go             # Authentication (PAT-only, non-interactive)
│   ├── doctor.go            # Diagnostics
│   ├── issue.go             # Issue CRUD
│   ├── issue_*.go           # Issue sub-commands
│   ├── flatten.go           # Flat JSON output helpers (issues, sprints)
│   ├── reference.go         # Self-documenting command reference
│   ├── sprint.go            # Sprint management
│   ├── board.go             # Board operations
│   ├── project.go           # Project management
│   ├── search.go            # JQL search
│   ├── user.go              # User operations
│   ├── filter.go            # Saved filters
│   └── epic.go              # Epic operations
├── internal/
│   ├── api/                 # Jira REST API v2 client + types
│   ├── audit/               # Write-operation audit logging (JSONL)
│   ├── config/              # Config file + env var management
│   └── output/              # Output formatting (tables, colors, flatten types)
├── scripts/
│   ├── install.js           # npm postinstall (downloads binary)
│   ├── run.js               # npm bin wrapper
│   └── e2e-full.ps1         # Full E2E integration tests (all commands)
├── skills/                  # AI Agent Skill (bundled for install-skill)
├── package.json             # npm distribution
├── .goreleaser.yml          # Release automation
├── Makefile                 # Local development
└── .github/workflows/       # CI/CD
```

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md). Release notes: [CHANGELOG.md](CHANGELOG.md).

## License

MIT © fatecannotbealtered
