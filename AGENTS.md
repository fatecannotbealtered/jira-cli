# Agent Entry

This repository follows the reusable AI-native tool specs in `.agent/`.

Start with `.agent/AGENT.md`, then read only the spec relevant to the task:

- `.agent/CLI-SPEC.md` for CLI output, errors, write confirmation, and self-description.
- `.agent/SKILL-SPEC.md` for bundled Skill behavior.
- Shared [`REPO-SPEC.md`](https://github.com/fatecannotbealtered/ai-native-cli-spec/blob/main/REPO-SPEC.md) for repository layout and release conventions.
- `.agent/SEC-SPEC.md` for security tier, credential, injection, and supply-chain rules.

For this tool, keep the CLI contract, README files, `skills/jira-cli/SKILL.md`,
E2E scripts, and `CHANGELOG.md` synchronized in the same change.
