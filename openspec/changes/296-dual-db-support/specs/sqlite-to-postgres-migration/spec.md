## ADDED Requirements

### Requirement: An existing SQLite database can be moved to PostgreSQL
The product SHALL offer a one-way migration that copies an existing SQLite database into a
PostgreSQL database, so that an operator already running Sectile is not forced to start over.
The reverse direction is not offered.

#### Scenario: A populated database is copied
- **GIVEN** a SQLite database holding projects, tasks, activities, users and settings
- **AND** an empty, initialised PostgreSQL database
- **WHEN** the operator runs the migration
- **THEN** every row of every table is present in PostgreSQL
- **AND** the counts per table match the source
- **AND** the source SQLite database is left unmodified.

#### Scenario: The server serves the migrated data
- **GIVEN** a migration that completed successfully
- **WHEN** the server is started against the PostgreSQL database
- **THEN** the boards, tasks, activities and settings are the ones the SQLite database held
- **AND** relationships between tasks, projects and users are intact.

#### Scenario: A report is produced
- **GIVEN** any migration run
- **WHEN** it finishes
- **THEN** it reports each table with the number of rows copied
- **AND** it states plainly whether the migration succeeded or failed.

### Requirement: The encryption key is verified before anything is copied
The migration SHALL verify, before copying its first row, that sealed credentials will still be
readable at the destination. Sealed tokens are bound to the user and the tracker, not to the
database, so a copy made without the matching key produces a database whose tokens are silently
unreadable — a failure that would otherwise surface weeks later.

#### Scenario: The key matches
- **GIVEN** a source database containing at least one credential sealed under the server key
- **AND** the same key available to the migration
- **WHEN** the migration starts
- **THEN** it opens one sealed credential as a check
- **AND** proceeds with the copy.

#### Scenario: The key is missing or different
- **GIVEN** a source database containing at least one credential sealed under the server key
- **AND** a different key, or no key, available to the migration
- **WHEN** the migration starts
- **THEN** it refuses before copying any row
- **AND** the error explains that the key must be carried across and names the variable that supplies it
- **AND** the destination database is left untouched.

#### Scenario: Nothing is sealed under the server key
- **GIVEN** a source database where no credential is sealed under the server key
- **WHEN** the migration starts
- **THEN** the check passes
- **AND** the migration proceeds without requiring a key.

### Requirement: The destination is protected from accidental overwrite
The migration SHALL NOT silently write into a PostgreSQL database that already holds data, so
that running it twice, or against the wrong target, cannot destroy or duplicate anything.

#### Scenario: The destination already holds data
- **GIVEN** a PostgreSQL database that already holds rows
- **WHEN** the operator runs the migration against it without asking for it to be overwritten
- **THEN** the migration refuses before writing
- **AND** the error says the destination is not empty
- **AND** no row is added, modified or removed.

#### Scenario: A failure leaves no half-migrated database
- **GIVEN** a migration that fails partway through
- **WHEN** the failure occurs
- **THEN** the destination does not keep a partial copy
- **AND** the error names the table that failed.

### Requirement: Migrated credentials remain usable
Personal tracker credentials SHALL be readable after the migration by the users who own them,
without re-entering them.

#### Scenario: A server-key-sealed token still opens
- **GIVEN** a user whose tracker token was sealed under the server key before the migration
- **AND** a destination server started with the same key
- **WHEN** that user's token is read
- **THEN** it opens successfully
- **AND** the user is not asked to enter it again.

#### Scenario: A passphrase-sealed token still opens
- **GIVEN** a user whose tracker token was sealed behind their own passphrase before the migration
- **WHEN** that user unlocks it with that passphrase after the migration
- **THEN** it opens successfully.
