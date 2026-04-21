# Copy this file to e2e.local.ps1 and fill in your credentials.
# e2e.local.ps1 is in .gitignore — it will NOT be committed.
#
# Usage:
#   pwsh ./scripts/e2e-full.ps1
#
# These variables are loaded automatically by e2e-full.ps1.

$env:JIRA_HOST = "https://jira.example.com"
$env:JIRA_TOKEN = "YOUR_PAT"

# Optional: force a specific project key (default: auto-detect)
# $env:JIRA_E2E_PROJECT = "PROJ"
