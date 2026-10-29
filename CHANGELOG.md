# Changelog

All notable changes are documented here using Keep a Changelog conventions.

## [Unreleased]

### Added

- Include MIT and third-party dependency notices in packaged Tauri applications, with Go notices checked against the selected Go source dependency graph.
- Restore the last valid project and workspace view on desktop launch, falling back to a project board when the saved conversation is no longer available.

### Fixed

- Identify retained tracker tickets as coming from the previous successful read after a refresh fails.
- Fleet attachment can no longer replace an already selected Local or Fleet runtime without an explicit migration.
- Paired Fleet setup now verifies the backend account still matches the account that paired this desktop.
- The signed-in profile is populated immediately after pairing or attaching a Fleet agent.

- Saved OAuth client identifiers no longer reach the renderer; configuration fields stay empty and masked, retaining saved credentials when left blank.
- Jira OAuth now recognizes tracker sources before they are linked to a project.
- Jira desktop sign-in now uses the Go backend’s fixed callback on port 52963; native configuration consumes the backend callback instead of constructing it.


### Changed

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
