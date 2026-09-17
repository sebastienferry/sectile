## ADDED Requirements

### Requirement: The web test suite runs without installed dependencies
`npm test --prefix web` SHALL run to completion and exit 0 in a checkout that has no
`web/node_modules`, so the suite can gate any branch or freshly created worktree before an install.

#### Scenario: A freshly created worktree
- **GIVEN** a worktree created by `git worktree add`, with no `web/node_modules` directory
- **WHEN** `npm test --prefix web` is run in it
- **THEN** the command exits 0
- **AND** no test file fails to load

#### Scenario: No test file depends on an installed package
- **GIVEN** the files under `web/tests/`
- **WHEN** their imports are inspected
- **THEN** none of them imports `typescript`
- **AND** none of them imports any package resolved from `node_modules`

#### Scenario: An install does not change the outcome
- **GIVEN** a checkout where `npm ci --prefix web` has been run
- **WHEN** `npm test --prefix web` is run
- **THEN** it exits 0, and reports the same tests as the same command run without `web/node_modules`

### Requirement: Each test exercises its subject through a direct source import
A test file SHALL obtain the module it exercises by importing its `.ts` source directly, by named
import, and SHALL NOT transpile that source or load it through a generated `data:` URL.

#### Scenario: The subject is imported by name
- **GIVEN** a test file exercising `web/src/lib/remoteRunIndicator.ts`
- **WHEN** it obtains `deriveRunIndicator`, `activeTaskIds` and `CANCELED_VISIBILITY_MS`
- **THEN** it imports them by name from `../src/lib/remoteRunIndicator.ts`

#### Scenario: A removed export fails loudly
- **GIVEN** a test file importing a named export from its subject
- **WHEN** that export no longer exists in the subject
- **THEN** the failure names the missing export

### Requirement: The change preserves the suite's coverage
Converting a test file to a direct source import SHALL NOT add, remove, weaken or reorder any
assertion, and the suite SHALL report the same number of tests as before the change.

#### Scenario: The test count is unchanged
- **GIVEN** the suite reported 78 tests before the change, on a checkout where it could run
- **WHEN** `npm test --prefix web` is run after the change
- **THEN** it reports 78 tests
- **AND** every one of them passes

#### Scenario: Only the import preamble differs
- **GIVEN** the diff of a converted test file
- **WHEN** it is reviewed
- **THEN** it touches only the lines that read, transpile and import the subject
- **AND** the helpers, fixtures and test bodies are byte-identical
