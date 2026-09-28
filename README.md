# Sectile

Sectile is a local-first workflow manager for software teams. It brings tracker
work, specification-driven development, coding agents, pull requests, and
worktree isolation into one board—without requiring an online coding agent.

It consists of three complementary components:

- **Server**: a Go application that serves the board, API, synchronization
  queue, and embedded web UI.
- **Workstation agent**: a local Go process that owns repository access,
  worktrees, coding CLI launches, and MCP integrations.
- **Desktop app**: an Electron companion for local execution, console output,
  project management, and worktree-change inspection.

## What it does

- Organizes work across **GitHub, GitLab, Jira, and local tasks**.
- Guides each task through **Clarify → Specify → Implement → Adjust → Done**.
- Supports **Spec Kit** and **OpenSpec** setup directly from the interface.
- Launches configured coding engines, tracks their activities, and keeps
  workflow transitions explicit.
- Provides Kanban and list views, saved cross-project views, filters, search,
  and keyboard-driven actions.
- Runs on SQLite by default, with PostgreSQL available for server deployments.

Clarification rounds publish their full report sections on the ticket and retain
Markdown history. In project workflow settings, `pushStageCommits` optionally
pushes clarification and specification commits; it is off by default. Installed
skills receive the updated instructions when the agent regenerates them.

## Requirements

- Go **1.26.6** or newer
- Node.js **22.18.0** or newer with npm
- Git

Optional integrations require the relevant local tools and credentials: GitHub
CLI, GitLab or Jira API tokens, coding CLIs, Docker, or PostgreSQL.

## Quick start

Install the web dependencies and build the server and workstation agent:

```sh
npm ci --prefix web
make server agent
```

Start the server with SQLite:

```sh
DB_PATH=./tasks.db PORT=8090 ./bin/server
```

Open [http://localhost:8090](http://localhost:8090). The server does not open
a browser, start a terminal, or run a coding agent on its own.

To develop the server and web UI, use two terminals:

```sh
make serve
```

```sh
npm run dev --prefix web
```

Vite runs on port 5173 and proxies API requests to port 8090.

## Connect a workstation

From the server profile, create a pairing code under **Pair a workstation**.
Then, on the machine that has the repository and coding tools:

```sh
./bin/agent pair --url http://localhost:8090 --code '<pairing code>'
./bin/agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone
```

The agent owns local Git operations and launches. Keep tracker credentials on
the server and personal write credentials in the profile; do not put tokens in
the agent configuration.

### Install Sectile in your coding CLI

The agent runs the workflow skills it finds installed; it never installs them
by itself. There are two ways to install them, with the Sectile MCP server:

- **Claude: the `sectile` plugin.** Build it with
  `make plugin VERSION=0.1.0 OUT=/tmp/sectile-plugin ARGS=-marketplace`, then
  `claude plugin marketplace add /tmp/sectile-plugin` and
  `claude plugin install sectile@sectile`. Claude asks for the server URL and
  the workstation API key (from the desktop's Connection settings) and keeps
  the key in its secure storage. The skills run as `/sectile:clarify-issue`
  and so on.
- **Any CLI: the direct setup.** `./bin/agent init --provider claude` (or
  `codex`, `agy`, `gemini`, `cursor`, `vibe`), or **Initialize** in the
  desktop's Deployment settings, copies the skills into the CLI's user folder
  and registers the MCP server. It is the only route for CLIs other than
  Claude.

Under **Settings → Execution defaults**, the desktop chooses which source a
dispatch tries first (direct copy by default) and whether a skill a project
edited in Sectile runs instead of the installed one (on by default). An edited
skill is handed to its run, so it needs no installation. See
[ADR 0039](docs/adrs/0039-sectile-is-installed-as-a-claude-plugin.md).

## Common commands

| Command | Purpose |
| --- | --- |
| `make server` | Build the embedded web UI and server binary. |
| `make agent` | Build the workstation agent. |
| `make serve` | Run the server from source. |
| `make run` | Build and launch the desktop app. |
| `make desktop` | Build the desktop app without launching it. |
| `make desktop-package` | Package the desktop application. |
| `make test` | Run Go and web checks. |
| `make release` | Cross-compile server and agent binaries into `dist/`. |
| `make plugin VERSION=X.Y.Z OUT=DIR` | Write the Sectile Claude plugin (`ARGS=-marketplace` wraps it in a one-plugin marketplace). |
| `make help` | List all supported targets. |

## Configuration

The interface is the preferred place to configure projects and trackers.
Administrators can set server-side credentials per provider; users supply their
own credentials for tracker writes, preserving authorship.

For headless or local development, copy the sample environment file and set
only the values you need:

```sh
cp .env.sample .env
```

The server reads `.env` at startup. Its most common settings are `PORT`,
`DB_PATH`, and server tracker credentials such as `SECTILE_GITHUB_TOKEN`.
See [`.env.sample`](./.env.sample) for the complete reference.

### MCP behind a hosting proxy

If MCP returns `403 Forbidden: invalid Host header` while the workstation agent
connects successfully, the hosting ingress may be forwarding to the server over
loopback while keeping its public hostname. The MCP loopback protection rejects
that combination unless the server explicitly trusts the public Host:

```sh
export SECTILE_MCP_ALLOWED_HOSTS='sectile.example.com'
```

Use the hostname from the server's public URL, including a generated hosting
domain. Set this variable on every **server** instance and restart/redeploy it;
the workstation agent and Codex MCP URL do not need to change. Multiple hosts
are comma-separated and matched exactly, ignoring case. Include `:port` if it
appears in the forwarded Host; do not include a scheme, path or wildcard.

With no setting, loopback requests still require a loopback Host. Configuring a
public Host adds only that authority to the loopback check; other hosts remain
blocked there, and connections to non-loopback interfaces retain their existing
behavior. Bearer authentication and rejection of browser origins always apply.
An unauthenticated `401` does not test Host acceptance: authenticate an MCP
initialization and verify `tools/list` after deployment. See
[ADR 0037](docs/adrs/0037-explicit-mcp-ingress-hosts.md).

### PostgreSQL

SQLite is the default and is what the desktop application ships with. A server
deployment can use PostgreSQL instead:

```sh
export DB_DRIVER=postgres
export DATABASE_URL='postgres://sectile:password@db.internal:5432/sectile?sslmode=require'
export SECTILE_SECRET_KEY='<64 hex characters>'
./bin/server
```

Use one server instance per database unless the deployment follows the shared
PostgreSQL design documented in the architecture guide.

## Desktop application

Build and start the local companion with:

```sh
make run
```

It includes the workstation agent and provides execution queues, an
agent-owned console, project settings, MCP connections, and a read-only view
of uncommitted worktree changes. For release installation and desktop-specific
configuration, read the [Desktop guide](./desktop/README.md).

## Testing

See [database test fixtures and performance](docs/TESTING.md) for isolated SQLite
fixtures, migration-test exceptions, and reproducible timing commands.

Run the standard validation suite:

```sh
make test
```

Web browser tests use Playwright and are intentionally separate from the unit
test command. Run them from `web/` with an absolute path to Playwright's
`index.mjs`; the browser-test scripts live in [`web/tests`](./web/tests).

## Documentation

- [User guide](./docs/USER_GUIDE.md): sign-in, tracker credentials, projects,
  workstation setup, coding prompts, autonomous runs, and Desktop.
- [Architecture](./docs/ARCHITECTURE.md): component boundaries, persistence,
  concurrency, worktrees, and console protocol.
- [Capabilities](./docs/CAPABILITIES.md): workflow stages, tracker support,
  agents, and task access policy.
- [API and data model](./docs/API_AND_DATA_SPEC.md): REST API, SQLite schema,
  and server-agent contracts.
- [UX components](./docs/UX_COMPONENTS.md): board, list, desktop, and diff
  experience.
- [Desktop guide](./desktop/README.md): installation, packaging, local
  configuration, and MCP setup.
- [Architecture decisions](./docs/adrs): durable technical decisions.
- [Changelog](./CHANGELOG.md): user-visible changes and release notes.

## Releases

Sectile releases are annotated Git tags in the form `vX.Y.Z`. The tag is the
single source of truth for the version reported by the server, agent, web UI,
and desktop app. See [`AGENTS.md`](./AGENTS.md) for the required release
procedure and [the changelog](./CHANGELOG.md) for release notes.

## License

No license is currently declared for this repository. Contact the repository
owner before redistributing or reusing the code.
