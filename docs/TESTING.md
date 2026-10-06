# Database test fixtures and performance

Design rationale: [ADR 0041](adrs/0041-sqlite-behavior-tests-copy-a-migrated-template.md).

Ordinary SQLite behavior tests use `internal/testsqlite.New` with `db.NewDB`
(or `NewDB` inside the database package). The existing `testDB`, `setupTestDB`
and `setupTestHandler` helpers already use it.

```go
path := filepath.Join(t.TempDir(), "test.db")
database, err := testsqlite.New(t, path, db.NewDB)
if err != nil {
    t.Fatal(err)
}
t.Cleanup(func() { database.Close() })
```

The first call in each test process builds a database through the real startup
and migration path, closes it, verifies that no uncheckpointed WAL remains, and
retains its bytes in memory. Subsequent calls write independent files and run
normal startup against the current schema. Connections, background queues,
instance identities and key files remain independent. The source directory is
removed immediately; there is no persistent fixture cache to invalidate when
migrations change. All callers in a binary must use the same constructor.

The helper refuses existing destination files. Use a private directory for each
fixture so its `secret.key` also remains independent. A deliberately configured
`SECTILE_SECRET_KEY` still has its normal production semantics.

## Tests that must initialize real databases

Call `db.NewDB`, `db.Open`, or the existing explicit migration harness directly
when testing first startup, seeding, schema upgrades, legacy data adoption,
restart/recovery, persistence, or multiple instances sharing a database. Do not
replace an existing database with a template to reopen it. PostgreSQL tests
continue to use their disposable integration database and serialized package
execution; never point them at a shared development database.

The database `SECTILE_TEST_POSTGRES_DSN` names must be encoded in UTF8, as the
server requires: the helpers fail at once, naming the variable, on any other
encoding. A cluster made with `initdb --locale=C` and no `--encoding` defaults
to SQL_ASCII, so pass `--encoding=UTF8`, or create the test database with
`CREATE DATABASE sectile_test ENCODING 'UTF8' TEMPLATE template0`.

Template checks in `internal/db/testfixture_test.go` compare complete schema
objects with a fresh database, verify migration version and SQLite integrity,
and exercise independent data, keys, runtime state and concurrent copies.

Copying fixtures does not make every consumer safe for `t.Parallel`: tests that
change environment variables or process globals still need serial execution.
Race detection and coverage stay enabled in CI. No production SQLite durability
settings or timeout budgets are changed by this optimization.

## Measuring changes

Run uncached tests with the same instrumentation as CI and retain JSON output:

```sh
go test -json -count=1 -race -covermode=atomic -timeout=15m \
  ./internal/db ./internal/handlers ./internal/taskmcp > /tmp/sectile-tests.json
```

Compare package elapsed times on the same machine and avoid concurrent benchmark
runs. Package durations overlap: summing them does not give CI wall time. Measure
CI cache restoration, compilation and cache upload separately from test execution.
A local speedup is not a demonstrated CI wall-time result.

For a repeatable comparison of fixture creation and close alone:

```sh
go test -run '^$' -race -covermode=atomic \
  -bench '^BenchmarkSQLiteFixture$' -benchtime=10x -count=3 ./internal/db
```

The benchmark excludes the template's one-time creation; whole-package timings
include it. Use the full suite to check the effect on migration-heavy tests that
intentionally retain fresh initialization, and check for skips and failures as
well as timing improvements.

## Initial local measurements (#595)

On darwin/arm64 with Go 1.26.6, uncached test execution with `-race
-covermode=atomic` produced the following package timings:

| Package | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| `internal/db` | 288.0 s | 105.6 s | 63% |
| `internal/handlers` | 98.7 s | 36.2 s | 63% |

The baseline ran these two packages; the final verification ran `./...`, so the
after measurement also includes contention from other packages. These are local
observations, not a controlled multi-run CI benchmark or a CI runtime promise.
Database coverage remained 72.8% and handler coverage 55.2%; no existing test in
these packages disappeared or gained a skip. The full Go run passed with race
detection and coverage; environment-dependent skips, including PostgreSQL tests
without a dedicated DSN, remain. CI timing and cache-transfer costs must still
be measured on the runner.

Three instrumented fixture benchmark samples (10 iterations each) measured
517.5–520.9 ms per fresh fixture versus 15.12–15.18 ms per template copy, about
34 times faster for creation and close. The template benchmark excludes its
one-time initialization. Twenty shuffled repetitions of the template checks and
foreign-PR fixture checks also passed under `-race` after synchronizing the fake
agent's mutable answers and call counter.
