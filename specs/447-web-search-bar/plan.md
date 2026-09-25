# Plan: search that ignores case and accents

Behaviour is in `spec.md`. This file says how, and where.

## Stack

- Go server, `internal/db` (shared queries, per-engine `dialect`), PostgreSQL 16
  in CI (`postgres:16` in `.gitlab-ci.yml`) and on the shared server.
- React/TypeScript web app, `web/src`, unit tests with `node --test` in
  `web/tests/*.test.mjs` importing `.ts` sources directly.

## Root cause

`GetTasksInScope` (`internal/db/db.go:1662`) and `GetActivities`
(`internal/db/db.go:5286`) match the query with a raw `column LIKE '%q%'`.
PostgreSQL's `LIKE` is case-sensitive; nothing folds accents on either engine;
`%`, `_` are passed through as wildcards.

## Architecture

Fold both sides of the comparison in SQL, with the same function, so the column
and the pattern can never be folded differently:

```
<fold>(column) LIKE <fold>(?) ESCAPE '!'
```

where `?` is `'%' + escapeLike(query) + '%'` built in Go.

### 1. Migration: the `unaccent` extension

Add version 14 to `migrations` in `internal/db/migrations.go` (the next free
number: 13 is `one_active_run.realign_macro`; renumber if `main` has landed
another one first):

```go
{
    // Search ignores case and accents on PostgreSQL (#447). unaccent is a
    // trusted contrib extension: a role holding CREATE on the database may
    // install it without being superuser.
    version: 14,
    name:    "extension.unaccent",
    postgres: []string{"CREATE EXTENSION IF NOT EXISTS unaccent;"},
},
```

- **SQLite form.** `statementsFor` falls back to `statements` when `sqlite` is
  empty, and `statements` is empty too, so SQLite runs nothing: `applyMigration`
  loops over zero statements and only stamps the version. Leave `sqlite` unset
  (drop it from the literal above). Never put the `CREATE EXTENSION` in
  `statements`.
- **Explicit failure (spec FR 8).** `applyMigrations` already wraps the error as
  `migration 14 (extension.unaccent): CREATE EXTENSION IF NOT EXISTS unaccent;:
  <pg error>`, which names the extension, and `migrateSchema`'s error stops the
  server. Add a hint to that one error without changing the generic path: in
  the migration list, a failure of this statement is reported as
  `the PostgreSQL extension "unaccent" could not be created (needs CREATE on
  the database and the contrib package): <err>`. Implement it as a small
  wrapper in `applyMigration` keyed on the migration name, or as an optional
  `hint string` field on `migration` appended to the error when set. Prefer the
  field: it stays declarative and costs one line.
- The migration runs inside the migration transaction; `CREATE EXTENSION` is
  transactional on PostgreSQL, so a failure leaves the database at version 13.
- Do not edit the baseline `CREATE TABLE` statements (ADR 0021; project memory
  "new columns only in migrations.go").

### 2. Dialect: one folding method

`internal/db/dialect.go`, next to `LowerASCII`:

```go
// FoldSearch wraps a TEXT expression in the fold used by free-text search:
// case and, where the engine can, diacritics. Both the column and the
// pattern go through it, so they fold the same characters.
FoldSearch(expr string) string
```

- `internal/db/dialect_postgres.go`:
  `LOWER(unaccent(<expr>) COLLATE "C")`. `unaccent` first maps `É` to `E`, `œ`
  to `oe`; `LOWER(... COLLATE "C")` then lowers ASCII independently of the
  cluster's collation, the same reasoning as `LowerASCII` (ADR 0025). Letters
  `unaccent` leaves non-ASCII keep their case (spec, out of scope).
- `internal/db/dialect_sqlite.go`: `LOWER(<expr>)`, today's behaviour.
- `internal/db/db.go`: a `(d *DB) foldSearch(expr string) string` wrapper that
  mirrors `lowerASCII`, including its nil-dialect fallback to `LOWER(expr)`.
- Any other `dialect` implementation (test fakes) must gain the method: `grep
  -rn 'LowerASCII(' internal` lists them.

`unaccent` is called unqualified. The extension is created in the current
schema by the migration, the same `search_path` the server queries under, so
no schema prefix is needed.

### 3. Search predicates

A helper in `internal/db/db.go`, next to `escapeLike`:

```go
// searchPredicate matches query anywhere in any of columns, ignoring case and,
// on PostgreSQL, diacritics. LIKE wildcards typed in the query are literal.
func (d *DB) searchPredicate(query string, columns ...string) (string, []interface{})
```

It returns `(<fold>(c1) LIKE <fold>(?) ESCAPE '!' OR <fold>(c2) LIKE ... )`
and one argument `"%" + escapeLike(query) + "%"` per column.

- `GetTasksInScope`: replace the `if query != ""` block with
  `searchPredicate(query, "key", "title", "description", "labels", "assignee",
  "parent_key", "parent_title")`. Keep the existing French comment above it,
  translated to English since the line is touched (AGENTS.md).
- `GetActivities`: replace the `if search != ""` block with
  `searchPredicate(search, "a.skill_name", "a.summary", "a.output", "t.key",
  "t.title")`. `t.key` and `t.title` are NULL for an activity with no task;
  `unaccent(NULL)` is NULL and the `LIKE` is false, as today.
- The `label` filter (`labels LIKE ?`, `db.go:1678`) is not touched (spec, out
  of scope).
- Performance: no index serves `%q%` today either; the fold adds a per-row
  function call on the same sequential scan. Acceptable at the board's size; no
  index is added (spec, out of scope).

### 4. Browser: one shared fold

New `web/src/lib/searchFold.ts`:

```ts
/** Folds text for free-text search: lower case, diacritics stripped. */
export const foldForSearch = (text: string | null | undefined): string =>
  (text || '').normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()

/** True when `query` (folded) occurs in any of `fields` (folded). */
export const matchesSearch = (query: string, ...fields: Array<string | null | undefined>): boolean
```

`matchesSearch` returns true for an empty or blank query and trims it, as the
current call sites do.

Call sites, each replacing its `toLowerCase()` pair:

| File | Where | Change |
| --- | --- | --- |
| `web/src/lib/roadmap.ts` | `matchesEpicSearch` (~l.268) | fold `query` before splitting into terms, fold the haystack |
| `web/src/components/TriageView.tsx` | search filter (~l.141) | `matchesSearch(searchQuery, t.key, t.title, t.sprint, t.parentKey, t.parentTitle, t.assignee)` |
| `web/src/components/ActivitiesView.tsx` | search filter (~l.147) | `matchesSearch(searchQuery, act.taskKey, act.taskTitle, act.skillName, act.summary, act.action, act.output)` |
| `web/src/components/TaskFilters.tsx` | assignee "Non assigné" test (~l.78), macro picker (~l.114) | fold query and the built-in labels (`'non assigné'`, `'sans macro'`, `'sans milestone'`) |
| `web/src/lib/lookups.ts` | private `matches` (~l.86), used by `valueLookup` (sprint, team, assignee pickers) and `macroLookup` | `matches = (haystack, query) => foldForSearch(haystack).includes(foldForSearch(query.trim()))` |

`TriageView.tsx:168` compares an exact option label to create a new value; it
is an equality check for de-duplication, not a search, and stays as it is.

The fold in the browser (`NFD` + strip marks) and `unaccent` agree on accented
Latin letters, which is what the spec requires. They may differ on ligatures
(`œ`, `ß`); nothing in the spec depends on those.

## Data contracts

No API change. `GET /api/tasks?q=` and `GET /api/activities?q=` keep their
parameters and response shapes; only the matching changes. No new column, no
backfill.

## Tests

### Go, PostgreSQL (`SECTILE_TEST_POSTGRES_DSN`)

In a new `internal/db/search_test.go` (or next to the smoke tests in
`postgres_smoke_test.go`, using `openPostgres`):

- `TestPostgresTaskSearchIgnoresCaseAndAccents`: seed tickets `Fix Bug in sync`,
  `Équipe plateforme`, `equipe mobile`, one whose parent title is
  `Refonte Écran`, one whose description holds `Réseau`; assert each query of
  the spec's scenarios through `GetTasksInScope`.
- `TestPostgresTaskSearchTreatsWildcardsLiterally`: `50%` vs `500`, `a_b` vs
  `axb`, and `!` in a title.
- `TestPostgresActivitySearchIgnoresCaseAndAccents`: an activity summary
  `Clarification terminée` found by `TERMINEE` through `GetActivities`; one
  activity with no task still returned by a summary match.
- `TestPostgresUnaccentExtensionIsInstalled`: after `openPostgres`,
  `SELECT 1 FROM pg_extension WHERE extname = 'unaccent'` returns a row.
  `TestPostgresStartingTwiceIsHarmless` already covers the second start.

These tests must run against PostgreSQL: on SQLite the case half would pass on
the buggy code. Run them locally with a throwaway server (project memory
"PostgreSQL tests need a DSN"; never point the DSN at a dev database, it is
truncated).

### Go, SQLite (default `go test ./...`)

- A SQLite test asserting `Bug` still finds `bug` (unchanged behaviour) and
  that `50%` does not find `500` (FR 5 applies on both engines).
- `migrations_test.go`: whatever asserts the latest version or the list of
  migrations is updated for version 14; a fresh SQLite database reaches it.

### Web (`node --test`, `web/tests/searchFold.test.mjs`)

- `foldForSearch`: `ÉQUIPE`, `Équipe`, `equipe` all fold to `equipe`; empty and
  `null` fold to `''`.
- `matchesSearch`: blank query matches; `helene` matches `Hélène`; a field of
  `undefined` is skipped.
- `matchesEpicSearch`: a row titled `Écran d'accueil` matches `ECRAN`, and two
  terms in any order still both have to match (extend an existing roadmap test
  file if one covers it).

Worktrees have no `node_modules` (project memory): symlink the main checkout's
for `tsc`/`oxlint`, then remove the link.

## Changelog

Under `## [Unreleased]`, `### Fixed` (create the section if absent):

> **Search ignores case and accents.** The search bar finds `Équipe` whether you
> type `equipe`, `Equipe` or `ÉQUIPE`, on the board, roadmap, triage, the
> activities view and the filter pickers. On a PostgreSQL server this needs the
> `unaccent` extension, which Sectile creates at start; a server whose database
> role cannot create it refuses to start and says so. (#447)

## Target files

- `internal/db/migrations.go`: migration 14, optional `hint` field.
- `internal/db/dialect.go`, `dialect_postgres.go`, `dialect_sqlite.go`:
  `FoldSearch`.
- `internal/db/db.go`: `foldSearch`, `searchPredicate`, `GetTasksInScope`,
  `GetActivities`.
- `internal/db/search_test.go` (new), `internal/db/migrations_test.go`.
- `web/src/lib/searchFold.ts` (new), `web/src/lib/roadmap.ts`,
  `web/src/lib/lookups.ts`, `web/src/components/TriageView.tsx`,
  `web/src/components/ActivitiesView.tsx`, `web/src/components/TaskFilters.tsx`.
- `web/tests/searchFold.test.mjs` (new).
- `CHANGELOG.md`.
- `README.md`, section "PostgreSQL instead of SQLite" (~l.193): add that the
  database role must be able to create the `unaccent` extension (trusted
  contrib extension, `CREATE` on the database), and that the server refuses to
  start otherwise.

## Rejected alternatives

- **ASCII-only fold with `lowerASCII`/`asciiLower`.** Recommended in round 1,
  withdrawn in round 2: it cannot fold accents.
- **`ILIKE`.** PostgreSQL only, and it does not fold accents either.
- **A stored, Go-maintained folded column.** Tickets are written by many paths
  (tracker sync, MCP, UI); each would have to keep it current, and it needs a
  backfill.
- **Folding the pattern in Go and the column in SQL.** Two implementations of
  the fold would drift; both sides go through the same SQL function.
- **Silent fallback to case-only search when `unaccent` is missing.** Refused
  by the owner in round 3.
