# Tasks #619 - Compare Taskativ and Sectile roadmap features

Order matters: the inventory is complete before any status is decided, and
statuses are decided before tickets are mapped.

## Setup

- [x] T1 `git fetch origin` and record the `origin/main` commit the evidence
      is read from. Record the Taskativ `HEAD` commit.
- [x] T2 Confirm the Taskativ checkout is readable and leave it untouched
      (FR-8): `git -C <taskativ> status --porcelain` before and after the
      study must print the same thing.

## Inventory (US1, FR-2, FR-3)

- [x] T3 List every FR-2 entry, then run the `openspec/` search of the plan
      and add any roadmap or timeline entry it finds.
- [x] T4 Read each entry's proposal (spec requirements for the five specs) and
      write one working line per entry: user-visible feature, view, entries
      that refine the same feature.
- [x] T5 Read Taskativ's `RoadmapView.tsx` and `SprintTimeline.tsx` for the
      features no entry describes, and add them.
- [x] T6 Collapse entries into features (FR-3) and move the out-of-scope ones
      to "Excluded entries" with their reason.

## Status (US1, FR-4)

- [x] T7 For each feature, find the Sectile evidence on `origin/main` and set
      `covered`, `partial` or `missing`, with a path (and line when one proves
      it) and, for `partial`, the missing part.

## Ticket mapping (US2, FR-5)

- [x] T8 Run the ticket search of the plan, record its date and query.
- [x] T9 For each `partial` or `missing` row, name the tracking issue and its
      state, or "none". For a closed issue, say whether it delivered the
      feature, and re-check the status when it did.

## Proposals (US3, FR-7)

- [x] T10 Group the "none" gaps into proposals, write each one's title,
      covered rows and description, and rank them with the FR-7 rule.
- [x] T11 Write the Summary (FR-6).

## Document

- [x] T12 Write `docs/taskativ-roadmap-comparison.md` with the plan's layout.
- [x] T13 Add the study to `docs/README.md`.

## Checks

- [x] T14 AC-1: every FR-2 entry appears in a row or under "Excluded entries".
- [x] T15 AC-2: every evidence path exists
      (`git ls-tree -r --name-only origin/main | grep -F <path>`).
- [x] T16 AC-3: every "none" in the ticket column is covered by a proposal.
- [x] T17 AC-4: run the leak check of the plan; nothing matches. Delete the
      local identifier list.
- [x] T18 Proofread for the repository's style rules (English, no em dash).

## Delivery

- [x] T19 Commit `docs: compare Taskativ and Sectile roadmap features (#619)`.
- [x] T20 Present the proposed tickets to the owner and ask for approval.
- [x] T21 Only on approval (AC-5): ask which macro they go under (OP-1),
      create the approved tickets, record their numbers in the study, commit.

## Test plan

There is no code, so there are no automated tests. The checks T14 to T17 are
the acceptance tests, and each one is run and its result reported in the
implementation note. A reviewer can re-run T15 and T17 from the plan alone.
