## ADDED Requirements

### Requirement: The backing engine is chosen by configuration
The server SHALL select its database engine from its environment at startup. When no engine is
named, it SHALL use SQLite, so that every existing deployment keeps working without being
reconfigured.

#### Scenario: No engine named
- **GIVEN** no database engine is named in the environment
- **WHEN** the server starts
- **THEN** it opens its SQLite database exactly as it does today
- **AND** the startup log reports the SQLite path and where that path came from.

#### Scenario: PostgreSQL named
- **GIVEN** the environment names PostgreSQL as the engine and supplies a connection string
- **WHEN** the server starts
- **THEN** it connects to that PostgreSQL database
- **AND** the startup log reports the engine and the target host and database name
- **AND** the log never contains the connection string's password.

#### Scenario: An unknown engine is refused
- **GIVEN** the environment names an engine that is neither SQLite nor PostgreSQL
- **WHEN** the server starts
- **THEN** it refuses to start
- **AND** the error names the value it was given and lists the engines it accepts.

### Requirement: An incomplete PostgreSQL configuration fails fast
When PostgreSQL is named but cannot be used, the server SHALL refuse to start rather than fall
back to SQLite. A silent fallback would serve an empty board from an unexpected store and look
like data loss.

#### Scenario: PostgreSQL named without a connection string
- **GIVEN** the environment names PostgreSQL but supplies no connection string
- **WHEN** the server starts
- **THEN** it refuses to start
- **AND** the error says which variable is missing
- **AND** no SQLite database is opened or created.

#### Scenario: The PostgreSQL server is unreachable
- **GIVEN** the environment names PostgreSQL with a connection string pointing at an unreachable server
- **WHEN** the server starts
- **THEN** it refuses to start
- **AND** the error reports the connection failure
- **AND** no SQLite database is opened or created.

### Requirement: The SQLite path keeps its current meaning
The variable naming the SQLite database file SHALL keep the exact meaning and discovery order it
has today, and SHALL be ignored when PostgreSQL is the selected engine.

#### Scenario: The SQLite discovery order is unchanged
- **GIVEN** SQLite is the engine and no explicit path is configured
- **WHEN** the server starts
- **THEN** it resolves its database file through the same order as before this change
- **AND** an existing database in the current directory or the data directory is opened, not replaced.

#### Scenario: The SQLite path is ignored under PostgreSQL
- **GIVEN** PostgreSQL is the engine and a SQLite path is also set in the environment
- **WHEN** the server starts
- **THEN** it connects to PostgreSQL
- **AND** the file named by the SQLite path is neither read, created nor modified.

### Requirement: Identical behaviour on both engines
Every feature reachable through the API SHALL behave identically whichever engine backs it, for
the same sequence of operations. Engine choice is an operational concern and SHALL NOT be
observable in the product's behaviour.

#### Scenario: A task survives a round trip on either engine
- **GIVEN** a server running on either engine
- **WHEN** a task is created, read back, updated, listed and deleted
- **THEN** each response carries the same values and the same shape on both engines.

#### Scenario: Timestamps read back as they were written
- **GIVEN** a server running on either engine
- **WHEN** a record carrying a timestamp is written and then read back
- **THEN** the value read equals the value written
- **AND** the result does not depend on the server's local time zone.

#### Scenario: An upsert updates rather than duplicates
- **GIVEN** a server running on either engine and an existing row with a given natural key
- **WHEN** a write targeting that same natural key is performed
- **THEN** the existing row is updated
- **AND** no second row with that key exists afterwards.

### Requirement: A new PostgreSQL database is created at the current schema
Pointing the server at an empty PostgreSQL database SHALL produce a complete, usable schema. The
historical repair steps that only make sense for databases created by older versions SHALL NOT
run on PostgreSQL.

#### Scenario: Empty database is initialised
- **GIVEN** an empty PostgreSQL database
- **WHEN** the server starts against it
- **THEN** every table the application needs exists with its complete set of columns
- **AND** the server serves normally
- **AND** the seed data an empty installation expects is present.

#### Scenario: Starting twice is harmless
- **GIVEN** a PostgreSQL database already initialised by a previous start
- **WHEN** the server starts against it again
- **THEN** it starts normally
- **AND** no existing row is modified, duplicated or removed by the initialisation.

### Requirement: The encryption key is supplied by the operator under PostgreSQL
Under PostgreSQL the server SHALL take its encryption key from the environment only, and SHALL
NEVER generate or persist one. Generating a key silently would produce a different key on each
restart of a container without persistent storage, making every stored personal tracker token
unreadable without anyone noticing.

#### Scenario: The key is supplied
- **GIVEN** PostgreSQL is the engine and the environment supplies an encryption key
- **WHEN** the server starts
- **THEN** personal tracker tokens can be stored and read back
- **AND** no key file is written anywhere.

#### Scenario: The key is absent
- **GIVEN** PostgreSQL is the engine and the environment supplies no encryption key
- **WHEN** the server starts
- **THEN** it starts and serves normally
- **AND** it logs a warning naming the variable that supplies the key
- **AND** no key file is written or generated
- **AND** operations that need the key refuse with a message saying why.

#### Scenario: A passphrase-sealed token is unaffected
- **GIVEN** PostgreSQL is the engine, no encryption key is supplied, and a user sealed their token behind a passphrase
- **WHEN** that user unlocks their token with their passphrase
- **THEN** the token is readable
- **AND** the absent server key does not prevent it.

#### Scenario: SQLite keeps its generated key file
- **GIVEN** SQLite is the engine and the environment supplies no encryption key
- **WHEN** the server starts for the first time
- **THEN** a key file is generated beside the database exactly as before this change
- **AND** personal tracker tokens can be stored and read back without any operator action.
