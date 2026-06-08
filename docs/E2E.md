# E2E Testing

The canonical integration suite is `scripts/e2e-full.ps1`.
`scripts/e2e-all-commands.ps1` is a compatibility wrapper that delegates to it.

## Credentials

Provide a Jira Data Center host and Personal Access Token by one of these
methods:

```powershell
$env:JIRA_HOST = "https://jira.example.com"
$env:JIRA_TOKEN = "<PAT>"
pwsh ./scripts/e2e-full.ps1
```

or copy `scripts/e2e.local.example.ps1` to `scripts/e2e.local.ps1` and fill in
the values. The local file is ignored by git.

## Modes

| Variable | Default | Meaning |
|----------|---------|---------|
| `JIRA_CLI_BIN` | `jira-cli` | Binary under test |
| `JIRA_E2E_PROJECT` | auto-detect | Project key to use |
| `JIRA_E2E_MUTATE` | `1` | Set `0` for read-only validation |
| `JIRA_E2E_SPRINT` | `0` | Set `1` to include sprint write tests |
| `JIRA_E2E_CLEANUP` | `1` | Set `0` to keep created test resources |

## Contract Coverage

The suite exercises:

- Default JSON envelope parsing through `.data`.
- Text mode for human-readable paths.
- Raw mode where supported.
- `--fields`, `--compact`, `--quiet`, and `--dry-run`.
- JSON write commands through `--dry-run` then `--confirm <token>`.
- Issue, board, sprint, epic, project, user, filter, attachment, worklog, and
  comment operations.

The suite writes a CSV report to `scripts/e2e-report.csv`.
