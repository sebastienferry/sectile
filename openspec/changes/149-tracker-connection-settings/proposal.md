# GitHub and GitLab connection parameters live in the user configuration

## Why
Jira is configured from the interface: `jiraUrl`, `jiraEmail` and a write-only
`jiraApiToken` sit on the server `Settings` row. GitHub is not: `internal/trackerapi.NewClient`
reads `SECTILE_GITHUB_API_URL` and a token from the process environment, once, at
startup. GitLab has nothing at all. A user who installs the desktop application
cannot connect it to GitHub without editing an environment file and restarting the
server, and cannot name a GitLab instance anywhere.

## What Changes
The server `Settings` row gains the GitHub and GitLab connection parameters, shaped
exactly like the Jira ones: `githubApiUrl`, `githubToken`, `gitlabUrl`,
`gitlabProject`, `gitlabToken`, each token write-only and reported back only through
a `...Set` / `...FromEnv` flag pair. A project may override the instance URL, the
repository/project slug and the token, so one workstation can drive two GitHub
organisations or a GitHub repository next to a GitLab one.

Credentials are resolved per request — project override, then stored settings, then
the existing environment variables — instead of being frozen in a process-wide
client at startup. Stored configuration wins; the environment stays a working
fallback for headless and CI deployments, surfaced in the interface as "provided by
the environment".

The tracker setup screen becomes tracker-aware instead of Jira-shaped, and
check-before-save is implemented for GitHub and GitLab against their `GET /user`
endpoints.

Storing GitLab parameters does not make GitLab a usable tracker: no `gitlab` adapter
is registered in `internal/tracker.Registry`, and this change does not write one.

## Capabilities
### Added Capabilities
- `tracker-connection-settings`: user-configured GitHub and GitLab connection
  parameters, their per-project override, their resolution order and their
  verification before save.

## Impact
`internal/models/models.go` (`Settings`, `Project`, their request shapes),
`internal/db/db.go` (settings columns and migrations, project columns, per-request
credential resolution, the tracker client wiring at `NewDB`),
`internal/trackerapi/client.go` (credentials become per-call rather than
per-process), `internal/handlers/handlers.go` (`HandleTrackerSetup`),
`web/src/components/TrackerSetup.tsx`, `ProjectModal.tsx`, `AppContext.tsx`, README.

Additive SQLite migrations only. A server configured today through
`SECTILE_GITHUB_TOKEN` keeps working with no change.
`~/.config/sectile/settings.json` stays secret-free, per `internal/agentconfig`'s
versioned contract.
