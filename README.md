# Sectile Tauri

An MIT-licensed, local desktop supervisor for tracker-backed development workflows.
Built with Tauri 2, Rust, React and TypeScript. The initial integration reuses the
existing Sectile Go server, workstation agent and MCP infrastructure.

## Current scope

- Single-view desktop workspace with searchable project navigation, refresh and connection diagnostics.
- System, light and dark themes, including the streamed console.
- Board and backlog list of GitHub, GitLab and Jira tickets returned by the local agent, with search, multi-selection, agent skill actions, priority and the same optional epic colors as Sectile.
- Create a tracker ticket from the project board or project menu, and edit a ticket from its card, backlog row or workflow. Title, Markdown description, priority, issue type and ordered PR/MR links are saved through the Go companion's ticket API; a project without a tracker cannot create a ticket. PR/MR links can be added after creation.
- Columns use remote tracker statuses. Local tasks and unknown sources are excluded. Ticket menus can launch configured agent skills; a selected batch uses the Go `pickup_issues` skill after confirmation.
- Native authenticated loopback bridge; the agent token never enters the webview.
- Project-scoped free prompts with Claude Code or Codex, using a lightweight React controller over the Go conversation engine.
- Free-prompt conversations show concise activity and execution status; ticket-linked conversations add their skill, tracker status, PR/MR and workflow stage.
- Conversation composers keep folder attachment, terminal launch and MCP verification in a compact action row. The project folder path beneath the title can be copied. Folder attachment is shared with the project and applies to a running conversation on its next message.
- Ticket conversations show linked pull requests and merge requests with their last known forge state; the status strip and ticket workflow list every link, while the sidebar provides a compact shortcut to the primary link. An unavailable forge credential is shown explicitly rather than implying a current state.
- Ticket conversations use the ticket title in the sidebar instead of the agent skill command.
- Task launch with project-provided skills, inherited or explicit execution mode, and optional instructions.
- Ticket workflows keep unverified provider preparation in a compact disclosure; actual MCP configuration conflicts remain visible before launch.
- Ticket workflows use one return action to the board, with project prompts and settings available from the project sidebar.
- Workflow stages stay in a balanced two-row layout in compact windows, including when the project sidebar is expanded.
- Run monitoring every three seconds, waiting/authentication states and acknowledged cancellation.
- Read-only streamed console output with bounded scrollback and reconnect controls.
- Launch of saved named orchestration agents with launched/skipped ticket reports.
- Native setup for a local profile, personal tracker credentials, ticket sources and projects, including folder selection when associating an existing project.
- AI engine settings use the Claude and OpenAI marks already used by Sectile Electron, with provider-specific colors.
- Settings use their own navigation while configuring a new profile; the header returns to the first project once setup creates it.
- Use Cmd/Ctrl+K for navigation, Cmd/Ctrl+, for Settings, Cmd/Ctrl+B for the project sidebar, and Cmd/Ctrl+N for a new prompt in the selected project. Drag the sidebar grip or focus it and use the arrow keys to resize it.
- At narrow window widths, the sidebar can expand while leaving 500 pixels for the conversation; its saved width returns when the window is wider.
- Native ticket-source editing and deletion for Local users and Fleet administrators; deletion is available only after the source is no longer linked to projects or tickets.
- Explicit Local or Fleet runtime selection, with native one-time-code Fleet pairing and private workstation credentials.
- Automatic startup and reconnection of the Go companion in Local mode or the workstation agent in Fleet mode.
- If the companion is temporarily unavailable, an open conversation keeps its history and draft, blocks sends, and refreshes its status when the connection returns.
- Service diagnostics in native Settings, with a bounded Local companion or Fleet agent log view that works even when startup fails.
- French runtime UI; English source documentation.

This is a bootstrap, not feature parity with Sectile Desktop. Supported Fleet
backends can provide OAuth for GitHub.com, GitLab.com and Jira Cloud through
backend-managed applications. Existing Local installations can keep their
configured OAuth apps.
Self-hosted forge OAuth, OS credential-vault storage, provider-app handoff and
automatic service upgrades are not implemented.
The Go server owns tracker authentication and synchronization. Board refresh
rereads its synchronized data; it does not force a remote tracker synchronization.
In Local mode, synchronization uses the connected installation account's
personal token or Jira OAuth grant. Fleet synchronization uses its backend's
provider-level credential. The tracker setup screen shows completion or failure
for a requested synchronization.
There is no independent local task store and no fixture data in the application.

## Requirements

- Node.js 22.18+ and npm
- Stable Rust and Cargo
- [Tauri platform prerequisites](https://v2.tauri.app/start/prerequisites/)
- Go and a compatible Sectile source checkout for local service builds, or an existing paired Sectile agent

## Start development

```sh
npm ci
npm run tauri dev
```

## Configure a standalone desktop

Build the Go services from a checkout containing the local-runtime isolation
changes and authenticated `/api/local-runtime` probe (protocol 1):

```sh
SECTILE_SOURCE_DIR=/path/to/compatible/sectile npm run prepare:services
npm run tauri dev
```

Without `SECTILE_SOURCE_DIR`, the build script uses `../sectile`. Services are
compiled for the current host into ignored `src-tauri/binaries/` and included as
application resources when packaging. The script also checks that the Go
third-party notices match that source's `go.mod`, then bundles the notice and
license files beside the executables. Go code remains an external dependency;
this repository does not contain or relicense it. Source compatibility is required:
an older server without the authenticated probe cannot be used as a managed service.

Each service build embeds the Go source commit and commit date in `--version`.
The ignored `src-tauri/binaries/build-manifest.json` records that identity, the
source-tree cleanliness, release eligibility, and SHA-256 hashes for all three
services. The manifest is packaged alongside the executables. Set
`SECTILE_SOURCE_COMMIT` to require a specific source revision; set
`SECTILE_REQUIRE_CLEAN_SOURCE=1` to reject a modified checkout. A build is marked
release-eligible only when it comes from a clean commit with an exact Git tag.

Open **Paramètres**, then:

1. Choose **Démarrer en mode Local** to create separate local storage and start the companion. For Fleet setup, see [Connect to Fleet](#connect-to-fleet) below.
2. Save an email and display name under **Profil local**. This creates the local
   identity, pairs this workstation and starts its agent. It is not a provider login.
3. Under **Comptes et trackers**, save a personal provider API token in Local
   mode. Existing Local OAuth applications remain available in the secondary
   disclosure. Fleet uses OAuth applications configured by its backend; sign in
   through the system browser, or expand the advanced manual-token option.
   See [OAuth setup](docs/oauth-setup.md) for application registration and scopes.
4. Add a **Source de tickets**, naming the repository or project key. Credentials
   are personal and shared by sources of the same provider; saving replaces that
   provider's stored access. Account confirmation failures do not undo a saved token.
   GitLab website addresses entered in either form are converted to its `/api/v4`
   endpoint before they are saved.
5. Open a local folder as a project, then add a registered tracker from the
   project's configuration or **Settings → Projects** when needed. You can also create a tracker-backed project first
   and associate its local folder afterward. The tracker association keeps the
   project's existing local board and folder.
6. Use **Synchroniser** for the tracker. Settings follows the Go activity and
   reports its completion or failure below that source; the project's board
   refreshes when the activity finishes. Background tracker synchronization is
   also enabled. When Settings detects an active background synchronization, it
   follows that activity and disables the manual button until it finishes.

The local profile currently has one identity per installation; its display name
can be changed. Native setup adds sources and projects; advanced editing and
self-hosted forge OAuth are not part of this setup screen.

After activation, every app launch checks the private local server and starts
missing services. Closing the desktop leaves Go services and executions running;
reopening reconnects to them. Fleet pairing starts its managed workstation agent
and reconnects it on later launches; an attached pre-existing Fleet agent remains
managed by its original installation. This is not an OS login service or a
continuous crash-restart watchdog. **Vérifier / démarrer le compagnon local**
retries Local startup. An occupied port belonging to another process is refused,
not taken over.

Data lives under Tauri's application data directory in `local-runtime/`, shown
in Settings. The Local installation and Fleet workstation credentials stay in
private native files; tracker credentials are encrypted in the Local server
database or managed by the selected Fleet backend. On Unix the directory uses
mode 0700 and credential files use mode 0600. This is file-based credential
protection, not an OS keychain. `server.log`, `companion.log` and `agent.log`
provide startup diagnostics. Provider processes do not inherit the desktop
gateway credential. Existing Sectile databases and global provider registrations
are not imported.
The **Services** section reads the latest 256 KiB of the selected mode's
desktop-managed log directly from disk. Before a mode is selected, it offers
both companion and Fleet-agent logs so a failed first startup can be diagnosed.
An attached Fleet agent started by a different installation may keep its log
in that installation instead.
If the selected Fleet backend is unavailable, Settings keeps Fleet selected,
shows the disconnected agent and backend error, and leaves the Fleet agent log
available. The workstation agent continues its own reconnection attempts;
**Actualiser** reads its current status without relaunching it. Backend-owned
profile, tracker and project controls become available again after reconnection.
If the local Fleet agent gateway itself cannot be read, the selected Fleet mode
and its log remain visible with the gateway error. Refreshing after it returns
also reloads the signed-in profile; unsaved profile edits in an already ready
session are not overwritten by a status refresh.

## Connect to Fleet

Choose **Services locaux**, enter the backend HTTPS URL and a one-time pairing
code generated from the signed-in Fleet profile, then select **Associer ce poste
à Fleet**. Tauri redeems the code with the Go backend, stores the workstation key
in its private data directory and starts the lightweight agent. No API token
needs to be copied into the form. For an agent already paired through another
Sectile installation, use **Associer l’agent Fleet existant** to copy its local
connection into Tauri's private directory without modifying that installation.
The normal request path only reads the selected mode's private connection; it
never falls back to the shared Electron connection file. With a one-time-code
pairing, native settings can manage the signed-in profile, personal tracker
credentials, project setup and OAuth account connections through the selected
Fleet backend. OAuth app credentials and shared tracker administration remain
server-admin operations. Fleet OAuth browser tickets are opened only on the
selected backend origin. Tauri keeps the API credential in private native
storage and does not return it to the renderer. An explicitly attached legacy
agent without a native API credential can read projects through the agent, but
native profile, tracker and project setup requires one-time-code pairing.
With a paired Fleet agent, **Ouvrir un dossier** creates a folder-only project
on the selected backend and attaches its folder through the local agent.
**Choisir un dossier** or **Changer de dossier** associates a folder with an
existing Fleet project through the native picker; subsequent prompts use the
new folder. Only `http://127.0.0.1:<port>`
workstation-agent origins are accepted by Tauri; redirects and proxies are
disabled.

`npm run dev` starts a browser preview of the shell. Native integration is intentionally
unavailable there and displays an explanatory message. Use `npm run tauri dev` for live data.

## Start, monitor and stop work

**Nouveau prompt** starts a free Claude Code or Codex conversation in the project,
without requiring a tracker ticket. The message field receives focus on entry.
The context bar shows the actual working
directory and **Dossiers · N**. The composer combines saved profiles and
standalone Claude Code/Codex choices in one menu; model and mode remain editable
beside it.
**Ajouter un dossier** opens the native selector;
the folder joins the project, rather than becoming a message-private upload.
Existing conversations use it from the next message. **Configurer le projet**
lists attached folders and offers **Détacher**, which never deletes files. Its
advanced folder section maps declared repositories and Macro/Issue specification
folders. Git repositories and plain folders retain Go's worktree/write semantics.

A ticket card opens its guided workflow: remote status, six-stage progression,
next skill, configured pickup stopping point, optional instructions, execution
history and PR/MR access. The board opens with workflow stage columns and can
group the same remote tickets by tracker status; that choice is remembered.
Batch controls appear when tickets are selected, keeping the default board
toolbar compact. The project header keeps the free prompt prominent and exposes
settings through its labeled icon.
Stage labels, per-tracker mappings and statuses determine
the presentation. The desktop stores no independent task truth.

**Étape suivante** rereads the ticket, project and executions before submitting.
An accepted request stays pending until its run appears; it is never automatically
resent. A process exit and a skill result are displayed separately: the server's
activity and stage must confirm completion. A missing PR at the implemented stage
returns to the project's PR creation owner. Autonomous Codex chains require an updated Go agent with sandboxed automatic
review and JSONL trace support (Go `85da2f24` or a qualified successor, tested
with Codex CLI 0.165.0). Their tools stop on agent death; initial run identity is
saved before startup. Ordinary autonomous trace tails remain periodic snapshots.
User-authored command templates keep their own permission/output configuration.

**Lancer la chaîne** displays whether
pickup stops at Implemented or Reviewed. Manual review validation is a separate
explicit declaration, requires a justification and does not merge a PR.

The shared execution monitor polls without overlapping requests. Stale state or a
disconnected server blocks launches; local cancellation remains available when the
agent is reachable. Structured conversations render questions and approvals directly;
legacy terminal output remains read-only. **Revue des changements** reads the
selected run folder's Git diff on demand, supports additional run folders and opens
a configured workstation editor. Patches are rendered as text.

**Paramètres → Moteurs IA → Skills et connexion MCP Sectile** separates provider
sign-in from workflow preparation. Choose a source project to install skills.
For the standalone Local runtime and managed native Fleet agents, MCP is supplied to native Claude/Codex launches
through process arguments under `sectile_companion`; shared user registrations
remain untouched. An external agent retains its explicit registration controls
and requires consent before replacement. No skills are installed on startup.
Existing plugins/manual installations can be usable even when
not verified by the companion; configuration validity is not a live MCP-session test.
Installation may require stopping open executions first; Go reports the refusal.
The same screen exposes explicit default permission preferences for Claude and
Codex, while native sandbox rules remain in force. Conversations can request an
engine-reported MCP status without sending a provider prompt; Claude's idle
companion transport check is labelled separately. Transport readiness alone
does not prove that an existing registration points to this companion.
Codex MCP form consent is answered in the conversation with one-time approval or
refusal. Boolean and enumerated form questions are supported; unsupported forms
are refused. No approval is silently made permanent.

**Configurer le projet** exposes settings progressively: the working directory and
default conversation profile
stay visible. Profile management and additional folders, repository/specification
roles, workflow policy and orchestrators stay in collapsed sections. Closing a
section preserves its draft and does not save or launch anything.

Advanced configuration includes named orchestration agents with Markdown policy
files, previews/reload, enablement and shared
concurrency. Unsaved agent edits block leaving for supervision. Saving does not
launch work. **Orchestration** evaluates a saved policy on demand and reports
launched/skipped tickets, active local executions and individual stop controls.
Capacity usage comes from the server's project state; an unavailable value is
shown as unverified rather than inferred from workstation executions.
There is no client-side scheduling loop. Disabling orchestration leaves existing
runs active. Native controls do not send messages to third-party desktop apps.

See [the reviewed parity plan](docs/desktop-parity-plan.md) and
[ADR 0010](docs/adrs/0010-electron-parity-through-existing-go-contracts.md).

## Validate and package

```sh
npm run check
npm test
npm run test:ui
npm run build
cargo fmt --manifest-path src-tauri/Cargo.toml -- --check
npm run test:rust
# After preparing compatible Go services:
npm run test:services
npm run package:macos
```

`npm run package:macos` builds an arm64 app and DMG under
`src-tauri/target/release/bundle`, then verifies that both embed the exact Go
service manifest and hashes from `src-tauri/binaries`, include dependency notices
and license texts, and pass `hdiutil verify`. It refuses dirty Go service
builds. Use `npm run tauri build` only for development packages that do not need
this qualification. Bundles use Sectile's existing desktop icon across macOS,
Windows and Linux; the vector source is `src-tauri/icons/source.svg`. V1 uses manual
DMG updates. A release requires a Developer ID Application signing identity in
the macOS keychain and Apple notarization credentials. Set
`APPLE_SIGNING_IDENTITY` and either the App Store Connect API key variables
(`APPLE_API_ISSUER`, `APPLE_API_KEY`, `APPLE_API_KEY_PATH`) or Apple's supported
notarization account variables before `npm run package:macos`. Keep credentials
outside the repository; see the [Tauri macOS signing guide](https://v2.tauri.app/distribute/sign/macos/).
After packaging, verify the app and every bundled Go
executable with `codesign --verify --strict`, assess the app with `spctl`, and
validate the stapled ticket before testing the DMG on a clean macOS account.
The full gate is in the [V1 release checklist](docs/v1-release-checklist.md).
Bundles include the
MIT license and third-party dependency notices in the application resources;
see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and the
[notice maintenance guide](docs/third-party-notices.md).
The V1 packaging decision is documented in
[ADR 0018](docs/adrs/0018-v1-desktop-packaging-and-updates.md).

### Preparing a managed Local update

For a configured Local installation, open **Settings → Services locaux →
Mettre à jour l’application** and choose **Arrêter et quitter pour mettre à
jour**. Finish active executions/preparations and close other desktop instances
first. The desktop asks the bundled Go helper to authenticate the Local runtime,
stop it gracefully, and verify both transports and the data-store lifetime lock
are released. Only then does it quit. On failure it keeps automatic startup
suspended; retry preparation or explicitly start the companion to keep working.
Idle durable Claude Code and Codex conversations remain resumable through this
shutdown, including a Codex app-server with its RPC channel open between turns.
This requires the updated Go companion (`f55e8768` or a qualified successor);
older candidates can incorrectly refuse an idle conversation. Active turns,
queued messages and approvals still block shutdown. The action is absent in
Fleet and does not download an update. The initial
unpaired/bootstrap state is not yet covered by this native action.

Closing the desktop leaves its Go companion running. Replacing the app bundle
alone does not replace that process: the next desktop can reconnect to the old
companion. For source-checkout maintenance, finish its active runs, quit **all** Tauri
windows and, with Node.js installed, run:

```sh
npm run stop:local -- --data-dir "$HOME/Library/Application Support/com.sebferry.sectile.tauri/local-runtime"
```

Use the actual `local-runtime` directory belonging to the installation. The
example identifier above must match the installed bundle; development and
qualification bundles can use different identifiers. The command follows the
selected managed generation, authenticates both loopback services, checks their
Local relationship, and requests the existing graceful shutdown. It refuses
Fleet mode, active executions/preparations, redirects, unknown generations and
mismatched services. Credentials remain in private files and HTTP headers.
It never sends process signals, edits data or automatically stops active runs.
A failed command does not authorize replacement. Keep Tauri closed until the
command confirms both connections stopped, then replace the app and reopen it.

This is a source-checkout maintenance command, not a bundled updater. It checks
transport shutdown; native clean-user installation, service replacement and
signed distribution remain release qualification gates. The native action adds verification that all store users have drained and
released their lifetime lock; this source-checkout fallback only checks the
transports. Native app exit and bundle replacement still require qualification.


## Layout

| Path                      | Responsibility                                   |
| ------------------------- | ------------------------------------------------ |
| `src/App.tsx`             | Desktop shell, project selection and board       |
| `src/domain/board.ts`     | Tracker-only filtering and status grouping       |
| `src/lib/agent.ts`        | Typed frontend boundary for native commands      |
| `src-tauri/src/agent.rs`  | Discovery, token handling and fixed agent routes |
| `src-tauri/src/local.rs` | Isolated service startup and native setup APIs    |
| `src/components/LocalSettings.tsx` | Native onboarding forms                  |
| `src-tauri/capabilities/` | Native window permissions                        |
| `docs/adrs/`              | Architecture decisions and migration boundaries  |
| `tests/`                  | Board behavior tests                             |

## Direction

Sectile should own orchestration, workspace isolation, progress and human checkpoints.
Provider runtimes execute tasks through supported APIs or CLIs. Keep the execution
adapter separate from the UI so a supported provider-app handoff can be added later.
MCP exposes Sectile tools to agents; it does not by itself control desktop conversations.
See [the supervisor architecture](docs/adrs/0001-local-desktop-supervisor.md) and
[the standalone local Go decision](docs/adrs/0003-standalone-local-go-services.md).

## License

New code in this repository is licensed under [MIT](LICENSE). The Go services
remain maintained in the separate Sectile repository, which is also MIT-licensed.
Tauri packages their built executables and includes the corresponding dependency
notices; it does not relicense the Go source.

## Try the project prompt prototype

Associate a local repository with a project in Settings and authenticate the
chosen provider CLI on this workstation. Open **Nouveau prompt**, choose Claude
Code or Codex, and send a request. Follow the answer, inspect tool details, allow
or deny a tool request, and interrupt the current turn from the conversation.
Select the conversation in the project sidebar to return to it; navigating away
does not stop Go. Ticket-specific skill launch stays on each board card.
The conversation status strip shows execution, skill, ticket and workflow stage;
its execution menu opens the run or project queue. The project menu groups
conversations by recent activity, workflow stage or tracker status. Each
conversation's sidebar menu can stop its Go session. Once stopped, it can be
archived; the project menu switches to archived conversations so they can be
opened or restored. Archive visibility is a preference on this desktop, while
Go retains the transcript and run history.
Conversation headers keep long titles and discussion actions on one line;
response state stays beside the composer. Click the project in the sidebar to
return to its board. Ticket conversations show a PR/MR link in the sidebar and
status strip when the tracker supplies one.

The conversation view uses a lightweight React controller over native IPC. Go owns
history, execution, permissions and interruption. The controller polls while a
turn runs, preserves observation across navigation, prevents duplicate sends and
never replays a prompt after an ambiguous connection failure. Interactive questions support choices and free answers directly in the conversation.
Permanent tool permission grants remain unsupported. See
[ADR 0006](docs/adrs/0006-react-conversation-controller.md) for the replacement
of the CopilotKit experiment and measured bundle savings.

When an existing project has no local folder, **Nouveau prompt** displays
**Choisir un dossier** before the composer. Picking a folder associates it with
that project on this workstation and opens the composer immediately; no duplicate
project is created. Cancelling leaves setup available. Existing conversation
histories remain accessible even if a project currently has no folder mapping.

Open **Configurer le projet → Dossier local** to inspect the current association
and use **Changer de dossier** to pick a replacement with the native selector.
Unmapped projects offer **Choisir un dossier** in the same section. The new path
is saved for this project on this workstation and shown immediately in its toolbar.
Cancelling or a failed association retains the previous path. New conversations use
the replacement; existing conversations keep their original working directory.

## Project profiles and conversation names

Open **Configurer le projet** from a project's toolbar (or its entry under
**Paramètres → Projets**) to create named Claude Code/Codex profiles. Choose a
model, Plan/execution mode, and the project's default. Profiles are shared on this
workstation; editing one affects future prompts in projects using it. Open
conversations retain their current settings. Workstation defaults, command-template
profiles and profiles used by other projects/tasks cannot be deleted here.

**Nouveau prompt** preselects the project's profile. You can still change the
provider, model and mode before sending. Saving a profile never starts a task.
Profiles use the Go engine catalogue and require the updated bundled companion
(settings layout 7).

Free conversations receive a title from the first prompt. Use **Renommer** in the
conversation header to change it. The title is stored locally with the conversation
journal and survives agent restarts; renaming never sends a provider message or
changes remote tracker tickets. Existing saved histories derive titles from their
first user message and can also be renamed.

Ticket workflow screens keep draft instructions scoped to the selected project
and ticket. Switching tickets starts with a fresh workflow view.
Next-step, chain and custom-skill launches all require a configured project.
Once the configured chain target is reached, the workflow shows that result
and keeps the next manual step available. Chain launch checks the latest ticket
and policy again before dispatching.

## Configure AI provider access

Open **Paramètres → Moteurs IA** to detect Claude Code and Codex using the Go
agent's executable lookup. Each card reports CLI presence and the official
authentication status command's exit status. Detection starts once the Local or
Fleet agent is available, including after first-time setup. **Se connecter** opens the official
CLI login in a visible terminal (and its browser flow). The desktop checks status
every five seconds for two minutes; **Détecter / vérifier les connexions** retries
after installation or a longer login. Opening a terminal never counts as a
successful login. Credentials remain managed by the provider CLI.
Both provider cards appear together in the compact settings layout. Installation
details and Sectile skills/MCP preparation remain available from each card.

The CLI must already be installed. macOS uses Terminal, Windows uses a new command
window, and Linux requires `x-terminal-emulator`. Older CLI versions with no
authentication status subcommand can report an indeterminate state. Authentication
status reflects local credentials, not a paid model request or entitlement check.

Official commands: [Claude Code](https://code.claude.com/docs/en/cli-reference)
and [Codex](https://learn.chatgpt.com/docs/developer-commands).

## Open a local folder without a tracker

Click **Ouvrir un dossier** in the sidebar and choose a folder in the native
folder picker. Its name becomes the project name, its path is associated locally,
and the free-prompt view opens immediately. Git and tracker configuration are
optional for this conversation workflow. Reopening the same mapped folder selects
its existing project. Cancelling creates nothing. Activate the local runtime and
profile first; provider CLI authentication remains under **Moteurs IA**.

Folder projects store project metadata in the local Go server and the path in
workstation settings. They do not create tracker tickets. Git-based task workflows
still require a Git repository. If creation succeeds but association fails, use
**Réessayer l’association** to recover the existing project.

## Verify real Codex interruption

With Codex installed and signed in, run:

```sh
SECTILE_TEST_REAL_CODEX=1 npm run test:services
```

This opt-in test uses the workstation's Codex authentication with disposable
server, agent and project data. It waits for a shell-written start marker,
interrupts a 90-second delayed writer, waits 95 seconds, checks that the delayed
file is absent and verifies that the same thread resumes with `RESUME_OK`.
The normal service test does not send prompts to a provider.

The equivalent Claude Code scenario uses:

```sh
SECTILE_TEST_REAL_CLAUDE=1 npm run test:services
```

It verifies an actual response, shell tool events and results, a running-shell
start marker before interruption, absence of the delayed file after 95 seconds,
and resumption of the same conversation. Any required Bash approval is granted
only once and only when the requested command exactly matches the test command.

For a read-only Fleet MCP isolation check with signed-in CLIs, run
`npm run test:fleet-mcp` with `SECTILE_TEST_REAL_CODEX=1` and/or
`SECTILE_TEST_REAL_CLAUDE=1` and the selected server/agent binary paths below.
The fixture supplies the native Fleet scoped endpoint, approves only the fixture
project-context read, requires a real tool result and provider answer, and checks
that shared Claude/Codex MCP registrations remain unchanged (Python 3.11+ parses
Codex TOML independently of unrelated CLI preferences). It does not set the
refresh-disable switch, so scoped isolation must work by itself. This is an agent
API qualification, not a native window or full workflow test.

For a Fleet agent crash test with either or both signed-in CLIs, run
`npm run test:fleet-crash` with `SECTILE_TEST_REAL_CODEX=1` and/or
`SECTILE_TEST_REAL_CLAUDE=1`. Set `SECTILE_TEST_SERVER_BINARY` and
`SECTILE_TEST_AGENT_BINARY` to the packaged executables to qualify. The harness
allocates a free loopback Fleet port, or uses `SECTILE_TEST_FLEET_PORT` when set;
no Local callback-port rebuild is needed. Local runtime discovery stays disabled.
The test creates
an isolated server, account, agent and Git folder, kills only that agent during
an active 90-second shell tool, and checks after 95 seconds that the delayed
file is absent. It restarts the agent and verifies history, model, mode,
contextual response, a new tool call, and no replay of the interrupted tool.
The test requires a Unix host and removes its temporary runtime data afterward.

For an opt-in external write smoke on the authorized GitHub `sebastienferry/sectile`
and GitLab `smartadserver/private/shared-platform/taskativ` projects, provide an
authenticated `gh` CLI, `GITLAB_TOKEN`, and `SECTILE_TEST_SERVER_BINARY` pointing
to a disposable Go server built with an unused Local runtime port. Run
`npm run test:live-trackers`. It creates a uniquely named issue on each forge,
updates its title through Sectile, waits for the queued tracker activity,
verifies the remote result, and deletes the issue. It deliberately stores the
GitLab website address without `/api/v4` to cover older saved configurations.
Do not use the regular
52963 companion binary or data directory for this test.

## Read conversation output

The conversation follows new output while you stay near the bottom. Scroll up
to read earlier messages without being moved; **Aller en bas** resumes following.
Sending your own message also returns to the latest output. The composer remains
visible while the transcript scrolls.

Assistant replies support Markdown headings, lists, tables, quotes and inline
code. Fenced code blocks show their language and a **Copier** button, preserving
indentation and horizontal scrolling. Raw HTML is displayed as text; remote images
are not fetched automatically. HTTP(S) links open in the system browser.

Tool calls and their results share collapsible cards. Consecutive calls are
grouped, and results are matched by tool ID even when they arrive out of order.
Summary labels distinguish running, completed, failed and missing results.
Approvals remain explicit controls outside collapsed tool output.

## Answer agent questions

Claude Code and Codex questions appear above the composer while the agent waits.
Choose an option, select several when the question permits it, or enter a custom
answer. Answer every question and click **Envoyer les réponses**. A custom answer
replaces a single choice; multiple choices and extra text are sent as comma-separated
answers, matching the Go agent contract. **Passer la demande** remains available.

The desktop sends answers to the pending question request, not as a new prompt.
Drafts survive polling updates while the request stays the same. Duplicate sends
are blocked; uncertain acknowledgements never trigger automatic replay.
Codex questions marked secret use password fields and are not stored in browser
storage; the existing Go adapter redacts them in history. Go continues to validate
the pending request and translate question IDs for each provider. Questions must
be emitted by the provider: the UI does not manufacture questions or change its
execution mode.

## Choose a conversation model and mode

Use **Modèle** and **Mode** under **Prochain message**, both in the new-prompt form
and in an existing conversation. A selection is sent together with the next
message, never while the current turn is running. **Conserver** leaves the current
choice unchanged. Accepted choices remain in that Go conversation; global engine
settings are not changed.

Model choices combine the workstation's configured provider catalogue with live
Codex models when available. The current model is retained even if absent from
the catalogue. **Identifiant personnalisé** accepts an explicit CLI model ID; the
provider still decides whether that model is available to the signed-in account.
Changing the provider before the first prompt clears pending model/mode choices.

**Plan** maps to the provider's plan mode. **Exécution** maps to Claude's normal
`default` permission mode (ask when needed), or Codex's `workspace-write` sandbox
with approval on request and network access disabled by the existing Go policy.
No bypass or automatic-approval mode is exposed. Existing read-only or other
configured modes are preserved until an explicit selection changes them.

## Conversation restart recovery

New free Claude Code and Codex conversations preserve their transcript, provider
session, chosen model and mode across companion agent restarts. The next message
resumes the provider session on the same workstation. Opening a history does not
replay prior prompts or tools. Pending messages and approvals are not resubmitted.

Conversations explicitly stopped by the user and legacy histories saved without
provider session identity stay read-only. Provider-owned local session files must
still exist. Ticket discussions with injected environments retain read-only
history recovery.

Claude Code and Codex conversation tools follow the Go agent lifetime. On Unix,
an independent supervisor terminates their process tree when the agent dies,
including SIGKILL; on Windows a private kill-on-close job owns the provider and its
descendants. Closing the desktop still leaves the agent running. Completed side
effects are not rolled back.

Conversation messages, calls, results and streamed text are synced incrementally
before display. After a crash, unfinished text stays in history as an interrupted
reply; the next message resumes the provider session without replaying tools.
Periodic checkpoints compact the journal rather than defining a five-second loss
window. Storage failures are displayed and prevent unsaved new prompt dispatch.
See ADR 0009 for tests and Go ADR 0064 for the storage protocol.

## Workspace and configuration

The desktop uses compact conversation controls and settings navigation for AI
engines, profile, accounts/trackers, projects and local services. Switching settings
sections preserves unsaved form values and ongoing authentication polling. Open a
local folder directly from Projects without configuring a tracker. OAuth application
settings and CLI installation details expand only when needed.

The prompt composer sends with Enter and inserts a line break with Shift+Enter.
Suggestions fill the composer without starting execution. Model and mode choices
still apply only when sending. Open settings with the header button,
even with the project sidebar collapsed. Conversation navigation reuses already
loaded transcript titles and provider/model metadata without fetching histories
for every sidebar entry. Light, dark and system themes remain available.

The Go companion owns the fixed local OAuth origin `http://127.0.0.1:52963`. Register `http://127.0.0.1:52963/auth/jira/callback` in the Atlassian developer console. Tauri discovers this contract from the bundled server and displays its callback; it does not allocate an OAuth port. If the port is occupied, startup fails rather than changing the registered callback. Older installations on a different port require migration with local services stopped. Run isolated service integration tests only with the desktop local services stopped, because they exercise the same fixed port.

Saved OAuth application credentials are not prefilled in the desktop. Both editing fields are masked, cleared after submission or closing the form, and may be left empty to retain existing values. The native layer returns only presence flags for the saved client ID and secret.

## Sectile V1 migration

The [V1.0.0 migration plan](docs/sectile-v1-migration-plan.md) records the
current phase status and evidence. Stages 0–4 are delivered on the dedicated
branches; stages 6–7 are complete as product ownership and distribution
contracts. The seven scoped Electron-to-Tauri UX convergence lots are complete.
Stages 5 and 8 remain open for exact release-binary, external integration and
macOS installation qualification. Local/Fleet selection does not silently
transfer data or credentials between runtimes. Existing web planning remains
the V1 owner and is unchanged. See the [candidate support matrix](docs/v1-support-matrix.md)
for demonstrated combinations rather than interpreting a protocol check as
full Fleet feature support. No V1 release has been tagged or approved.
