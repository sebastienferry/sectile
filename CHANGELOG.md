# Changelog

All notable changes are documented here using Keep a Changelog conventions.

## [Unreleased]

### Added

- Open desktop settings, toggle the project sidebar and start a project prompt with keyboard shortcuts matching Electron; a visible sidebar grip supports pointer and keyboard resizing.

- Edit or remove ticket sources from native Settings. Existing board and synchronization settings stay intact when editing; a source linked to a project or containing tickets cannot be removed.

- Prepare a configured Local installation for manual update from Services: stop its idle companion, verify store release and quit the desktop, with explicit retry/restart controls on failure. A source-checkout transport-checking command is also available.

- Add a tracker to an existing local or Fleet project from its configuration or desktop Settings while retaining its local board and folder.
- Read bounded Local companion or Fleet agent diagnostics directly in desktop Services, including when the service is unavailable.
- Include MIT and third-party dependency notices in packaged Tauri applications, with Go notices checked against the selected Go source dependency graph.
- Restore the last valid project and workspace view on desktop launch, falling back to a project board when the saved conversation is no longer available.

### Fixed

- Preparing a Local update preserves idle Claude Code and Codex conversations, including Codex sessions with an open RPC connection, while still refusing active work.

- Bundled autonomous Codex workflows use sandboxed automatic review and stream their tools and progress; autonomous Claude/Codex runs stop their tools on agent crash and save their initial history before starting.

- Conversation history now describes approval decisions in French instead of exposing protocol values such as `allow` and `deny`.

- Keep managed Fleet Claude Code and Codex sessions connected to their selected backend without changing Electron’s shared MCP registration.

- Ticket workflows now show when the chain's configured target is reached instead of offering another chain launch, including when the ticket advances just before a click.
- Workflow chain and custom-skill actions now stay unavailable until the project is configured, matching the next-step action.
- Switching tickets now clears the previous ticket's workflow and draft instructions while the next ticket loads.
- AI provider detection now starts after the Local or Fleet agent is available, avoiding stale connection errors after first-time setup.
- Project tracker setup now shows a completed state after all available sources are linked and follows the project settings form layout.
- Tracker setup now follows an active background synchronization and disables the manual sync button until it finishes, avoiding a second request from that screen.
- GitLab website addresses entered in native tracker and manual credential setup now resolve to the API endpoint required for ticket operations.
- Fleet Services now distinguishes the desktop-managed agent from an agent attached from another installation.
- Fleet Jira connection guidance now directs members to an administrator when adding a ticket source requires one.
- A failed Claude Code or Codex detection now reports that provider's error while preserving the other provider's connection state.
- Cancelling an OAuth sign-in no longer lets an in-flight status check show a connected result or send a duplicate cancellation.
- Fleet projects can now attach or change a local folder through the native picker, and opening a new folder can create a Fleet-backed project for free prompts.
- A selected Fleet connection remains identifiable when its local agent gateway fails; refreshing after reconnection restores the signed-in profile in native Settings.
- An unavailable Fleet backend now leaves the selected mode, agent log and connection error visible; refreshing the status no longer restarts the retrying agent.
- A failed first Local or Fleet startup now leaves both service logs accessible in Settings.
- Folderless projects now offer the native folder picker directly on their board, and a board refresh error no longer follows users into the prompt.
- Workspace headers no longer repeat navigation actions for the already open project settings or blank prompt.
- Switching projects no longer shows the previous project's tickets or search filter while loading; Local boards keep their correct controls and let users retry a failed context read.
- Finished ticket workflows now show their result and history without disabled launch controls or irrelevant provider setup warnings.
- Historical ticket executions no longer show a disabled stop action; active executions identify an already requested stop.
- Switching projects clears the previous project's unsent prompt and prevents a delayed conversation creation from reopening that project.
- Opening a ticket workflow no longer gets stuck on loading when a previous request is invalidated during navigation or React remounting.
- Ticket cards now label their workflow shortcut as navigation and use an open arrow, including for completed tickets.
- In an active Codex conversation, model choices now follow the signed-in CLI's live catalogue while keeping the current model visible.
- Keep waiting-question controls visible in narrow conversation layouts, while the transcript and response area handle their own scrolling.
- Keep execution launch form styles scoped to the launch panel so a conversation opened from execution history retains its own composer layout.
- Identify retained tracker tickets as coming from the previous successful read after a refresh fails.
- Fleet attachment can no longer replace an already selected Local or Fleet runtime without an explicit migration.
- Paired Fleet setup now verifies the backend account still matches the account that paired this desktop.
- The signed-in profile is populated immediately after pairing or attaching a Fleet agent.

- Saved OAuth client identifiers no longer reach the renderer; configuration fields stay empty and masked, retaining saved credentials when left blank.
- Jira OAuth now recognizes tracker sources before they are linked to a project.
- Jira desktop sign-in now uses the Go backend’s fixed callback on port 52963; native configuration consumes the backend callback instead of constructing it.


### Changed

- Show Claude and OpenAI brand marks in desktop engine settings and keep both connection controls visible at compact window sizes.
- Show profile, engine, model and mode labels beside the free-prompt composer controls.
- Local tracker setup now shows personal-token access first and places the retained local OAuth application flow in a secondary disclosure; Fleet keeps backend-managed OAuth first.
- Keep section names readable on one line in the desktop settings index at compact window widths.
- Tracker boards open with compact ticket cards, with a saved control to restore the detailed layout.
- Ticket columns can receive keyboard focus and scroll horizontally in narrow desktop windows.
- Large tracker boards now show tickets progressively within each status column; search still covers every ticket and active workflow tickets stay in the first visible batch.
- OAuth settings now identify disconnected accounts and offer reconnection instead of silently hiding them.
- Personal credential and ticket-source forms now keep separate provider and site drafts, so configuring one cannot silently change the other.
- Desktop packages and browser previews use Sectile's existing app icon instead of the Tauri starter icon.
- Tracker setup now follows each requested synchronization through to completion or failure, including the number of tickets read when available.
- Lay out project prompt controls in a compact two-row grid on narrow windows so model and mode guidance remains readable.
- Keep workflow executions on their owning ticket cards, with run history in the project executions view instead of a duplicate project-rail row.
- Hide project navigation while Settings is open and return to the previous workspace from the header.
- Show conversation turn readiness in project navigation instead of treating an idle provider session as an active response.
- Distinguish Fleet-backed settings and agent status from settings kept on the local workstation.
- Label prompt folder selection as a project-wide attachment so its scope is clear before adding it.
- Distinguish an empty remote tracker board from a local project with no tracker.
- Choose an existing project's local folder with the native picker instead of entering an absolute path manually.
- Match the Electron dark and light workspace surface palette and remove duplicate Tauri color overrides.
- Show attached workspace folders in the prompt context so users can see what the agent will use without opening the folder details.
- Project navigation hides the “Tableau” action while the board is already open.
- Local projects backed only by the synthetic local tracker now show a prompt-ready empty state instead of remote tracker filters and read-only tracker status; the duplicate prompt button has been removed.
- Compact the project prompt on short windows so its launch controls, prompt suggestions and keyboard hint stay visible together.
- Show ticket refresh only on the board, and keep execution refresh beside execution monitoring.

- The connection status distinguishes the Fleet backend from the locally running agent.

- Simplified project configuration to show the working directory and default conversation profile first, with profile management and advanced settings in collapsible sections that retain drafts.
- Refined workspace spacing, conversation typography and the prompt composer to follow Sectile Desktop, with compact model/mode controls, Enter-to-send and non-executing prompt suggestions.
- Organized native configuration into AI engines, profile, accounts/trackers, projects and services; drafts survive navigation, installation details stay collapsed, and Projects can open a local folder directly.
- Made settings accessible from the header even with the sidebar hidden, and improved conversation navigation with cached prompt titles and provider/model metadata.

### Added

- Re-associate an unbound conversation history with the selected runtime before resuming it.
- Fleet pairing now enables native profile, personal tracker access, OAuth account connection and project setup through the selected backend; server-wide OAuth apps and shared trackers remain administrator-managed.

- Resize the project sidebar with the mouse or keyboard and keep its width after restarting the desktop.
- Open actions, projects and recent executions from a searchable command palette with Cmd/Ctrl+K.
- Local desktop now runs one Go companion for the backend and execution agent. Existing managed installations are upgraded through a verified private backup, with prior conversations, tracker setup and Jira OAuth retained.
- Add project-scoped folders directly from new prompts and existing conversations, inspect actual run context, detach associations, and configure repository and specification-folder roles natively.
- Guide tracker tickets through their next workflow skill or configured pickup chain, with fresh-state launch checks, server-confirmed skill results, PR access and explicit review declarations.
- Group the same tracker tickets by remote status or agentic workflow stage, and inspect run-bound Git diffs with folder selection and workstation editor access.
- Prepare skills and MCP explicitly from AI engine settings, with consent before replacing a mismatched existing MCP entry.
- Edit provider permission preferences explicitly and check the engine-reported MCP status from a conversation without sending a prompt.
- Configure named orchestration agents, Markdown policy files, shared concurrency and workflow policy in the desktop; supervise saved agents and stop individual task executions.

- Change a project’s local folder from its configuration with the native folder picker; the displayed path updates immediately and existing conversations keep their working directory.

- Projects without a local folder offer a native folder picker before opening a new prompt, associating the folder with the existing project without requiring Git or a tracker.

- Configure named Claude Code and Codex conversation profiles per project, with reusable model/mode choices and a default for new prompts.
- Automatically name free conversations from their first message and rename them locally, including saved read-only histories; titles survive agent restarts.

- Choose the model and Plan/execution mode before a first prompt or the next conversation message for Claude Code and Codex, with configured/live catalogues and custom model IDs.

- Answer Claude Code and Codex interactive questions directly in conversations with single/multiple choices, custom text, and masked Codex secret inputs; skipping remains available.

- Conversation replies follow incoming output automatically, pause when scrolling up, and offer a return-to-bottom button with the composer kept visible.
- Assistant Markdown rendering with readable tables, quotes, inline code and copyable fenced code blocks.
- Collapsible tool groups pair calls with their results and summarize completion or errors, including out-of-order results.

- Open a local folder as a project and start a free prompt without tracker setup or Git initialization; reopening a mapped folder selects its existing project.

- Detect installed Claude Code and Codex CLIs in desktop settings, check their sign-in status and open official login flows with automatic status refresh.

- Experimental project free prompts through CopilotKit headless, with Go-owned execution, conversation history, tool details, explicit one-time approvals and turn interruption.

- Connect GitHub.com, GitLab.com and Jira Cloud accounts through the system browser from native settings after one-time OAuth application configuration; completed grants are saved locally without copying personal tokens.
- OAuth progress, cancellation and expiry feedback; manual tokens are now an advanced option.

- Native setup for a local profile, personal GitHub/GitLab/Jira access, tracker sources, projects and repository associations.
- Isolated Go services start automatically after activation and reconnect when the desktop reopens; running tasks continue after closing the desktop.
- Tracker synchronization from native settings, with separate recovery for project creation and repository mapping.

- Project sidebar with direct navigation between boards, task launch forms, orchestration and execution details.
- Remembered system, light and dark appearance with matching console colors.
- Shared execution monitoring across views, with output streaming only for the active console.

- Tauri desktop shell with a searchable, read-only board of synchronized remote tracker tickets.
- Project discovery through the existing local Sectile agent and connection diagnostics.

- Start remote tracker tasks using project skills and execution modes from the supervisor.
- Monitor local executions and their live console output, and stop individual runs with confirmed cancellation feedback.
- Launch saved orchestration agents and inspect launched/skipped backlog tickets.

### Changed

- Make Local and Fleet runtime selection explicit; Fleet supports one-time-code pairing or deliberate attachment of an existing workstation agent, stores credentials in Tauri's private data directory, and never falls back to the shared Electron connection file.

- Keep agent connectivity in one labelled status location and remove the unrelated global project refresh control.
- Project conversations now use a lightweight React controller instead of CopilotKit, retaining prompts, tool details, approvals, interruption and history while reducing the lazy conversation bundle by about 99%.

- Removed workspace tabs in favor of one main view controlled from the sidebar. Navigation leaves executions running.

### Fixed

- Isolated the local companion's Claude Code and Codex MCP connection per launch, preserving Electron's shared registrations and displaying Codex MCP consent in conversations.

- Save conversation messages, tool calls/results and streaming replies incrementally before display; agent crash recovery keeps unfinished text labeled as an interrupted reply and reports local persistence failures.

- Claude Code and Codex conversation tools terminate when their owning Go agent crashes, including abrupt termination; provider supervision also reaches tools in separate Unix sessions.

- Resume free Claude Code and Codex conversations after restarting the local agent, preserving history, provider context, model and mode without replaying previous turns.

- Codex interruption now stops the dedicated local provider process tree before returning idle; the next message resumes the same Codex thread.
