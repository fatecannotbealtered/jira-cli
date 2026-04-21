# Contributing

Thank you for improving jira-cli. This document describes how to build, test, and submit changes.

**Note:** This is a side project shared for learning and personal use; maintainers do not offer commercial support or production guarantees—see the readme disclaimer.

## Development setup

- Go **1.23+** (see `go.mod`)
- Optional: **Node.js 16+** if you work on npm install scripts
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
| Build with version | `make build` (Unix) or `go build -ldflags "-s -w -X github.com/fatecannotbealtered/jira-cli/cmd.version=dev" -o bin/jira-cli.exe ./cmd/jira-cli` (Windows) |

CI mirrors `.github/workflows/ci.yml`: tidy modules, `gofmt` check (Linux), golangci-lint, `go vet`, build, `go test -race`, and a `--help` smoke test.

## Pull requests

1. **One logical change per PR** when possible.
2. **Tests**: add or update tests for behavior changes in `internal/` or stable CLI contracts.
3. **Docs**: update `README.md` / `README_zh.md` if user-facing flags or flows change; add a line to `CHANGELOG.md` under *Unreleased*.
4. **Commits**: clear messages; no secrets or real tokens in code or docs.

## AI Agent skill bundle

Bundled skills live under `skills/`. After editing, run `jira-cli install-skill` from a built binary (or from repo root with `./skills`) to confirm files copy correctly. npm installs place the binary under `bin/` and skills under `../skills` relative to the binary; the CLI resolves both layouts.

## Security

Do not open public issues for undisclosed security vulnerabilities. See [SECURITY.md](SECURITY.md).
