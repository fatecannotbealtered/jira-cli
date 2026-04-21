# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-05-03

Initial release of jira-cli for Jira Data Center.

### Features

- **Jira Data Center only**: Bearer PAT authentication, DC `name`-based user references, `/rest/api/2/` exclusively.
- **Complete coverage**: issues, sprints, boards, epics, projects, users, filters — CRUD + advanced operations.
- **AI Agent friendly**:
  - `--json` outputs token-efficient flat format by default; `--raw` returns full Jira API response.
  - `--fields key,summary,status` to output only specified fields.
  - `--quiet` suppresses all non-JSON stdout (clean pipe output).
  - `--dry-run` on all write commands — previews without executing.
  - `--force` skips interactive confirmation prompts.
  - Machine-readable error codes (`errorCode`) and actionable hints (`hint`) in JSON error responses.
  - Semantic exit codes: 0=OK, 2=bad args, 3=auth, 4=not found, 5=forbidden, 6=rate limited, 7=network.
  - `reference` command: structured markdown listing of all commands and flags for AI self-discovery.
- **Smart retry**: automatic exponential backoff on 429 rate limits and 5xx errors.
- **Beautiful output**: colored tables with CJK character width support.
- **Custom fields**: set custom fields during create and edit via `--field "Name=Value"`.
- **Environment variables**: `JIRA_HOST` and `JIRA_TOKEN` override config file for CI/Agent use.
- **npm distribution**: `npm install -g @fatecannotbealtered-/jira-cli` with bundled AI Agent Skill.
- **Cross-platform**: Linux, macOS, Windows (x64 + arm64) via GoReleaser.
- **E2E test scripts**: Comprehensive PowerShell E2E script (`scripts/e2e-full.ps1`) covering all 55+ commands against a real Jira DC instance, with CSV report output and read-only mode.
- **Audit logging**: Automatic JSONL audit trail for all write commands (`~/.jira-cli/audit/`), with monthly file rotation and configurable retention (default 3 months). Disable with `JIRA_NO_AUDIT=1`.

### Documentation

- Bilingual README (English + Chinese) with CI/Report Card/License/npm badges.
- SKILL.md with JSON output schemas, error codes, exit codes, and complete flag reference.
- GitHub PR template for contributors.

[Unreleased]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/fatecannotbealtered/jira-cli/releases/tag/v1.0.0
