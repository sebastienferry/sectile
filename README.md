# Sectile (React + Go + SQLite)

Desktop now provides a read-only **Changes** view for each local execution, comparing current worktree contents with the default-branch common ancestor. See [Inspect worktree changes](desktop/README.md#inspect-worktree-changes).

The desktop supports persistent workstation project disconnection, with active
execution protection and explicit re-add. See [Remove a local project](desktop/README.md#remove-a-local-project).

Outil moderne et agentique de gestion des tâches pour développeurs et équipes techniques, construit avec **Go**, **React 19**, **Tailwind CSS v4**, et **SQLite**.

---

## ✨ Fonctionnalités implémentées

- 🔄 **Support Multi-Trackers Hybride (Linear, GitHub CLI, Jira & Local)** :
  - **Chargement & Synchronisation complète** :
    - `POST /api/sync/all` : Synchronise Linear, GitHub et Jira en un seul clic.
    - `POST /api/sync/linear` : Synchronise les tickets d'équipe Linear.
    - `POST /api/sync/github` : Synchronise les issues GitHub du repository configuré.
    - `POST /api/sync/jira` : Synchronise les tickets du projet Jira via la CLI Atlassian (`acli jira workitem list`).
  - **Création d'Issues avec routage CLI** :
    - Choix de la destination lors de l'ajout rapide (<kbd>N</kbd> ou `+`) : 🟣 **Linear**, 🐙 **GitHub**, 🔷 **Jira**, ou 📁 **Local SQLite**.
    - Exécution transparente de `linear issue create`, `gh issue create` ou `acli jira workitem create` en arrière-plan avec récupération automatique des identifiants et URLs.
  - **Mise à jour d'état bidirectionnelle** :
    - Déplacer une carte dans le Kanban ou la Liste met à jour automatiquement l'état sur Linear (`linear issue update --state`), sur GitHub (`gh issue close` / `reopen`) et sur Jira (`acli jira workitem transition --state`).
    - Les rapports d'exécution des skills sont postés en commentaire sur le ticket distant (`acli jira workitem comment` pour Jira).
  - **Configuration Jira par projet** :
    - Champ *Projet Jira* (clé passée à `acli --project`) et *URL Jira* pour construire les liens `/browse/<KEY>`.
    - Détection des statuts réels du workflow Jira via `acli jira workitem search`, avec repli sur *To Do / In Progress / In Review / Done*.
  - **Filtres par source & Badges d'origine** :
    - Filtrez en 1 clic dans la barre latérale : *Toutes les sources*, *Linear*, *GitHub*, *Jira*, *Local*, avec compteurs en temps réel.
    - Badges d'origine avec lien direct vers le ticket dans le navigateur.

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

## Task access from workflow skills

Workflow skills use the local TaskFlow agent's exposed task-management interface first. When that interface is unavailable, `http://localhost:8090` is a temporary fallback and the integration failure must be recorded. Resolve the project and full task ID before mutations: a ticket key alone can match another repository. Managed runs retain ownership of result validation and stage transitions. See [the workflow access policy](docs/CAPABILITIES.md#task-access-from-agent-sessions).

## 🚀 Quick Start / Démarrage rapide

### 1. Mode Production (Serveur unique Go servant le frontend React & l'API)
```bash
make build # compile le frontend et produit bin/sectile
make run   # lance ./bin/sectile
```
L'application est disponible sur **http://localhost:8080**.

---

### 2. Mode Développement (Hot-Reloading React + Go API)
Dans deux terminaux séparés :
```bash
# Terminal 1: Go Backend API sur le port 8080
make dev-server

# Terminal 2: React Vite Dev Server sur le port 5173 (avec proxy API automatique)
make dev-web
```
Puis ouvrez **http://localhost:5173**.

---

### 3. Release Multiplateforme
```bash
make release
```
Génère les exécutables autonomes dans `dist/` :
- `dist/sectile-darwin-arm64`
- `dist/sectile-darwin-amd64`
- `dist/sectile-linux-amd64`
- `dist/sectile-linux-arm64`
- `dist/sectile-windows-amd64.exe`

---

## 🔄 Upgrade & Backwards Compatibility

TaskFlow has been renamed to **Sectile** (executable `bin/sectile`). All existing databases, local configurations, and automation remain fully backwards-compatible without manual migration:

| Contract | Retained Compatibility Policy |
| --- | --- |
| **Database resolution** | Existing search order is preserved: explicit `DB_PATH` > `./tasks.db` > `$APP_DIR/taskflow/tasks.db` > legacy `$APP_DIR/taskacao/tasks.db`. No database files are moved or deleted. |
| **Configuration** | Projects continue using the `.taskflow/` configuration directory (`.taskflow/config.json`) and `.tasks/` worktree paths. Ownership markers (`<!-- taskflow:project-context:start -->`) remain intact. |
| **Environment variables** | `TASKFLOW_*` and legacy `TASKACAO_*` variables (`TASKFLOW_TASK_KEY`, `TASKFLOW_TASK_ID`, `TASKFLOW_API_URL`, etc.) continue to be injected and supported. |
| **Health & Protocol** | Machine-facing health check identifier remains `taskflow-api` (`GET /api/health`). Terminal execution protocol markers remain `__TASKFLOW_*`. |
| **Custom launchers** | Existing launchers pointing to `taskflow` can be updated to `sectile`, or aliased locally via `alias taskflow=sectile` or a symlink `ln -s bin/sectile bin/taskflow`. |
| **Repository coordinates** | Git remote and tracker URLs remain unchanged (`git@github.com:sebastienferry/taskflow.git`). |

---

## 📚 Documentation Technique Complète

Une suite documentaire complète pour développeurs et LLMs est disponible dans le dossier [`/docs`](./docs) :

- 🏛️ [**Architecture & Conception Générale** (`docs/ARCHITECTURE.md`)](./docs/ARCHITECTURE.md) : Modèle de concurrence, persistance SQLite, isolation Git Worktrees, PTY ZSH & WebSockets.
- ⚡ [**Capacités & Workflows Agentiques** (`docs/CAPABILITIES.md`)](./docs/CAPABILITIES.md) : Multi-projets, pipeline de 5 skills, Auto-Pilot, synchronisation Linear / GitHub.
- 🎨 [**Composants UX & Design Frontend** (`docs/UX_COMPONENTS.md`)](./docs/UX_COMPONENTS.md) : Kanban drag-and-drop, vue liste, terminal interactif Xterm.js, inspecteur de Diff Git.
- 🔌 [**Spécification API & Schéma de Données** (`docs/API_AND_DATA_SPEC.md`)](./docs/API_AND_DATA_SPEC.md) : Schéma SQLite complet, endpoints REST et protocole WebSocket `/ws/terminal`.
- 🤖 [**Guide de Ré-implémentation pour LLMs** (`docs/REIMPLEMENTATION_GUIDE.md`)](./docs/REIMPLEMENTATION_GUIDE.md) : Blueprint étape par étape pour reconstruire TaskFlow de zéro.

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

TaskFlow exposes eight typed tools at the Streamable HTTP endpoint `/mcp`:

- `taskflow_list_projects`: discover project primary keys, names and Git remotes.
`taskflow_get_task`, `taskflow_transition_stage`, `taskflow_add_comment`,
`taskflow_list_tasks`, and `taskflow_get_project_context`. Stage updates use the
existing validation and tracker queue; a successful response atomically records
the local transition and its report, then queues tracker synchronization.

Run the central server with `TASKFLOW_SERVER_TOKEN` set to a shared agent
credential, then start the workstation agent in an existing clone:

```sh
export TASKFLOW_AGENT_TOKEN='<same credential as TASKFLOW_SERVER_TOKEN>'
taskflow agent --url https://taskflow.example.com --project '<project-id>' --repo /path/to/clone
```

The agent fetches `GET /api/v1/agent/config`, creates or validates local Git
worktrees, installs effective project skills, and launches the configured AI CLI.
Local command templates support task and repository placeholders, including
`{prompt}`, `{issueTitle}` and `{repoPath}`; see the [desktop placeholder guide](desktop/README.md).
It does not open a database. Server filesystem paths and tracker credentials are
excluded from the configuration contract. The old agent `--db` option is removed.
A disconnected or incompatible configuration API prevents execution.

Before launching an LLM CLI, the local agent automatically registers its own
`taskflow mcp --url <active-gateway>` bridge in that CLI's project configuration.
It refreshes the entry on each dispatch, including dynamic gateway ports. Existing
settings and other MCP servers are preserved; global configuration and TaskFlow
bearer tokens are not written. Native workspace/MCP trust prompts still apply.
Malformed configuration causes a visible launch error rather than being overwritten.

| Target CLI | Project configuration |
| --- | --- |
| Codex | `.codex/config.toml` |
| Claude | `.mcp.json` |
| Antigravity (`agy`) | `.agents/mcp_config.json` |
| Gemini | `.gemini/settings.json` |
| Cursor | `.cursor/mcp.json` |
| Vibe | `.vibe/config.toml` |

Custom command templates can use these providers. A `custom` provider is inferred
from the template's executable name; unknown executables produce an explicit
bootstrap error. The `taskflow` MCP name is reserved for the agent-managed entry.
JSON/TOML files are serialized when updated; unrelated setting values are retained.

The External terminal button works without a skill selection and launches the
configured interactive agent. An explicit command is executed as supplied; a
selected skill is passed as the initial prompt. The server waits for the local
agent's launch result, so configuration and terminal-launch errors reach the UI.
Explicit external requests do not silently fall back to a hidden PTY.

For clients started outside TaskFlow, manual registration is still available.
A typical JSON client configuration is:

```json
{
  "mcpServers": {
    "taskflow": {
      "command": "/absolute/path/to/taskflow",
      "args": ["mcp"],
      "env": {"TASKFLOW_AGENT_URL": "http://127.0.0.1:8091"}
    }
  }
}
```

The gateway attaches the daemon's authentication token. If port 8091 is occupied,
use the gateway URL printed by the agent. Terminals launched by the agent inherit
the actual `TASKFLOW_AGENT_URL`, including a dynamically allocated port.
For direct server access, use `taskflow mcp --url https://taskflow.example.com`
and set `TASKFLOW_AGENT_TOKEN` in that client's environment. Protocol output uses
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
`--repo` directly. Existing worktrees must match the assigned branch; TaskFlow
never resets them to accommodate a dispatch.

Skill refresh installs the current server-owned content and records hashes in
`.taskflow/agent-manifest.json`. Changed local copies are backed up under
`.taskflow/skill-backups/` before replacement. Personal skills outside the declared
paths are untouched. Put persistent skill overrides in `~/.config/taskflow/settings.json`.
The effective `.taskflow/remote-config.json` snapshot is diagnostic only: it is
never used as an offline fallback. These generated files are ignored by Git.

The new machine endpoints and agent handshake validate `TASKFLOW_SERVER_TOKEN`
when configured. Without it, legacy single-user mode accepts any nonempty token.
This does not add multi-user login or authentication to the existing web/REST UI;
remote deployments still need their existing access-control boundary.

### Start the server and native local agent

The web manages tasks, the agent hosts native coding CLI consoles, and the optional desktop displays them. The
following commands start the standalone local agent. For the console
application, see the desktop setup section below. Start the server in one terminal:

```sh
npm ci --prefix web
make build
export TASKFLOW_SERVER_TOKEN='<your shared token>'
./bin/taskflow
```

Start the local launcher in another terminal, using the project ID shown in
TaskFlow and an existing local clone:

```sh
export TASKFLOW_AGENT_TOKEN='<the same shared token>'
./bin/taskflow agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone --terminal terminal
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

### Browser startup

Starting the server does not open a browser automatically. Open
`http://localhost:8090` manually. Set `TASKFLOW_OPEN_BROWSER=1` to opt in to
automatic opening; `TASKFLOW_NO_BROWSER=1` always disables it.

Native agent launches are recorded as launch activities, not managed workflow
steps. The native skill reports its verified stage through MCP; only actual
server-managed workflow steps retain the structured-result transition gate.
Server-managed skills are refreshed during skill sync, including their MCP
transition instructions. Local differences are backed up before replacement.

### Server/agent contract

See [the version 1 contract](docs/contracts/server-agent-v1.md) for the configuration
fields, launch messages, precedence, skill ownership and error behavior. Every
launch downloads fresh configuration; there is no offline execution fallback.

### One local agent for multiple projects

With `TASKFLOW_AGENT_TOKEN` set, discover projects and start the agent:

```sh
taskflow agent --url http://localhost:8090 --list-projects
taskflow agent --url http://localhost:8090
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
and a copyable launch command. Set `TASKFLOW_AGENT_TOKEN` in your terminal before
running it; the UI does not store or display the server credential.

Task cards and the task clarification panel provide **Copy skill command**.
Choose Codex or Claude and a workflow skill, then copy the interactive terminal
command. Commands use the task primary key and project identity with MCP
instructions. Run them in a local repository where the project skills and
TaskFlow MCP are already configured.

Remote work is shown on task cards and list rows with a **Remote execution** badge.
The MCP tools `taskflow_start_run` and `taskflow_finish_run` track the invocation
independently of stage transitions. Updated standalone skills and copied commands
report this lifecycle; existing installed skills need to be refreshed. An abruptly
closed client may leave an activity to cancel manually in the activity view.

Agent-owned remote executions have a **Stop** button on the task. TaskFlow waits
for the local supervisor to confirm process termination before marking the run
canceled. Worktree changes are preserved. This requires restarting the local
agent with the updated binary; previously launched or independent Codex/Claude
processes cannot be controlled by the new supervisor.

Project settings include **Create PR/MR**: choose the default **Draft after implementation**, or **Draft after specification** to review specs in an early draft.
Skills reuse the same PR/MR during implementation and attach its URL through MCP. Adjust never creates a PR. Missing PRs recover through the configured earlier stage without downgrading completed work. `Create PR` (`create_pr`, `/create-pr`) remains available under Additional skills. It creates or reuses a PR without advancing the task stage or joining the automatic workflow. The legacy `review` invocation resolves to Adjust; inherited review customizations require reconciliation in Skills.

To regenerate skills from the desktop app, open the project gear menu, select **Deployment**, and click **Deploy server skills**. Configure and save the local repository first, and stop active executions before deployment. The agent fetches the current server skill content; **Refresh from server** alone refreshes settings without deploying files. Skills are also refreshed when preparing task executions.

### Desktop console host

Use TaskFlow Desktop to follow native Codex/Claude terminals locally:

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
export TASKFLOW_AGENT_TOKEN='your-server-token'
taskflow agent --url http://localhost:8090 --repo /path/to/repository
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
packaged desktop app. Server and agent currently share `bin/taskflow`; start
the agent with its `agent` subcommand. Use `make server-build` or
`make agent-build` for the shared executable only, and `make desktop-build`
for the desktop development assets. On Apple Silicon the app is produced at
`desktop/release/TaskFlow-darwin-arm64/TaskFlow.app`.

The optional companion groups local executions under projects in a collapsible
sidebar. Add projects by discovering the server catalog and mapping a local Git
directory. Local worktree preferences are stored per project in
`~/.config/taskflow/settings.json`. Repository layout, remote URL, SDD selection and skill
content remain server-owned and read-only. Explicit deployment buttons install
the server skills or initialize its SDD framework in the mapped directory.
The profile is a placeholder for future account management.

### Execution defaults and local overrides

The server project supplies `useWorktrees` and `parallelism` (1–3) defaults.
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

Server and agent currently share `bin/taskflow`. Launch targets use existing
builds and do not rebuild. Pass agent arguments with, for example,
`make start ARGS="--url http://localhost:8090"`; provide authentication through
`TASKFLOW_AGENT_TOKEN`.

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

Task IDs in the desktop sidebar open the task directly on the configured TaskFlow server. Server links use `?task=<task-primary-key>` and open task details independently of board filters.

For `agy`, the local agent registers the TaskFlow stdio bridge in
`~/.gemini/config/mcp_config.json`; this CLI does not read the workspace
`.agents/mcp_config.json`. Other MCP registrations and explicit tool policies are
preserved. The shared entry contains no token or gateway URL: agent-launched
sessions inherit `TASKFLOW_AGENT_URL` and `TASKFLOW_AGENT_TOKEN`. Standalone agy
sessions must supply those variables themselves. Restart agy after registration
so it loads the updated MCP tools.
