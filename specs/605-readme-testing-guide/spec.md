# Spec #605 - README lists the testing guide in its Documentation section

Clarification: [docs/clarifications/605.md](../../docs/clarifications/605.md).

## User story (P1)

As a contributor reading `README.md`, I find the testing guide from the
Documentation section, so I learn how database tests are set up and measured
without searching the `docs/` folder.

## Functional requirements

- **FR1** The Documentation section of `README.md` lists the testing guide
  with exactly this bullet:

  ```markdown
  - [Testing guide](./docs/TESTING.md): SQLite test fixtures, tests that need a
    real database, and how to measure test performance.
  ```

- **FR2** The bullet sits immediately after the **UX components** bullet and
  immediately before the **Desktop guide** bullet.
- **FR3** The link target resolves to the existing `docs/TESTING.md`.
- **FR4** No other line of `README.md` changes.

## Acceptance scenarios

1. **Given** the README Documentation section, **when** a reader scans the
   list, **then** a "Testing guide" bullet appears between "UX components"
   and "Desktop guide", wrapped like its neighbours.
2. **Given** the rendered README, **when** the reader follows the "Testing
   guide" link, **then** it opens `docs/TESTING.md`.
3. **Given** the change against `main`, **when** the diff of `README.md` is
   read, **then** it adds two lines and removes none.

## Out of scope

- Any change to `docs/TESTING.md`.
- A `CHANGELOG.md` entry: the README index is not a user-visible product
  change.

## Open requirements

None.
