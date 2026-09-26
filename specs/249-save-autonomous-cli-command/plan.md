# Plan — #249 Editing a project drops its autonomous CLI command

Spec: [`spec.md`](spec.md).

## Stack and touched areas

- Go server, SQLite store: `internal/db/db.go` (`UpdateProjectAs`).
- React web UI (TypeScript, tests with `node --test`): `web/src/components/ProjectModal.tsx`,
  `web/src/lib/commandTemplate.ts`.
- No migration: the `ai_command_template_autonomous` column is already read and written.
- No change to `internal/models` (the request already has
  `AICommandTemplateAutonomous *string`), the handlers, the agent or the desktop app.

## Data contract

`PUT /api/projects/{id}` body, unchanged shape:

| Field | Absent (`nil`) | `""` | Non-empty |
| --- | --- | --- | --- |
| `aiCommandTemplate` | keep stored | clear | store (trimmed) |
| `aiCommandTemplateAutonomous` | keep stored | clear | store (trimmed) |

`omitempty` on a `*string` only drops `nil`, so an empty string reaches the server.

## Design

1. **Server.** In `UpdateProjectAs`, next to the interactive command, apply
   `req.AICommandTemplateAutonomous` when non-nil. Both values are trimmed, the way
   `CreateProject` trims them, so a padded value is stored the same whether it
   was saved at creation or on edit.
2. **Web payload.** A pure helper `projectAgentCommands(useCustomAgent, interactive, autonomous)`
   in `web/src/lib/commandTemplate.ts` returns both fields as trimmed strings, empty when
   "custom agent" is off. `ProjectModal` spreads it into the payload, replacing the two
   `undefined`-producing ternaries. Putting it in `lib` makes it testable without a
   component harness, which this project does not have.
3. **Web toggle.** `hasCustomAgent` also looks at `aiCommandTemplateAutonomous`; otherwise a
   project storing only an autonomous command would open with the toggle off and the
   next save would now clear it (FR-5).

### Rejected alternatives

- Sending `null` to clear: Go decodes `null` into a `nil` pointer, indistinguishable
  from "absent". The empty string is what `aiModel` already uses.
- A desktop change: not needed per the clarification; the desktop reads the server value.

## Target files

- `internal/db/db.go`
- `internal/db/projectcommands_test.go` (new)
- `web/src/lib/commandTemplate.ts`
- `web/src/components/ProjectModal.tsx`
- `web/tests/commandTemplate.test.mjs`
- `CHANGELOG.md` — `Fixed` entry under `[Unreleased]`.

## Test plan

- Go: create a project with both commands; update the autonomous only → persisted,
  interactive intact; update with neither field → both intact; update with both empty
  → both cleared; the agent config for the project carries the saved autonomous command.
- Web: `projectAgentCommands` returns trimmed values when on, empty strings for a
  blank field, and two empty strings when "custom agent" is off.
- `go build ./...`, `go vet ./...`, `go test ./internal/db/...` (under WSL on Windows),
  `cd web && npm test` and the web type-check/build.
