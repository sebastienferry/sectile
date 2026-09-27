# Design

`AnswerRemoteRunWait` will use one conditional `UPDATE` whose `WHERE` clause includes the supplied `waiting_since`, owner, agent action, running status, remote-run kind, and empty wait reason. The affected-row count determines whether to notify listeners. The database performs timestamp comparison and write atomically, across instances.

The existing read-then-write flow is rejected because the instance mutex cannot guard writes made through a different database connection or server. A transaction with a locking read would also work, but adds engine-specific locking for a single-column predicate. Keep timestamp arguments as `time.Time`; the existing agent JSON round trip and database drivers establish the mark's precision.

The regression test uses two PostgreSQL stores. It races an answer to the old mark against clearing and redeclaring a wait, then asserts that any newly declared mark survives. A serialized stale-answer case and existing SQLite tests cover the deterministic boundaries.
