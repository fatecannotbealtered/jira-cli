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

