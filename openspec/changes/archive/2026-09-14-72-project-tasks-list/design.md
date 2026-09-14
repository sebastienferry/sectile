# Design

## Decisions
Add optional `startedAt` to the desktop run response. Capture UTC time when the execution command is successfully submitted to its PTY; keep `createdAt` as enqueue time. Never assign a start to queued/preparing runs or failed launches. Retain the start through terminal completion.

Extract a pure desktop ordering helper. Rank running/preparing as active (0), queued (1), and final/unknown states (2). Within rank, compare parsed instants descending: use valid `startedAt` for started states, otherwise valid `createdAt`; queued/preparing use creation time. Missing/invalid timestamps sort last. Break ties by task identity then run identity ascending.

Choose the first ordered execution as each task's representative, then sort task groups by those representatives. Filter archived runs before this operation. Do not mutate the source run list or the separate chronological history list. Keep selection keyed by execution ID across refreshes.

## Alternatives
Sorting API responses alone cannot correctly order aggregated task rows. Creation-only order does not meet the actual-start requirement after queue delays. Choosing the newest queued execution over an older running execution hides active work. Changing queue scheduling or history ordering is unnecessary.

## Validation
Go tests cover start timestamp capture/lifecycle and JSON exposure, including runs that never start. Pure JavaScript tests cover ranks, timestamp parsing/fallback, ties, and multiple runs. Desktop UI tests verify rendered order after refresh and retained selection/history. Run OpenSpec strict validation, Go build/vet/tests, desktop build/UI tests, and the repository's web tests/typecheck/lint. No architectural change requiring an ADR.
