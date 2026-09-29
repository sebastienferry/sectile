# Specification #619 - Compare Taskativ and Sectile roadmap features

- Ticket: https://github.com/sebastienferry/sectile/issues/619
- Branch: `claude/clarify-issue-gh-11a4f59c-217c0f`
- Clarification: `docs/clarifications/619.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

Taskativ, Sectile's predecessor, kept growing its Roadmap and Timeline views
after Sectile forked from it. Some of that work was ported, some was not, and
nobody holds the list. This ticket delivers a study that lists every roadmap
and timeline feature Taskativ has, says whether Sectile covers it, and, for
each gap, names the Sectile ticket that already tracks it or proposes a new
one.

## Scope

In scope:

- The Roadmap view and the Timeline (sprint timeline) view of Taskativ.
- The epic data those two views display or write: epic sync, epic labels,
  horizons, priority, quarter, axes and their tracker fields, epic template
  fields, readiness, remote roadmap projects.
- Comparison in one direction: what Taskativ has that Sectile lacks or covers
  partially.

Out of scope:

- Porting or fixing any feature. The study only describes and proposes.
- The macro workflow skills (`refine-macro`, `realign-macro`, macro worktrees),
  ported by #423 and #426.
- Taskativ features outside the two views (kanban, triage, terminal, skills,
  translation, quality gate).
- What Sectile does better than Taskativ.

## User stories

### US1 (P1) - Read which Taskativ roadmap features Sectile lacks

As the owner, I open one document and see, for every Taskativ roadmap or
timeline feature, whether Sectile has it.

1. Given the study, when I read its comparison table, then each row names one
   Taskativ feature in a sentence a user understands, its Taskativ source (an
   openspec spec or change directory, or a Taskativ file when no openspec entry
   exists), a Sectile status among `covered`, `partial` and `missing`, and the
   Sectile file that shows the status.
2. Given a row marked `partial`, when I read it, then it says in one sentence
   what Sectile lacks compared with Taskativ.
3. Given a row marked `missing`, when I read it, then its Sectile evidence says
   where the feature would belong (the view or file), or "no counterpart".
4. Given the Taskativ openspec entries listed in the inventory rule (FR-2),
   when I compare them with the table, then each one appears in at least one
   row, or in the "Excluded entries" list with the reason it falls outside the
   scope.
5. Given a feature Taskativ has without an openspec entry (for example the
   NOW/NEXT horizons or the timeline itself), when it belongs to the two
   views, then it has its own row, sourced from the Taskativ file.

### US2 (P1) - See the gaps mapped to existing Sectile tickets

As the owner, I do not want duplicate tickets: each gap points to the Sectile
ticket that already tracks it.

1. Given a `partial` or `missing` row, when a Sectile GitHub issue, open or
   closed, tracks that gap, then the row names it (`#430`) and its state.
2. Given a gap tracked by a closed issue, when the study reads it, then the
   row says whether the closed issue delivered the feature (and the status is
   then re-checked) or closed without it.
3. Given a gap no Sectile issue tracks, when I read the row, then its ticket
   column reads "none" and the gap appears in the proposed tickets list (US3).

### US3 (P2) - Validate a ranked list of new tickets

As the owner, I get a short list of tickets to create for the uncovered gaps,
ranked, and nothing is created before I say so.

1. Given the gaps with no ticket, when I read the "Proposed tickets" section,
   then each proposal carries a title in the project's `Area | Summary` style,
   the rows it covers, a two to four sentence description written for a user,
   and a rank.
2. Given several gaps that only make sense together, when they are proposed,
   then they are grouped into one ticket rather than one per row.
3. Given the list, when the study is delivered, then no ticket has been created
   on the tracker.
4. Given the owner's explicit approval of the list, in whole or in part, when
   the tickets are created, then only the approved ones are created, each
   linked back to #619, and the study records their numbers next to the
   proposals.

### US4 (P1) - A public document that leaks nothing internal

As the owner of a public repository, I publish the study without exposing the
company that uses Taskativ.

1. Given the study, when I search it, then it contains no Jira or GitLab
   project key of the company, no internal host name, no ticket key of the
   company's tracker, and no epic or ticket count taken from its data.
2. Given a Taskativ proposal that cites such data as its motivation, when the
   study summarises it, then the summary keeps the user need and drops the
   data.

## Functional requirements

- **FR-1** The study is one Markdown document in English in the Sectile
  repository's `docs/` folder, linked from `docs/README.md`.
- **FR-2** Inventory rule. The Taskativ inventory covers:
  - the openspec specs `roadmap-epic-grouping`, `roadmap-remote-projects`,
    `epic-quarter-label`, `epic-template-fields`, `tracker-priority-mapping`;
  - the archived changes 51, 65, 68 and 76, and the open changes
    `epic-priority-label`, 20, 29, 35, 36, 38, 52, 53, 54, 55, 80, 81, 82, 85,
    86, 87, 89, 90, 91, 92, 93 and 94;
  - any other Taskativ openspec entry whose proposal names the Roadmap or
    Timeline view, found by a search of `openspec/`;
  - the features of Taskativ's `web/src/components/RoadmapView.tsx` and
    `SprintTimeline.tsx` that no openspec entry describes.
  An entry that turns out to fall outside the scope is listed under
  "Excluded entries" with its reason, not silently dropped.
- **FR-3** Several Taskativ entries that deliver one user-visible feature
  collapse into one row, which cites all of them.
- **FR-4** Each status is backed by evidence read in Sectile's code on
  `origin/main` at the time of the study: a file path, with a line number when
  one location proves it. A status inferred from a file name alone is not
  evidence.
- **FR-5** The ticket mapping searches all Sectile GitHub issues, open and
  closed, and the study states the date and the search used.
- **FR-6** The study opens with a summary: counts of `covered`, `partial` and
  `missing` rows, and the three gaps it ranks highest.
- **FR-7** Ranking of proposals: first the gaps that other gaps depend on,
  then by user value (a feature used at every roadmap review before an
  occasional one), then by smaller size. The study states this rule.
- **FR-8** The study is read only on Taskativ: nothing is written in the
  Taskativ checkout.
- **FR-9** No `CHANGELOG.md` entry: the study changes nothing a Sectile user
  sees.

## Acceptance criteria

- **AC-1** Every entry of FR-2 appears in a row or in "Excluded entries".
- **AC-2** Every row has a status and a Sectile evidence path that exists on
  `origin/main`.
- **AC-3** Every `partial` or `missing` row has a ticket column filled with an
  issue number or "none", and every "none" is covered by a proposal.
- **AC-4** A search of the study for the company's project keys, host names
  and ticket keys returns nothing. The list searched for is built on the
  workstation and never committed, since the specification and the plan are
  public too.
- **AC-5** No ticket is created without the owner's explicit approval in the
  session or on #619.
- **AC-6** `docs/README.md` links the study.

## Open points

- **OP-1** The macro #619 belongs to is not readable from the task data
  (`parentType` is `macro` without a key). It only matters for US3.4: before
  creating approved tickets, ask the owner which macro they go under, or
  whether they go under none. It blocks nothing else.
