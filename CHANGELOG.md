# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the release note every Sectile surface shows: the web interface
serves it from the running server, and the desktop app embeds it at build time.
It is written for the people who use Sectile, not for the people who wrote it —
one line per user-visible change, in English, and nothing about refactorings,
test fixtures or internal plumbing.

## [Unreleased]

### Security

- **An agent, MCP client or machine API call now needs a real workstation key.**
  A server that had issued no key used to accept any nonempty token and treat
  its holder as the `default` account, which may be an administrator. That
  fallback is gone, and deleting the last account holding a key no longer
  reopens it. `SECTILE_SERVER_TOKEN`, still deprecated, remains the one way to
  keep an agent running while you pair a workstation for it.

## [0.1.0] - 2026-09-22

First tagged release. Sectile had been running from `main` since its first
commit; this entry names what that unversioned history had built, and the
release mechanism that will keep the following entries short.

### Added

- **Versioning and releases.** Every executable is built from a Git tag and
  reports it: `sectile-server --version`, `sectile-agent --version`, the
  `GET /api/version` route, the web interface footer, and the General section of
  the desktop app's settings. A build cut outside a tag reports `dev` rather
  than claiming a release.
- **Changelog in the product.** This file is served by the server at
  `GET /api/changelog` and shown in the web interface from the version in the
  footer; the desktop app embeds it at build time and shows it in its settings.
- **Multi-project board.** Projects backed by GitHub, Jira or a local tracker,
  with per-project board columns, sprints, teams, epics and the mapping from a
  tracker status to a Sectile workflow stage.
- **Workflow stages.** Clarify, specify, implement and adjust, each driven by a
  skill, with the pull request as the deliverable and the human as the merger.
- **Local agent.** A workstation daemon paired to the server once, which runs
  the coding CLIs, owns the Git worktrees and exposes the Sectile MCP server to
  the agents it launches.
- **Desktop app.** Execution consoles, the worktree diff, the task queue and the
  local agent's own controls, in an Electron shell around the same agent.
- **Sign-in and roles.** Mandatory sign-in through OpenID Connect or a local
  e-mail identity, an admin and a member role, and executions owned by whoever
  started them.
- **Personal tracker credentials.** Each account stores its own tracker tokens,
  encrypted at rest and optionally sealed behind a passphrase, so a write lands
  under the name of whoever asked for it.
- **PostgreSQL as an alternative store.** `DB_DRIVER=postgres` alongside the
  default SQLite file, with a one-shot migration tool.
- **Container image and workstation binaries.** A distroless server image, and
  `sectile-server` / `sectile-agent` cross-compiled for macOS, Linux and Windows.

[Unreleased]: https://github.com/sebastienferry/sectile/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sebastienferry/sectile/releases/tag/v0.1.0
