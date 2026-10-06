# Plan #680 - Map the epic priority and quarter to Jira custom fields

Behaviour: `spec.md`. This file holds the implementation choices. It follows
the part C plan of `specs/635-epic-axes-tracker-fields/plan.md`, corrected by
`docs/clarifications/680.md`.

## Stack and constraints

- Server: Go. `internal/models` (types and pure deductions), `internal/db`
  (storage, pushes, import, pending pushes), `internal/tracker` (optional
  interface), `internal/trackerapi` (Jira adapter), `internal/handlers`
  (HTTP).
- Web: React and TypeScript, `web/src/components/ProjectModal.tsx` (Tracker
  tab), `web/src/types/index.ts`, `web/src/context/AppContext.tsx`,
  `web/src/locales/projectSettings.ts`, a pure helper in `web/src/lib`.
- Migration 48, `projects.epic_axis_fields`, `TEXT NOT NULL DEFAULT '{}'`, in
  `internal/db/migrations.go` only. Every rewind helper that drops
  `priority_mapping` drops it too.
- Server messages added by this change are in English (owner's rule since
  #735); web strings in French and English.
- The repository is public: synthetic field ids (`cf-epic-priority`), never
  the `customfield_<n>` pattern, and synthetic option labels.

## Data

`internal/models/epicaxisfields.go` (new):

```go
// EpicAxisFields maps the epic priority and quarter to Jira custom fields
// (#680). A nil axis is written and read as labels only.
type EpicAxisFields struct {
    Priority *EpicAxisField `json:"priority,omitempty"`
    Quarter  *EpicAxisField `json:"quarter,omitempty"`
}

type EpicAxisField struct {
    ID      string            `json:"id"`
    Name    string            `json:"name"`
    Kind    string            `json:"kind"`              // "select" or "cascade"
    Options map[string]string `json:"options,omitempty"` // axis value -> option path
    Manual  []string          `json:"manual,omitempty"`  // hand-set axis values
}

// EpicFieldCandidate is a closed-list custom field of an epic's edit screen.
type EpicFieldCandidate struct {
    ID      string            `json:"id"`
    Name    string            `json:"name"`
    Kind    string            `json:"kind"`
    Options []EpicFieldOption `json:"options"`
}

type EpicFieldOption struct {
    ID       string            `json:"id"`
    Value    string            `json:"value"`
    Children []EpicFieldOption `json:"children,omitempty"`
}
```

`Project.EpicAxisFields EpicAxisFields` (`json:"epicAxisFields"`) and
`UpdateProjectRequest.EpicAxisFields *EpicAxisFields`. `Task` gains
`AxisFieldValues map[string]string` with `json:"-"`: the raw option path of
each mapped field an epic search returned, never stored.

Pure functions in the same file, table-tested on synthetic options:

- `DeduceEpicAxisOptions(axis string, candidate EpicFieldCandidate) map[string]string`:
  `axis` is `"priority"` or `"quarter"`; walks leaves (flat options, or
  cascade children with `parent/child` paths); first match wins.
- `FillEpicAxisOptions(field EpicAxisField, deduced map[string]string) (EpicAxisField, bool)`:
  adds the deduced lines whose value has no line and is not hand-set; reports
  whether anything was added.
- `(f *EpicAxisField) OptionFor(value string) string` and
  `(f *EpicAxisField) ValueOf(path string) string` (reverse map).
- Quarter deduction uses a local regex
  (`^(\d{4})\s*(?:-\s*)?q([1-4])$` on the trimmed, lower-cased label, so
  `2026 - Q4`, `2026-Q4`, `2026 Q4`) and a cascade child `^q([1-4])$` under a
  parent `^\d{4}$`. The value is written `2026-Q4`, the `NormalizeQuarter`
  form.

`internal/db/epicaxisfields.go` (new):

- `parseEpicAxisFields(raw string) models.EpicAxisFields`.
- `editEpicAxisFields(stored, sent models.EpicAxisFields) (models.EpicAxisFields, error)`:
  validates (FR-4) and wraps `ErrInvalidEpicAxisFields`. Per axis: nil sent
  removes; another field id replaces the entry and keeps the `Manual` it
  carries; the same field id compares each value of the union of both maps
  and marks the changed, added or removed ones hand-set. Quarter keys are
  normalized with `NormalizeQuarter`, priority keys with
  `NormalizeEpicPriority`.
- `storeLearnedQuarterOptions(projectID, fieldID, deduced)`: under `d.mu`,
  re-reads the row, applies `FillEpicAxisOptions` when the stored quarter
  field still has that id, writes the column. Same pattern as
  `RefreshPriorityMapping`: merged against the row as it is now.
- `EpicAxisFieldCandidates(ctx, projectID)`: picks the first local macro of
  the project that is its own epic (not a milestone, not foreign, not
  closed first), asks the adapter, returns the candidates with their
  deduced maps; `noEpic` when there is none.

The project SELECTs, INSERT and UPDATE in `db.go` carry the column next to
`priority_mapping`; `UpdateProject` applies `editEpicAxisFields` when
`req.EpicAxisFields` is set; the handler's 400 list gains the new error.

## Tracker

`internal/tracker/ticketing.go`:

```go
// EpicAxisFieldManager is implemented by an adapter whose epics can carry
// the epic axes in custom fields (Jira, #680).
type EpicAxisFieldManager interface {
    EpicAxisFieldCandidates(ctx context.Context, project *models.Project, epicKey string) ([]models.EpicFieldCandidate, error)
    SetEpicAxisField(ctx context.Context, project *models.Project, epicKey string, field models.EpicAxisField, optionPath string) error
}
```

`internal/trackerapi/jira_epicfields.go` (new):

- `EpicAxisFieldCandidates`: `GET /rest/api/3/issue/{key}/editmeta`, keeps
  the fields whose `schema.custom` ends with `:select` or
  `:cascadingselect`, decodes `allowedValues` (`id`, `value`, and for a
  cascade `children`).
- `SetEpicAxisField`: `PUT /rest/api/3/issue/{key}` with
  `fields[id] = {"id": optionId}` for a select,
  `{"id": parentId, "child": {"id": childId}}` for a cascade, `null` for an
  empty path.
- `epicAxisFieldIDs(project)` lists the mapped ids; `ListEpics` appends them
  to the search field list and, only when non-empty, decodes each raw issue's
  `fields[id]` into `Task.AxisFieldValues` (select: `id`; cascade:
  `id/child.id`; null or absent: no entry). `search` takes the extra field
  list as an argument; every other caller passes nil, so their request is
  unchanged.
- `roadmapProjectView` copies the project, so the epics of a declared
  roadmap project are searched with this project's mapped fields.

## Domain

`internal/db/macroaxes.go`:

- `PushMacroPriorityLabel` and `PushMacroQuarterLabel` keep their label
  write; on success, when the project maps that axis, they call
  `d.pushEpicAxisField(ctx, proj, key, axis, value)` and append its note to
  the activity output.
- `pushEpicAxisField`:
  1. resolves the adapter through `macroTracker` (same refusals, so
     `RoadmapAxisWrites` governs a foreign epic exactly as for the label);
     an adapter without `EpicAxisFieldManager` returns no note;
  2. empty value: clears the field;
  3. quarter missing from the map: reads the candidates of that epic, finds
     the mapped field by id (absent: note "field X is not on the edit screen
     of KEY"), deduces, stores the learned lines, retries the lookup;
  4. still no option: note "VALUE has no option in field X, field left as
     it is";
  5. writes the field; a failure returns
     `label written but field X not: <err>`.

`internal/db/macrohorizons.go`:

- `ImportMacroHorizons`: `epicAxisValue(field, epic)` gives the field's value
  through the reverse map; the priority and quarter use it first, else the
  label.
- `pendingAxisPushes`: an axis is late when its label disagrees (today) or
  when the field is mapped, the decided value has an option, and the epic's
  raw field path differs from it.

## HTTP

`GET /api/projects/{id}/epic-axis-fields` in `HandleProjectDetail`, beside
the priority-mapping refresh: answers
`{"epicKey": "...", "noEpic": false, "candidates": [{...candidate, "deduced": {"priority": {...}, "quarter": {...}}}]}`.
400 with the message on a tracker error; `noEpic: true` and no tracker
request when the project has no epic of its own.

## Web

- `web/src/types/index.ts`: `EpicAxisField`, `EpicAxisFields`,
  `EpicFieldCandidate`, `Project.epicAxisFields`.
- `web/src/lib/epicAxisFields.ts` (new, pure): `optionChoices(candidate)`
  (flat list of `{path, label}`, `Parent / Child` for a cascade),
  `pickField(candidate, axis, deduced)` (a fresh entry with the deduced map),
  `setOption(field, value, path)`. Tested with `node --test`.
- `AppContext.tsx`: `loadEpicAxisFieldCandidates(projectId)`.
- `ProjectModal.tsx`, Tracker tab, Jira only, under the axis prefixes: a
  "Champs Jira des axes" block with a "Lire les champs d'une épic" button;
  per axis a field select (none, the candidates, the stored field when not
  among them), then the map: four rows for the priority, the mapped quarters
  plus an "add a quarter" row for the quarter, each with an option select.
  Without loaded candidates the stored map is shown read-only. Saved with the
  project.
- `web/src/locales/projectSettings.ts`: French and English strings.

### Implementation notes

- The #680 clarification placed the settings in a "Roadmap tab"; the project
  modal has none (Roadmap is an optional view card), and the axis prefixes
  live in the Tracker tab under a "Roadmap:" title. The fields block sits
  right below them, which is what the #635 specification asked.
- The modal sends `epicAxisFields` only once the person edited it: a save of
  any other setting carrying a stale copy would read every option learned at
  write time meanwhile as cleared by hand.

## Target files

| File | Change |
| --- | --- |
| `internal/models/epicaxisfields.go` (+ test) | types, deductions, fill |
| `internal/models/models.go` | `Project`, `UpdateProjectRequest`, `Task.AxisFieldValues` |
| `internal/db/migrations.go` | migration 48 |
| `internal/db/db.go` | project statements, `UpdateProject` |
| `internal/db/epicaxisfields.go` (+ test) | parse, edit, learn, candidates |
| `internal/db/macroaxes.go`, `macrohorizons.go` (+ tests) | field write, read-back, pending |
| rewind helpers in `internal/db/*_test.go` | drop the column |
| `internal/tracker/ticketing.go` | `EpicAxisFieldManager` |
| `internal/trackerapi/jira_epicfields.go` (+ test), `jira.go` | candidates, write, search fields |
| `internal/handlers/handlers.go` (+ test) | candidates endpoint, 400 mapping |
| `web/src/types/index.ts`, `web/src/lib/epicAxisFields.ts`, `web/tests/epicAxisFields.test.mjs` | types and helper |
| `web/src/context/AppContext.tsx`, `web/src/components/ProjectModal.tsx`, `web/src/locales/projectSettings.ts` | settings UI |
| `CHANGELOG.md` | `Unreleased` / `Added` line |
| `docs/` | the roadmap doc section on axes, if one covers the prefixes |

## Rejected alternatives

- **Deducing on the client.** The write path needs the same deduction for a
  missing quarter; one Go implementation serves both, and the endpoint
  returns its result.
- **A new generic "custom fields" setting.** The axes are the only values
  Sectile owns on an epic; a generic setting invites field ids for anything.
- **Reading the field with a second request per epic.** The mapped ids join
  the one search the import already makes.
- **Writing the field inside the label `UpdateIssue`.** It would change the
  shared `UpdateIssueRequest` for one adapter and make a field refusal undo
  the label write; two requests keep the label as today and name the field's
  failure on its own.
- **A field id or option name in the code.** Forbidden by the guardrails.

## Test plan

- `internal/models/epicaxisfields_test.go`: deductions (exact level, spaces,
  case, `P1 - High` refused, the three quarter formats, cascade year/quarter,
  cascade full-form child with a mismatching year refused, first match wins),
  fill (holes only, hand-set skipped, cleared hand-set value not refilled).
- `internal/db/epicaxisfields_test.go`: edit validation and hand-set
  tracking, field switch, removal, the learned-quarter store keeping a
  concurrent hand edit; candidates with no epic (no tracker request).
- `internal/db/macroaxes_test.go` / `macrohorizons_test.go` with a fake
  tracker implementing `EpicAxisFieldManager`: write with option, missing
  quarter learned and stored, no option named, field absent, clear, field
  failure after label; import field first, unmapped value ignored; pending on
  disagreement; no field: no manager call.
- `internal/trackerapi/jira_epicfields_test.go` with an `httptest` site and a
  request recorder: candidates by schema suffix, select and cascade payloads,
  clear payload, `ListEpics` field list with and without mapped fields (no
  extra field when none) and decoding.
- `internal/handlers`: the endpoint with no epic, and with candidates.
- Migrations: `go test ./internal/db/...` on SQLite and, when available,
  PostgreSQL.
- Web: `node --test web/tests`, `npx tsc --noEmit`, `npx oxlint`.
- Gates: `go vet ./...`, `gofmt -l`, `go test ./...`.
