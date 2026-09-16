# Sectile (React + Go + SQLite)

Desktop now provides a read-only **Changes** view for each local execution, comparing current worktree contents with the default-branch common ancestor. See [Inspect worktree changes](desktop/README.md#inspect-worktree-changes).

The desktop supports persistent workstation project disconnection, with active
execution protection and explicit re-add. See [Remove a local project](desktop/README.md#remove-a-local-project).

Outil moderne et agentique de gestion des tâches pour développeurs et équipes techniques, construit avec **Go**, **React 19**, **Tailwind CSS v4**, et **SQLite**.

---

## ✨ Fonctionnalités implémentées

- **Server-side tracker integration**:
  - GitHub REST and Linear GraphQL support synchronization, issue creation, updates and comments without an online agent.
  - Configure explicit server credentials and repository/team identifiers. CLI login state is not used by the server.
  - Local tasks remain in SQLite. Jira metadata remains readable, but this baseline does not implement Jira synchronization or mutations.
  - Tracker queues expose actual API errors in Activities.

- 📐 **Frameworks Spec-Driven Design installables (Spec Kit & OpenSpec)** :
  - **Installation réelle de la chaîne d'outils depuis l'interface** (onglet *Compétences IA & SDD* d'un projet, ou palette <kbd>Cmd+K</kbd>) :
    - **GitHub Spec Kit** : installe la CLI `specify` via `uv` / `uvx` depuis `git+https://github.com/github/spec-kit.git`, puis exécute `specify init --here`. Scaffolde `.specify/` et `specs/` (spec.md, plan.md, tasks.md) ainsi que les commandes `/speckit.*` de l'agent.
    - **OpenSpec** : installe la CLI `openspec` via `npm` / `npx` depuis `@fission-ai/openspec`, puis exécute `openspec init`. Scaffolde `openspec/` (propositions de changement, deltas de specs `ADDED` / `MODIFIED` / `REMOVED`, checklists).
  - `GET /api/spec-framework/status` : indique, par framework, si la CLI est trouvée dans le PATH et si le répertoire de travail est déjà initialisé.
  - `POST /api/spec-framework/install` : lance l'installation et renvoie **chaque commande exécutée** avec sa sortie, de sorte qu'un échec soit diagnosticable et rejouable à la main.
  - Chaque installation est tracée comme une activité dans la vue *Activités*.
  - Le framework choisi pilote le contenu de la skill `/specify-issue` scaffoldée dans le projet et le prompt envoyé à l'agent IA.

- 🤖 **Agent Copilot & Moteur IA Configurable (`agy`, `vibe`, `claude`)** :
  - **Choix du moteur d'IA** :
    - `agy` : Antigravity CLI (`agy -p "{prompt}" --dangerously-skip-permissions`).
    - `vibe` : Mistral Vibe CLI (`vibe -p "{prompt}" --auto-approve`).
    - `claude` : Claude Code CLI (`claude -p "{prompt}"`).
    - `custom` : Template de commande shell entièrement personnalisable avec variables d'injection.
  - **Personnalisation des Prompts par Skill** :
    1. 🔍 **Clarify** (`/clarify-issue`) : Analyse les ambiguïtés et génère les questions de cadrage.
    2. 📝 **Specify** (`/specify-issue`) : Rédige la spec (Spec Kit ou OpenSpec, selon le framework du projet) et initialise la branche Git.
    3. 💻 **Implement** (`/code-issue`) : Plan de code, modification des fichiers et tests unitaires.
    4. **Adjust** (`/adjust-issue`): Review the full branch, address findings and available PR feedback, run final checks, and update the existing PR before human merge.
    5. ⚡ **Auto-Pilot** (`/pick-issue`) : Routeur intelligent qui enchaîne automatiquement l'étape optimale.
  - **Panneau de statut des CLI** : Vérification en temps réel de l'installation et de l'authentification de `git`, `gh`, `linear`, `acli`, `agy`, `vibe`, `claude`, `gemini`, `codex`, ainsi que des outils SDD `uv`, `specify` et `openspec`.

- 🗂 **Sidebar complète & Workflow Stages** :
  - `Backlog` ➔ `À clarifier` ➔ `Spécifié` ➔ `En cours` ➔ `À valider` ➔ `Terminé` avec compteurs en temps réel.
  - Bascule des vues (`Tableau Kanban` / `Vue Liste`).
  - Filtres rapides (`Mes tâches`, `Priorité Haute`, `Étiquettes/Tags`) et filtre par source (`Linear`, `GitHub`, `Jira`, `Local`).
  - Repli / Dépli fluide de la barre latérale.

- 👤 **Profil & Ergonomie Personnalisée** :
  - **Couleur d'accent dynamique** : *Indigo, Violet, Émeraude, Ambre, Rose, Cyan, Bleu, Orange*.
  - **Thème** : Mode Sombre (Dark) / Mode Clair (Light).
  - **Multi-langues** : Français (FR) / English (EN) avec bascule instantanée.
  - **Taille d'affichage & Densité** :
    - *Compact* (13px, espacements réduits, idéal écrans denses).
    - *Standard* (14px, vue équilibrée).
    - *Confortable* (15px, espacements aérés).

- 🔀 **Tableau Kanban & Vue Liste (Drag & Drop)** :
  - **Vue Tableau Kanban** : Glisser-déposer fluide entre colonnes avec mise à jour automatique Linear/GitHub.
  - **Vue Liste** : Regroupement par statut, tri multi-colonnes et édition inline.

- 🔍 **Recherche Rapide (`/`) & Palette d'actions (`Cmd+K`)** :
  - Raccourci clavier `/` pour cibler immédiatement la recherche.
  - Palette d'actions avec recherche floue et exécution directe des skills au clavier.
  - Dans la barre du terminal PTY, démarrer l'agent puis sélectionner un skill : Codex reçoit son nom sans `/`, les autres moteurs conservent le slash. Les noms personnalisés du projet sont respectés.

---

In Sectile Desktop, click the connected server address in the header (or focus it and press Enter) to open the board in your default browser. The shortcut is available while connected.

## Task access from workflow skills

Workflow skills use the local Sectile agent's exposed task-management interface first. When that interface is unavailable, `http://localhost:8090` is a temporary fallback and the integration failure must be recorded. Resolve the project and full task ID before mutations: a ticket key alone can match another repository. Managed runs retain ownership of result validation and stage transitions. See [the workflow access policy](docs/CAPABILITIES.md#task-access-from-agent-sessions).

## Quick start

Install Go and the web dependencies, then build the two runtimes:

```sh
npm ci --prefix web
make server agent
```

Start the server with its persistent database and shared agent credential:

```sh
export SECTILE_SERVER_TOKEN='<shared agent credential>'
export SECTILE_TRACKER_TOKEN='<tracker API token>'
# Serving GitHub and Linear at once? Override per provider:
# export SECTILE_GITHUB_TOKEN='<GitHub API token>'
# export SECTILE_LINEAR_API_KEY='<Linear API key>'
DB_PATH=/path/to/tasks.db PORT=8090 ./bin/sectile-server
```

Open **http://localhost:8090**. The server never opens a browser or starts local
Git, tracker CLI, terminal, editor or LLM processes. A server deployment needs
only its binary, writable database/configuration storage and network access to
its trackers. Put it behind your deployment's access-control boundary; the
existing browser REST API is still a single-user interface.

On the workstation:

```sh
export TOKEN='<same shared agent credential>'
./bin/sectile-agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone
```

Install and authenticate the coding CLI and Git tools on that workstation.
`make agent` requires only Go; it does not build the web UI. `make desktop`
packages the agent with the optional console companion. `make run` opens it.
For development, use `make dev-server` and `make dev-web` in separate terminals.

### Server tracker credentials

| Setting | Meaning |
| --- | --- |
| `SECTILE_TRACKER_TOKEN` | Tracker API credential, used by every provider that has no override below. |
| `SECTILE_GITHUB_TOKEN` | GitHub-only override; takes precedence over `SECTILE_TRACKER_TOKEN`. `GH_TOKEN` then `GITHUB_TOKEN` are environment-only fallbacks. |
| `SECTILE_GITHUB_API_URL` | REST base URL; defaults to `https://api.github.com`. GitHub Enterprise uses `https://<host>/api/v3`. |
| `SECTILE_LINEAR_API_KEY` | Linear-only override; takes precedence over `SECTILE_TRACKER_TOKEN`. `LINEAR_API_KEY` is an environment-only fallback. |
| `SECTILE_LINEAR_API_URL` | GraphQL endpoint; defaults to `https://api.linear.app/graphql`. |

The token is read from the environment of the **server process itself**, at
startup only. `make serve`, `go run ./cmd/server` and `./bin/sectile-server`
inherit the shell they are launched from, so exporting the variable in another
terminal — or after the server is already running — has no effect: restart the
server. A `gh` login on the same machine is not picked up either; for GitHub only
`SECTILE_GITHUB_TOKEN`, then `SECTILE_TRACKER_TOKEN`, then `GH_TOKEN`, then
`GITHUB_TOKEN` are consulted. The provider-specific variable comes first so a
server driving both GitHub and Linear cannot send one provider's credential to
the other.

For a GitHub project the token needs, at minimum, read and write access to the
issues of the configured repositories, plus repository metadata. A fine-grained
token therefore grants **Issues: read and write** and **Metadata: read** on those
repositories; a classic token uses the `repo` scope. For a quick local setup,
`SECTILE_TRACKER_TOKEN="$(gh auth token)"` reuses an existing `gh` login, which is
convenient but tied to that CLI session rather than being a durable credential.

Without a usable token the server keeps serving the board from its database, but
every tracker round-trip fails with `configure SECTILE_TRACKER_TOKEN on the
server`: task comments do not load and workflow stage transitions do not reach
the ticket. Verify the server picked the credential up by opening a task and
checking that its comments load — that read goes through the tracker API.

Credentials are read at server startup and are excluded from agent configuration.
Supply access to the configured repositories/teams and the operations you use
(issues, comments, milestones and PR reads). Configure `githubRepo` as
`owner/repository` and `linearTeam` as the team key. The server never discovers
these through a local clone or CLI credential store. Missing credentials and
API failures fail the operation visibly; there is no workstation fallback.

### Releases and migration

`make release` emits `sectile-server-<os>-<arch>` and
`sectile-agent-<os>-<arch>` under `dist/`, with `.exe` for Windows.
Supported targets are Darwin arm64/amd64, Linux arm64/amd64 and Windows amd64.

| Previous invocation | Replacement |
| --- | --- |
| `sectile` or `sectile` (server) | `sectile-server` |
| `sectile agent ...` | `sectile-agent ...` |
| `sectile mcp ...` or `sectile mcp ...` | `sectile-agent mcp ...` |
| Unified `stage` / `sync-skills` commands | Agent MCP `transition_stage` / project skill deployment through the agent |

Upgrade the server and agent together. No unified compatibility executable is
built. Update service units, native client MCP registrations and custom launchers.
The agent refreshes generated MCP registrations on dispatch. `SECTILE_OPEN_BROWSER`
and `SECTILE_NO_BROWSER` are obsolete. The old server `/ws/terminal` and terminal
session routes return 410; use the agent-owned desktop console.

The product remains **Sectile**. Database lookup, `.taskflow/` configuration,
`.tasks/` worktrees, `SECTILE_*`/legacy `TASKACAO_*` execution context and the
`sectile-api` health identifier remain compatible. No database files are moved
or deleted. Local CLI credentials remain available to agent-side coding and PR
commands; configure the server credentials separately.

## 📚 Documentation Technique Complète

Une suite documentaire complète pour développeurs et LLMs est disponible dans le dossier [`/docs`](./docs) :

- 🏛️ [**Architecture & Conception Générale** (`docs/ARCHITECTURE.md`)](./docs/ARCHITECTURE.md) : Modèle de concurrence, persistance SQLite, isolation Git Worktrees, PTY ZSH & WebSockets.
- ⚡ [**Capacités & Workflows Agentiques** (`docs/CAPABILITIES.md`)](./docs/CAPABILITIES.md) : Multi-projets, pipeline de 5 skills, Auto-Pilot, synchronisation Linear / GitHub.
- 🎨 [**Composants UX & Design Frontend** (`docs/UX_COMPONENTS.md`)](./docs/UX_COMPONENTS.md) : Kanban drag-and-drop, vue liste, terminal interactif Xterm.js, inspecteur de Diff Git.
- 🔌 [**Spécification API & Schéma de Données** (`docs/API_AND_DATA_SPEC.md`)](./docs/API_AND_DATA_SPEC.md) : Schéma SQLite complet, endpoints REST et agent-owned console protocol.
- 🤖 [**Guide de Ré-implémentation pour LLMs** (`docs/REIMPLEMENTATION_GUIDE.md`)](./docs/REIMPLEMENTATION_GUIDE.md) : Blueprint étape par étape pour reconstruire Sectile de zéro.

---

## ⌨️ Raccourcis Clavier

| Raccourci | Action |
|---|---|
| `/` | Cibler la barre de recherche globale |
| `Cmd+K` ou `Ctrl+K` | Ouvrir la palette de commandes & skills |
| `N` ou `C` | Ouvrir la modale d'ajout rapide de tâche |
| `Esc` | Fermer la modale / vider la recherche |
| `↑` / `↓` + `Entrée` | Naviguer et valider dans la palette d'actions |

## Remote execution and MCP

Sectile exposes eight typed tools at the Streamable HTTP endpoint `/mcp`:

- `list_projects`: discover project primary keys, names and Git remotes.
- `get_task`: read task details and comments.
- `transition_stage`: record a verified workflow stage and queue tracker synchronization.
- `add_comment`: post a task comment.
- `list_tasks`: list tasks with optional filters.
- `get_project_context`: read project execution settings and effective instructions.
- `start_run`: start or reuse the invocation's remote run.
- `finish_run`: finish that run without advancing the task stage.

HTTP and stdio both identify the server as `sectile`. Tool arguments, results,
authentication and workflow validation retain their existing contracts.

### Signing in and pairing a workstation

A deployment shared by several people signs them in through an OpenID Connect
provider. Configure it on the server:

```sh
export SECTILE_OIDC_ISSUER='https://example.okta.com'
export SECTILE_OIDC_CLIENT_ID='<client id>'
export SECTILE_OIDC_CLIENT_SECRET='<client secret>'
export SECTILE_OIDC_REDIRECT_URL='https://sectile.example.com/auth/callback'
```

The provider's endpoints are discovered from its metadata document at startup.
A provider that cannot be reached stops the server rather than serving the
interface unauthenticated. Without these variables the interface keeps a single
implicit user, which is how a personal deployment runs.

Each person then pairs their workstation from the profile dialog: generate a
pairing code, and enter it once in the desktop app. The code is single use and
expires in ten minutes; it is exchanged for a credential stored on that
machine, revocable per workstation without disturbing the others.

`SECTILE_DEV_IDENTITY=1` allows naming a user through an `X-Sectile-User`
header, to exercise several accounts before a provider exists. It is an
impersonation switch: it is ignored once a provider is configured, and it must
stay off elsewhere.

Run the central server with `SECTILE_SERVER_TOKEN` set to a shared agent
credential, then start the workstation agent in an existing clone:

```sh
export TOKEN='<same credential as SECTILE_SERVER_TOKEN>'
sectile-agent --url https://sectile.example.com --project '<project-id>' --repo /path/to/clone
```

The agent fetches `GET /api/v1/agent/config`, creates or validates local Git
worktrees, installs effective project skills, and launches the configured AI CLI.
Local command templates support task and repository placeholders, including
`{prompt}`, `{issueTitle}` and `{repoPath}`; see the [desktop placeholder guide](desktop/README.md).
It does not open a database. Server filesystem paths and tracker credentials are
excluded from the configuration contract. The old agent `--db` option is removed.
A disconnected or incompatible configuration API prevents execution.

Before launching an LLM CLI, the local agent automatically registers its own
`sectile-agent mcp --url <active-gateway>` bridge in that CLI's **user-level**
configuration, and installs the managed skills there too. It refreshes both on each
dispatch, including dynamic gateway ports. Existing settings and other MCP servers
are preserved; bearer tokens are not written. Native workspace/MCP trust prompts
still apply. Malformed configuration causes a visible launch error rather than
being overwritten.

Repositories and worktrees receive no Sectile-managed skill, command, MCP or
project-context file. Copies written by earlier releases are retired from the
checkout on the next dispatch when they are unchanged, and preserved when edited.

| Target CLI | User MCP registration | Managed skills |
| --- | --- | --- |
| Claude | `~/.claude.json` | `~/.claude/skills` |
| Codex | `~/.codex/config.toml` | `~/.agents/skills` |
| Antigravity (`agy`) | `~/.gemini/config/mcp_config.json` | `~/.gemini/config/skills` |
| Gemini | `~/.gemini/settings.json` | none |
| Cursor | `~/.cursor/mcp.json` | none |
| Vibe | `~/.vibe/config.toml` | none |

The agent that runs the tasks is always set up. A project can additionally set up
Claude, Codex and Antigravity through the **Agents à configurer** checkboxes in the
project's AI tab; unchecking an agent retires its installation on the next dispatch.

Custom command templates can use these providers. A `custom` provider is inferred
from the template's executable name; unknown executables produce an explicit
bootstrap error. The `sectile` MCP name is reserved for the agent-managed entry.
JSON/TOML files are serialized when updated; unrelated setting values are retained.

The External terminal button works without a skill selection and launches the
configured interactive agent. An explicit command is executed as supplied; a
selected skill is passed as the initial prompt. The server waits for the local
agent's launch result, so configuration and terminal-launch errors reach the UI.
Explicit external requests do not silently fall back to a hidden PTY.

For clients started outside Sectile, manual registration is still available.
A typical JSON client configuration is:

```json
{
  "mcpServers": {
    "sectile": {
      "command": "/absolute/path/to/sectile",
      "args": ["mcp"],
      "env": {"SECTILE_AGENT_URL": "http://127.0.0.1:8091"}
    }
  }
}
```

The gateway attaches the daemon's authentication token. If port 8091 is occupied,
use the gateway URL printed by the agent. Terminals launched by the agent inherit
the actual `SECTILE_AGENT_URL`, including a dynamically allocated port.
For direct server access, use `sectile-agent mcp --url https://sectile.example.com`
and set `SECTILE_AGENT_TOKEN` in that client's environment. Against a local
agent gateway, that variable holds the agent session secret, not a server
credential: the gateway attaches the workstation's own credential upstream.
Protocol output uses
stdout; diagnostics use stderr. The stdio bridge never falls back to another
database or server after an error.

Optional workstation overrides belong in `~/.config/taskflow/settings.json`:

```json
{
  "projects": {"project-id": "/path/to/clone"},
  "terminal": "ghostty",
  "aiProvider": "claude",
  "aiCommandTemplate": "claude {prompt}",
  "skills": {"implement": "Project-specific local skill instructions"}
}
```

Overrides remain local. The explicit `--terminal` flag takes precedence, followed
by an explicit terminal choice on the launch request, local overrides, remote
project/global settings, and environment/auto-detection. The legacy project
`.taskflow/config.json` is not used as a terminal override. Wildcard agents (`--project all`) require a local
project mapping or a matching Git origin. A registered concrete project can use
`--repo` directly. Existing worktrees must match the assigned branch; Sectile
never resets them to accommodate a dispatch.

Skill refresh installs the current server-owned content and records hashes in
`.taskflow/agent-manifest.json`. Changed local copies are backed up under
`.taskflow/skill-backups/` before replacement. Personal skills outside the declared
paths are untouched. Put persistent skill overrides in `~/.config/taskflow/settings.json`.
The effective `.taskflow/remote-config.json` snapshot is diagnostic only: it is
never used as an offline fallback. These generated files are ignored by Git.

The new machine endpoints and agent handshake validate `SECTILE_SERVER_TOKEN`
when configured. Without it, legacy single-user mode accepts any nonempty token.
This does not add multi-user login or authentication to the existing web/REST UI;
remote deployments still need their existing access-control boundary.

### Start the server and native local agent

The web manages tasks, the agent hosts native coding CLI consoles, and the optional desktop displays them. The
following commands start the standalone local agent. For the console
application, see the desktop setup section below. Start the server in one terminal:

```sh
npm ci --prefix web
make server agent
export SECTILE_SERVER_TOKEN='<your shared token>'
./bin/sectile-server
```

Start the local launcher in another terminal, using the project ID shown in
Sectile and an existing local clone:

```sh
export TOKEN='<the same shared token>'
./bin/sectile-agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone --terminal terminal
```

`terminal` selects Terminal.app on macOS. Other supported choices include
`ghostty` and `iterm`; omit the option for automatic detection. The native coding
CLI must be installed and authenticated separately. Keep the agent running while
using its MCP bridge. Quit the superseded Electron app before starting this agent
so that it does not register a competing launcher.

For an explicitly configured project, connecting the agent installs the project
skills and MCP bridge in the selected repository. You can then open Codex or
Claude in that repository and ask it to use `pickup-issue` for a ticket. The skill
reads task context through MCP, prepares/reuses a worktree, executes the workflow,
and reports stages through MCP. Native client trust and tool approvals apply.

Alternatively, invoke the pickup skill from the web.
The agent prepares the task worktree, installs MCP there, and launches the native
CLI. Execution and approvals stay in that CLI. The server owns task data and
tracker synchronization; the launcher has no local task database.

### Browser startup and workflow completion

Open `http://localhost:8090` manually. The server does not launch a browser.
Agent launch acknowledgements and remote run completion are separate from workflow
transitions. Native skills submit verified stages through MCP. PR stages combine
server-side forge evidence with agent checkout evidence; the server never opens
that checkout. The old server-managed temporary result-file worker is retired.

### Server/agent contract

See [the version 1 contract](docs/contracts/server-agent-v1.md) for the configuration
fields, launch messages, precedence, skill ownership and error behavior. Every
launch downloads fresh configuration; there is no offline execution fallback.

### One local agent for multiple projects

With `TOKEN` set, discover projects and start the agent:

```sh
sectile-agent --url http://localhost:8090 --list-projects
sectile-agent --url http://localhost:8090
```

The agent defaults to all projects. The current checkout is matched by its Git
origin; map other project primary keys to local repositories in
`~/.config/taskflow/settings.json` in the starting directory (or the directory passed with
`--repo`):

```json
{"projects":{"project-primary-key-a":"/path/to/repo-a","project-primary-key-b":"/path/to/repo-b"}}
```

Use `--project <primary-key>` to restrict the agent to one project. Terminal and
skill settings are downloaded from the server before each launch; no
`--terminal` argument is necessary. See the
[server/agent contract](docs/contracts/server-agent-v1.md) for identity and mapping rules.

The profile dialog includes a **Local agent** section with an editable server URL
and a copyable launch command. Set `TOKEN` in your terminal before
running it; the UI does not store or display the server credential.

Task cards and the task clarification panel provide **Copy skill command**.
Choose Codex or Claude and a workflow skill, then copy the interactive terminal
command. Commands use the task primary key and project identity with MCP
instructions. Run them in a local repository where the project skills and
Sectile MCP are already configured.

Remote work is shown on task cards and list rows with a single run icon: spinning
while running, a clock while queued, and a crossed circle for a few seconds after a
cancellation. Hovering or focusing an icon for a run owned by your own agent turns it
into a stop control that cancels the run in place.
The MCP tools `start_run` and `finish_run` track the invocation
independently of stage transitions. Updated standalone skills and copied commands
report this lifecycle; existing installed skills need to be refreshed. An abruptly
closed client may leave an activity to cancel manually in the activity view.

Agent-owned remote executions have a **Stop** button on the task. Sectile waits
for the local supervisor to confirm process termination before marking the run
canceled. Worktree changes are preserved. This requires restarting the local
agent with the updated binary; previously launched or independent Codex/Claude
processes cannot be controlled by the new supervisor.

Project settings include **Create PR/MR**: choose the default **Draft after implementation**, or **Draft after specification** to review specs in an early draft.
Skills reuse the same PR/MR during implementation and attach its URL through MCP. Adjust never creates a PR. Missing PRs recover through the configured earlier stage without downgrading completed work. `Create PR` (`create_pr`, `/create-pr`) remains available under Additional skills. It creates or reuses a PR without advancing the task stage or joining the automatic workflow. The legacy `review` invocation resolves to Adjust; inherited review customizations require reconciliation in Skills.

To regenerate skills from the desktop app, open the project gear menu, select **Deployment**, and click **Deploy server skills**. Configure and save the local repository first, and stop active executions before deployment. The agent fetches the current server skill content; **Refresh from server** alone refreshes settings without deploying files. Skills are also refreshed when preparing task executions.

### Desktop console host

Use **Open agent console** on a configured project to start Codex or Claude in a
local TTY without a task or initial prompt. See [Free agent console](desktop/README.md#free-agent-console).

The desktop **Agent logs** toolbar action shows recent local-agent diagnostics even
when disconnected, with a bounded snapshot and Refresh. Logs fill the workspace
beside the project sidebar and omit terminal control sequences for readability.
See [desktop usage](desktop/README.md#use).

Use Sectile Desktop to follow native Codex/Claude terminals locally:

```sh
make desktop-build
cd desktop
npm start
```

Configure the server connection in the desktop window, then launch tasks from
the web. The desktop hosts consoles, stop controls, log export and local project
mappings. A status line beneath the task console offers the next workflow skill,
using current task state and blocking duplicate active executions. Closing the
window keeps the agent running. The integrated terminal,
branch switcher, diff viewer and worktree controls have been removed from the web.

See [desktop setup](desktop/README.md) and [ADR 0003](docs/adrs/0003-local-desktop-consoles.md).

The desktop **Local agent** panel provides configuration and explicit start,
stop and restart controls. Stopping or restarting requires confirmation and
confirmed termination of active executions. Closing the desktop leaves the
agent running.

### Optional desktop companion

The server, local agent and desktop app are independent components. Start the
agent without the app:

```sh
export TOKEN='your-server-token'
sectile-agent --url http://localhost:8090 --repo /path/to/repository
```

The agent owns PTYs, supervision and console history. The desktop discovers it
through `~/.taskflow/agent-connection.json` (private, mode 0600), including when
opened after executions begin. Closing the app leaves executions running.
The desktop can also start the same agent when none is running.
`--desktop` is a deprecated no-op; `--terminal` is accepted for compatibility
but executions always use agent-owned consoles. `--desktop-info` can override
the discovery file for isolated instances; the app automatically discovers the
default file and its legacy private connection file.

### Build all components

Run `make all` to build the embedded web server, standalone local agent and
packaged desktop app. The outputs are `bin/sectile-server` and
`bin/sectile-agent`; the agent starts directly. Use `make server` or
`make agent` to build independently, and `make desktop-build`
for the desktop development assets. On Apple Silicon the app is produced at
`desktop/release/Sectile-darwin-arm64/Sectile.app`.

The optional companion groups local executions under projects in a collapsible
sidebar. Add projects by discovering the server catalog and mapping a local Git
directory. Local worktree preferences are stored per project in
`~/.config/taskflow/settings.json`. Repository layout, remote URL, SDD selection and skill
content remain server-owned and read-only. Explicit deployment buttons install
the server skills or initialize its SDD framework in the mapped directory.
The profile is a placeholder for future account management.

### Execution defaults and local overrides

The server project supplies `useWorktrees` and `parallelism` (1 to 5) defaults.
In the desktop project settings, **Inherit worktrees from server** and
**Inherit from server** for parallel executions remove local overrides.
Workstation overrides are saved in `~/.config/taskflow/settings.json` as project-ID maps:

```json
{
  "projects": {"project-id": "/path/to/repository"},
  "worktrees": {"project-id": true},
  "parallelism": {"project-id": 2}
}
```

Without effective worktrees, the agent enforces one execution and the UI
disables parallelism selection. Requests are acknowledged when queued; their
remote run remains active until completion or cancellation. The agent reserves
capacity before repository preparation, admits queued requests in order within
each project, and holds capacity until confirmed process exit. Queued executions
can be canceled from either UI. Tasks sharing an unisolated repository, or the
same task worktree, cannot execute concurrently. Settings are resolved at
admission into the queue; changes apply to subsequent submissions. Queue and
console history are held in memory for the agent lifetime.

### User configuration and commands

Agent settings and project mappings live in
`~/.config/taskflow/settings.json`, shared by the CLI agent and companion.
Writes preserve connection fields, use atomic replacement and mode 0600.
Legacy repository mappings remain readable and are migrated on the next save.

| Command | Action |
| --- | --- |
| `make server` | Build the server |
| `make agent` | Build the local agent |
| `make desktop` | Build and package the desktop |
| `make all` | Build all components |
| `make start` | Start the local agent |
| `make serve` | Start the server |
| `make run` | Start the desktop |

Server and agent are built as `bin/sectile-server` and `bin/sectile-agent` by the `build-*` targets.
The `serve`, `start` and `run` targets run from source and need no prior build. Pass agent
arguments with, for example, `make start ARGS="--url http://localhost:8090"`; provide
authentication through `TOKEN`.

### Browse desktop project tasks

Hover or keyboard-focus a desktop project row and activate its **Open tasks**
list icon to browse that project's unfinished server tickets immediately.
Search by title or key, or submit an empty search to restore the full open list.
Choose a skill and **Launch** to pick up a ticket; pickup is the default when
available. Loading the list never starts an execution. Failed requests can be
retried with Search, and launching requires a configured local repository.

### Desktop Quick add

Click **New task (+)** beside a desktop project to choose **Run an existing
ticket** or **Quick add task**. Both paths target the clicked project, even
when another project's execution is selected. Existing tickets open the search
and skill launcher; Quick add preselects the project and offers **Launch task**
after successful creation.

Press **Cmd+K** (macOS) or **Ctrl+K** to open the command palette and choose
**Quick add task**. The selected project's identity is prefilled; without a
selection, choose a project explicitly. Enter a title and optional description.
The server creates the task using its project tracker configuration.
GitHub and Linear creation must succeed remotely; errors do not silently create
a local fallback. Local projects remain local. Jira remote creation is not
implemented and returns an explicit error. Creation does not start an execution;
the success screen offers a separate **Launch task** action.

Task IDs in the desktop sidebar open the task directly on the configured Sectile server. Server links use `?task=<task-primary-key>` and open task details independently of board filters.

For `agy`, the local agent registers the Sectile stdio bridge in
`~/.gemini/config/mcp_config.json`; this CLI does not read the workspace
`.agents/mcp_config.json`. Other MCP registrations and explicit tool policies are
preserved. The shared entry contains no token or gateway URL: agent-launched
sessions inherit `SECTILE_AGENT_URL` and `SECTILE_AGENT_TOKEN`, the gateway
address and its session secret. The credential that identifies the user stays
inside the agent process and is never exported. Standalone agy
sessions must supply those variables themselves. Restart agy after registration
so it loads the updated MCP tools.

## MCP naming upgrade

MCP tool names now omit the `sectile_` prefix and the managed server registration
is `sectile`. This intentionally breaks old MCP calls: no aliases or fallback
calls are supported. The bridge and desktop client identities are `sectile-stdio`
and `sectile-desktop-agent`.

1. Upgrade the central server and workstation agent together. Mixed versions are
   unsupported; the stdio bridge rejects incompatible upstream catalogs. Stop
   existing native sessions before switching.
2. On the next normal agent dispatch, bootstrap migrates the reserved `sectile`
   registration to one `sectile` entry. It refreshes the connection settings and
   preserves unrelated entries and explicit restrictions for all six providers.
3. Resolve any migration error before retrying. Recognized server-scoped tool
   lists map exact old names to generic names. Conflicting registrations,
   unsupported patterns and legacy references in external policy files require
   manual reconciliation; bootstrap leaves the original file unchanged. Preserve
   deny rules and approval requirements when changing tool or server names.
4. Update user-owned stored skill overrides and custom instructions manually.
   Built-in instructions use generic tools; normal managed-file refresh retains
   its backup behavior for local edits. Checked-in historical reports stay intact.
5. Reconnect native clients to discard cached tool catalogs and accept their
   normal workspace/MCP trust prompts. Verify the eight generic tools under
   `sectile` before starting new work.

For Vibe, review root `enabled_tools`, `disabled_tools` and `[tools.<name>]`
policies manually: its client prefixes tools with the server name. See the
[Vibe MCP permission reference](https://docs.mistral.ai/vibe/code/cli/mcp-servers).
Antigravity's `disabledTools` list also maps exact old tool names; see its
[MCP configuration reference](https://www.antigravity.google/docs/mcp).
Gemini's server-scoped `includeTools` and `excludeTools` retain their filtering
roles during exact-name migration; see the
[Gemini MCP reference](https://geminicli.com/docs/tools/mcp-server/).
Bootstrap does not rewrite separate user or enterprise policy files. Operators
must also update restrictions supplied by enterprise policy, plugins or custom
configuration paths before reconnecting clients.

`SECTILE_*` variables (including `SECTILE_RUN_ID`), `.taskflow/`, database paths,
`/mcp`, machine markers and repository/module names remain unchanged. There is no
data migration. Rollback requires coordinating both binaries and restoring the
matching client registration and custom instructions.
