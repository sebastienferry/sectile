# Desktop parity implementation plan

Status: Implemented and verified on 2026-10-29. Reviewed by an independent UX
subagent before implementation on 2026-10-28.

## Product constraints

Keep a fast standalone local desktop, native setup, no tabs, free project prompts
without mandatory tracker tickets, and tracker-authoritative task state. Go owns
execution, permissions, worktrees and transitions. Rust exposes narrow native
operations; React presents state. Do not create a second workflow engine.

## Reference findings

Electron distinguishes a primary working directory, mapped project repositories,
Macro/Issue specifications folders and additional attached folders. Attaching from
a conversation persists in project settings; conversational providers see it from
the next message. These attachments are not private message uploads. Git repositories
and plain folders have different execution/worktree behavior.

The ticket workflow has six stages: new, clarified, specified, implemented,
reviewed, finished. Stage labels, per-tracker board mappings and internal statuses
resolve its state. The next action also depends on PR presence and project policy.
A process exiting does not prove that its skill or tracker transition completed.
The pickup chain has a configured stopping stage rather than an unconditional
handoff. Orchestration evaluates saved Markdown policies on demand, under shared
capacity; disabling future launches does not stop existing work.

Provider sign-in alone does not establish skills/MCP readiness. Active Electron
settings expose independent installation/registration operations; the old startup
setup wizard is not the UI to reproduce.

## Reviewed delivery order

1. **Multiple folders in the prompt.** Show the actual execution directory and
   expandable folder context. Native add-folder in new/existing prompts, explicit
   project scope and next-message effect. Project configuration lists attachments
   and detaches without deleting files. Keep advanced repository/specification
   roles available progressively. Test cancellation, duplicates, unavailable
   folders, both providers and restart persistence.
2. **Provider preparation.** Extend native engine settings with existing Go
   skills, MCP and permission/readiness operations. Explicit actions only; no
   startup wizard, automatic installation or silent replacement of another
   desktop's configuration. Distinguish saved configuration from verified access.
3. **A complete ticket/skill journey.** Ticket details show remote status, stage,
   next skill and PR. Next-step, configured pickup chain and advanced skill choice
   reread fresh state before submission. Show waits, server-confirmed skill result
   and next action together; never advance a stage from logs or process exit.
4. **Stage presentation and review.** Regroup the same remote tickets by tracker
   status or workflow stage. Add run-folder-aware diffs, PR links and editor
   access. Keep manual stage declarations separate from skill execution.
5. **Native orchestrator configuration.** Saved named Markdown policies,
   preview/reload, enablement and shared concurrency; explicit evaluation,
   launched/skipped reasons, capacity, attention states and individual stops.

## Review changes adopted

- Folder context is a compact project/execution surface, not per-message chips
  with misleading local removal buttons.
- Keep one ticket collection with alternative grouping, not duplicate boards.
- Deliver next-step controls with result confirmation, not as separate unfinished
  launches. Name the pickup stopping point instead of promising a complete chain.
- Provider readiness precedes meaningful workflow validation.
- Read metadata currently discarded by the Tauri bridge (labels, PRs, tracker
  mappings, skills and chain policy) through a bounded safe project view.

## Validation

Each lot gets meaningful UI/domain/IPC coverage and the repository's required
checks. Exercise provider/folder behavior only in disposable local projects.
Use controlled fixtures or a test server for workflow writes; never mutate real
tracker tickets merely to test the interface. Keep existing conversation,
interruption, recovery and onboarding behavior. Do not add heavy dependencies.

## Delivery and verification record

All five implementation lots are present in the desktop codebase. The retained
entries remain project prompts, tickets, executions, orchestration and native
configuration; no tabs or additional runtime dependencies were added.

- Static/type checking, the production build, 81 UI tests, 8 domain tests and 21
  Rust tests pass. Coverage includes PR policy, tracker-specific stage mapping,
  stale/concurrent launch prevention, unmatched skill results, setup replacement
  consent, saved-only orchestration, folder scope and text-only diff rendering.
- Final audit also covers pending folder attachment before submission, retained
  launch errors during refresh, canonical pickup execution mode, bounded provider
  permission preferences, idle MCP status polling and server-reported capacity.
  Non-Git projects retain their mapped folder without claiming Git readiness.
- Real local configuration checks saved two inactive Markdown agent definitions,
  reloaded a changed preview, refused enablement with a missing policy file and
  saved workflow policy without starting work. Restart persistence is checked
  separately from tracker-backed execution behavior.
- After rebuilding and restarting the owned companion, those definitions and
  policy survived and no task started. Explicit MCP checks left both disposable
  conversation histories, models and modes unchanged. Claude reported connected;
  Codex correctly reported unknown after restart because no app-server session was
  open. No provider message was sent and no global registration was replaced.
- Real Claude Code and Codex sessions read sentinel files from two newly attached
  disposable folders, both before and after a graceful companion Go restart.
  Their original working directories, history prefix, model and mode were
  preserved. Restart added notices only and did not replay a tool automatically.
- Existing CLI MCP registrations were inspected but not changed. The local runtime
  currently has no remote tickets suitable for a live workflow test, so transition,
  skill and orchestration writes were verified against controlled test fixtures.
- Native verification passed after the Mac became accessible: the rebuilt app
  displays the compact composer context, original conversation working directory,
  engine preparation, persisted workflow policy and inactive orchestrator controls.
  Cancelling the native picker leaves context unchanged. Selecting a temporary
  folder adds a project attachment without changing the primary directory; it
  remains visible after a Go restart. Detaching removes its association while
  leaving the directory on disk. No real tracker or provider registration was
  changed by these UI checks.
- Main JavaScript entry is approximately 299 KB (90 KB gzip); terminal and Markdown
  payloads remain separately loaded. This preserves the lightweight controller.
