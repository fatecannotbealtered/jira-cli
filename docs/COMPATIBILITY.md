# Compatibility

jira-cli targets Jira Data Center and Jira Server style REST APIs. Jira Cloud is
not a supported target because user identity, authentication, and API behavior
are different.

## Verified Surface

| Area | API | Notes |
|------|-----|-------|
| Issues, projects, users, filters, fields | `/rest/api/2` | PAT Bearer auth, Data Center username (`name`) references |
| Boards, sprints, backlog, epics | `/rest/agile/1.0` | Agile plugin endpoints, paginated |
| Attachments | `/rest/api/2/issue/{key}/attachments` | Upload uses `X-Atlassian-Token: no-check`; downloads are restricted to the configured Jira host |

## Expected Requirements

- Jira host must be HTTPS.
- Personal Access Tokens must be enabled by the Jira administrator.
- The authenticated user must already have Jira project permissions for the
  command being run.
- Proxies and VPN access are environment concerns; verify with `jira-cli doctor`.

## Non-Goals

- Jira Cloud accountId-based APIs.
- OAuth flows.
- Administrative Jira configuration changes outside the authenticated user's
  ordinary project permissions.
