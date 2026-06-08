package jiracli

import _ "embed"

// ChangelogMarkdown is embedded from the repository root and used by the CLI
// changelog command. CHANGELOG.md remains the single human-maintained source.
//
//go:embed CHANGELOG.md
var ChangelogMarkdown string
