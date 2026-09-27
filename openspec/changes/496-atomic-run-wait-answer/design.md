# Design

`AnswerRemoteRunWait` will use one conditional `UPDATE` whose `WHERE` clause includes the supplied `waiting_since`, owner, agent action, running status, remote-run kind, and empty wait reason. The affected-row count determines whether to notify listeners. The database performs timestamp comparison and write atomically, across instances.

The existing read-then-write flow is rejected because the instance mutex cannot guard writes made through a different database connection or server. A transaction with a locking read would also work, but adds engine-specific locking for a single-column predicate. Keep timestamp arguments as `time.Time`; the existing agent JSON round trip and database drivers establish the mark's precision.

SQLite stores bound timestamps as local-offset text. The echoed mark can be UTC after its JSON round trip, so bind it in the server's local zone; PostgreSQL compares timestamps by instant and is unaffected by that conversion.

The regression test uses two PostgreSQL stores. A transaction writes the newer mark but leaves it uncommitted, while the other instance's answer blocks on that row. After confirming the answer reached the blocked update, the test commits the newer mark and asserts it survives. A serialized stale-answer case and existing SQLite tests cover the other boundaries.
