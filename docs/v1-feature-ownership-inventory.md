# V1 feature ownership inventory

Date: 2026-10-29
Status: source inventory started; migration and client deprecation decisions remain open.

This inventory records the current product surfaces before any web or Electron
feature is removed. It is based on the current Tauri source and the Sectile Go
repository's `web/src/App.tsx`, `web/src/components/`, and `desktop/src/` and
`desktop/electron/` modules. It is an ownership map, not an authorization to
delete an API, page, database field, or client.

## Current feature surfaces

| Product capability | Electron/web evidence | Tauri evidence | V1 owner and migration rule |
| --- | --- | --- | --- |
| Tracker ticket board, list, triage and ticket conversations | Web `BoardView`, `ListView`, `TriageView`; Electron board, task page and conversation modules | `Supervisor`, `WorkflowTask`, `ConversationTranscript`, `ProjectPrompt` | Tauri is the primary operational board. Tracker state remains authoritative remotely; local run and conversation state belongs to the selected Go runtime. |
| Agentic execution and workflow | Electron `run-engine`, `workflow`, `orchestration`, `batch-pickup`, `run-console`, `tool-cards`, `queue` | Tauri `Supervisor`, `ExecutionMonitor`, `WorkflowTask`, `RunConsole`, `RunReview`, provider controls | Tauri + Go agent own launching, observation, interruption, questions, review and resume. Preserve workflow and MCP contracts across Local and Fleet. |
| Workspaces, folders, Git and local CLIs | Electron `run-folders`, `folder-menu`, `git-init`, `engines`, `engine-setup`, project run modules | Tauri `ProjectFolderSetup`, `FolderContext`, `AdvancedFolders`, provider setup | Go agent owns filesystem/process operations; Tauri owns native folder selection and clear project/prompt context. Folder selection must be visible at the point of starting work. |
| Project/profile/provider configuration | Electron settings, `run-project`, `engine-setup`, `sandbox-settings`, `orchestration-settings`, `mcp-settings` | Tauri `LocalSettings`, `ProjectProfiles`, `ProviderSettings`, `OrchestrationSettings`, `WorkflowSettings` | Tauri should offer a coherent native setup path. Backend owns persisted tracker and execution rules; secrets remain in the selected runtime's private store. |
| Runtime connection and desktop lifecycle | Electron `runtime`, `connection-settings`, `pairing`, `agent-identity`, `desktop-updates`, `notifications` | Tauri native Local/Fleet selection, companion lifecycle and status UI | Tauri owns Local/Fleet selection and companion lifecycle. Electron compatibility remains until its support/deprecation window and MCP registration migration are agreed. |
| Skills and MCP | Electron `mcp-settings`, skill mapping modules and workflow modules; web `SkillsView` | Tauri provider/workflow controls; Go MCP and skill APIs | Go owns MCP tools and shared workflow semantics. Tauri owns desktop configuration and visibility. Existing Electron MCP registration must continue to work through the supported transition. |
| Product planning and portfolio | Web `RoadmapView`, `SprintTimelineView`, `TeamView`, tracker backlog, project/board views and admin; Electron task/board modules consume task and project APIs | No equivalent full planning portfolio; Tauri operational dashboard is project/task focused | Do not port the full planning suite into Tauri by default. Keep web/API/domain behavior during transition. Select an external owner and define stable IDs, links, sync direction and preservation before deprecation. |
| Server administration and tracker administration | Web `AdminView`, `TrackerSetup`, `SyncView`, `ActivitiesView`; Go server routes and tracker adapters | Tauri has profile, tracker and project setup screens but not all shared/admin operations | Keep shared OAuth app secrets and server-wide tracker administration on the backend/admin surface. Personal setup belongs in Tauri only where the selected backend exposes an authorized API. |
| Desktop productivity affordances | Electron command palette, notifications, updates, previews, editor/diff, task shortcuts, appearance and attached-folder UI | Tauri command palette, conversation, folder context, settings and native shell; no demonstrated parity for every Electron affordance | Classify each affordance by operational value before calling parity complete. Carry over high-value task-start, navigation, folder and execution affordances; do not copy incidental Electron chrome. |

## Ownership boundaries

- Tauri is the default user interface for operational agentic work: choose a
  project/workspace, attach folders, prompt, follow execution, answer questions,
  review changes, and resume conversations.
- Go remains the shared business and execution layer: tracker adapters, auth,
  workflow state, API/MCP, process execution, durable journals and authorization.
- Remote trackers remain the source of truth for tickets. The local runtime may
  cache tracker data but must not imply that a local board owns remote task state.
- The current web application remains available until a replacement contract,
  client migration and deprecation window are explicitly agreed.
- Electron remains a supported client/consumer until its desktop lifecycle,
  provider/MCP registrations and compatibility window have a migration decision.
- An external roadmap tool is not selected. No planning data is to be deleted,
  copied unidirectionally, or made inaccessible while this decision is open.

## Client transition paths

| Current consumer | Transition path | Exit gate before removal or default change |
| --- | --- | --- |
| Sectile web application | Continue using the regular Go server, which keeps serving the current web UI and its planning views. The optional headless build is a separate artifact and does not replace this server. | Choose the roadmap destination and data/link contract; publish a support window; verify web users can reach or export every retained planning object. |
| Sectile Electron desktop | Keep its current backend, agent and MCP registrations working while Tauri becomes the primary desktop for operational work. Do not copy credentials or switch a shared MCP registration during Tauri onboarding. | Pass the supported Electron/server/agent matrix, verify a live Electron MCP call after the Tauri migration, then announce a deprecation and support window. |
| Sectile Tauri Local | Run the Go companion as the local API/MCP/backend and execution service. Keep the project database, OAuth records, identity and conversations in the selected Local installation. | Complete upgrade recovery, provider interruption/resume, local tracker/OAuth and packaged platform qualification. |
| Sectile Tauri Fleet | Use the explicitly selected Fleet server through its paired or already-paired local agent. Keep remote API/MCP/auth and business state on that backend. | Complete real one-time pairing, native profile/project setup and supported remote Fleet reconnect/outage checks. |
| Third-party MCP clients | Continue using the current Go MCP/API contracts. The new headless server omits the browser surface only; it does not remove tools or alter caller authorization. | Publish a version compatibility matrix and prove every retained tool against the supported server versions before changing a contract. |

These paths keep current clients usable while the product destination is
implemented. They do not select the external roadmap product or set final
deprecation dates; those remain Stage 7/8 decisions.

## Incoherences to resolve in Tauri

These are concrete UI ownership or information-architecture issues found in the
current Tauri surface and should be handled in small, reviewable increments:

1. Project and workspace context is split between project settings, prompt
   attachment controls and advanced folders. The user needs one clear place to
   see the active project and all attached folders before starting a prompt.
2. Profile/provider setup, workflow policy and project configuration are spread
   across separate screens. The primary project view should expose essential
   choices in context and keep infrequent policy controls in advanced settings.
3. Trackerless local projects share too much visual language with tracker-backed
   boards. Empty/local states must not show remote tracker filters or imply a
   tracker connection is required to run a prompt.
4. Connection mode, service readiness and project/task navigation should have a
   single consistent status and navigation hierarchy; duplicate indicators or
   controls obscure the active runtime and project.
5. The current UX audit has no controlled screenshot comparison using equivalent
   data, selected project, theme and window dimensions. Broad styling changes
   should follow that baseline rather than copying Electron styles by memory.

The first fixes for trackerless board controls and redundant current-board
navigation are already recorded in the UX audit and tested in Tauri. Remaining
items require renewed inspection of the current UI before implementation.

## Decisions still required before Stage 7 can close

1. Select the external roadmapping product, or explicitly defer migration and
   commit to retaining web planning for a defined support period.
2. Define identity mapping and link behavior for projects, epics, tasks, sprints,
   users and workflow artifacts, including any synchronization direction.
3. Set support and deprecation windows for web, Electron, API and MCP clients.
4. Name the admin surface for shared OAuth client secrets and tracker-wide setup.
5. Define the public distribution licensing/provenance requirements for bundled
   Go services.

Until these decisions and compatibility gates are complete, Stages 6 and 7 are
in progress and no existing surface is considered removable.
