# Sectile

Sectile is a local-first workflow manager for software teams. It unifies issue
trackers, specification-driven development (SDD), coding agents, pull requests,
and isolated Git worktrees into a single board—while keeping repository checkouts
and coding CLIs on your own workstations.

It consists of three complementary components:

- **Server**: a Go application that serves the web board, REST API, tracker
  synchronization queue, MCP endpoint, and embedded web UI.
- **Workstation agent**: a local Go process (`sectile-agent`) that owns
  repository access, Git worktree isolation, coding CLI launches, and local MCP
  bridges. Multiple workstations can stay connected simultaneously and report
  which project checkouts they hold.
- **Desktop app**: an Electron companion (`Sectile Desktop`) bundling the
  workstation agent with native PTY terminals, an interactive Antigravity,
  Claude Code & Codex conversation view, a desktop project board, and a live
  worktree diff inspector.

## What it does

- Synchronizes work across **GitHub, GitLab, Jira, and local tasks** with
  credential split: every action you take, from the web, the desktop app or an
  agent working for you, uses your own access (a sealed token, or a Jira,
  GitHub or GitLab OAuth connection); server credentials serve the background
  sync.
- Guides tasks through a six-column SDD lifecycle:
  **New → Clarified → Specified → Implemented → Reviewed → Finished**
  (`clarify-issue`, `specify-issue`, `implement-issue`, `create-pr` /
  `adjust-issue`, and `handoff-issue`), plus single-ticket (`pickup-issue`) and
  batch (`pickup-issues`) chains.
- Offers on-demand [backlog orchestration](./docs/CAPABILITIES.md):
  autonomous task chains run as the requesting user, assess project eligibility
  guidelines before changes, share a project concurrency limit, and stop at
  **Implemented** for human review.
  Desktop shows compact agent rows in **ORCHESTRATION**, with an **Agent overview**
  for task activity, workflow stages, waiting states, sign-in recovery, console
  access and **Run agent**. Shared project capacity appears above
  **PROJECTS**, only while a project is enabled or task agents are active.
  Configure it in **Settings → project → Orchestration**: enablement, concurrency
  and multiple named agents, each with its own local guidelines file (`AGENTS.md`
  by default, or another project-local `.md` file). **Add agent** creates another
  policy; **Run agent** reads that agent's file again before launching. All agents
  share the project concurrency limit.
- Frames and slices multi-ticket initiatives with **Macros** (`refine-macro` and
  `realign-macro`).
- Scaffolds **Spec Kit** and **OpenSpec** directly inside project repositories.
- Provides **Board** (Workflow & Statuses modes), **Backlog**, **Triage**,
  **Roadmap (Macros)**, **Timeline (Sprints)**, **Activities**, and custom
  cross-project filtered views.
- Runs on **SQLite** by default (zero external dependencies) with **PostgreSQL**
  support and a built-in migration tool (`sectile-migrate`) for server
  deployments.

## Requirements

- Go **1.26.6** or newer (when building from source)
- Node.js **22.18.0** or newer with npm (when building the web or desktop UI)
- Git

Optional integrations require the relevant local tools and credentials: GitHub
CLI (`gh`), GitLab (`glab`) or Jira API tokens, coding CLIs (`claude`, `codex`,
`agy`), `uv` / `specify` or `openspec`, Docker, or PostgreSQL.

## Quick start (from source)

Build the embedded web UI, server, and workstation agent (`make server` installs
`web/` npm dependencies automatically):

```sh
make server agent
```

Start the server with SQLite:

```sh
DB_PATH=./tasks.db PORT=8090 ./bin/server
```

Open [http://localhost:8090](http://localhost:8090) (or
[http://localhost:8090/help/guide.html](http://localhost:8090/help/guide.html)
for the embedded User & Workflow Guide). The server does not open a browser,
start a terminal, or run a coding agent on its own.

To develop the server and web UI with hot reload, use two terminals:

```sh
make serve
```

```sh
npm run dev --prefix web
```

Vite runs on port `5173` and proxies `/api` and `/ws` requests to port `8090`.

## Desktop application

### Install a release

On **macOS 13 (Ventura) or later**, install and update **Sectile Desktop** (which
bundles `sectile-agent`) via Homebrew:

```sh
brew install --cask sebastienferry/sectile/sectile
brew upgrade --cask sectile
```

On **Windows (64-bit)**, install and update via Scoop (no administrator rights
required):

```powershell
scoop bucket add sectile https://github.com/sebastienferry/scoop-sectile
scoop install sectile/sectile
scoop update sectile
```

Prebuilt archives for macOS (`arm64`, `amd64`), Linux (`amd64`), and Windows
(`amd64`) are also published on every GitHub Release with `SHA256SUMS`.

### Run from source

Build the workstation agent and launch the Electron companion from source:

```sh
make run
```

Sectile Desktop includes:

- **Execution consoles & queues**: interactive PTY terminals, autonomous run
  logs, and contextual next-stage banners (**Clarify**, **Specify**,
  **Implement**, **Create PR**, **Adjust**, **Handoff**).
- **Conversation view**: an integrated chat UI for **Antigravity**, **Claude
  Code** and **Codex** (**Settings → General → AI consoles → Conversation**)
  with inline diff cards, tool approval prompts, slash commands, and `$skill`
  completion.
- **Autonomy in sandbox**: configurable under **Claude settings** to run
  sandboxed shell commands automatically while blocking unsandboxed retries.
- **Desktop project board & task editor**: six-column workflow board (**Open
  board**), full Markdown task editor (`Cmd+N` / `Ctrl+N`), and a read-only
  worktree **Changes** inspector.

For installation details, platform notes, and desktop-specific configuration,
read the [Desktop guide](./desktop/README.md).

## Connect a workstation

On each machine that holds your git clones and coding CLIs, pair once. `pair`
opens your browser to sign in, and Sectile Desktop offers the same flow from its
**Sign in with your browser** button:

```sh
./bin/agent pair --url http://localhost:8090
./bin/agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone
```

On a headless machine without a browser, generate a one-time pairing code under
**Profile → Workstations & Agent → Pair a workstation** in the web UI and pass
`--code '<pairing code>'`.

Multiple workstations can stay connected at the same time. Each workstation
reports which project repositories it has checked out locally; when launching a
task or batch from the web UI, you can let Sectile route automatically (keeping
workstation affinity) or target a specific eligible workstation.

Keep tracker read credentials on the server and personal write credentials
sealed in your user profile—never in the agent configuration.

### Install Sectile in your coding CLI (Skills & MCP)

The workstation agent executes the workflow skills installed on the workstation
and exposes the Sectile MCP server (task context, stage transitions, worktree
preparation, pull request recording, and macro management). There are two ways
to install the skills and register the MCP server:

- **Claude Code plugin (`sectile`)**: build with
  `make plugin VERSION=0.5.0 OUT=/tmp/sectile-plugin ARGS=-marketplace`, then
  run `claude plugin marketplace add /tmp/sectile-plugin` and
  `claude plugin install sectile@sectile`. Claude prompts for the server URL and
  workstation API key and stores the key in its secure credential store. Skills
  are invoked as `/sectile:clarify-issue`, `/sectile:specify-issue`, etc. See
  [ADR 0039](docs/adrs/0039-sectile-is-installed-as-a-claude-plugin.md).
- **Direct CLI setup (Claude, Codex, Antigravity)**: run
  `./bin/agent init --provider claude` (or `codex`, `agy`), or click
  **Initialize** in Sectile Desktop's **Deployment** settings. This installs the
  workflow skills into the CLI's user configuration directory and registers the
  MCP server.

Under **Settings → Execution defaults** in Sectile Desktop, you can choose which
skill source a dispatch tries first (direct copy by default) and whether
project-customized skills in Sectile override the installed ones (enabled by
default).

## Common commands

| Command | Purpose |
| --- | --- |
| `make server` | Install web dependencies, build the embedded web UI, and compile `bin/server`. |
| `make agent` | Compile the workstation agent binary (`bin/agent`). |
| `make build-migrate` | Compile the SQLite-to-PostgreSQL migration tool (`bin/sectile-migrate`). |
| `make serve` | Run the server from source (`go run ./cmd/server`). |
| `make start` | Build and run the workstation agent (`./bin/agent`). |
| `make run` | Build the agent and launch Sectile Desktop from source. |
| `make desktop` | Build the desktop app without launching it. |
| `make desktop-package` | Package the desktop application for distribution. |
| `make test` | Run `gofmt` check, Go tests, web unit tests, TypeScript check, and `oxlint`. |
| `make release` | Cross-compile server and agent binaries into `dist/`. |
| `make image` | Build the server container image (`sectile-server:local`). |
| `make plugin VERSION=X.Y.Z OUT=DIR` | Generate the Sectile Claude plugin (`ARGS=-marketplace` wraps it in a marketplace). |
| `make reset-db` | Delete local SQLite database files (`tasks.db`, `-wal`, `-shm`). |
| `make help` | List all supported Make targets. |

## Configuration

Projects, trackers, and credentials can be configured directly from the web
interface (**Project Settings** and **Administration**). Administrators can
store sealed server-side tracker credentials for background synchronization,
while users connect their accounts, or seal their own personal tokens, in
**Profile → Tracker Credentials** so everything they do is published under
their own identity.

For headless or local development, copy the sample environment file and set only
the values you need:

```sh
cp .env.sample .env
```

Only the server reads `.env` at startup. Its most common settings are `PORT`,
`DB_PATH`, OIDC settings (`SECTILE_OIDC_*`), and fallback server tracker
credentials such as `SECTILE_GITHUB_TOKEN`, `SECTILE_GITLAB_TOKEN`, or
`SECTILE_JIRA_*`. See [`.env.sample`](./.env.sample) for the complete reference.

### MCP behind a hosting proxy

If MCP returns `403 Forbidden: invalid Host header` while the workstation agent
connects over a reverse proxy that forwards to loopback (`127.0.0.1` / `::1`)
with the public `Host` header preserved, explicitly allow the public hostname on
the server:

```sh
export SECTILE_MCP_ALLOWED_HOSTS='sectile.example.com'
```

Multiple hosts are comma-separated and matched case-insensitively (include
`:port` if sent by the proxy; no scheme, path, or wildcard). Bearer
authentication and browser-origin rejection always apply. See
[ADR 0037](docs/adrs/0037-explicit-mcp-ingress-hosts.md).

### PostgreSQL

SQLite is the default store. Server deployments can run on PostgreSQL instead:

```sh
export DB_DRIVER=postgres
export DATABASE_URL='postgres://sectile:password@db.internal:5432/sectile?sslmode=require'
export SECTILE_SECRET_KEY='<64 hex characters>'
./bin/server
```

The PostgreSQL database must use `UTF8` encoding (the server verifies this at
startup):

```sql
CREATE DATABASE sectile ENCODING 'UTF8' TEMPLATE template0;
```

To migrate an existing SQLite database to PostgreSQL, build and run
`./bin/sectile-migrate` (`make build-migrate`) while carrying over the same
`SECTILE_SECRET_KEY`.

## Testing

Run the full validation suite (`gofmt`, Go tests, web unit tests, TypeScript,
and `oxlint`):

```sh
make test
```

See the [Testing guide](docs/TESTING.md) for isolated SQLite test fixtures,
migration-test rules, and Playwright browser tests in [`web/tests`](./web/tests).

## Documentation

- [Embedded web help guide](./web/public/help/guide.html): interactive HTML
  user and developer workflow guide with screenshots and step-by-step How-Tos,
  served at `/help/guide.html` from the web status bar and command palette (`?`).
- [User guide](./docs/USER_GUIDE.md): markdown reference for sign-in, tracker
  credentials, projects, workstation setup, and coding workflows.
- [Desktop guide](./desktop/README.md): installation (`brew`, `scoop`, archives),
  conversation view, local configuration, and MCP setup.
- [Architecture](./docs/ARCHITECTURE.md): component boundaries, persistence,
  concurrency, worktrees, and console protocol.
- [Capabilities](./docs/CAPABILITIES.md): workflow stages, tracker support,
  agents, and task access policy.
- [API and data model](./docs/API_AND_DATA_SPEC.md): REST API, database schema,
  and [server-agent contracts](./docs/contracts/server-agent-v1.md).
- [UX components](./docs/UX_COMPONENTS.md): board, list, desktop, and diff
  experience.
- [Testing guide](./docs/TESTING.md): SQLite test fixtures, tests that need a
  real database, and how to measure test performance.
- [Architecture decisions](./docs/adrs): durable technical records (ADRs).
- [Changelog](./CHANGELOG.md): user-visible changes and release notes.

## Releases

Sectile releases are annotated Git tags in the form `vX.Y.Z`. The tag is the
single source of truth for the version reported by the server, agent, web UI,
and desktop app. See [`AGENTS.md`](./AGENTS.md) for the release procedure and
[`CHANGELOG.md`](./CHANGELOG.md) for release notes.

## License

No license is currently declared for this repository. Contact the repository
owner before redistributing or reusing the code.
