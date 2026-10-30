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
  GitHub or GitLab OAuth connection); Fleet server credentials serve background
  sync, while a Local companion uses its installation account.
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
Each release includes Go dependency attributions and license texts in a
`sectile-third-party-notices-<version>.tar.gz` archive. The same notices are
included beside the bundled agent in each Sectile Desktop archive. See
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).

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

### Autonomous Codex workflows

Built-in Codex task chains require a CLI supporting `codex exec --approve-for-me`
and `--json` (qualified with Codex CLI 0.165.0). The agent keeps workspace-write
sandboxing, routes tool approvals through Codex's automatic reviewer, and streams
command, file and MCP progress to the desktop's read-only execution trace. An
approval denial remains a denial; the launcher never supplies an unrestricted
sandbox or ignores managed rules. Upgrade an older CLI if it rejects these flags.
Autonomous provider processes use the same owner-lifetime guard as conversations,
so an agent crash stops their delegated tools as well as explicit interruption.
Their initial identity is checkpointed before startup; a configured store that
cannot save it refuses the launch. Autonomous trace tails still use periodic
snapshots, while free conversations retain their per-event journal.
Interactive conversations and user-authored command templates keep their own
permission and output settings. No global Codex configuration is rewritten.
See [ADR 0069](docs/adrs/0069-codex-autonomous-runs-keep-sandbox-and-stream-progress.md).

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
| `make build-server-headless` | Compile `bin/server-headless` with API, MCP and authentication routes but no browser UI assets; does not require Node. |
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
GitLab tracker and personal-credential URLs may be the instance website
address or its explicit `/api/v4` endpoint; the API client resolves either
form to the REST API root.

The standalone Local companion has one installation identity. Its manual and
scheduled tracker synchronization uses that identity's OAuth grant or personal
API token. Fleet synchronization continues to use a provider-level server
credential; see [ADR 0068](docs/adrs/0068-local-tracker-sync-uses-installation-identity.md).

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

This repository is licensed under the MIT License. See [`LICENSE`](./LICENSE)
for the full text.

### Isolated desktop companions

A companion hosting its own server can set `SECTILE_SERVER_HOST=127.0.0.1`
to bind only IPv4 loopback and `SECTILE_SERVER_DATA_DIR` to an absolute,
private directory for its database, keys and server environment file. `DB_PATH`
still overrides the database location; clear inherited deployment settings
before starting a separate local instance. The default server still listens
on all interfaces when `SECTILE_SERVER_HOST` is absent.

For a separate agent, use `SECTILE_DESKTOP_DATA_DIR` and `--desktop-info`
with companion-owned locations. Set `SECTILE_DISABLE_MCP_REFRESH=1` to prevent
startup and reconnect from rewriting existing provider MCP registrations.
This switch does not prohibit an explicit user-requested MCP repair.
For native Claude/Codex launches, `SECTILE_COMPANION_MCP_URL` supplies an isolated
MCP connection to that loopback server. The process-only `sectile_companion`
bridge inherits the paired key privately; it never rewrites user registrations.
In this mode setup reports session scope, and direct CLI templates use the
`{mcpArgs}` slot for their isolated connection. Managed native Fleet agents use
`SECTILE_DESKTOP_MCP_URL` for their selected backend instead: HTTPS origins and
HTTP loopback are accepted, without credentials, path, query or fragment. A
scoped endpoint also suppresses automatic shared-registration refresh; the paired
key and device identity travel through the child environment. Both scoped
variables absent preserves Electron and workstation registration behavior.

A local companion can also set a random `SECTILE_LOCAL_RUNTIME_KEY` to enable
authenticated `GET /api/local-runtime` discovery (protocol 1). It requires
`SECTILE_SERVER_HOST=127.0.0.1` and an explicit server data directory. Send the
key as a bearer credential to this route only; it does not authenticate user API
requests. Browser origins and unknown keys receive 404.

Desktop OAuth integration is described in [the companion API contract](docs/desktop-oauth.md).

Tracker workflows launched by a desktop retain its connection identity when the
server dispatch arrives. Provider questions and MCP approvals stay interactive
for that connection; another selected connection cannot control the run.
This requires the server's launch response to expose `runId`; the launch
activity's `id` identifies a separate record and cannot substitute for it.
The launcher's run ID can be adopted only while queued or running. A new skill
invocation after completion calls `start_run` without `runId` and uses its
returned ID for reporting, even when `SECTILE_RUN_ID` still names the first run.

Packaged migration recovery can be checked on disposable data with
`SECTILE_TEST_PACKAGED_COMPANION`; see [Testing](docs/TESTING.md#packaged-local-migration-recovery).

Free Claude Code and Codex conversations retain their provider session, transcript,
model and mode across agent restarts. They resume on the next explicit message;
previous prompts and tools are not replayed. Older histories without saved session
identity and explicitly stopped conversations remain read-only. See
[ADR 0062](docs/adrs/0062-resumable-local-provider-conversations.md).

Claude Code and Codex conversation processes follow the Go agent lifetime. An
independent Unix guard or Windows kill-on-close job terminates their local tools
if the agent crashes, including SIGKILL on Unix. See
[ADR 0063](docs/adrs/0063-provider-tools-follow-the-agent-lifetime.md).

Free Claude Code and Codex conversations synchronously journal normalized events
and streamed reply fragments before displaying them. Recovery keeps unfinished
text as interrupted history, without replaying tools. Periodic writes compact the
journal rather than defining a five-second conversation-loss window. Storage
errors are visible and prevent unsaved new prompts from being dispatched. See
[ADR 0064](docs/adrs/0064-conversation-events-use-a-synchronous-journal.md).

### Desktop project conversation preferences

The local desktop companion exposes authenticated conversation-profile and title
endpoints. Profiles reuse the workstation engine catalogue (settings layout 7);
project defaults apply to new conversations, and edits leave open provider sessions
unchanged. Free conversation titles persist with local run histories. See
[ADR 0065](docs/adrs/0065-desktop-project-profiles-and-conversation-titles.md).

The isolated desktop companion uses the Go-owned loopback origin `http://127.0.0.1:52963`. Register `http://127.0.0.1:52963/auth/jira/callback` in the Atlassian OAuth application. `sectile-companion --local-runtime-info` exposes the public transport contract without opening the database; ordinary web and Electron deployments keep their configured ports and callbacks. See [desktop OAuth](docs/desktop-oauth.md).

## Local/Fleet runtime evolution

[Local/Fleet runtime contracts](docs/desktop-runtime-contracts.md) and [ADR 0066](docs/adrs/0066-compose-a-local-companion-without-changing-fleet.md) record the companion and Fleet compatibility boundaries. Phase 1 extracts backend assembly into `internal/backend`, first used by the standalone server. Phase 2 adds store ownership, stoppable backend/agent lifecycles and a Local work gate. The combined companion and journaled migration are now integrated into the Tauri Local launcher. The product-level V1 roadmap is maintained in the Tauri repository at `docs/sectile-v1-migration-plan.md`.

Build the combined Local executable with `go build -o sectile-companion ./cmd/companion`. Tauri invokes it with `--data-dir /absolute/private/root`; on first launch it also invokes `--upgrade` with the packaged legacy server and agent to migrate an existing managed installation. Do not run `--upgrade` against a live or manually managed installation. Managed live-process migration is currently qualified on macOS; on other platforms stop both legacy services before retrying. The original data and verified backup remain in the private installation directory. See the [runtime contract](docs/desktop-runtime-contracts.md) for qualification limits and recovery behavior.


Before a manual desktop update, finish Local executions and preparations. The
native Services action invokes `sectile-companion --prepare-update --data-dir
/absolute/selected/root`, then quits the desktop only after confirmation. The
helper authenticates the Local companion and its discovered agent, honors busy
refusals, requests graceful shutdown, and verifies both loopback transports have
closed and the Local lifetime/store lock can be acquired. It does not open the
database, signal processes, change credentials or affect Fleet. The desktop
holds its installation/startup lock and inhibits automatic startup during this
operation; close other desktop instances before preparing an update. Structured
stdout contains readiness or a bounded failure code, never credentials. A
failed preparation must not be treated as permission to replace the bundle.

Idle durable free Claude Code and Codex conversations do not count as active
executions for update preparation. Their provider sessions, history, model and
mode are checkpointed through normal daemon cleanup for the next explicit
message. Active turns, queued input, pending authorizations, tracker-bound
sessions and unavailable history storage still prevent shutdown. Do not use
conversation Stop to prepare an update: Stop is an explicit terminal action.
