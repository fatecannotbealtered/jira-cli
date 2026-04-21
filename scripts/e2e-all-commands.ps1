#requires -Version 5.1
<#
.SYNOPSIS
  Exercise every jira-cli subcommand against a real Jira Data Center instance.

  Environment (required):
    JIRA_HOST   - e.g. https://jira.example.com  (no trailing slash)
    JIRA_TOKEN  - Personal Access Token (PAT)

  Optional:
    JIRA_CLI_BIN       - Path to jira-cli executable (default: jira-cli on PATH)
    JIRA_E2E_PROJECT   - Project key for tests (default: first project from list)
    JIRA_E2E_MUTATE=0  - Read-only: skip creates/deletes/writes
    JIRA_E2E_SPRINT_WRITE=1 - Allow sprint create/update/close/move (default: off)
#>

$ErrorActionPreference = "Stop"

function Write-Step($msg) { Write-Host "`n=== $msg ===" -ForegroundColor Cyan }

function Invoke-Jira {
    param([string[]]$Args)
    & $script:JiraBin @Args
    if ($LASTEXITCODE -ne 0) { throw "Command failed (exit $LASTEXITCODE): $JiraBin $($Args -join ' ')" }
}

$JiraBin = if ($env:JIRA_CLI_BIN) { $env:JIRA_CLI_BIN } else { "jira-cli" }

$hostUrl = $env:JIRA_HOST
$token = $env:JIRA_TOKEN

if ([string]::IsNullOrWhiteSpace($hostUrl) -or [string]::IsNullOrWhiteSpace($token)) {
    throw "Set JIRA_HOST and JIRA_TOKEN"
}
$hostUrl = $hostUrl.TrimEnd('/')

$cfgDir = Join-Path $env:USERPROFILE ".jira-cli"
New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
$cfgObj = [ordered]@{ host = $hostUrl; token = $token }
$jsonText = ($cfgObj | ConvertTo-Json -Compress)
[System.IO.File]::WriteAllText((Join-Path $cfgDir "config.json"), $jsonText, [System.Text.UTF8Encoding]::new($false))

$mutate = ($env:JIRA_E2E_MUTATE -ne "0")
$sprintWrite = ($env:JIRA_E2E_SPRINT_WRITE -eq "1")

Write-Step "A. Config & connectivity"
Invoke-Jira @("doctor")
Invoke-Jira @("user", "me", "--json")
Invoke-Jira @("user", "search", "--query", "a", "--json")

Write-Step "B. Projects & metadata"
$projRaw = & $JiraBin @("project", "list", "--json") | Out-String
if ($LASTEXITCODE -ne 0) { throw "project list failed" }
$projArr = $projRaw | ConvertFrom-Json
$projectKey = $env:JIRA_E2E_PROJECT
if ([string]::IsNullOrWhiteSpace($projectKey)) {
    if ($projArr -is [Array] -and $projArr.Count -gt 0) { $projectKey = $projArr[0].key }
}
if ([string]::IsNullOrWhiteSpace($projectKey)) { throw "No project key; set JIRA_E2E_PROJECT" }
Write-Host "Using PROJECT_KEY=$projectKey"

Invoke-Jira @("project", "get", $projectKey, "--json")
Invoke-Jira @("project", "components", $projectKey, "--json")
Invoke-Jira @("project", "versions", $projectKey, "--json")
Invoke-Jira @("project", "issue-types", $projectKey, "--json")
Invoke-Jira @("project", "fields", "--json")
Invoke-Jira @("project", "fields", $projectKey, "--json")

Write-Step "C. Search & issues (read)"
Invoke-Jira @("search", "project = $projectKey ORDER BY updated DESC", "--limit", "5", "--json")
Invoke-Jira @("search", "project = $projectKey", "--count")
Invoke-Jira @("issue", "list", "--project", $projectKey, "--limit", "5", "--json")

$issueKey = $null
$listRaw = & $JiraBin @("issue", "list", "--project", $projectKey, "--limit", "1", "--json") | Out-String
if ($LASTEXITCODE -eq 0 -and $listRaw.Trim().Length -gt 2) {
    $listArr = $listRaw | ConvertFrom-Json
    if ($listArr -is [Array] -and $listArr.Count -gt 0) { $issueKey = $listArr[0].key }
}

if ($issueKey) {
    Write-Host "Using ISSUE_KEY=$issueKey for read-only issue commands"
    Invoke-Jira @("issue", "get", $issueKey, "--json")
    Invoke-Jira @("issue", "transitions", $issueKey, "--json")
    Invoke-Jira @("issue", "watchers", $issueKey, "--json")
    Invoke-Jira @("issue", "comment", "list", $issueKey, "--json")
    Invoke-Jira @("issue", "worklog", "list", $issueKey, "--json")
    Invoke-Jira @("issue", "attachments", $issueKey, "--json")
    Invoke-Jira @("issue", "link-types", "--json")
    Invoke-Jira @("issue", "remote-links", $issueKey, "--json")
} else {
    Write-Warning "No existing issue in project; skipping issue-specific read commands"
}

Write-Step "D. Boards & sprints (read)"
Invoke-Jira @("board", "list", "--json")
$boardRaw = & $JiraBin @("board", "list", "--project", $projectKey, "--json") | Out-String
$boardId = 0
if ($LASTEXITCODE -eq 0 -and $boardRaw.Trim().Length -gt 2) {
    $boardsArr = $boardRaw | ConvertFrom-Json
    if ($boardsArr -is [Array] -and $boardsArr.Count -gt 0) { $boardId = [int]$boardsArr[0].id }
}
if ($boardId -le 0) {
    $allRaw = & $JiraBin @("board", "list", "--json") | Out-String
    $allArr = $allRaw | ConvertFrom-Json
    if ($allArr -is [Array] -and $allArr.Count -gt 0) { $boardId = [int]$allArr[0].id }
}
if ($boardId -gt 0) {
    Write-Host "Using BOARD_ID=$boardId"
    Invoke-Jira @("board", "get", "--board", "$boardId", "--json")
    Invoke-Jira @("board", "backlog", "--board", "$boardId", "--json")
    Invoke-Jira @("board", "epics", "--board", "$boardId", "--json")
    Invoke-Jira @("board", "sprints", "--board", "$boardId", "--json")
    Invoke-Jira @("sprint", "list", "--board", "$boardId", "--json")
    Invoke-Jira @("sprint", "active", "--board", "$boardId", "--json")
    Invoke-Jira @("epic", "list", "--board", "$boardId", "--json")
} else {
    Write-Warning "No board id; board/sprint/epic list commands skipped"
}

Write-Step "E. Epics (issues in epic) & filters"
if ($issueKey) {
    if ($boardId -gt 0) {
        & $JiraBin @("epic", "issues", $issueKey, "--board", "$boardId", "--json") 2>$null
        if ($LASTEXITCODE -ne 0) { Write-Warning "epic issues: skipped (key may not be an epic)" }
    }
}
Invoke-Jira @("filter", "list", "--json")

Write-Step "F. install-skill (local files)"
$repoRoot = Join-Path $PSScriptRoot ".."
$repoRoot = [System.IO.Path]::GetFullPath($repoRoot)
if (Test-Path (Join-Path $repoRoot "skills")) {
    Push-Location $repoRoot
    try { Invoke-Jira @("install-skill", "--json") } finally { Pop-Location }
} else {
    Write-Warning "No skills directory at repo root; install-skill skipped"
}

if (-not $mutate) {
    Write-Host "`nJIRA_E2E_MUTATE=0 — skipping write tests." -ForegroundColor Yellow
    exit 0
}

Write-Step "G. Mutations (issue lifecycle)"
$tag = [Guid]::NewGuid().ToString("N").Substring(0, 8)
$summary = "jira-cli E2E $tag"
$createdRaw = & $JiraBin @("issue", "create", "--project", $projectKey, "--summary", $summary, "--type", "Task", "--json") | Out-String
if ($LASTEXITCODE -ne 0) { throw "issue create failed" }
$newKey = ($createdRaw | ConvertFrom-Json).key
if (-not $newKey) { throw "could not parse created issue key" }
Write-Host "Created $newKey"

Invoke-Jira @("issue", "edit", $newKey, "--description", "E2E description $tag")
Invoke-Jira @("issue", "assign", $newKey, "me")
Invoke-Jira @("issue", "watch", $newKey)
Invoke-Jira @("issue", "vote", $newKey)
Invoke-Jira @("issue", "comment", "add", $newKey, "--body", "e2e comment $tag")
Invoke-Jira @("issue", "worklog", "add", $newKey, "--time", "15m", "--comment", "e2e work")

$tf = Join-Path $env:TEMP "jira-cli-e2e-$tag.txt"
"e2e attachment $tag" | Set-Content -Path $tf -Encoding UTF8
try {
    Invoke-Jira @("issue", "attach", $newKey, "--file", $tf)
} finally {
    Remove-Item -Force $tf -ErrorAction SilentlyContinue
}

$cloneRaw = & $JiraBin @("issue", "clone", $newKey, "--json") | Out-String
if ($LASTEXITCODE -ne 0) { throw "issue clone failed" }
$cloneKey = ($cloneRaw | ConvertFrom-Json).key

$ltRaw = & $JiraBin @("issue", "link-types", "--json") | Out-String
$linkTypes = $ltRaw | ConvertFrom-Json
$ltName = $null
if ($linkTypes -is [Array] -and $linkTypes.Count -gt 0) { $ltName = $linkTypes[0].name }
if ($ltName) {
    Invoke-Jira @("issue", "link", $newKey, "--to", $cloneKey, "--type", $ltName)
}

Invoke-Jira @("issue", "remote-link", $newKey, "--url", "https://example.com/e2e-$tag", "--title", "e2e remote $tag")

$toName = $null
$transRaw = & $JiraBin @("issue", "transitions", $newKey, "--json") | Out-String
$transArr = $transRaw | ConvertFrom-Json
if ($transArr -is [Array] -and $transArr.Count -gt 0) {
    $toName = $transArr[0].to.name
    if ($toName) {
        & $JiraBin @("issue", "transition", $newKey, $toName)
        if ($LASTEXITCODE -ne 0) { Write-Warning "issue transition may be invalid for current status; continuing" }
    }
}

if ($toName) {
    Invoke-Jira @("issue", "bulk-transition", $toName, "--issues", $cloneKey, "--json")
} else {
    Write-Warning "No transition target; bulk-transition skipped"
}

Invoke-Jira @("issue", "unvote", $newKey)
Invoke-Jira @("issue", "unwatch", $newKey)
Invoke-Jira @("issue", "unassign", $newKey)

$commentsRaw = & $JiraBin @("issue", "comment", "list", $newKey, "--json") | Out-String
$commentsArr = $commentsRaw | ConvertFrom-Json
if ($commentsArr -is [Array] -and $commentsArr.Count -gt 0) {
    $cid = $commentsArr[0].id
    if ($cid) { Invoke-Jira @("issue", "comment", "delete", $newKey, "--id", "$cid") }
}

$filterName = "e2e-filter-$tag"
Invoke-Jira @("filter", "create", "--name", $filterName, "--jql", "project = $projectKey ORDER BY updated DESC")
$filtersRaw = & $JiraBin @("filter", "list", "--json") | Out-String
$filtersArr = $filtersRaw | ConvertFrom-Json
$fid = ($filtersArr | Where-Object { $_.name -eq $filterName } | Select-Object -First 1).id
if ($fid) {
    Invoke-Jira @("filter", "get", "$fid", "--json")
    Invoke-Jira @("filter", "run", "$fid", "--limit", "3", "--json")
    Invoke-Jira @("filter", "delete", "$fid")
}

if ($sprintWrite -and $boardId -gt 0) {
    Write-Step "H. Sprint writes (optional)"
    $spName = "e2e-sprint-$tag"
    Invoke-Jira @("sprint", "create", "--board", "$boardId", "--name", $spName)
    $sprintsRaw = & $JiraBin @("sprint", "list", "--board", "$boardId", "--json") | Out-String
    $sprintsArr = $sprintsRaw | ConvertFrom-Json
    $newSprint = $sprintsArr | Where-Object { $_.name -eq $spName } | Select-Object -First 1
    if ($newSprint -and $newSprint.id) {
        $sid = [int]$newSprint.id
        Invoke-Jira @("sprint", "update", "--sprint", "$sid", "--goal", "e2e goal")
        Invoke-Jira @("sprint", "move", "--sprint", "$sid", "--issues", $cloneKey)
        Invoke-Jira @("sprint", "issues", "--sprint", "$sid", "--json")
        Invoke-Jira @("sprint", "close", "--sprint", "$sid", "--force")
    }
}

Write-Step "I. Cleanup (delete issues)"
$cloneKey | & $JiraBin @("issue", "delete", $cloneKey, "--force")
if ($LASTEXITCODE -ne 0) { throw "delete clone failed" }
$newKey | & $JiraBin @("issue", "delete", $newKey, "--force")
if ($LASTEXITCODE -ne 0) { throw "delete original failed" }

Write-Host "`nAll e2e steps completed." -ForegroundColor Green
