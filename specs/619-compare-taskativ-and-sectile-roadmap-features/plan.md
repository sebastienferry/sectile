# Plan #619 - Compare Taskativ and Sectile roadmap features

## Nature of the change

Documentation only. No Go, TypeScript, migration or skill changes, and no
test code. The deliverable is one Markdown study, one link in the docs index
and, after the owner's approval, tracker tickets.

## Target files

| File | Change |
| --- | --- |
| `docs/taskativ-roadmap-comparison.md` | New: the study. |
| `docs/README.md` | Add one index entry for the study, next to the other standalone documents. |

Nothing is written in the Taskativ checkout (`~/Sources/apps/taskativ` on the
owner's workstation). `CHANGELOG.md` is left as it is (FR-9).

The name follows the flat `docs/` layout already used by
`db-concurrency-audit.md` and `pr-state-icons.md`; a `docs/studies/` folder
would hold a single file.

## Sources

### Taskativ (read only)

- `openspec/specs/<name>/spec.md` for the five specs of FR-2.
- `openspec/changes/<id>/proposal.md` (and `design.md`, `specs/` when the
  proposal is not enough) for the open changes of FR-2.
- `openspec/changes/archive/2026-09-*-taskativ-{51,65,68,76}-*` for the
  archived ones.
- `web/src/components/RoadmapView.tsx` and `SprintTimeline.tsx`, for the
  features that predate openspec (horizons NOW/NEXT/LATER, timeline, filters,
  details panel). Read by their component and handler structure, not line by
  line.
- `internal/db/epics.go`, `internal/db/roadmapprojects.go` and the tracker
  readers under `internal/tracker/`, for the epic data side.
- A search `grep -ril -E 'roadmap|timeline' openspec/` finds any entry FR-2
  does not list by number.

### Sectile (evidence, on `origin/main`)

- Web: `web/src/components/RoadmapView.tsx`, `SprintTimelineView.tsx`,
  `MacroLabelGroups.tsx`, `MacroTaskRow.tsx`, `web/src/lib/roadmap.ts`,
  `roadmapDisplayMode.ts`, `roadmapProjects.ts`, `sprints.ts`,
  `optionalViews.ts`, and the `planning` and `sprints` locales.
- Server: `internal/db/macros.go`, `macrohorizons.go`, `roadmapprojects.go`,
  `board.go`, `internal/models/models.go`, the tracker readers under
  `internal/tracker/`.
- ADR 0024 (optional views per project) and `docs/clarifications/266.md`, 273's
  history (Roadmap and Timeline reintroduced).

Evidence is taken from `origin/main` fetched at the start of the study, and
the study records that commit.

### Sectile tickets

`gh issue list --repo sebastienferry/sectile --state all --limit 1000
--json number,title,state,labels,body`, filtered on roadmap, timeline, epic,
macro, horizon, quarter, priority, sprint and axis in title or body, then read
one by one for the candidate rows. The study records the date and this query.

## Study layout

```
# Taskativ and Sectile roadmap features

<one paragraph: purpose, direction, date, Sectile commit, Taskativ commit>

## Summary
<counts per status, the three highest-ranked gaps>

## Comparison
| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
<one row per feature, grouped by: Roadmap layout, Epic classification
 (horizon, priority, quarter, axes), Epic data and sync, Remote projects,
 Epic template and readiness, Timeline>

## Excluded entries
<entry, reason>

## Proposed tickets
<ranking rule, then per proposal: rank, title, rows covered, description,
 created ticket once approved>

## Method
<inventory rule, ticket search and date, what "covered/partial/missing" mean>
```

Status meanings, stated in the Method section:

- `covered`: a Sectile user can do what the Taskativ feature lets them do,
  even if the interface differs.
- `partial`: part of it; the Gap column says which part is missing.
- `missing`: no counterpart.

## Leak check (US4)

Before committing, build a list of the company's identifiers on the
workstation, in `$TMPDIR`, from the Taskativ proposals read (project keys,
host names) and Taskativ's roadmap project settings, and search the study for
them together with the generic pattern `\b[A-Z][A-Z0-9]+-[0-9]+\b`
(tracker ticket keys). Sectile's own `#<n>` references do not match it. The
list is deleted afterwards and never committed.

## Ticket creation (US3.4)

Only after the owner approves the list, in the session or on #619:

- ask which macro the tickets go under (OP-1);
- create each approved ticket with `create_task` on project
  `11a4f59c-b6bd-4747-8ea5-0a10d5b78da4`, title and description taken from the
  study, description ending with "From the study of #619";
- write the returned numbers into the study's "Proposed tickets" section and
  commit that update.

Without approval during the implementation stage, the list stays a proposal
and the stage still completes; the report says so.

## Rejected alternatives

- **One row per Taskativ openspec entry.** Several entries refine one feature
  (quarter: 86, 87, 91, 92, 93); per-entry rows would repeat the same gap.
  FR-3 collapses them.
- **Creating the tickets during the study.** Refused by the owner at
  clarification.
- **Putting the study in the clarification file or the spec folder.** Both are
  stage artefacts; the study is a document meant to be read after #619 closes.
