# ADR 0013: Labels carry the workflow stage on Jira, transitions only close

Status: Accepted

## Context

Ticket #180 reinstated a Jira tracker, this time as a `tracker.TicketingSystem`
adapter over the Jira Cloud REST API. Jira differs from the trackers Sectile
already drives on one point that decides the shape of the adapter: it has a real
workflow. A work item is in a named status, and it moves between statuses only
through the transitions the project's workflow allows from where it currently
is.

Sectile has its own six-stage workflow (`new`, `clarified`, `specified`,
`implemented`, `reviewed`, `finished`). On GitHub it carries that stage as a
label and uses the issue state only for the last one. On Jira, two readings were
available:

1. The stage is a label, exactly as on GitHub, and the Jira status is left to
   the humans working the board.
2. The stage is a Jira status, resolved through the project's column mapping
   (`trackerColumns` / `stageColumns`), and every stage change runs a transition.

The second is tempting: the board mapping already exists, and a Jira user
expects a card to move. It is also what the previous, removed integration
leaned towards.

## Decision

**Labels carry the stage. A transition runs only when the work item reaches its
end of life**, that is when a task is finished or deleted, or when a caller
names a target status explicitly (a column drag, `Writer.Transition`,
`UpdateIssueRequest.TargetStatus`).

Import is symmetric: the Sectile status comes from the workflow label when the
work item carries one, and from the Jira status category (`done` → finished)
otherwise. The Jira status name is still stored as `trackerStatus`, so the board
mapping keeps working for the views that use it.

The closing transition is chosen by **status category**, not by name: the first
transition whose target status is of the `done` category. A workflow offering
none is an error naming the work item and the transitions that were available,
never a silent no-op.

## Consequences

- A Jira project works from the first synchronisation, with no column mapping to
  configure. This matters because the mapping is per project and nobody has it
  on day one.
- Sectile never fails a stage change because a workflow forbids a move. A ticket
  that cannot go from "To Do" to "In Review" still advances in Sectile.
- The Jira board does not follow the agentic stage by itself. A team that wants
  that maps its columns and drags the card, which runs a real transition.
- The labels Sectile writes (`new`, `clarified`, …) appear on the Jira work
  item. They are the same spelling every tracker uses (#179), so a work item
  moved between trackers keeps its stage.
- Closing is the one place where Jira's workflow can refuse us. The error is
  explicit rather than swallowed, which is what lets a human fix the workflow or
  close the ticket by hand.

## Alternatives rejected

- **Status-driven stages.** Every stage change resolves a target status through
  the project's column mapping and runs a transition. Rejected: it gates a
  working Jira project on a per-project mapping, and it makes a normal stage
  advance fail whenever the workflow has no path from the current status.
- **Deleting instead of closing.** `DeleteIssue` could call Jira's delete
  endpoint. Rejected: it is irreversible and destroys history, where the board
  only means "this card is done". Closing is what the operation means here, and
  `CloseOnly` is therefore honoured whether it is set or not.
- **Matching the transition by its own name** ("Close Issue", "Done", "Terminer").
  Rejected: the transition's name is not its target status, it is localised, and
  it differs per workflow. The status category is the one thing Jira defines
  identically everywhere.
