# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2026-06-06

### Added

- **Agent JSON envelope** — default JSON success and failure responses now share a stable envelope: `ok`, `schema_version`, `data`/`error`, and `meta.duration_ms`.
- **Confirm-token write flow** — JSON write commands now support the non-interactive `--dry-run` → `--confirm <token>` flow. Dry-run responses include a change preview, `confirm_token`, and `expires_at`; execution validates that the token still matches the operation.
- **Self-description commands** — added `context`, and expanded `reference` and `doctor` output for agent discovery and environment checks.
- **Structured error taxonomy** — error envelopes now include stable `E_*` codes and `retryable` hints for automated retry decisions.

### Changed

- **JSON is the agent contract** — stdout now contains exactly one JSON document for normal command responses; human-readable output requires `--format text`.
- **Error JSON moved to stdout** — machine-readable failures now use the same stdout channel and envelope shape as successes. Progress, prompts, warnings, and text-mode errors remain on stderr.
- **Exit code semantics** — exit codes now follow the agent contract: `2` bad args, `3` not found, `4` auth/permission, `5` confirmation required, `6` conflict, `7` retryable transient failure, `8` timeout.
- **Interactive login is text-only** — default JSON mode requires `jira-cli login --host <url> --token <pat>`; interactive login requires `--format text`.
- **`doctor` output** — JSON output now reports a `checks` list with `check`, `status`, `message`, and `fix` fields instead of the old `authValid` shape.

### Fixed

- **JSON write safety** — confirmed JSON writes no longer fall through to stdin prompts after token validation.
- **Stable confirmation hashing** — write-command confirm tokens include the full operation details and stable slice handling in tests.

## [1.0.6] - 2026-06-05

### Added

- **Built-in `update` command** — standalone binaries can check GitHub Releases, verify `checksums.txt`, and replace the current executable. Package-manager installs are detected and guided back to the package manager unless `--force` is used.
- **Global output format control** — `--format json|text|raw` now controls CLI output across commands. JSON is the default for agent-friendly parsing, `text` keeps human-readable tables and prompts, and `raw` is explicitly supported only by commands that can return raw output.

### Changed

- **Agent-friendly defaults** — commands now emit stable JSON by default, while `--json` remains a compatibility alias for `--format json`.
- **Reference output** — `jira-cli reference` now defaults to structured JSON; use `jira-cli --format text reference` for the Markdown command reference.
- **Output flag rules** — `--compact` and `--fields` are JSON-only, `--quiet` no longer suppresses JSON/raw result bodies, and unsupported `raw` output returns an argument error instead of silently downgrading.

### Fixed

- **Structured argument errors** — Cobra argument validation errors now return machine-readable JSON by default instead of plain text.
- **Clean JSON stdout** — interactive prompts and cancellation/error paths no longer pollute JSON success output.

## [1.0.5] - 2026-06-02

### Added

- **`issue attachments` download** — the command can now fetch attachment content, not just list metadata. `--out <dir>` downloads all attachments into a directory; add `--id <attachmentId>` to download a single one (exit code 4 if not found). With `--json` it prints `{id, filename, path, mimeType}` per saved file. Downloads stream to disk (handles large files like screen recordings) and validate the content URL points at the configured Jira host. The `attachments --json` listing now also includes the `content` download URL.

## [1.0.4] - 2026-05-31

### Fixed

- **`issue get` description** — `description` is now included in the default flat output and selectable via `--fields description` (previously only reachable through `--raw`). Bulk outputs (`issue list`, `search`, `filter run`, `sprint issues`) stay lean and omit `description` by default to preserve token efficiency. Handles both Data Center (plain string) and Cloud/v3 (ADF) descriptions.

## [1.0.3] - 2026-05-26

### Fixed

- **Semantic exit codes** — validation, auth, and config failures now return non-zero exit codes (via `SilentErr`); `doctor` and `login` work correctly in CI scripts.
- **Audit logging on failed writes** — write commands are logged even when the operation fails.
- **`FilterSprintFields`** — camelCase sprint fields (`startDate`, `endDate`) filter correctly with `--fields`.
- **Config file corruption detection** — invalid JSON in `~/.jira-cli/config.json` returns a clear parse error.
- **`issue edit --description`** — plain text is converted to ADF, matching `issue create`.
- **Agile API pagination** — sprint/board list, backlog, epics, and epic issues fetch all pages instead of truncating at ~50 items.
- **`ListWorklogs` pagination** — worklogs beyond the first page are included.
- **`search --order-by`** — skips appending `ORDER BY` when JQL already contains one.
- **`issue delete --dry-run`** — no longer prompts for confirmation before previewing.
- **`issue bulk-transition`** — `skipped` and `failed` counts are reported separately in JSON output.
- **E2E script** — sprint move (K7) runs before issue cleanup; `JIRA_E2E_CLEANUP=0` is honored.

### Added

- **`cmd.Run()` / `Main()`** — testable CLI entry point with correct exit code propagation.
- **Comprehensive unit tests** — 100% statement coverage across all packages; httptest-based command integration tests.
- **`internal/api/agile_page.go`** — shared pagination helper for Jira Agile API endpoints.

### Changed

- **Minimum Go version** — `go.mod` updated to Go 1.24 (uses `testing.T.Chdir` in tests).

### Documentation

- Clarify `search --fields` (Jira fetch) vs `issue get/list --fields` (output trimming).
- Document `issue delete --dry-run` skips confirmation; remove misleading `sprint close --force` wording.
- Add `epic list` / `epic issues` to README; document `filter run --raw/--fields`.
- Document stdout (success JSON) vs stderr (error JSON); `doctor` exit code and `authValid` checks.
- Note npm install requires `curl`; add `install-skill` and `issue list` vs `search` JSON/jq examples.

## [1.0.2] - 2026-05-14

### Fixed

- **Retry loop now respects context cancellation** — long-running API calls with rate limiting or server errors can now be interrupted by the caller.
- **Upload retry loop now respects context cancellation** — file uploads honor context cancellation during retries.
- **Token sanitization improved** — audit logging now handles `--token=value` format for all case variations (e.g., `--TOKEN=`, `--Token=`).
- **Config file corruption detection** — invalid JSON in `~/.jira-cli/config.json` now returns an error instead of silently continuing with empty values.
- **Timing attack prevention** — confirmation prompts use constant-time comparison to prevent timing-based information leakage.

### Security

- **Audit logging uses constant-time comparison** to prevent timing attacks on confirmation inputs.
- **Sensitive flags aligned with JDC-only PAT authentication** — audit sanitization targets only `--token` and `-t` since this CLI uses PAT only, no password support.

## [1.0.1] - 2026-05-06

### Fixed

- **Audit log `sanitizeArgs` now handles `--token=value` format** — previously `--token=secret` would be logged in plaintext.
- **`SearchAll` capped at 10,000 issues** to prevent unbounded memory usage on large result sets.
- **`sprint active` supports multiple active sprints** — previously only the first was returned.
- **`issue clone` extracts project key from source issue** instead of scanning the issue key string for `-`.
- **Removed empty `internal/api/compat.go`** file.

### Added

- **`filter run` supports `--raw` and `--fields`** flags, consistent with `search` and `issue list`.
- **`board backlog` and `board epics` support `--limit`** flag (default 50, max 200).
- **`issue watch`/`unwatch`/`vote`/`unvote` produce JSON output** with `--json` flag.
- **`parseTimeSpent` supports `d` (days) and `w` (weeks)`** units — `1d` = 8h, `1w` = 5d (Jira defaults).
- **`make test` now runs `gofmt` and `go vet`** before unit tests.
- **Release workflow extracts version-specific notes** from CHANGELOG.md for GitHub Release body.

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

[Unreleased]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.6...v1.1.0
[1.0.6]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.5...v1.0.6
[1.0.5]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/fatecannotbealtered/jira-cli/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/fatecannotbealtered/jira-cli/releases/tag/v1.0.0
