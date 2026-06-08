# Open Source Checklist

Use this checklist before public pushes and before each tagged release.

## Repository

- [ ] README and README_zh describe the same install, auth, output, and safety flows.
- [ ] CHANGELOG.md has an Unreleased section and versioned entries.
- [ ] CHANGELOG.md is the single source for release notes and runtime changelog.
- [ ] LICENSE, CONTRIBUTING.md, SECURITY.md, NOTICE.md, and CODE_OF_CONDUCT.md are present.
- [ ] docs/COMPATIBILITY.md and docs/E2E.md are current.
- [ ] No generated release notes, binaries, dist folders, or local credential files are committed.

## CLI Contract

- [ ] JSON mode emits exactly one success or failure envelope on stdout.
- [ ] Error codes, exit codes, and retryable flags are aligned.
- [ ] Write commands require `--dry-run` then `--confirm <token>` in JSON mode.
- [ ] `reference`, `context`, `doctor`, and `changelog` are available.
- [ ] External Jira content returned by default JSON is tagged with `_untrusted`.

## Security

- [ ] Risk tier is recorded in SECURITY.md and `jira-cli reference`.
- [ ] Saved PATs are encrypted at rest and config files use owner-only permissions.
- [ ] npm install verifies checksums and fails closed.
- [ ] No token, cookie, Authorization header, or PAT appears in stdout, stderr, docs, tests, or audit logs.

## Verification

- [ ] `gofmt -w .` has no pending formatting changes.
- [ ] `go vet ./...` passes.
- [ ] `go test ./...` passes.
- [ ] E2E read-only mode passes against a real Jira Data Center instance when credentials are available.
