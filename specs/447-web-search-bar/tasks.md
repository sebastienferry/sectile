# Tasks: search that ignores case and accents

Ordered checklist. Each task names what proves it done. Requirement numbers
refer to `spec.md`, design to `plan.md`.

## 0. Before starting

- [x] `git fetch` and compare `feat/447` with `origin/main`: another agent may
      have landed a migration 14 or touched `GetTasksInScope`. Renumber the
      migration if needed.
- [x] Start a throwaway PostgreSQL 16 and export `SECTILE_TEST_POSTGRES_DSN`
      pointing at an empty test database (the suite truncates it).

## 1. Tests first (red)

- [x] `internal/db/search_test.go`: the four PostgreSQL tests of `plan.md`
      (task search case/accents, wildcards, activity search, extension
      installed). Run them: the case and accent assertions fail on `main`.
- [x] SQLite test: `Bug` finds `bug`; `50%` does not find `500` (fails today:
      `%` is a wildcard).
- [x] `web/tests/searchFold.test.mjs`: `foldForSearch`, `matchesSearch`,
      `matchesEpicSearch` on an accented title. Fails: module missing.

## 2. Migration (FR 8)

- [x] `internal/db/migrations.go`: migration 14 `extension.unaccent`, PostgreSQL
      form only.
- [x] Optional `hint` field on `migration`, appended by `applyMigrations` to
      the error; set on migration 14 to name `unaccent`, the `CREATE` privilege
      and the contrib package.
- [x] Unit test (SQLite): `applyMigrations` on a hinted migration whose
      statement fails returns an error that contains the hint.
- [x] Update `migrations_test.go` for the new latest version (nothing to
      change: it derives the latest version from the list).
- [x] Check: `TestPostgresUnaccentExtensionIsInstalled` and
      `TestPostgresStartingTwiceIsHarmless` pass; `go test ./internal/db/`
      passes on SQLite.

## 3. Dialect (FR 1, 2, 6)

- [x] `dialect.go`: `FoldSearch(expr string) string` with its doc comment.
- [x] `dialect_postgres.go`: `LOWER(unaccent(<expr>) COLLATE "C")`.
- [x] `dialect_sqlite.go`: `LOWER(<expr>)`.
- [x] `db.go`: `foldSearch` wrapper with the nil-dialect fallback.
- [x] Unit test beside `boardviews_test.go:439`: the SQL each dialect returns.

## 4. Server search (FR 1 to 6)

- [x] `db.go`: `searchPredicate(query, columns...)` next to `escapeLike`.
- [x] `GetTasksInScope`: use it for the seven columns; rewrite the touched
      French comment in English.
- [x] `GetActivities`: use it for the five columns.
- [x] Leave the `label` filter untouched.
- [x] Check: all tests of step 1 on the Go side pass, on PostgreSQL and SQLite.

## 5. Browser searches (FR 7)

- [x] `web/src/lib/searchFold.ts`: `foldForSearch`, `matchesSearch`.
- [x] `web/src/lib/lookups.ts`: private `matches` folds both sides.
- [x] `web/src/lib/roadmap.ts`: `matchesEpicSearch` folds query and haystack.
- [x] `web/src/components/TriageView.tsx`: search filter via `matchesSearch`.
- [x] `web/src/components/ActivitiesView.tsx`: search filter via
      `matchesSearch`.
- [x] `web/src/components/TaskFilters.tsx`: "Non assigné" and "Sans macro" /
      "Sans milestone" tests fold the query and the label.
- [x] Check: `cd web && npm test` passes; `tsc` and `oxlint` clean (symlink the
      main checkout's `node_modules` if the worktree has none, then remove it).
- [x] No user-visible string changed (FR 9): `git diff web/src/locales` is empty.

## 6. Documentation

- [x] `CHANGELOG.md`: the `Fixed` line of `plan.md` under `[Unreleased]`
      (FR 10).
- [x] `README.md`, "PostgreSQL instead of SQLite": the `unaccent` prerequisite
      and the refusal to start without it.

## 7. Final verification

- [x] `go build ./...`, `go vet ./...`, `go test ./...` (SQLite).
- [x] `SECTILE_TEST_POSTGRES_DSN=... go test ./internal/db/ -run Postgres`.
- [x] `cd web && npm test`, then a web build; restore `webui/.gitkeep` if the
      build deleted it.
- [x] Walk the acceptance scenarios of `spec.md` against the tests: each one is
      covered by a Go or a web test, or noted as manual.
- [ ] Manual check on a copy of a PostgreSQL database, never the dev one: start
      the branch server, search `equipe` in the header on the board and on the
      roadmap. Start it without tracker tokens so nothing is written to Jira or
      GitHub.
      Not done in the implementation run: the header search reaches
      `GetTasksInScope` unchanged, which the PostgreSQL tests exercise end to
      end; the refusal to start and the recovery after an administrator creates
      the extension were checked by hand on a throwaway PostgreSQL 18 with a role
      lacking `CREATE` on the database.

## Test plan summary

| Requirement | Covered by |
| --- | --- |
| FR 1 case | `TestPostgresTaskSearchIgnoresCaseAndAccents`, activity test |
| FR 2 accents | same tests, `searchFold.test.mjs` |
| FR 3 substring | parent title and description scenarios |
| FR 4 fields | parent title, description, activity summary scenarios |
| FR 5 wildcards | `TestPostgresTaskSearchTreatsWildcardsLiterally`, SQLite test |
| FR 6 SQLite unchanged | SQLite test |
| FR 7 browser | `searchFold.test.mjs`, roadmap case |
| FR 8 extension | `TestPostgresUnaccentExtensionIsInstalled`, error hint unit test |
| FR 9 no text change | locales diff empty |
| FR 10 changelog | review |
