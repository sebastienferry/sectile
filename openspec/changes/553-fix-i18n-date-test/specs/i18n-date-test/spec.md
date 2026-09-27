# i18n date test portability

## ADDED Requirements

### Requirement: Import the i18n module from a hash-named worktree

The spawned Node ESM script SHALL import the intended i18n module when the worktree path contains `#`.

#### Scenario: Execute the date test in a task worktree

- **GIVEN** the test runs in a worktree whose path contains `#`
- **WHEN** it spawns the Node ESM script for the date and time-zone check
- **THEN** the script imports the i18n module successfully
- **AND** the test does not fail with a path-fragment module resolution error

### Requirement: Preserve date and instant expectations

The test SHALL retain its existing calendar-day and instant-formatting expectations in both configured time zones.

#### Scenario: Format values in Los Angeles and Tokyo

- **GIVEN** the child script runs with `America/Los_Angeles` and `Asia/Tokyo`
- **WHEN** it formats `2026-09-01` and `2026-09-01T02:00:00Z`
- **THEN** the calendar day is `Sep 1, 2026` in both zones
- **AND** the instant is `Aug 31, 19:00` in Los Angeles and `Sep 1, 11:00` in Tokyo
