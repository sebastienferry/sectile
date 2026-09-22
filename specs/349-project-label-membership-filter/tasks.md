# #349 — Implementation checklist

Ordered so the tree builds after every step. Behaviour: [`spec.md`](./spec.md).
Choices: [`plan.md`](./plan.md).

## 1. The setting

- [x] T1 — `internal/models/models.go`: add `ProjectLabel string \`json:"projectLabel"\`` to
      `Project` (next to `IssueTypes`), `ProjectLabel string` to `CreateProjectRequest`,
      `ProjectLabel *string` to `UpdateProjectRequest`.
- [x] T2 — `internal/models/projectlabel.go` (new): `NormalizeProjectLabel(raw string) (string, error)`
      — trim, accept empty, refuse inner whitespace with `project label cannot contain whitespace`.
- [x] T3 — `internal/models/projectlabel_test.go` (new): empty, trimmed, valid, inner space,
      inner tab. **Run `go test ./internal/models/...`.**
- [x] T4 — `internal/db/migrations.go`: migration 2 `projects.project_label`, the only place
      the column is declared (ADR 0021 froze the baseline, and it is the only form that
      reaches an existing PostgreSQL database). `internal/db/db.go`: the column in the two
      project SELECTs, the INSERT and the UPDATE, with its scan target and argument.
- [x] T5 — `internal/handlers`: call `models.NormalizeProjectLabel` on project create and
      update, returning HTTP 400 with the error message; store the normalised value.
- [x] T6 — `internal/db/migrations_test.go`: `forgetSchemaVersion` drops the post-baseline
      columns so a fixture really looks pre-versioning, and the two synthetic migrations are
      numbered from `latestVersion()` instead of colliding with version 2.
      **Run `go test ./internal/db/...`.**

## 2. The membership filter

- [x] T7 — `internal/db/projectlabel.go` (new): `escapeLike`, `addProjectLabel`, and
      `(*DB) membershipConditionUnsafe(projectID, userID string) (string, []interface{})` as
      specified in plan D3 — one query for the labelled projects in scope, one
      `(project_id NOT IN (id, slug) OR LOWER(labels) LIKE ? ESCAPE '\')` term each,
      `""` when none.
- [x] T8 — `internal/db/db.go`: `membershipAll bool` on `GetTasksForUser` and
      `GetTasksForUser`'s wrapper `GetTasks` (passing `false`); append the condition when it
      is false.
- [x] T9 — `internal/db/db.go`: same parameter on `GetTaskFacetsForUser` / `GetTaskFacets`;
      append the condition to `scopeSQL` so all facet queries inherit it.
- [x] T10 — Fix every other caller of the four functions in the repository.
      **Run `go build ./...`.**
- [x] T11 — `internal/handlers/handlers.go`: parse `membership` on `GET /api/tasks` and
      `GET /api/tasks/facets`; `all` widens, anything else (including absent) does not.
- [x] T12 — `internal/db/projectlabel_test.go` (new): labelled project returns only the
      carriers; a differently-cased label still matches; `team-alphabet` does not match
      `team-alpha`; a label containing `_` does not match a look-alike; an unlabelled
      project returns everything; `membershipAll` returns everything; the "all projects"
      scope mixes a labelled and an unlabelled project correctly; the facets follow the
      same scope. **Run `go test ./internal/db/...`.**

## 3. Creation stamps the label

- [x] T13 — `internal/db/db.go` `CreateTaskAs`: `req.Labels = addProjectLabel(req.Labels, proj)`
      right after the `SetWorkflowLabel(req.Labels, "#new")` line.
- [x] T14 — `internal/db/db.go` `ConvertTaskToRemote` (the second `CreateIssue` path; no
      `PushTaskToTracker` exists): the same on `task.Labels` after its `SetWorkflowLabel` call.
- [x] T15 — `internal/db/projectlabel_test.go`: a task created in a labelled project carries
      the label locally; a task whose submitted labels already carry it in another case is
      not given a duplicate; a task created in an unlabelled project is unchanged.
      **Run `gofmt -l .` and `go test ./...`.**

## 4. Frontend — plumbing

- [x] T16 — `web/src/types/index.ts`: `projectLabel?: string` on `Project`;
      `showAllTickets`, `setShowAllTickets` and `projectLabel` on the context type.
- [x] T17 — `web/src/lib/projectLabel.ts` (new): `membershipParam(showAll: boolean): string | null`
      and `projectLabelState(taskLabels: string[], projectLabel?: string)` returning
      `{ label, carries, applicable }`, both pure so the node test can assert them.
- [x] T18 — `web/src/context/AppContext.tsx`: `showAllTickets` state + setter with
      `persistFilter({ showAllTickets: value ? '1' : null })`; restore it in the per-project
      effect; append `membership=all` in `buildTaskQuery` and in `fetchTaskFacets`; derive
      and expose the selected project's `projectLabel`; extend the dependency arrays and the
      context value.
- [x] T19 — `web/tests/projectLabel.test.mjs` (new): `membershipParam` and
      `projectLabelState` — carried, not carried, different case, no project label.
      **Run `cd web && npm test`.**

## 5. Frontend — the three touch points

- [x] T20 — `web/src/components/ProjectModal.tsx`: "Label du projet" input in the left
      column of the `general` tab under the slug, stripping whitespace on change, with the
      hint about an empty value; wire it into the state, the `editingProject` hydration and
      the submitted payload.
- [x] T21 — `web/src/components/TaskFilters.tsx`: the "Tout le board" toggle next to
      "Épinglés" and "En cours", rendered only when the selected project has a label.
- [x] T22 — `web/src/components/TaskCard.tsx`: "Ajouter au projet" / "Retirer du projet" in
      the common part of the burger menu, driven by `projectLabelState`, calling
      `updateTask(task.id, { labels })`, with a toast; hidden when the project has no label.
      The project lookup falls back to matching the slug.
- [x] T23 — **Run `cd web && npm test && npx tsc --noEmit -p tsconfig.app.json && npx oxlint src`.**

## 6. Documentation and close

- [x] T24 — `CHANGELOG.md`: one `Added` line under `## [Unreleased]`, written for whoever
      uses Sectile.
- [x] T25 — Re-read the whole diff against the default branch as a reviewer would.
- [x] T26 — **Full gate: `gofmt -l .`, `go build ./...`, `go test ./...`, and the three web
      commands.** Quote the real output in the report.

## Test plan a reviewer can replay

1. `gofmt -l .` prints nothing.
2. `go build ./...` succeeds.
3. `go test ./...` passes.
4. `cd web && npm test` passes.
5. `cd web && npx tsc --noEmit -p tsconfig.app.json` passes.
6. `cd web && npx oxlint src` passes.
7. Manual: on a project with no label, the board is unchanged and no new toggle or menu
   entry appears. Set a label; the board narrows, the toggle appears and widens it, the
   burger menu attributes and un-attributes a card, and a ticket created from that project
   already carries the label.
