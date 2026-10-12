# Plan #821 - Choose the target sprint when closing a sprint

## Stack

Go server (`internal/models`, `internal/db`), React web client
(`web/src/components/SprintTimelineView.tsx`, `web/src/lib`), French and
English locale catalogs. No migration, no new tracker capability:
`TicketingSystem.SetSprint` already moves work items into any sprint id.

## Data contract

`models.SprintPatch` gains:

```go
// MoveOpenToSprint names the sprint a "sprint" MoveOpenTo moves the work to.
MoveOpenToSprint *string `json:"moveOpenToSprint,omitempty"`
```

`moveOpenTo` accepts `next`, `backlog` or `sprint`. A separate field keeps a
sprint id from ever being read as a keyword. The web type follows:
`moveOpenTo?: 'next' | 'backlog' | 'sprint'`, `moveOpenToSprint?: string`.

## Server

`internal/db/sprints.go`:

- `UpdateProjectSprint` passes the patch's sprint id to `moveOpenWork`.
- `moveOpenWork` gains a `sprint` case: look the id up with `findSprint` in
  the project's sprints, refuse an empty id, an unknown one, the closing
  sprint and a closed one, with a French message as the existing ones, before
  the tracker is called. The rest of the function (tracker move, then local
  relink) is unchanged.
- `MoveOpenTo` without a close stays refused.

## Web

`web/src/lib/sprints.ts`:

- `nextSprintAfter` follows the server's `nextSprint`: when the closing sprint
  is undated, or no dated sprint follows, the first undated `future` sprint in
  board order is next.
- New `closeTargets(sprints, closing)`: the sprints that are not closed and
  are not the closing one, dated ones by start date, then undated ones in
  board order.

`web/src/lib/sprintApi.ts`: the `SprintPatch` type above.

`web/src/components/SprintTimelineView.tsx`:

- Destination state stays `'next' | 'backlog' | 'keep'`, `next` now meaning
  "a sprint"; a new state holds the chosen target's key (`id || name`).
- `handleStartCloseSprint` preselects `nextSprintAfter(...)`, else the first
  of `closeTargets(...)`; selects `backlog` when the list is empty.
- `handleConfirmCloseSprint`, tracker path: sends
  `{ state: 'closed', moveOpenTo: 'sprint', moveOpenToSprint: target.id }`
  when the destination is a sprint; the sprint to start is the target, or the
  next sprint by start date for backlog/keep.
- Local path: moves unfinished tasks to the target with `setTasksSprint`, and
  activates the target (or the next sprint by start date) by identity instead
  of `index + 1`.
- Dialog: the first option holds a `<select>` of `closeTargets`; the start
  checkbox names the sprint it applies to and is shown only when that sprint
  is `future`.

`web/src/locales/sprints.ts` (fr and en): `toNext` becomes "Move to a sprint
(recommended)" / "Transférer vers un sprint (Recommandé)", a label for the
select, `activateNextHint` speaks of the chosen sprint. The interface already
speaks French and English; both catalogs change together.

## Tests

- `internal/db/sprints_test.go`: closing to a chosen sprint (work lands in
  it, not in the next one); refusal of an unknown, a closed, the closing
  sprint and a missing id, each with no tracker call and the sprint left open.
- `web/tests/sprintManagement.test.mjs`: `nextSprintAfter` undated fallback;
  `closeTargets` content and order.
- Existing suites: `go test ./internal/db/...`, `node --test web/tests`,
  `tsc`, `oxlint`.

## Target files

- `internal/models/models.go`
- `internal/db/sprints.go`, `internal/db/sprints_test.go`
- `web/src/lib/sprints.ts`, `web/src/lib/sprintApi.ts`
- `web/src/components/SprintTimelineView.tsx`
- `web/src/locales/sprints.ts`
- `web/tests/sprintManagement.test.mjs`
- `CHANGELOG.md`

## Rejected alternatives

- Accepting a sprint id in place of `next`: an id that reads like a keyword
  would be ambiguous.
- A single dropdown holding every destination: rejected by the owner (round 2).
