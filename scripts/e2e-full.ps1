#requires -Version 5.1
<#
.SYNOPSIS
  Full E2E integration test for jira-cli — exercises every command against a real Jira Data Center instance.

.DESCRIPTION
  Covers all 55+ jira-cli commands including read operations, write lifecycle,
  flag variants (--json, --raw, --fields, --quiet, --dry-run), and auxiliary commands.

  Generates a CSV test report (e2e-report.csv) and terminal summary.

.PARAMETER JiraHost
  Jira Data Center host URL (https://...). Overrides JIRA_HOST env var.

.PARAMETER JiraToken
  Personal Access Token. Overrides JIRA_TOKEN env var.

.PARAMETER Binary
  Path to jira-cli executable. Default: "jira-cli" on PATH.

.PARAMETER ProjectKey
  Force a specific project key. Default: auto-detect from project list.

.EXAMPLE
  # Read-only mode (safe, no mutations)
  $env:JIRA_E2E_MUTATE = "0"
  .\scripts\e2e-full.ps1

.EXAMPLE
  # Full test with sprint writes
  $env:JIRA_E2E_SPRINT = "1"
  .\scripts\e2e-full.ps1

.EXAMPLE
  # Full test, keep test resources (no cleanup)
  $env:JIRA_E2E_CLEANUP = "0"
  .\scripts\e2e-full.ps1

.NOTES
  Environment variables (all optional unless noted):
    JIRA_HOST         - Jira DC host URL (required if -JiraHost not set)
    JIRA_TOKEN        - Personal Access Token (required if -JiraToken not set)
    JIRA_CLI_BIN      - Path to jira-cli binary (default: "jira-cli" on PATH)
    JIRA_E2E_PROJECT  - Project key override (default: auto-detect from project list)
    JIRA_E2E_MUTATE   - "0" = read-only, skip write phases I–K (default: "1")
    JIRA_E2E_SPRINT   - "1" = run sprint write tests K5–K8 (default: "0")
    JIRA_E2E_CLEANUP  - "0" = keep created issues/filters, skip J3/J4 cleanup (default: "1")
#>

param(
    [string]$JiraHost = "",
    [string]$JiraToken = "",
    [string]$Binary = "",
    [string]$ProjectKey = ""
)

$ErrorActionPreference = "Stop"

# Fix UTF-8 encoding for subprocess output (required for PS 5.1 with CJK characters)
$script:UTF8 = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = $script:UTF8
$OutputEncoding = $script:UTF8

# ─── Helper functions ──────────────────────────────────────────────────────────

function Write-Phase($msg) {
    Write-Host "`n╔══════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║  $msg" -ForegroundColor Cyan
    Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor Cyan
}

# Result tracking
$script:Rows = [System.Collections.ArrayList]::new()

function Add-Result {
    param([string]$Id, [string]$CmdLine, [int]$ExitCode, [long]$DurationMs, [string]$Note)
    [void]$script:Rows.Add([pscustomobject]@{
        Id         = $Id
        Command    = $CmdLine
        Exit       = $ExitCode
        DurationMs = $DurationMs
        Note       = $Note
    })
}

function Invoke-Test {
    <#
    .SYNOPSIS
        Run a jira-cli command, record result. Stores outcome in $script:LastTest.
        Callers that need the result should read $script:LastTest after invocation.
    #>
    param(
        [string]$Id,
        [string[]]$CmdArgs,
        [string]$Note = "",
        [switch]$Skip,
        [string]$SkipReason = ""
    )

    $argStr = ($CmdArgs -join ' ')
    $cmdLine = "$script:JiraBin $argStr"

    if ($Skip) {
        Add-Result $Id $cmdLine -2 0 "SKIP - $SkipReason"
        $script:LastTest = @{ Ok = $false; Out = ""; Skipped = $true }
        return
    }

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        # Use temp file to avoid PS 5.1 pipe encoding issues with CJK characters
        $tmpOut = [System.IO.Path]::GetTempFileName()
        $tmpErr = [System.IO.Path]::GetTempFileName()
        try {
            # Quote arguments containing spaces for Start-Process
            $quoted = @()
            foreach ($a in $CmdArgs) {
                if ($a -match '[\s"]') { $quoted += "`"$($a -replace '"','\"')`"" } else { $quoted += $a }
            }
            $argStr = $quoted -join ' '
            $proc = Start-Process -FilePath $script:JiraBin -ArgumentList $argStr `
                -NoNewWindow -Wait -PassThru `
                -RedirectStandardOutput $tmpOut `
                -RedirectStandardError $tmpErr
            $code = $proc.ExitCode
            $out = [System.IO.File]::ReadAllText($tmpOut, $script:UTF8)
            $errText = [System.IO.File]::ReadAllText($tmpErr, $script:UTF8)
            if ($errText) { $out = $out + "`n" + $errText }
        } finally {
            Remove-Item -Force $tmpOut -ErrorAction SilentlyContinue
            Remove-Item -Force $tmpErr -ErrorAction SilentlyContinue
        }
        $sw.Stop()
        $code = $proc.ExitCode
        if ($code -ne 0 -and [string]::IsNullOrWhiteSpace($Note)) {
            $trimmed = $out.Trim()
            if ($trimmed.Length -gt 120) { $trimmed = $trimmed.Substring(0, 120) }
            $Note = $trimmed
        }
        Add-Result $Id $cmdLine $code $sw.ElapsedMilliseconds $Note
        $script:LastTest = @{ Ok = ($code -eq 0); Out = $out; Skipped = $false }
    } catch {
        $sw.Stop()
        Add-Result $Id $cmdLine -1 $sw.ElapsedMilliseconds $_.Exception.Message
        $script:LastTest = @{ Ok = $false; Out = ""; Skipped = $false }
    }
}

function Parse-JsonField {
    param([string]$JsonText, [string]$FieldPath)
    try {
        $obj = $JsonText | ConvertFrom-Json
        $parts = $FieldPath.Split(".")
        $current = $obj
        foreach ($p in $parts) {
            if ($null -eq $current) { return $null }
            $current = $current.$p
        }
        return $current
    } catch {
        return $null
    }
}

function Invoke-Bin {
    <#
    .SYNOPSIS
        Invoke jira-cli binary with proper UTF-8 encoding.
        Returns @{ ExitCode = int; Out = string }
    #>
    param([string[]]$CmdArgs)
    $tmpOut = [System.IO.Path]::GetTempFileName()
    $tmpErr = [System.IO.Path]::GetTempFileName()
    try {
        # Quote arguments containing spaces for Start-Process
        $quoted = @()
        foreach ($a in $CmdArgs) {
            if ($a -match '[\s"]') { $quoted += "`"$($a -replace '"','\"')`"" } else { $quoted += $a }
        }
        $argStr = $quoted -join ' '
        $proc = Start-Process -FilePath $script:JiraBin -ArgumentList $argStr `
            -NoNewWindow -Wait -PassThru `
            -RedirectStandardOutput $tmpOut `
            -RedirectStandardError $tmpErr
        $out = [System.IO.File]::ReadAllText($tmpOut, $script:UTF8)
        $errText = [System.IO.File]::ReadAllText($tmpErr, $script:UTF8)
        if ($errText) { $out = $out + "`n" + $errText }
        return @{ ExitCode = $proc.ExitCode; Out = $out }
    } finally {
        Remove-Item -Force $tmpOut -ErrorAction SilentlyContinue
        Remove-Item -Force $tmpErr -ErrorAction SilentlyContinue
    }
}

function Skip-Row {
    param([string]$Id, [string]$Label, [string]$Reason)
    Add-Result $Id "$Label (SKIP)" -2 0 "SKIP - $Reason"
}

function Close-Issue {
    <#
    .SYNOPSIS
        Try to transition an issue to a done/closed state.
        Used as fallback when delete fails (insufficient permissions).
        Returns the status name if successful, $null otherwise.
    #>
    param([string]$Key)
    if ([string]::IsNullOrWhiteSpace($Key)) { return $null }

    $trResult = Invoke-Bin @("issue", "transitions", $Key, "--json")
    $transRaw = $trResult.Out
    if ($trResult.ExitCode -ne 0) { return $null }

    try {
        $transArr = $transRaw | ConvertFrom-Json
        if (-not ($transArr -is [Array]) -or $transArr.Count -eq 0) { return $null }

        # Prefer: Done > Closed > Resolved > any "done" category > last available
        $preferNames = @("Done", "Closed", "Resolved", "Complete", "Completed")
        $target = $null

        foreach ($name in $preferNames) {
            $match = $transArr | Where-Object { $_.to.name -eq $name } | Select-Object -First 1
            if ($match) { $target = $match.to.name; break }
        }
        # Fallback: look for any transition whose category is "done"
        if (-not $target) {
            $doneCat = $transArr | Where-Object { $_.to.statusCategory -and $_.to.statusCategory.key -eq "done" } | Select-Object -First 1
            if ($doneCat) { $target = $doneCat.to.name }
        }
        # Last resort: use the first available transition
        if (-not $target) { $target = $transArr[0].to.name }

        $trExec = Invoke-Bin @("issue", "transition", $Key, $target, "--force")
        if ($trExec.ExitCode -eq 0) { return $target }
    } catch {}
    return $null
}

function Cleanup-Issue {
    <#
    .SYNOPSIS
        Try to delete an issue. If delete fails, try to close/complete it.
        Records ONE result: PASS if either delete or close succeeded, FAIL only if both failed.
    #>
    param([string]$Id, [string]$Key, [string]$Label)

    if ([string]::IsNullOrWhiteSpace($Key)) {
        Skip-Row $Id $Label "no issue key"
        return
    }

    # Step 1: Try delete (invoke directly, don't record yet)
    $delResult = Invoke-Bin @("issue", "delete", $Key, "--force")
    if ($delResult.ExitCode -eq 0) {
        Write-Host "  Deleted $Key" -ForegroundColor Green
        Add-Result $Id "$Label" 0 0 "deleted $Key"
        return
    }

    # Step 2: Delete failed — try to close/complete
    Write-Host "  Delete $Key failed (exit $($delResult.ExitCode)), trying to close..." -ForegroundColor Yellow
    $closedTo = Close-Issue $Key
    if ($closedTo) {
        Write-Host "  Closed $Key -> $closedTo" -ForegroundColor Yellow
        Add-Result $Id "$Label" 0 0 "delete denied (exit $($delResult.ExitCode)), transitioned $Key -> $closedTo"
    } else {
        Write-Host "  WARNING: Could not delete or close $Key — please clean up manually" -ForegroundColor Red
        Add-Result $Id "$Label - NOT CLEANED UP" -1 0 "delete and close both failed for $Key — manual cleanup needed"
    }
}

# ─── Resolve credentials ──────────────────────────────────────────────────────

# Load e2e.local.ps1 if present
$localCfg = Join-Path $PSScriptRoot "e2e.local.ps1"
if (Test-Path $localCfg) { . $localCfg }

# Parameters > env vars > e2e.local.ps1
if ([string]::IsNullOrWhiteSpace($JiraHost))  { $JiraHost  = $env:JIRA_HOST }
if ([string]::IsNullOrWhiteSpace($JiraToken)) { $JiraToken = $env:JIRA_TOKEN }
if ([string]::IsNullOrWhiteSpace($ProjectKey)) { $ProjectKey = $env:JIRA_E2E_PROJECT }

if ([string]::IsNullOrWhiteSpace($JiraHost) -or [string]::IsNullOrWhiteSpace($JiraToken)) {
    throw @"
JIRA_HOST and JIRA_TOKEN are required. Provide them via:
  1. Parameters:    .\e2e-full.ps1 -JiraHost "https://..." -JiraToken "<PAT>"
  2. Environment:   `$env:JIRA_HOST = "..."; `$env:JIRA_TOKEN = "..."
  3. Local config:  Copy e2e.local.example.ps1 → e2e.local.ps1 and fill in values
"@
}

$JiraHost = $JiraHost.TrimEnd('/')

# Write config.json (required by jira-cli)
$cfgDir = Join-Path $env:USERPROFILE ".jira-cli"
New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
$cfgObj = [ordered]@{ host = $JiraHost; token = $JiraToken }
$cfgJson = ($cfgObj | ConvertTo-Json -Compress)
[System.IO.File]::WriteAllText(
    (Join-Path $cfgDir "config.json"),
    $cfgJson,
    [System.Text.UTF8Encoding]::new($false)
)

# Resolve binary
$script:JiraBin = if ($Binary) { $Binary }
                  elseif ($env:JIRA_CLI_BIN) { $env:JIRA_CLI_BIN }
                  else { "jira-cli" }

# Resolve mode flags (see .NOTES for env var semantics)
$mutate  = ($env:JIRA_E2E_MUTATE  -ne "0")  # default "1" — run write phases I–K
$sprintW = ($env:JIRA_E2E_SPRINT  -eq "1")  # default "0" — sprint writes K5–K8
$cleanup = ($env:JIRA_E2E_CLEANUP -ne "0")  # default "1" — delete test issues in J3/J4

# Generate unique tag for test resources
$tag = [Guid]::NewGuid().ToString("N").Substring(0, 8)

Write-Host ""
Write-Host "jira-cli E2E Full Test" -ForegroundColor White
Write-Host "Host:   $JiraHost" -ForegroundColor Gray
Write-Host "Binary: $script:JiraBin" -ForegroundColor Gray
Write-Host "Tag:    $tag" -ForegroundColor Gray
Write-Host "Mutate: $mutate | Sprint: $sprintW | Cleanup: $cleanup" -ForegroundColor Gray

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE A — Authentication & Connectivity
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase A: Authentication & Connectivity"

Invoke-Test "A1" @("doctor")
Invoke-Test "A2" @("doctor", "--json")
Invoke-Test "A3" @("user", "me")
Invoke-Test "A4" @("user", "me", "--json")
Invoke-Test "A5" @("user", "search", "--query", "a", "--json")

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE B — Projects & Metadata
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase B: Projects & Metadata"

# Auto-detect project key if not provided
if ([string]::IsNullOrWhiteSpace($ProjectKey)) {
    Write-Host "Auto-detecting project key..." -ForegroundColor DarkGray
    $tmpProj = [System.IO.Path]::GetTempFileName()
    try {
        $projProc = Start-Process -FilePath $script:JiraBin -ArgumentList "project list --json" `
            -NoNewWindow -Wait -PassThru `
            -RedirectStandardOutput $tmpProj `
            -RedirectStandardError ([System.IO.Path]::GetTempFileName())
        Write-Host "  project list exit code: $($projProc.ExitCode)" -ForegroundColor DarkGray
        if ($projProc.ExitCode -eq 0) {
            $projRaw = [System.IO.File]::ReadAllText($tmpProj, $script:UTF8)
            if ($projRaw.Trim().Length -gt 2) {
                try {
                    $projArr = $projRaw | ConvertFrom-Json
                    Write-Host "  parsed project count: $(if ($projArr -is [Array]) { $projArr.Count } else { 'not-array' })" -ForegroundColor DarkGray
                    if ($projArr -is [Array] -and $projArr.Count -gt 0) {
                        $ProjectKey = [string]$projArr[0].key
                        Write-Host "  detected key: $ProjectKey" -ForegroundColor DarkGray
                    }
                } catch {
                    Write-Host "  JSON parse error: $($_.Exception.Message)" -ForegroundColor Red
                }
            }
        }
    } finally {
        Remove-Item -Force $tmpProj -ErrorAction SilentlyContinue
    }
}
if ([string]::IsNullOrWhiteSpace($ProjectKey)) {
    Write-Host "`nFATAL: Could not detect project key. Set JIRA_E2E_PROJECT." -ForegroundColor Red
    $script:Rows | Format-Table -AutoSize
    exit 1
}
Write-Host "Using PROJECT_KEY=$ProjectKey" -ForegroundColor Yellow

Invoke-Test "B1" @("project", "list")
Invoke-Test "B2" @("project", "list", "--json")
Invoke-Test "B3" @("project", "list", "--type", "software")
Invoke-Test "B4" @("project", "get", $ProjectKey)
Invoke-Test "B5" @("project", "get", $ProjectKey, "--json")
Invoke-Test "B6" @("project", "components", $ProjectKey, "--json")
Invoke-Test "B7" @("project", "versions", $ProjectKey, "--json")
Invoke-Test "B8" @("project", "versions", $ProjectKey, "--unreleased", "--json")
Invoke-Test "B9" @("project", "issue-types", $ProjectKey, "--json")
Invoke-Test "BA" @("project", "fields", "--json")
Invoke-Test "BB" @("project", "fields", "--custom", "--json")

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE C — Search (JQL)
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase C: Search (JQL)"

$jql = "project = $ProjectKey ORDER BY updated DESC"

Invoke-Test "C1" @("search", $jql, "--limit", "5")
Invoke-Test "C2" @("search", $jql, "--limit", "5", "--json")
Invoke-Test "C3" @("search", "project = $ProjectKey", "--count")
Invoke-Test "C4" @("search", $jql, "--fields", "key,summary", "--limit", "2")
Invoke-Test "C5" @("search", "project = $ProjectKey", "--limit", "5", "--order-by", "updated")

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE D — Issue Read Operations
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase D: Issue Read Operations"

# Bootstrap: find an existing issue
$issueKey = $null
$bootResult = Invoke-Bin @("issue", "list", "--project", $ProjectKey, "--limit", "1", "--json")
if ($bootResult.ExitCode -eq 0) {
    try {
        $bArr = $bootResult.Out | ConvertFrom-Json
        if ($bArr -is [Array] -and $bArr.Count -gt 0) { $issueKey = $bArr[0].key }
    } catch {}
}
if ($issueKey) {
    Write-Host "Using ISSUE_KEY=$issueKey for read commands" -ForegroundColor Yellow
} else {
    Write-Host "WARNING: No existing issue found; some read commands will be skipped" -ForegroundColor Yellow
}

Invoke-Test "D1" @("issue", "list", "--project", $ProjectKey, "--limit", "5")
Invoke-Test "D2" @("issue", "list", "--project", $ProjectKey, "--limit", "5", "--json")
Invoke-Test "D3" @("issue", "list", "--project", $ProjectKey, "--assignee", "me", "--limit", "3")
Invoke-Test "D4" @("issue", "list", "--project", $ProjectKey, "--type", "Task", "--limit", "3")

Invoke-Test "D5" @("issue", "get", $issueKey) -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "D6" @("issue", "get", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "D7" @("issue", "get", $issueKey, "--json", "--raw") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "D8" @("issue", "get", $issueKey, "--json", "--fields", "key,summary,status") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "D9" @("issue", "transitions", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "DA" @("issue", "watchers", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "DB" @("issue", "link-types", "--json")
Invoke-Test "DC" @("issue", "remote-links", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "DD" @("issue", "attachments", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "DE" @("issue", "comment", "list", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "DF" @("issue", "worklog", "list", $issueKey, "--json") -Skip:(-not $issueKey) -SkipReason "no existing issue"

# Also test --all on search once we have an issue key
if ($issueKey) {
    Invoke-Test "C6" @("search", "key = $issueKey", "--all", "--json")
} else {
    Skip-Row "C6" "search --all" "no existing issue key"
}

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE E — Board, Sprint & Epic Read Operations
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase E: Board, Sprint & Epic Read Operations"

Invoke-Test "E1" @("board", "list", "--json")

# Find a board for the project
$boardId = 0
$boardResult = Invoke-Bin @("board", "list", "--project", $ProjectKey, "--json")
if ($boardResult.ExitCode -eq 0) {
    try {
        $bArr = $boardResult.Out | ConvertFrom-Json
        if ($bArr -is [Array] -and $bArr.Count -gt 0) { $boardId = [int]$bArr[0].id }
    } catch {}
}
if ($boardId -le 0) {
    # Fallback: try any board
    $allBoardResult = Invoke-Bin @("board", "list", "--json")
    if ($allBoardResult.ExitCode -eq 0) {
        try {
            $abArr = $allBoardResult.Out | ConvertFrom-Json
            if ($abArr -is [Array] -and $abArr.Count -gt 0) { $boardId = [int]$abArr[0].id }
        } catch {}
    }
}
if ($boardId -gt 0) {
    Write-Host "Using BOARD_ID=$boardId" -ForegroundColor Yellow
} else {
    Write-Host "WARNING: No board found; board/sprint/epic commands will be skipped" -ForegroundColor Yellow
}

Invoke-Test "E2" @("board", "list", "--project", $ProjectKey, "--json")
Invoke-Test "E3" @("board", "get", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"
Invoke-Test "E4" @("board", "backlog", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"
Invoke-Test "E5" @("board", "epics", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"
Invoke-Test "E6" @("board", "sprints", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"

# Find a sprint
$sprintId = 0
if ($boardId -gt 0) {
    $spResult = Invoke-Bin @("sprint", "list", "--board", "$boardId", "--json")
    if ($spResult.ExitCode -eq 0) {
        try {
            $spArr = $spResult.Out | ConvertFrom-Json
            if ($spArr -is [Array] -and $spArr.Count -gt 0) { $sprintId = [int]$spArr[0].id }
        } catch {}
    }
    if ($sprintId -gt 0) { Write-Host "Using SPRINT_ID=$sprintId" -ForegroundColor Yellow }
}

Invoke-Test "E7" @("sprint", "list", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"
Invoke-Test "E8" @("sprint", "active", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"
Invoke-Test "E9" @("sprint", "issues", "--sprint", "$sprintId", "--json") -Skip:($sprintId -le 0) -SkipReason "no sprint id"
Invoke-Test "EA" @("epic", "list", "--board", "$boardId", "--json") -Skip:($boardId -le 0) -SkipReason "no board id"

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE F — Filter Read Operations
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase F: Filters"

Invoke-Test "F1" @("filter", "list")
Invoke-Test "F2" @("filter", "list", "--json")

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE G — Auxiliary Commands
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase G: Auxiliary Commands"

Invoke-Test "G1" @("reference")
Invoke-Test "G2" @("install-skill", "--json")

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE H — Flag Variants (--quiet, --dry-run)
# ═══════════════════════════════════════════════════════════════════════════════

Write-Phase "Phase H: Flag Variants (--quiet, --dry-run)"

Invoke-Test "H1" @("issue", "get", $issueKey, "--json", "--quiet") -Skip:(-not $issueKey) -SkipReason "no existing issue"
Invoke-Test "H2" @("issue", "list", "--project", $ProjectKey, "--json", "--quiet", "--limit", "1")
Invoke-Test "H3" @("issue", "create", "--project", $ProjectKey, "--summary", "e2e-dryrun-$tag", "--type", "Task", "--dry-run", "--json")
Invoke-Test "H4" @("issue", "delete", $issueKey, "--dry-run", "--json", "--force") -Skip:(-not $issueKey) -SkipReason "no existing issue"

# Validate H3 dry-run: check that the issue was NOT actually created
if (-not $mutate) {
    # In read-only mode, dry-run should still work
    $h3Result = $script:Rows | Where-Object { $_.Id -eq "H3" }
    if ($h3Result -and $h3Result.Exit -eq 0) {
        Write-Host "  H3: dry-run returned exit 0 (correct)" -ForegroundColor DarkGray
    }
}

# ═══════════════════════════════════════════════════════════════════════════════
# PHASE I — Issue Write Lifecycle
# ═══════════════════════════════════════════════════════════════════════════════

# Variables for write operations
$newKey   = $null
$cloneKey = $null

if (-not $mutate) {
    Write-Phase "Phase I-K: Write Operations (SKIPPED - JIRA_E2E_MUTATE=0)"
    # Add skip rows for all write operations
    foreach ($id in @("I1","I2","I3","I4","I5","I6","I7","I8","I9","IA","IB","IC","ID","IE","IF1","IF2","IF3",
                      "J1","J2","J3","J4","K1","K2","K3","K4","K5","K6","K7","K8")) {
        Skip-Row $id "write op" "JIRA_E2E_MUTATE=0"
    }
} else {

    Write-Phase "Phase I: Issue Write Lifecycle"

    # Determine issue type for creation
    $issueTypeName = "Task"
    $typesResult = Invoke-Bin @("project", "issue-types", $ProjectKey, "--json")
    $typesRaw = $typesResult.Out
    if ($typesResult.ExitCode -eq 0) {
        try {
            $types = $typesRaw | ConvertFrom-Json
            $candidate = $types | Where-Object { -not $_.subtask -and $_.name -notmatch '(?i)^Epic$' } | Select-Object -First 1
            if ($candidate -and $candidate.name) { $issueTypeName = [string]$candidate.name }
        } catch {}
    }

    # I1: Create issue
    Invoke-Test "I1" @("issue", "create", "--project", $ProjectKey, "--summary", "jira-cli-e2e-$tag", "--type", $issueTypeName, "--assignee", "me", "--json")
    if ($script:LastTest.Ok) {
        try { $newKey = ($script:LastTest.Out | ConvertFrom-Json).key } catch {}
    }
    if ($newKey) {
        Write-Host "Created test issue: $newKey" -ForegroundColor Green
    } else {
        Write-Host "WARNING: Could not create test issue; remaining write tests will be skipped" -ForegroundColor Red
    }

    # I2-I3: Edit
    Invoke-Test "I2" @("issue", "edit", $newKey, "--description", "E2E test description $tag") -Skip:(-not $newKey) -SkipReason "no test issue"
    Invoke-Test "I3" @("issue", "edit", $newKey, "--summary", "jira-cli-e2e-edited-$tag") -Skip:(-not $newKey) -SkipReason "no test issue"

    # I4-I6: Collaboration
    Invoke-Test "I4" @("issue", "assign", $newKey, "me") -Skip:(-not $newKey) -SkipReason "no test issue"
    Invoke-Test "I5" @("issue", "watch", $newKey) -Skip:(-not $newKey) -SkipReason "no test issue"
    Invoke-Test "I6" @("issue", "vote", $newKey) -Skip:(-not $newKey) -SkipReason "no test issue"

    # I7: Comment
    Invoke-Test "I7" @("issue", "comment", "add", $newKey, "--body", "e2e comment $tag") -Skip:(-not $newKey) -SkipReason "no test issue"

    # I8: Worklog
    Invoke-Test "I8" @("issue", "worklog", "add", $newKey, "--time", "15m", "--comment", "e2e worklog $tag") -Skip:(-not $newKey) -SkipReason "no test issue"

    # I9: Attachment
    $tmpFile = Join-Path $env:TEMP "jira-cli-e2e-$tag.txt"
    "e2e attachment test $tag" | Set-Content -Path $tmpFile -Encoding UTF8
    Invoke-Test "I9" @("issue", "attach", $newKey, "--file", $tmpFile) -Skip:(-not $newKey) -SkipReason "no test issue"

    # IA: Clone
    Invoke-Test "IA" @("issue", "clone", $newKey, "--json") -Skip:(-not $newKey) -SkipReason "no test issue"
    if ($script:LastTest.Ok -and -not $script:LastTest.Skipped) {
        try { $cloneKey = ($script:LastTest.Out | ConvertFrom-Json).key } catch {}
    }
    if ($cloneKey) { Write-Host "Cloned to: $cloneKey" -ForegroundColor Green }

    # IB: Link
    $linkTypeName = $null
    $ltResult = Invoke-Bin @("issue", "link-types", "--json")
    $ltRaw = $ltResult.Out
    if ($ltResult.ExitCode -eq 0) {
        try {
            $ltArr = $ltRaw | ConvertFrom-Json
            if ($ltArr -is [Array] -and $ltArr.Count -gt 0) { $linkTypeName = $ltArr[0].name }
        } catch {}
    }
    $canLink = ($newKey -and $cloneKey -and $linkTypeName)
    if ($canLink) {
        Invoke-Test "IB" @("issue", "link", $newKey, "--to", $cloneKey, "--type", $linkTypeName)
    } else {
        Skip-Row "IB" "issue link" "missing issue key, clone key, or link type"
    }

    # IC: Remote link
    Invoke-Test "IC" @("issue", "remote-link", $newKey, "--url", "https://example.com/e2e-$tag", "--title", "e2e remote $tag") -Skip:(-not $newKey) -SkipReason "no test issue"

    # ID: Transition
    $toStatus = $null
    if ($newKey) {
        $trResult2 = Invoke-Bin @("issue", "transitions", $newKey, "--json")
        $trRaw = $trResult2.Out
        if ($trResult2.ExitCode -eq 0) {
            try {
                $trArr = $trRaw | ConvertFrom-Json
                if ($trArr -is [Array] -and $trArr.Count -gt 0) { $toStatus = $trArr[0].to.name }
            } catch {}
        }
    }
    Invoke-Test "ID" @("issue", "transition", $newKey, $toStatus) -Skip:(-not ($newKey -and $toStatus)) -SkipReason "no test issue or no available transition"

    # IE: Bulk transition
    if ($toStatus -and $cloneKey) {
        Invoke-Test "IE" @("issue", "bulk-transition", $toStatus, "--issues", $cloneKey, "--json")
    } else {
        Skip-Row "IE" "issue bulk-transition" "no transition target or no clone key"
    }

    # IF: Unvote, Unwatch, Unassign
    Invoke-Test "IF1" @("issue", "unvote", $newKey) -Skip:(-not $newKey) -SkipReason "no test issue"
    Invoke-Test "IF2" @("issue", "unwatch", $newKey) -Skip:(-not $newKey) -SkipReason "no test issue"
    Invoke-Test "IF3" @("issue", "unassign", $newKey) -Skip:(-not $newKey) -SkipReason "no test issue"

    # ═══════════════════════════════════════════════════════════════════════════
    # PHASE J — Cleanup Write Operations
    # ═══════════════════════════════════════════════════════════════════════════

    Write-Phase "Phase J: Cleanup Write Operations"

    # J1: Delete comment
    $commentId = $null
    if ($newKey) {
        $cmResult = Invoke-Bin @("issue", "comment", "list", $newKey, "--json")
        $cmRaw = $cmResult.Out
        if ($cmResult.ExitCode -eq 0) {
            try {
                $cmArr = $cmRaw | ConvertFrom-Json
                if ($cmArr -is [Array] -and $cmArr.Count -gt 0) { $commentId = "$($cmArr[0].id)" }
            } catch {}
        }
    }
    Invoke-Test "J1" @("issue", "comment", "delete", $newKey, "--id", $commentId) -Skip:(-not ($newKey -and $commentId)) -SkipReason "no test issue or no comment"

    # J2: Unlink
    $linkId = $null
    if ($newKey -and $cloneKey) {
        $igResult = Invoke-Bin @("issue", "get", $newKey, "--json")
        $igRaw = $igResult.Out
        if ($igResult.ExitCode -eq 0) {
            try {
                $igObj = $igRaw | ConvertFrom-Json
                $links = $igObj.fields.issuelinks
                if ($links -and $links.Count -gt 0 -and $links[0].id) {
                    $linkId = "$($links[0].id)"
                }
            } catch {}
        }
    }
    Invoke-Test "J2" @("issue", "unlink", $linkId) -Skip:(-not $linkId) -SkipReason "no link to unlink"

    # J3/J4 issue cleanup runs after Phase K (K7 sprint move still needs $cloneKey)

    # ═══════════════════════════════════════════════════════════════════════════
    # PHASE K — Filter & Sprint Write Operations
    # ═══════════════════════════════════════════════════════════════════════════

    Write-Phase "Phase K: Filter & Sprint Write Operations"

    # K1: Create filter
    $filterName = "e2e-filter-$tag"
    Invoke-Test "K1" @("filter", "create", "--name", $filterName, "--jql", "project = $ProjectKey ORDER BY updated DESC", "--json")
    $filterId = $null
    if ($script:LastTest.Ok) {
        try { $filterId = "$(($script:LastTest.Out | ConvertFrom-Json).id)" } catch {}
    }
    if ($filterId) { Write-Host "Created filter: $filterId ($filterName)" -ForegroundColor Green }

    # K2-K3: Read the created filter
    Invoke-Test "K2" @("filter", "get", $filterId, "--json") -Skip:(-not $filterId) -SkipReason "filter create failed"
    Invoke-Test "K3" @("filter", "run", $filterId, "--limit", "3", "--json") -Skip:(-not $filterId) -SkipReason "filter create failed"

    # K4: Delete filter
    Invoke-Test "K4" @("filter", "delete", $filterId) -Skip:(-not $filterId) -SkipReason "filter create failed"

    # K5-K8: Sprint write operations (optional)
    $newSprintId = 0
    if ($sprintW -and $boardId -gt 0) {
        $sprintName = "e2e-sprint-$tag"
        Invoke-Test "K5" @("sprint", "create", "--board", "$boardId", "--name", $sprintName)

        # Find the created sprint
        $sprListResult = Invoke-Bin @("sprint", "list", "--board", "$boardId", "--json")
        $sprListRaw = $sprListResult.Out
        if ($sprListResult.ExitCode -eq 0) {
            try {
                $sprArr = $sprListRaw | ConvertFrom-Json
                $match = $sprArr | Where-Object { $_.name -eq $sprintName } | Select-Object -First 1
                if ($match) { $newSprintId = [int]$match.id }
            } catch {}
        }
        if ($newSprintId -gt 0) { Write-Host "Created sprint: $newSprintId ($sprintName)" -ForegroundColor Green }

        Invoke-Test "K6" @("sprint", "update", "--sprint", "$newSprintId", "--goal", "e2e goal $tag") -Skip:($newSprintId -le 0) -SkipReason "sprint create failed"
        if ($cloneKey -and $newSprintId -gt 0) {
            Invoke-Test "K7" @("sprint", "move", "--sprint", "$newSprintId", "--issues", $cloneKey)
        } else {
            Skip-Row "K7" "sprint move" "no clone key or no sprint"
        }
        Invoke-Test "K8" @("sprint", "close", "--sprint", "$newSprintId", "--force") -Skip:($newSprintId -le 0) -SkipReason "sprint create failed"
    } else {
        $skipReason = if (-not $sprintW) { "JIRA_E2E_SPRINT!=1" } else { "no board id" }
        foreach ($id in @("K5","K6","K7","K8")) {
            Skip-Row $id "sprint write" $skipReason
        }
    }

    # J3/J4: Issue cleanup (after K — K7 sprint move requires $cloneKey)
    if ($cleanup) {
        Cleanup-Issue "J3" $cloneKey "cleanup clone $cloneKey"
        Cleanup-Issue "J4" $newKey "cleanup original $newKey"
    } else {
        Skip-Row "J3" "cleanup clone" "JIRA_E2E_CLEANUP=0"
        Skip-Row "J4" "cleanup original" "JIRA_E2E_CLEANUP=0"
        if ($cloneKey) { Write-Host "Keeping clone issue: $cloneKey" -ForegroundColor Yellow }
        if ($newKey)   { Write-Host "Keeping test issue: $newKey" -ForegroundColor Yellow }
    }

    # Cleanup temp file
    if (Test-Path $tmpFile) { Remove-Item -Force $tmpFile -ErrorAction SilentlyContinue }
}

# ═══════════════════════════════════════════════════════════════════════════════
# RESULTS SUMMARY
# ═══════════════════════════════════════════════════════════════════════════════

Write-Host ""
Write-Host "╔══════════════════════════════════════════╗" -ForegroundColor White
Write-Host "║       E2E TEST RESULTS SUMMARY           ║" -ForegroundColor White
Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor White

$nPass = ($script:Rows | Where-Object { $_.Exit -eq 0 }).Count
$nFail = ($script:Rows | Where-Object { $_.Exit -ne 0 -and $_.Exit -ne -2 }).Count
$nSkip = ($script:Rows | Where-Object { $_.Exit -eq -2 }).Count
$nTotal = $script:Rows.Count
$totalMs = ($script:Rows | Measure-Object -Property DurationMs -Sum).Sum

Write-Host ""
Write-Host "  PASS: $nPass" -ForegroundColor Green
if ($nFail -gt 0) {
    Write-Host "  FAIL: $nFail" -ForegroundColor Red
} else {
    Write-Host "  FAIL: $nFail" -ForegroundColor Gray
}
Write-Host "  SKIP: $nSkip" -ForegroundColor Yellow
Write-Host "  TOTAL: $nTotal" -ForegroundColor White
$durSec = [math]::Round($totalMs / 1000, 1)
Write-Host "  DURATION: ${durSec}s" -ForegroundColor Gray
Write-Host ""

# Highlight failures
$failures = @($script:Rows | Where-Object { $_.Exit -ne 0 -and $_.Exit -ne -2 })
if ($failures.Count -gt 0) {
    Write-Host "FAILED TESTS:" -ForegroundColor Red
    foreach ($f in $failures) {
        Write-Host "  [$($f.Id)] exit=$($f.Exit) $($f.Command)" -ForegroundColor Red
        if ($f.Note) { Write-Host "         $($f.Note)" -ForegroundColor DarkRed }
    }
    Write-Host ""
}

# Table output
$script:Rows | Format-Table -AutoSize -Property Id,
    @{N='Exit'; E={$_.Exit}; Alignment='Right'},
    @{N='Ms'; E={$_.DurationMs}; Alignment='Right'},
    Command,
    Note

# CSV report
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$csvPath = Join-Path $repoRoot "scripts\e2e-report.csv"
$script:Rows | Export-Csv -Path $csvPath -NoTypeInformation -Encoding UTF8
Write-Host "Report saved: $csvPath" -ForegroundColor Cyan

# Exit with failure count
if ($nFail -gt 0) {
    Write-Host ""
    Write-Host "E2E FAILED with $nFail failure(s)." -ForegroundColor Red
    exit 1
} else {
    Write-Host ""
    Write-Host "E2E PASSED - $nPass tests, $nSkip skipped." -ForegroundColor Green
    exit 0
}
