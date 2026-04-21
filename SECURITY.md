# Security

## Supported versions

Security fixes are applied to the latest minor release on the default branch (`main`). Release binaries are published via GitHub Releases and the npm package `@fatecannotbealtered-/jira-cli`.

## Reporting a vulnerability

Please **do not** file a public GitHub issue for undisclosed security vulnerabilities.

Instead, report privately via [GitHub Security Advisories](https://github.com/fatecannotbealtered/jira-cli/security/advisories/new) for this repository, or contact the maintainers through the contact options on the repository homepage.

Include:

- Description of the issue and impact
- Steps to reproduce (if safe to share)
- Affected versions or install methods (binary / npm / `go install`)

You should receive an acknowledgment as capacity allows. Thank you for helping keep users safe.

## Credential handling (design)

- Credentials are stored only in `~/.jira-cli/config.json` with file mode `0600` and directory `0700`.
- API tokens are read with hidden input in interactive terminals.
- Traffic is HTTPS-only to the configured Jira Data Center host (host must start with `https://`).
- Environment variables `JIRA_HOST` and `JIRA_TOKEN` take precedence over config file; prefer them in CI/Agent workflows to avoid persisting credentials on disk.

Review these assumptions when integrating jira-cli into automation or AI agent workflows.
