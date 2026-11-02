## Context and evidence

Live project context selects OpenSpec (`schema: spec-driven`). Worktree branch is `feat/869`; task is clarified with no PR recorded. The accepted round 4 explicitly distinguishes repository prompts from workstation engine configurations and includes creation, but not editing/deletion.

Existing integration points:
- `desktop/src/main.js`: project-row rendering, Project prompt menu, `openAgentConsole`.
- `desktop/src/engines.mjs`: existing engine labels, kept separate from profiles.
- `desktop/electron/preload.cjs`, `desktop/electron/main.cjs`: `launchConsole` and `launch-console` IPC to `POST /desktop/consoles`.
- `internal/agent/agent_desktop.go`: loopback route registration.
- `internal/agent/agent_console.go`: mapped-root resolution, session-only engine override, PTY and conversation admission.
- `internal/agent/agent_conversation.go`, `agent_codex_conversation.go`: Claude stream-json and Codex app-server sessions.
- `.agents/MEMORY.md`: instruction substitution must use the existing quoting-aware expansion; never introduce a second shell substitution path.

## Decisions

### Portable repository format

Store flat UTF-8 Markdown files at `<mapped-primary-checkout>/.sectile/agents/<id>.md`. The ID is also the display name: 1–64 ASCII lowercase letters/digits with single internal hyphens, starting and ending with a letter/digit. No front matter; the entire file is the supplemental prompt. Normalize CRLF to LF on creation; reject invalid UTF-8, NUL, whitespace-only text, and prompts exceeding 64 KiB encoded. Do not trim meaningful prompt whitespace. No recursive discovery and no discovery in attached context repositories. No files are created merely by listing.

This provider-independent directory avoids overloading `.agents/skills`, Claude subagents, or global engine profiles. Plain files are reviewable and portable through normal repository workflows. Provider-native registries and a database catalogue are rejected because they duplicate repository ownership or couple profiles to one engine. A new profile defaults to the no-profile state until the user creates/selects one; no default-profile setting is introduced.

### Filesystem ownership and safety

The local agent resolves `projectId` through the existing mapped-root resolver, never an arbitrary renderer path. Add a focused `agent_profiles.go` module. Profile requests carry IDs, not file paths. Reject symlinks/reparse-point escapes for the profile directory and file, including existing ancestors inside the checkout; resolve links at or above the mapped checkout path once, since that path is the user's own choice (for example `/tmp` on macOS); validate containment on every list/create/read, with a platform-safe no-follow create/open strategy that prevents check/use substitution. Do not follow an alternate Git worktree path supplied by the client. Readonly context repositories are never written.

Create exclusively and atomically: prepare a complete same-directory temporary file and publish without replacing an existing destination (or equivalent exclusive no-partial-file implementation), cleaning it on failure. Serialize agent-owned create/list operations; filesystem publication must still resist external concurrent writers. Use directory mode 0755 and file mode 0644 subject to umask. Never invoke Git. Duplicate creation returns conflict, including concurrent requests. Invalid files are reported as warnings and excluded; missing directory means an empty list, not an error.

### Local API and IPC contract

Add local authenticated Desktop routes (not remote server tracker APIs):
- `GET /desktop/agent-profiles?projectId=<id>` -> `{profiles:[{id,name}],warnings:[{id,message}]}`; sorted by ID.
- `POST /desktop/agent-profiles` body `{projectId,id,prompt}` -> 201 `{profile:{id,name}}` after successful persistence.
- Extend `POST /desktop/consoles` with optional `agentProfileId`; absence/empty preserves existing launches. Reject invalid IDs and resolve/read contents again before admitting a run.

Use bounded bodies (80 KiB for creation, retaining the existing small console-launch body), method checks and the existing Desktop authorization boundary. Responses: 400 invalid request, 404 missing selected profile/project, 409 unavailable checkout or duplicate destination, 422 unsupported profile delivery, and 500 filesystem failure. Preserve existing fetchConfig/upstream error handling. Return useful sanitized error messages without exposing prompt contents. The Desktop bridge exposes only list/create and optional profile ID on launch, not unrestricted filesystem access. Older agents without the route leave ordinary launches available with a capability notice and disable profile creation/selection.

### Instruction delivery and lifecycle

Read and validate the selected file once at admission, snapshot its text, ID and name in controlled-run/conversation state, and carry them in existing local persistence where necessary for restored conversations. Do not log prompt text or send it to the Sectile server. Keep run profile metadata separate from EngineID/EngineName. A new session launched from an old one may preselect the profile only if still available; launching still rereads it. Existing active sessions never reread modified profile files.

Claude: append the snapshot via `--append-system-prompt` to built-in PTY argv and to `claudeConversationCommand` on initial and resumed processes. Local `claude --help` confirms this additive flag; do not use replacement `--system-prompt` or disable CLAUDE.md discovery. Use argv for direct processes and the existing quoting helpers for PTY command construction.

Codex: use the session-level `developer_instructions` configuration override for built-in PTY launch (`-c` with correctly serialized TOML string) and `developerInstructions` on app-server `thread/start`/`thread/resume`. Merge any preexisting developer instructions rather than replacing them; do not override baseInstructions or AGENTS.md discovery. Preserve the snapshot when constructing collaboration-mode settings, which currently explicitly send `developer_instructions: nil`. Verify the installed app-server schema and configuration composition during implementation before enabling this path; use a capability error for versions unable to honor additive delivery. Test the actual constructed thread and turn requests, not only stored state.

Profile support is defined for built-in Claude and Codex engines only, in both current views. A selected engine with a custom command template or another provider returns 422 for a nonempty profile selection. This avoids pretending an arbitrary shell command supports instruction delivery; ordinary no-profile launches retain existing behavior. No automatic fallback may discard a selected profile. Do not submit the snapshot as an initial user request or paste it into an interactive terminal.

All new child commands use `agentexec.Hidden` except existing user-requested visible terminals; retain Windows CREATE_NO_WINDOW assertions for conversation commands. Do not change permission, sandbox or approval settings to load instructions.

### Desktop interaction

Place a project-specific plus button with the row actions, visible on hover and keyboard focus and usable on touch. Stop row-selection/collapse propagation. Disable with an explanation when no path is mapped. Keep the global PROJECTS plus and context-menu entry.

Extend the existing dialog with an independent Agent profile selector, initially None, and a Create profile action opening name and multiline agent-instructions fields. Clearly distinguish these persistent reusable instructions from a user request. Preserve engine default preselection and console view preference. Show loading, empty, warnings and error states separately. Prevent duplicate save/launch submission; retain draft values on save errors. A successful save refreshes the list and selects the saved profile without launching. Canceling restores the launch dialog and focus. Escape and keyboard focus follow existing dialog conventions. Late async responses must not update a closed dialog or another project.

## Documentation and trade-offs

Update Desktop usage documentation (README if it describes the launch feature), add a short profile-format guide under docs, and one Added changelog entry during implementation. Record the repository-owned portable format in an ADR during implementation because it establishes a lasting configuration convention. No infrastructure edits or migrations.

## Verification and open points

No unresolved product decisions. Provider schema/version compatibility is an implementation verification gate, not permission to silently omit instructions. Use fake provider executables/protocol fixtures to prove safe delivery, persistence and instruction composition; manually smoke-test real supported versions with repository instructions and a distinctive profile. Profile prompts go to the selected AI provider like other session instructions, never to another session.

Specification artifacts are not ignored (`git check-ignore -q` returns 1). The owner authorized committing the specification and creating a draft PR, overriding the specification-stage PR restriction and authorizing a retry of the previously denied Git write. Publication still requires successful remote synchronization and PR creation checks. No production code is changed here.

## Implementation verification notes — 2026-10-27

The existing local run store restores read-only history and cannot resume provider
conversations after daemon restart. Consequently prompt snapshots remain in the
controlled run's memory; only profile identity is persisted. Active Claude and
Codex process reconnects retain the snapshot. This preserves instruction isolation
without storing unused prompt copies. ADR 0061 documents this lifecycle.

Codex interface checks use the inspected 0.162.1 schema and conservatively require
CLI 0.162 or newer. Admission reads effective developer instructions through
config/read before composing a session override. Version 0.164.0 was also verified.

The owner delegated authenticated Claude verification to #876 on the professional
workstation. It remains an explicit follow-up, not a successful local smoke test.
The local real Codex conversation and native CLI override/no-profile checks pass.
The interactive TUI reached the waiting composer, but its inference harness was
removed after it accidentally activated the provider's updater. No provider-menu
automation is retained in the tests.

Windows device names (con, prn, aux, nul, com1–com9, lpt1–lpt9) are rejected
consistently on every platform. They fit the original ASCII grammar but cannot
name regular Markdown files through Windows filesystem APIs. This technical
validation refinement keeps repository profiles portable instead of failing only
when a profile is shared with a Windows workstation.
