# Project memory

- The user explicitly removed tabs; do not reintroduce them.
- Electron source is a behavioral reference, not a mandate to reproduce every
  control. See docs/desktop-parity-plan.md for the reviewed execution plan.
- A primary working directory is not the entire workspace. Project attachments,
  repository mappings and Macro/Issue specification roots already exist in Go.
  /desktop/run-folder persists project-wide and takes effect at the next message
  for free Claude/Codex conversations. Never imply message-private attachments.
- Existing conversations retain their run.directory after project remapping.
  Present that directory rather than the current project path in their context.
- A connected CLI is not evidence of installed skills or working MCP. Active
  Electron settings use explicit independent operations, not its retired wizard.
- A terminated process is not proof of completed skill/validated tracker stage.
  Next actions must use fresh task, project policy and /desktop/run-result data.
- Plan UX review completed before implementation, as explicitly requested.

- The independent reviewer found provider preparation missing from the original
  plan. Explicit skills/MCP operations and replacement consent were implemented;
  do not automatically register this companion over another desktop's entry.
- Workflow stage grouping is a presentation over the same remote tickets. Unknown
  stages stay unknown. An implemented ticket without a PR returns to the PR
  creation owner (specify or implement), not a generic create_pr command.
- Keep workflow and terminal imports lazy. A static RunConsole import folded
  xterm into the main entry during implementation; a production build caught it.
- Parity implementation is present; see the delivery record in the reviewed plan.
  Real multi-folder Claude/Codex reads passed before/after Go restart, preserving
  the original run directories and context. No tracker or MCP configuration was
  modified during provider verification. Native visual and picker checks passed
  after unlock: cancel, project attachment, restart persistence, original run path
  and detach without deleting the directory. Provider registrations remain intact.
- An attachment is asynchronous: disable sending until native attachment confirms,
  otherwise the first turn can miss it. Keep launch errors independent from polling
  errors so a metadata refresh cannot hide an ambiguous dispatch failure.
- Canonical `pickup` must use autonomous mode even from the advanced chooser.
  Server capacity is authoritative; local execution counts cannot replace it.
- A non-Git mapped folder is usable for conversations/specifications even when
  `/desktop/project` reports no Git configuration. Fill its path from the mapped
  projects view without promoting `configured` to true.
- Local-only projects may still have a synthetic `local` tracker row. The Go
  desktop board projection must carry tracker providers; classify as local only
  when there are no external providers. For old agents, the legacy
  `issueTracker=local` fallback is safe only for a single selected tracker.
- Companion MCP uses process-scoped `sectile_companion` configuration (ADR 0011).
  Never repair its connection by replacing Electron's shared `sectile` entry.
  Codex deep-merges configuration tables; changing a shared entry from STDIO to
  HTTP retains incompatible fields. Use a distinct server and disable the old
  entry in the companion process only. MCP consent arrives as
  `mcpServer/elicitation/request`, not command-execution approval.

- Phase 0 approved 2026-10-29: separate Local companion, internal HTTP/WebSocket retained initially, and current OAuth must keep working (including reconnect/refresh). Runtime topology and tracker auth are independent. The Go companion implementation and Tauri Local/Fleet integration are in the dedicated branches `refactor/embedded-local-backend` and `feat/local-fleet-modes`; see docs/phase0-runtime-baseline.md and docs/sectile-v1-migration-plan.md for current qualification and remaining V1 gates.
