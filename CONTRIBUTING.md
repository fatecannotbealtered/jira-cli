# Contributing

Thank you for improving jira-cli. This document describes how to build, test, and submit changes.

**Note:** This is a side project shared for learning and personal use; maintainers do not offer commercial support or production guarantees. See the readme disclaimer.

## Development setup

- Go **1.25+** (see `go.mod`)
- Optional: **Node.js 16+** if you work on the npm wrapper or platform-package scripts
- Optional: **golangci-lint** (CI runs it on Linux)

Clone and verify:

```bash
git clone https://github.com/fatecannotbealtered/jira-cli.git
cd jira-cli
go mod download
go test ./...
go build -o bin/jira-cli ./cmd/jira-cli
./bin/jira-cli --help
```

If `go mod download` is slow, try a regional proxy, for example:

```bash
# Example (China)
set GOPROXY=https://goproxy.cn,direct
```

## Commands

| Goal | Command |
|------|---------|
| Run tests (race) | `go test -race ./...` |
| Format | `gofmt -w .` |
| Vet | `go vet ./...` |
| Lint | `golangci-lint run ./...` (or `make lint` on Unix) |
| npm supply-chain check | `npm ci --ignore-scripts && npm audit --audit-level=high` |
| Build with version | `make build` (Unix) or `go build -ldflags "-s -w -X github.com/fatecannotbealtered/jira-cli/cmd.version=dev" -o bin/jira-cli.exe ./cmd/jira-cli` (Windows) |

CI mirrors `.github/workflows/ci.yml`: tidy modules, `gofmt` check (Linux), golangci-lint, `go vet`, build, `go test -race`, and a `--help` smoke test.

## Functional contract coverage

Release standard: **Functional Contract Coverage = 100%**. Every public behavior documented in README, Skill, `jira-cli reference`, `--help`, `context`, `doctor`, `changelog`, or `update` must have automated command-level tests.

For each new or changed command, cover success, invalid arguments, config/auth/permission failure where applicable, upstream failure or timeout where applicable, JSON envelope shape, output schema, exit code, stdout/stderr boundary, and non-interactive behavior. Every bug fix that changes observable behavior needs a regression test.

Numeric line coverage is tracked separately and may ratchet upward, but it does not replace missing contract tests.

Release readiness is machine-readable:

- `stable`: FCC is 100%, mock upstream/contract tests cover success and failure paths, and live smoke/E2E evidence is recorded for the release candidate.
- `beta`: FCC is 100% and mock upstream/contract tests are complete, but live smoke/E2E evidence is missing or explicitly unavailable.
- `unpublishable`: any public behavior lacks command-level tests, or mock upstream/contract tests cover only happy paths.

Keep `jira-cli reference` `release_readiness` and `jira-cli doctor`'s `release_readiness` check honest when test evidence changes.

## Pull requests

1. **One logical change per PR** when possible.
2. **Tests**: add or update tests for behavior changes in `internal/` or stable CLI contracts.
3. **Docs**: update `README.md` / `README_zh.md` if user-facing flags or flows change; add a line to `CHANGELOG.md` under *Unreleased*.
4. **Commits**: clear messages; no secrets or real tokens in code or docs.
5. **Agent contract**: if CLI output, write flow, errors, security posture, or Skill guidance changes, check `.agent/AGENT.md` and the matching spec before opening a PR.

## AI Agent skill bundle

Bundled skills live under `skills/`. After editing, verify the Skill files are included with `npm pack --dry-run --json`; initial Skill installation is handled by `npx skills add ...`, not by the Jira CLI binary.

The Skill must stay small and defer command/flag/schema truth to `jira-cli reference`. If the Skill starts using a new command or output field, raise `metadata.requires.min_version` and verify `jira-cli doctor` reports the bundled minimum.

## Security

Do not open public issues for undisclosed security vulnerabilities. See [SECURITY.md](SECURITY.md).
