# Sectile (React + Go + SQLite or PostgreSQL)

Desktop now provides a read-only **Changes** view for each local execution, comparing current worktree contents with the default-branch common ancestor. See [Inspect worktree changes](desktop/README.md#inspect-worktree-changes).

The desktop supports persistent workstation project disconnection, with active
execution protection and explicit re-add. See [Remove a local project](desktop/README.md#remove-a-local-project).

Outil moderne et agentique de gestion des tâches pour développeurs et équipes techniques, construit avec **Go**, **React 19**, **Tailwind CSS v4**, et **SQLite**.

---

## ✨ Fonctionnalités implémentées

- **Server-side tracker integration**:
  - GitHub REST supports synchronization, issue creation, updates and comments without an online agent.
  - Jira Cloud REST supports the same, plus the sprint, team and epic fields GitHub does not have, over the account's API token.
  - Configure explicit server credentials and repository/team identifiers. CLI login state is not used by the server.
  - Local tasks remain in SQLite.
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
  - **Choix du moteur d'IA** : sans modèle de commande, chaque fournisseur est lancé
    avec la ligne que Sectile atteste pour le mode d'exécution demandé. Pour `claude` :
    - interactif : `claude --model <modèle> '<prompt>'`
    - autonome : `claude -p --permission-mode bypassPermissions --model <modèle> '<prompt>'`

    Les autres fournisseurs : `agy -i` en interactif, `vibe -p --auto-approve` et
    `codex exec` en autonome. `agy`, `gemini` et `cursor` n'ont pas de mode autonome
    attesté et refusent un lancement headless plutôt que d'en deviner un.
  - **Modèle par lancement** : la liste des modèles de chaque moteur se règle globalement
    (section *Moteur IA* du profil) et c'est elle que proposent le lanceur de la vue détail
    et le menu `...` d'une carte. Le modèle résolu par la configuration y est le choix par
    défaut : le garder ne change rien à la commande, en choisir un autre n'écrit aucun
    réglage. Sur une carte, le choix est une sélection que la carte conserve, affichée en
    quatre caractères devant ses boutons d'action ; tous ses lancements l'utilisent, chaîne
    complète comprise.
  - **Commandes personnalisées** : chaque mode a son champ, au global comme par projet,
    et les deux s'héritent indépendamment. La commande autonome sert les lancements
    headless ; laissée vide, ce sont les lancements headless qui retombent sur la
    commande interactive, laquelle doit alors porter le
    marqueur `{mode:AUTONOMOUS|INTERACTIVE}` pour dire quels mots appartiennent à
    quel mode. Les écrans de réglages affichent les deux lignes résultantes.
  - **Personnalisation des Prompts par Skill** :
    1. 🔍 **Clarify** (`/clarify-issue`) : Analyse les ambiguïtés et génère les questions de cadrage.
    2. 📝 **Specify** (`/specify-issue`) : Rédige la spec (Spec Kit ou OpenSpec, selon le framework du projet) et initialise la branche Git.
    3. 💻 **Implement** (`/code-issue`) : Plan de code, modification des fichiers et tests unitaires.
    4. **Adjust** (`/adjust-issue`): Review the full branch, address findings and available PR feedback, run final checks, and update the existing PR before human merge.
    5. ⚡ **Auto-Pilot** (`/pick-issue`) : Routeur intelligent qui enchaîne automatiquement l'étape optimale.
  - **Panneau de statut des CLI** : Vérification en temps réel de l'installation et de l'authentification de `git`, `gh`, `agy`, `claude`, `codex`, ainsi que des outils SDD `uv`, `specify` et `openspec`.

- 🗂 **Sidebar complète & Workflow Stages** :
  - `Backlog` ➔ `À clarifier` ➔ `Spécifié` ➔ `En cours` ➔ `À valider` ➔ `Terminé` avec compteurs en temps réel.
  - Bascule des vues (`Tableau Kanban` / `Vue Liste`).
  - Filtres rapides (`Mes tâches`, `Priorité Haute`, `Étiquettes/Tags`) et filtre par source (`GitHub`, `Jira`, `Local`).
  - **User project bookmarks & dropdown search**: Personal project bookmarks with star toggles, dropdown project search across shared workspaces, and "All projects" board/facets filtered strictly to bookmarked projects. Bookmarked projects are also prioritized in task creation, clone, and detail modals.
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
  - **Vue Tableau Kanban** : Glisser-déposer fluide entre colonnes avec mise à jour automatique du tracker.
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

Start the server with its persistent database. [`.env.sample`](./.env.sample)
documents every variable the server, the agent and the MCP bridge read; copy it
to `.env`, which the server loads at startup and which is gitignored:

```sh
# Optional: the tracker credential is usually typed in the interface instead,
# but a headless deployment can export it here.
export SECTILE_TRACKER_TOKEN='<tracker API token>'
# Serving several providers at once? Override per provider:
# export SECTILE_GITHUB_TOKEN='<GitHub API token>'
# export SECTILE_GITLAB_TOKEN='<GitLab API token>'
DB_PATH=/path/to/tasks.db PORT=8090 ./bin/server
```

### PostgreSQL instead of SQLite

SQLite is the default and the only engine the desktop application ships with. A
server deployment that already runs PostgreSQL can use it instead, for managed
backups, point-in-time recovery and the ops tooling that comes with them:

```sh
export DB_DRIVER=postgres
export DATABASE_URL='postgres://sectile:password@db.internal:5432/sectile?sslmode=require'
# The only source of the encryption key under PostgreSQL. There is no database
# file to generate one beside, and a key invented on each restart would silently
# make every stored tracker token unreadable. Generate: openssl rand -hex 32
export SECTILE_SECRET_KEY='<64 hex characters>'
./bin/server
```

`DB_PATH` is ignored in this mode. A PostgreSQL configuration that cannot be
opened stops the server rather than falling back to SQLite: falling back would
serve an empty board out of an unexpected store, which reads as data loss.

One server instance per database. The job queue and the synchronisation loop run
in-process and are not coordinated between instances, so two servers sharing a
database would run every queued skill twice.

To move an existing SQLite database across, once:

```sh
SECTILE_SECRET_KEY='<the same key the SQLite server uses>' \
  ./bin/sectile-migrate -from ./tasks.db -to "$DATABASE_URL"
```

The key matters: a stored tracker token is sealed to its owner and its tracker,
not to the database, so the rows copy perfectly well under a different key and
nobody notices until a tracker call fails. The migration opens one sealed
credential as a check before it copies a single row, and refuses a destination
that already holds data.

Open **http://localhost:8090**. The server never opens a browser or starts local
Git, tracker CLI, terminal, editor or LLM processes. A server deployment needs
only its binary, writable database/configuration storage and network access to
its trackers. Put it behind your deployment's access-control boundary; the
existing browser REST API is still a single-user interface.

On the workstation, pair once with a code from **Pair a workstation** in the
profile dialog, then start the agent:

```sh
./bin/agent pair --url http://localhost:8090 --code '<pairing code>'
./bin/agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone
```

Install and authenticate the coding CLI and Git tools on that workstation.
`make agent` requires only Go; it does not build the web UI. `make desktop`
builds the agent with the optional console companion, and `make desktop-package`
packages it. `make run` opens it. For development, run `make serve` and
`npm run dev --prefix web` in separate terminals; the Vite server listens on
port 5173 and proxies `/api` to `http://localhost:8090`.

### Tracker connection parameters

**The interface is the primary way to configure them.** *Connecter votre tracker*
asks for the instance URL, the repository or project slug and the token of the
selected tracker — Jira, GitHub or GitLab — checks them against the instance, and
saves them in the user configuration only once the instance has accepted them. No
file to edit on the server, and no restart. A project can override the instance,
the slug and the token for itself, which is what lets two GitHub organisations
with two different tokens live side by side.

Tokens are write-only: the API never returns one. It reports `githubTokenSet` /
`gitlabTokenSet` / `jiraApiTokenSet` instead, plus `...FromEnv` when no token is
stored and the server environment supplies one. Saving with an empty token field
keeps the stored token; sending the sentinel `__clear__` deletes it.

**A Jira credential is personal, and only personal.** An Atlassian account
belongs to a site, so the site, the account e-mail and the token travel
together: all three are stored from the person's own profile, in *Connecter
votre tracker*. A project put on Jira prefills its tracker URL from the
instance of whoever creates it. No server-wide Jira credential appears in the
interface at all; the `SECTILE_JIRA_*` variables remain only as a fallback for
unattended work. An operation somebody asked for either carries their own token
or is refused, because writing it under the server account would put a name on
it that nobody chose.

**A tracker credential can be personal.** On Jira a comment, an assignment and
a transition are attributed to the account whose token made the call, so a
shared token makes the whole team sign as one integration account. *Profil >
Trackers* therefore holds one zone per tracker Sectile can drive. Jira accepts
only a personal credential; GitHub accepts either, and falls back to the server
token where nobody stored one. A personal token is encrypted with AES-256-GCM,
bound to its owner and to its tracker, with the key held outside the database
(`SECTILE_SECRET_KEY`, or a 0600 file beside it — `secret.key`, which belongs
in no backup the database is in). A row moved from one user to another stops
opening. The server starts without the key and refuses only what would need it.

Optionally, a **sealing passphrase** derives the key instead, through Argon2id,
and is never stored. Nothing can then open that token without its owner, the
server included. The cost is stated in the screen at the moment of the choice:
Sectile writes to trackers from a background queue, and a sealed token is
unusable there until its owner unlocks it. A locked credential fails the
operation rather than falling back to the server token, which would write under
a name nobody chose. See [ADR 0014](./docs/adrs/0014-personal-tracker-credentials-are-sealed.md).

The background queue carries whoever asked: a sync, a field update and every
tracker operation record the acting user on the job, and the worker puts them
back before resolving a credential. Only work nobody asked for — the auto-sync
timer — names nobody and keeps the server credential.

Jira asks for the site (`mon-org.atlassian.net`), the account e-mail and an
Atlassian API token, which authenticate as `email:token`. Its environment
fallbacks are `SECTILE_JIRA_URL`, `SECTILE_JIRA_EMAIL` and `SECTILE_JIRA_TOKEN`,
then `SECTILE_TRACKER_TOKEN` and `JIRA_API_TOKEN`. A project overrides the site
through its `trackerUrl`; the e-mail and the token stay global, one Atlassian
token being valid on every site of the account.

GitLab parameters can be stored, but no GitLab ticketing adapter is registered
yet: a project whose tracker is GitLab still fails with the tracker registry's
unconfigured-tracker error. That adapter is a separate piece of work.

The environment variables below stay supported, as the fallback for headless and
CI deployments where no one opens the interface. **Stored configuration wins**:
for each parameter the server resolves the project override, then the user
configuration, then the environment.

| Setting | Meaning |
| --- | --- |
| `SECTILE_TRACKER_TOKEN` | Tracker API credential, used by every provider that has no override below. |
| `SECTILE_GITHUB_TOKEN` | GitHub-only override; takes precedence over `SECTILE_TRACKER_TOKEN`. `GH_TOKEN` then `GITHUB_TOKEN` are environment-only fallbacks. |
| `SECTILE_GITHUB_API_URL` | REST base URL; defaults to `https://api.github.com`. GitHub Enterprise uses `https://<host>/api/v3`. |
| `SECTILE_GITLAB_TOKEN` | GitLab-only override; `GITLAB_TOKEN` is an environment-only fallback. |
| `SECTILE_GITLAB_API_URL` | GitLab REST base URL; defaults to `https://gitlab.com/api/v4`. |
| `SECTILE_GITLAB_PROJECT` | Default GitLab project slug, e.g. `group/app`. |

Environment variables are read from the environment of the **server process
itself**, at startup only. `make serve`, `go run ./cmd/server` and
`./bin/server` inherit the shell they are launched from, so exporting a
variable in another terminal — or after the server is already running — has no
effect: restart the server, or, better, type the value in the interface, which
takes effect on the next request. A `gh` login on the same machine is not picked
up either; for GitHub only `SECTILE_GITHUB_TOKEN`, then `SECTILE_TRACKER_TOKEN`,
then `GH_TOKEN`, then `GITHUB_TOKEN` are consulted. The provider-specific
variable comes first so a server driving several providers cannot send one
provider's credential to another.

The tokens are kept in the server database and in the agent's reach only through
the server: `~/.config/sectile/settings.json`, the agent's own configuration,
stays free of credentials.

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
`owner/repository`. The server never discovers
these through a local clone or CLI credential store. Missing credentials and
API failures fail the operation visibly; there is no workstation fallback.

### Releases and migration

`make release` emits `server-<os>-<arch>` and `agent-<os>-<arch>` under
`dist/`, with `.exe` for Windows. Install them under the canonical command
names `sectile-server` and `sectile-agent` used throughout this document.
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

### Container image and GitLab CI

`Dockerfile` builds the server as a static binary with the web interface
embedded, on a distroless non-root base. Everything it writes goes under
`/data` (`DB_PATH=/data/tasks.db`, plus the data directory resolved through
`XDG_CONFIG_HOME=/data/config`), so mount a persistent volume there:

```sh
docker build -t sectile-server .        # or: make image
docker run -p 8090:8090 -v sectile-data:/data sectile-server
```

The GitHub repository is pull-mirrored into GitLab, where `.gitlab-ci.yml`
runs the Go, web and desktop test suites on every mirrored branch and tag,
then publishes two things: the server image
(`<registry>/server:<pipeline>-<ref-slug>`, plus `latest` on `main` and the
tag name on a tag) and the cross-compiled `sectile-server-*` /
`sectile-agent-*` binaries, uploaded to the project's Generic Package Registry
under the package `sectile` with the same version string. The agent is never
part of the image: it runs on workstations, next to the coding CLIs.

A branch other than `main` is also published as `server:preview-<commit sha>`,
the tag the test environments pull. Those are declared in argocd-sp
(`apps/sectile/dev`, feature `testenv`): a merge request opened on the GitLab
mirror for the mirrored branch and labelled `testenv` gets its own board at
`https://testenv-<merge request number>-sectile.internal.eqtv.dev`, with its
own database, following the head of the branch until the merge request closes.
The GitHub pull request alone spawns nothing: the generator only reads GitLab.

## 📚 Documentation Technique Complète

Une suite documentaire complète pour développeurs et LLMs est disponible dans le dossier [`/docs`](./docs) :

- 🏛️ [**Architecture & Conception Générale** (`docs/ARCHITECTURE.md`)](./docs/ARCHITECTURE.md) : Modèle de concurrence, persistance SQLite, isolation Git Worktrees, PTY ZSH & WebSockets.
- ⚡ [**Capacités & Workflows Agentiques** (`docs/CAPABILITIES.md`)](./docs/CAPABILITIES.md) : Multi-projets, pipeline de 5 skills, Auto-Pilot, synchronisation GitHub / Jira.
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

Sectile exposes ten typed tools at the Streamable HTTP endpoint `/mcp`:

- `list_projects`: discover project primary keys, names and Git remotes.
- `get_task`: read task details and comments.
- `transition_stage`: record a verified workflow stage and queue tracker synchronization.
- `add_comment`: post a task comment.
- `list_tasks`: list tasks with optional filters.
- `get_project_context`: read project execution settings and effective instructions.
- `create_task`: file a new ticket on an explicitly named project, remotely whenever its tracker supports it.
- `update_task`: update mutable descriptive fields of an existing task (title, description, priority, issueType, labels).
- `start_run`: start or reuse the invocation's remote run.
- `finish_run`: finish that run without advancing the task stage.

HTTP and stdio both identify the server as `sectile`. Tool arguments, results,
authentication and workflow validation retain their existing contracts.

`/mcp` is stateful: every connected client holds one server session, so two
clients sharing the same credential stay distinct and a client that goes away is
noticed. A run started with `start_run` belongs to the session that started it.
When that session ends — the client quits, its process is killed, or it falls
silent past the idle timeout — the server closes the runs it still owns as
canceled, with a note saying the client disconnected. `finish_run` remains how a
run reports its own outcome and always wins over that fallback. A run reused from
a launcher keeps its dispatching agent as owner, since that agent already watches
the real process.

`GET /api/mcp/sessions` lists the live sessions, what each client calls itself,
and the runs it owns. The board's status bar shows that count and opens a panel
naming each connected client, how long it has been attached, and the runs that
would close with it. `SECTILE_MCP_SESSION_TIMEOUT` (default `15m`) bounds a
silent session, and `SECTILE_MCP_CLIENT` names a bridge in that list. A server
restart destroys every session at once, so startup closes the runs they owned as
canceled; runs dispatched to an agent are preserved, because that agent
reconnects and reports the real process exit.

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
interface unauthenticated. Without these variables the interface falls back to
the local e-mail sign-in below. Signing in is mandatory either way: an
unauthenticated visitor sees the sign-in screen and nothing else.

Each person then **pairs their workstation** from the profile dialog: generate a
pairing code, single use and valid ten minutes, and spend it once with
`sectile-agent pair` or in the desktop connect screen. The workstation receives
its own API key and keeps it; nobody handles the key. It is the one credential
every machine surface takes: the agent, the desktop app, `/mcp` on the server
and the agent gateway. Keys expire after 90 days by default and are renewed or
revoked per workstation from the same list, without disturbing the others. The
agent warns in its log ten days before its key expires, and a refused key says
`API key expired` rather than asking you to check for a typo.

A key is shown in clear only in the panel's advanced case: an MCP client
configured by hand on a machine with no agent to pair for it.

Roles can come from the provider. Name the claim carrying a person's groups or
roles and the value that grants admin, and each sign-in sets the role from it:

```sh
export SECTILE_OIDC_ROLE_CLAIM='groups'              # Okta: a groups claim on the authorization server
export SECTILE_OIDC_ADMIN_GROUP='sectile-admins'     # Auth0: a namespaced claim set by a post-login Action
```

The two are set together or not at all. The claim is read from UserInfo, then
from the ID token, and it may be a single string or a list. It is the authority:
it overwrites a role an admin changed by hand at that person's next sign-in.
Without it, the first person to sign in while no admin exists becomes the admin.

### Before an identity provider: the local sign-in

Without `SECTILE_OIDC_ISSUER` the interface offers a local sign-in: an e-mail
address and nothing else. An unknown address creates the account, the first
account created is the admin, and everyone after that is a member.

The form also takes an optional sealing passphrase, the one protecting your own
tracker tokens. It is never a login password: a wrong one signs you in anyway
and leaves those tokens locked until you unlock them from your profile.

It identifies people; it does not authenticate them. Anyone who types a
colleague's address is that colleague, so keep it to a trusted network and treat
it as the step before connecting Okta or Auth0, which disables it. A deployment
with no account yet shows the sign-in screen and no board; the first person to
sign in becomes the admin.

### Personal and deployment settings

Presentation (theme, accent, language, density, default view, scale, detail
mode), the displayed identity and the workstation commands (editor, external
terminal) are **personal**: each account keeps its own, and an execution opens
the terminal of whoever owns it. The trackers, the repository path, auto-sync,
the AI configuration and the prompts are the **deployment's** and are an admin's
to change. An account that has never saved a preference sees the deployment's
values, so an upgrade changes nothing on screen.

### Roles

| | Admin | Member |
|---|---|---|
| Board, tasks, transitions, comments | yes | yes |
| Launch and stop **their own** executions | yes | yes |
| See everyone's running executions | yes | yes |
| Stop **someone else's** execution, dispatch to their agent | yes | no |
| Global settings, tracker credentials | yes | no, beyond their own preferences |
| Create, edit and delete projects | yes | no |
| List users, change roles, other people's workstations | yes | no |

Executions record who started them. A stop is delivered to the owner's agent,
which is what keeps a colleague's run from being closed while its process is
still running. The profile's **Users** section, visible to admins, lists the
accounts and changes roles; the last admin cannot be demoted.

Then start the workstation agent in an existing clone, with its key:

```sh
sectile-agent pair --url https://sectile.example.com --code '<pairing code>'
sectile-agent init --provider <provider>    # bootstrap MCP and skills locally
sectile-agent --url https://sectile.example.com --project '<project-id>' --repo /path/to/clone
```

You can also run `sectile-agent init --provider <provider>` anytime to bootstrap
MCP registration and install managed skills for a specific provider locally
without launching the background daemon.

`SECTILE_SERVER_TOKEN`, the former shared agent credential, is still accepted
for one release with a startup warning; a server without it that has issued no
key yet also keeps accepting any nonempty token, and closes that door with the
first key. Workstations paired before keys expired keep working as keys without
expiry, and the profile offers to set one.

The agent fetches `GET /api/v1/agent/config`, creates or validates local Git
worktrees, installs effective project skills, and launches the configured AI CLI.
Local command templates support task and repository placeholders, including
`{prompt}`, `{issueTitle}` and `{repoPath}`; see the [desktop placeholder guide](desktop/README.md).
It does not open a database. Server filesystem paths and tracker credentials are
excluded from the configuration contract. The old agent `--db` option is removed.
A disconnected or incompatible configuration API prevents execution.

Before launching an LLM CLI, the local agent automatically registers Sectile in
that CLI's **user-level** configuration, and installs the managed skills there
too. Claude Code, Cursor and Gemini get a Streamable HTTP entry addressing the
server's `/mcp` with the workstation key as bearer, so their Sectile tools keep
working while the agent is stopped; the other CLIs get the
`sectile-agent mcp --url <server>` stdio bridge with the key in its environment.
The file is written owner-only. Existing settings and other MCP servers are
preserved. Native workspace/MCP trust prompts still apply. Malformed
configuration causes a visible launch error rather than being overwritten.

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

The Discuss action opens the configured agent on a task without running a skill:
the provider is launched alone, with no skill command and no generated prompt, in
the task's own checkout and branch. It is offered in the task menu, in the task
detail and in the desktop launch selectors. The session carries the usual
`SECTILE_*` environment, so the agent can read the task through the Sectile MCP
when asked, but the discussion transitions no stage, records no skill result and
reports nothing to the tracker. It is listed, stoppable and replayable like any
other execution.

For clients started outside Sectile, manual registration is still available,
and the local agent is not required: create a key under the profile's advanced
case. A client that speaks Streamable HTTP addresses the server directly with
that key as bearer:

```json
{
  "mcpServers": {
    "sectile": {
      "type": "http",
      "url": "https://sectile.example.com/mcp",
      "headers": {"Authorization": "Bearer sectile_…"}
    }
  }
}
```

A client limited to stdio runs the bridge with the same key:

```json
{
  "mcpServers": {
    "sectile": {
      "command": "/absolute/path/to/sectile-agent",
      "args": ["mcp", "--url", "https://sectile.example.com"],
      "env": {"SECTILE_AGENT_TOKEN": "sectile_…"}
    }
  }
}
```

The bridge also reads `SECTILE_AGENT_URL`. Terminals launched by the agent
inherit it, set to the server, together with `SECTILE_AGENT_TOKEN`. The agent
gateway on `http://127.0.0.1:8091` still proxies `/mcp` and `/api/` and takes the
same key. Set `SECTILE_MCP_CLIENT`, or pass `--client`, to name that client in
the session list; the bridge otherwise reports its host and process id.
Protocol output uses
stdout; diagnostics use stderr. The stdio bridge never falls back to another
database or server after an error.

Optional workstation overrides belong in `~/.config/sectile/settings.json`:

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
paths are untouched. Put persistent skill overrides in `~/.config/sectile/settings.json`.
The effective `.taskflow/remote-config.json` snapshot is diagnostic only: it is
never used as an offline fallback. These generated files are ignored by Git.

The machine endpoints and the agent handshake validate the workstation API key.
`SECTILE_SERVER_TOKEN`, when configured, is still accepted for one release; a
server without it accepts any nonempty token only until its first key is issued.
This does not add multi-user login or authentication to the existing web/REST UI;
remote deployments still need their existing access-control boundary.

### Start the server and native local agent

The web manages tasks, the agent hosts native coding CLI consoles, and the optional desktop displays them. The
following commands start the standalone local agent. For the console
application, see the desktop setup section below. Start the server in one terminal:

```sh
npm ci --prefix web
make server agent
./bin/server
```

Start the local launcher in another terminal, using the project ID shown in
Sectile and an existing local clone:

```sh
./bin/agent pair --url http://localhost:8090 --code '<pairing code>'   # once
./bin/agent --url http://localhost:8090 --project '<project-id>' --repo /path/to/clone --terminal terminal
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

Once the workstation is paired, discover projects and start the agent:

```sh
sectile-agent --url http://localhost:8090 --list-projects
sectile-agent --url http://localhost:8090
```

The agent defaults to all projects. The current checkout is matched by its Git
origin; map other project primary keys to local repositories in
`~/.config/sectile/settings.json` in the starting directory (or the directory passed with
`--repo`):

```json
{"projects":{"project-primary-key-a":"/path/to/repo-a","project-primary-key-b":"/path/to/repo-b"}}
```

Use `--project <primary-key>` to restrict the agent to one project. Terminal and
skill settings are downloaded from the server before each launch; no
`--terminal` argument is necessary. See the
[server/agent contract](docs/contracts/server-agent-v1.md) for identity and mapping rules.

The profile dialog includes a **Local agent** section with an editable server URL
and a copyable launch command. Pair the workstation once with
`sectile-agent pair` and a code from the panel above.

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
sectile-agent pair --url http://localhost:8090 --code '<pairing code>'   # once
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
packaged desktop app. The outputs are `bin/server` and
`bin/agent`; the agent starts directly. Use `make server` or
`make agent` to build independently, and `make desktop-build`
for the desktop development assets. On Apple Silicon the app is produced at
`desktop/release/Sectile-darwin-arm64/Sectile.app`.

The optional companion groups local executions under projects in a collapsible
sidebar. Add projects by discovering the server catalog and mapping a local Git
directory. Local worktree preferences are stored per project in
`~/.config/sectile/settings.json`. Repository layout, remote URL, SDD selection and skill
content remain server-owned and read-only. Explicit deployment buttons install
the server skills or initialize its SDD framework in the mapped directory.
The profile is a placeholder for future account management.

### Formatting and checks

Go sources are `gofmt`-clean: `gofmt -l .` must report nothing at the repository
root. `make test` enforces it through its `fmt-check` dependency, so an
unformatted file fails the suite before any test runs. Run `gofmt -w .` to fix
it, or `make fmt-check` to see the offending files on their own.

### Execution modes

A skill run is either **interactive** (a terminal window you answer, and the
stage moves when you confirm) or **autonomous** (the CLI runs headless, with the
provider's non-interactive approval mode, its output recorded on the run
activity, and it posts its own stage transition through the Sectile MCP tools).
When an autonomous run of a workflow step closes without having moved the task,
the run says so: the server checks the hand-back but never invents a transition
the work may not have earned.

The mode of one launch is resolved in this order, first opinion winning: the
one-off override chosen for that launch, then the skill's own setting in the
skill editor, then the project's `defaultSkillMode`, then interactive.

The one-off override is offered wherever you explicitly trigger a skill: the web
task card menu, the web task detail modal, and the desktop Launch and Relaunch
dialogs. The desktop next-step button stays a single click on the resolved mode.

`claude -p --permission-mode bypassPermissions`, `codex exec` and
`vibe -p --auto-approve` are the attested headless invocations. A discussion and
a bare terminal stay interactive whatever the project default says.
On `agy`, `gemini`, `cursor`, or a custom `aiCommandTemplate` with no
`{mode:AUTONOMOUS|INTERACTIVE}` placeholder, an autonomous launch is refused by
name rather than silently run interactively.

The project also sets `fullChainStopStage`, where the **Full chain** (`>>`)
action stops: `implemented` (before the pull request) or `reviewed` (default).
A full chain run is always autonomous, and each step enqueues the next one when
it closes having advanced the stage, until the stop stage. Merging stays manual.

### Execution defaults and local overrides

The server project supplies the `useWorktrees` default, which **Inherit worktrees
from server** restores in the desktop project settings. Parallel executions
(1 to 10, set with a slider) are workstation-owned: the server neither stores nor
supplies a value, the desktop app is the only surface that sets one, and a
project without a local value runs a single execution at a time.
Workstation settings are saved in `~/.config/sectile/settings.json` as project-ID maps:

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
`~/.config/sectile/settings.json`, shared by the CLI agent and companion.
Writes preserve connection fields, use atomic replacement and mode 0600.
Legacy repository mappings remain readable and are migrated on the next save.

| Command | Action |
| --- | --- |
| `make server` | Build the server |
| `make agent` | Build the local agent |
| `make desktop` | Build the desktop app, without packaging |
| `make desktop-package` | Build and package the desktop app |
| `make all` | Build all components |
| `make start` | Start the local agent |
| `make serve` | Start the server |
| `make run` | Start the desktop |

Server and agent are built as `bin/server` and `bin/agent` by the `build-*` targets.
The `serve`, `start` and `run` targets run from source and need no prior build. Pass agent
arguments with, for example, `make start ARGS="--url http://localhost:8090"`;
the workstation must be paired first with `sectile-agent pair`.

### Browse desktop project tasks

Hover or keyboard-focus a desktop project row and activate its **Open tasks**
list icon to browse that project's unfinished server tickets immediately, in the
**Tickets** pane that takes the console's place. Search by title or key, or
submit an empty search to restore the full open list. Rows are ordered by
priority descending then task identity ascending, and the **Key**, **Title**,
**Stage** and **Priority** headers sort the list. A row's key opens that task in
Sectile. **Run** launches the task's next workflow step, and the **…** menu
offers pickup, the other skills, a discussion console and custom instructions.
**Run** is disabled while an execution is active on that task. Loading the list
never starts an execution. Failed requests can be retried with Search, and
launching requires a configured local repository.

### Desktop Quick add

Click **New task (+)** beside a desktop project to choose **Run an existing
ticket** or **Quick add task**. Both paths target the clicked project, even
when another project's execution is selected. Existing tickets open the Tickets
pane; Quick add preselects the project and offers **Launch task** after
successful creation, which opens that pane on the new ticket.

Press **Cmd+K** (macOS) or **Ctrl+K** to open the command palette, search its
actions, and choose **Quick add task** or **Tasks list**. Enter runs the first
matching action. **Tasks list** opens the Tickets pane for the selected project,
for the only configured project, or for a project you pick when several apply.
For **Quick add task**: The selected project's identity is prefilled; without a
selection, choose a project explicitly. Enter a title and optional description.
The server creates the task using its project tracker configuration.
GitHub and Jira creation must succeed remotely; errors do not silently create
a local fallback, and the site's own refusal is quoted, so a mandatory field it
requires is readable. Local projects remain local. Creation does not start an execution;
the success screen offers a separate **Launch task** action.

Task IDs in the desktop sidebar and in the Tickets pane open the task directly on the configured Sectile server. Server links use `?task=<task-primary-key>` and open task details independently of board filters.

For `agy`, the local agent registers the Sectile stdio bridge in
`~/.gemini/config/mcp_config.json`; this CLI does not read the workspace
`.agents/mcp_config.json`. Other MCP registrations and explicit tool policies are
preserved. The entry runs `sectile-agent mcp --url <server>` with the workstation
API key in its environment, so standalone agy sessions work without the agent.
Restart agy after registration so it loads the updated MCP tools.

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
