#requires -Version 5.1
<#
.SYNOPSIS
  Compatibility wrapper for the full jira-cli E2E suite.

.DESCRIPTION
  The canonical E2E flow lives in e2e-full.ps1. Keeping one implementation
  prevents command parsing, JSON envelope handling, and dry-run/confirm behavior
  from drifting across scripts.
#>

$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "e2e-full.ps1") @args
exit $LASTEXITCODE
