# ADR 0021: The schema carries its version, and every change is a numbered migration

Status: Accepted

Supersedes the schema-completeness consequences of
[ADR 0016](0016-postgresql-as-an-alternative-store.md).

## Context

Three mechanisms build the schema today, and not one of them records what a
given database has actually received:

- `CREATE TABLE IF NOT EXISTS`, which adds nothing to a table that exists;
- roughly ninety `ALTER TABLE ... ADD COLUMN`, idempotent by failure, gated
  behind `RunsLegacyMigrations()` and therefore SQLite-only;
- `lateColumns`, a hand-kept list of the columns declared since PostgreSQL
  support shipped, replayed as `ADD COLUMN IF NOT EXISTS` on every start.

Ask any database "which schema do you carry?" and there is no answer. The
consequence is not theoretical. `users.blocked_at` (#327) was declared in its
`CREATE TABLE` and in a legacy `ALTER`, and in neither form could it ever reach
a PostgreSQL database created before it: #337 fixed the column by hand-extending
the third list, weeks after the fact, and only because blocking an account
failed loudly enough to be reported. The same omission on a column read but
never written would still be there.

The cost is also daily. Adding one column today means writing it in the
`CREATE TABLE`, in the legacy `ALTER` and in `lateColumns`: three declarations
of one fact, policed by a test whose entire job is to catch the moment they
disagree.

ADR 0016 assumed that a PostgreSQL database "is created complete". That is true
of creation and false of every upgrade after it, which is the gap this record
closes.

## Decision

**The database states the version of the schema it carries.** One table,
`schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at
DATETIME NOT NULL)`, on both engines. One integer, shared: the engines run the
same numbered sequence.

**Version 1 is the schema as it stands today, and the code that produces it is
frozen.** The current startup path (the `init…Schema` and `ensure…Table`
helpers, the legacy block, `lateColumns`) becomes the *baseliner*. It runs once,
against a database carrying no version row, and stamps version 1. Every
population converges there: a SQLite file at any point of its history, a
PostgreSQL database at any schema an earlier binary left it, and a database
created from nothing. Reconstructing the ninety historical `ALTER` statements as
migrations 1..N was rejected: no existing database is empty, so the replay would
be unverifiable, and the SQLite files in the wild need the idempotent path
regardless.

Telling the two apart is not even necessary, which the implementation settled:
the baseliner is the old startup path, and that path is idempotent on both
populations by construction. It therefore runs whenever there is no version row,
and asks nothing about what it will find.

**From version 2 on, a migration is applied exactly once, so it no longer has to
be idempotent.** This is the property the current scheme cannot offer at any
price, and everything awkward about that scheme follows from its absence:
`IF NOT EXISTS`, "idempotent by failure", the deliberately ignored error, the
hand-kept list. A numbered migration is ordinary SQL that runs once and is
recorded.

**A column is declared once.** Not in a `CREATE TABLE`, not in a legacy `ALTER`,
not in `lateColumns`: in a migration. The `CREATE TABLE` literals stop meaning
"the current schema" and start meaning "the version 1 schema", and are never
edited again.

**One transaction per migration, the version row written inside it.** A
migration that stops halfway leaves the database exactly at the previous version,
which is the whole reason to number them. `pgx` refuses a multi-statement `Exec`
under its extended protocol, so a migration is a *list* of statements sent one by
one, never a script, the same constraint that keeps `migrateTasksKeyUnique`
SQLite-only.

**A migration that fails stops the server.** Today a failed schema repair is
logged and startup continues. That was tolerable when every statement was an
optional repair; it is not tolerable for a numbered change, because the
alternative is an application serving requests against a schema it does not
have. The transaction leaves the database at the previous version, so the
operator's recourse is to restart the previous binary.

**Engine-specific work only where the engine forces it.** Shared SQL is the
default: `RewriteDDL` already translates the two type names that differ. SQLite
can neither relax a `NOT NULL`, nor add a foreign key, nor add a `CHECK` through
`ALTER TABLE`, so those few migrations carry a per-engine variant, exactly as
`MigrateActivityAttachment` does today.

**PostgreSQL takes a session advisory lock for the duration of the run.** One
server instance per database is already the documented invariant, and it is
enough for steady state, but a rolling deploy overlaps two processes for a few
seconds, which is precisely when both would migrate. `pg_advisory_lock` costs
three lines and removes the case. SQLite needs nothing: one process, and
`busy_timeout` already set on the connection.

**What is not a migration stays out of the migrations.** Marking interrupted
runs `failed`, cancelling the remote runs whose client session died, seeding the
default project: these must run on *every* start, not once. Filing them as
migrations would break restart recovery outright.

This is not a hypothetical. Those two recovery statements live inside
`applyLegacyMigrations` today (`internal/db/db.go`), which never runs on
PostgreSQL, so **a PostgreSQL deployment has never recovered its interrupted
runs**, and a server restart leaves them `running` forever. Separating per-start
recovery from one-shot migration is part of this work, and fixes that on the way
through.

## Consequences

- The baseline is frozen, which makes
  `TestSchemaIsCompleteWithoutLegacyMigrations` cheap and permanent: it now
  guards a snapshot rather than a moving target.
- A database created from nothing runs the baseline and then every migration in
  order, rather than being built at the latest schema in one pass. It costs
  milliseconds, and it buys the best coverage available: every fresh install and
  every test run exercises the whole migration sequence.
- There are no down migrations. Rolling back a schema means restoring a backup,
  and the numbered version is what tells the operator which backup. A reversible
  migration is a promise the tests would never check.
- The three-place declaration disappears, and with it the class of bug #337
  belongs to. A column added tomorrow reaches an upgraded PostgreSQL deployment
  because the deployment knows it has not seen migration N, not because someone
  remembered a list.
- `lateColumns` is folded into the baseline and frozen, not deleted: it is what
  carries a PostgreSQL database created before #337 up to version 1, so removing
  it would break the very upgrade this record is about. Nothing may be added to
  it again. The upgrade test that came with #337 survives as the baseliner's own
  test: it is still the only test that covers a database created by an older
  binary.
- A one-shot repair is no longer replayed on every start, which is the point and
  is also a trap for the tests. Five fixtures simulated an old database by
  rebuilding a legacy shape on a database this binary had just created, and
  therefore just stamped; the reopen they relied on now skips the baseline. They
  say what they mean instead, through a helper that removes the version row,
  because a database genuinely written by an earlier binary carries none.
- Every future schema change now has a mandatory, mechanical home. That is a
  small tax on a one-column change, and it is the point.

## Alternatives rejected

- **Reconstructing the ~90 historical `ALTER` statements as migrations 1..N.**
  Faithful to the history and useless in practice: no existing database is
  empty, the replay is unverifiable against the files in the wild, and the
  idempotent path is needed anyway to reach the baseline.
- **`goose` or `golang-migrate`.** Both are sound, and both want their own
  driver layer and `.sql` files per dialect: two maintained sets for the six
  syntactic differences this package actually has, on top of a `modernc/sqlite`
  and `pgx` pairing already wired behind an existing seam. The runner this
  record calls for is a table, a sorted list and a transaction.
- **Keeping `lateColumns` and adding a review rule.** The rule already existed,
  written into ADR 0016 in bold, and #327 still shipped without it. A convention
  that has already failed once is not a control.
- **Down migrations.** See above: untested reversibility is worse than none.
