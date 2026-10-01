# Specification #636 - Roadmap: smaller gaps found in the Taskativ comparison

- Ticket: https://github.com/sebastienferry/sectile/issues/636
- Branch: `feat/636`
- Clarification: `docs/clarifications/636.md` (rounds 1 and 2, confirmed by
  the owner), plus the facts and decisions taken at specification time listed
  under D4 to D6
- Framework: Spec Kit

## Summary

Two small gaps from the #619 Taskativ comparison (rows 2 and 47).

1. The header's global search stops narrowing the tickets the Timeline reads,
   as it already does not narrow the Roadmap. The Timeline keeps its sprints,
   counters and backlog whole, and its backlog keeps its own search field.
2. The framing comment of a Jira epic is published as one comment on that
   epic, rewritten in place after each save and never posted again.

## Decisions this specification applies

- **D1 Jira only** (owner, round 2). The framing is published on Jira epics
  only. On GitHub (a milestone takes no comment), GitLab (a pair of labels)
  and local projects nothing is written, and the framing panel says that the
  framing stays in Sectile. The milestone description is not used as a
  carrier.
- **D2 On every save** (owner, round 2). Each save of the framing by a person
  schedules the publication. No "Publier sur le ticket" button is added.
- **D3 Declared roadmap projects are never written** (round 2, ADR 0043). The
  framing of an epic from a declared roadmap project stays in Sectile,
  whatever `RoadmapAxisWrites` says. A bulk edit never schedules a
  publication.
- **D4 The slicing checklist is already published** (fact, specification
  time). Row 47 asked for the framing and the checklist. Since the
  clarification, #663 merged on `main`: it copies the todos of a macro on its
  tracker (one comment on a Jira epic, a block in a GitHub milestone
  description, ADR 0046). The checklist half of row 47 is therefore
  delivered, and this ticket publishes the framing only. The checklist
  comment of #663 is left as it is.
- **D5 Same mechanism as the todos copy** (specification time, reversible).
  The framing comment is found again, scheduled, queued, retried and reported
  exactly as #663's todo comment is: a comment Sectile owns, marked by a
  comment property rather than by a footer signature (ADF keeps no hidden
  marker; the property survives a hand edit), a short debounce, the tracker
  activity queue, a status line with **Republier**. This replaces round 1's
  "found again by a readable footer signature", which predates #663.
- **D6 An emptied framing rewrites, never deletes** (specification time,
  reversible). Round 1 said an empty framing "publishes nothing and deletes
  nothing". This holds when no comment exists yet: nothing is created. When a
  comment already exists, leaving the former framing on the epic would show
  text Sectile no longer holds, so the comment is rewritten to say there is no
  framing any more, as #663 does for an emptied list. It is never deleted.

## Scope

In scope: the Timeline exemption from the server search; the one-way framing
comment on Jira epics, its status line in the framing panel, the republish
route; French and English strings; a changelog line.

Out of scope:

- A new search on the Timeline: its backlog keeps its own search field.
- The parent filter on the Timeline (it still applies there, as today).
- Publishing the macro description, horizon, priority, quarter or readiness:
  they have their own tracker writes.
- The slicing checklist copy (D4): unchanged.
- Reading a hand edit of the framing comment back into Sectile, in any form.
- Any framing copy on GitHub milestones, GitLab, local projects, a Jira macro
  with a local `M-<n>` key, or a declared roadmap project's epic (D1, D3).
- A framing write through MCP: `get_macro` already returns the framing; no
  tool saves it.
- Moving the comment when a macro is migrated to another project.

## Definitions

- **Framing**: the macro's `framingComment` field, the Markdown text edited
  under **Commentaire de cadrage** in the roadmap panel.
- **Framing copy**: the Jira comment that shows the framing, written by
  Sectile only.
- **Copied macro**: a macro the framing copy applies to (FR6). Every other
  macro keeps its framing **in Sectile**, for a reason the panel states.
- **Copy body**: the framing as the comment shows it (FR8). Two saves that
  produce the same body make no second write.
- **Up to date**: the last body written successfully equals the body of the
  current framing.

## User stories

### US1 (P1) - The global search leaves the Timeline whole

As a person planning sprints, I want the header search to leave the Timeline
alone, so that a search typed in a ticket view does not empty my sprints.

1. Given a search typed in the header, when the person opens the Timeline,
   then the Timeline shows every ticket of every sprint and of the backlog, as
   with no search, and the sprint counters and progress bars count them all.
2. Given the Timeline is open, when the person types in the header search,
   then the Timeline's tickets, counters and backlog do not change.
3. Given a search typed in the header, when the person is on the Timeline,
   then the header search field still shows the text and can still be cleared.
4. Given a search typed in the header and the Timeline open, when the person
   goes back to the board, the list or triage, then that search narrows the
   tickets again, with no retyping.
5. Given the Timeline's backlog search field, when the person types in it,
   then the backlog is narrowed as today, whatever the header search holds.
6. Given the Roadmap, then its behaviour with a header search is unchanged.
7. Given a parent filter set (#630), then it applies to the Timeline as it
   does today.

### US2 (P1) - The framing appears on the Jira epic

As the owner of a Jira epic, I want its framing to show on the epic, so that
people who never open Sectile read why the epic exists and how it is cut.

1. Given a copied Jira epic with no framing comment yet, when the owner saves
   a non-empty framing, then within a few seconds one comment appears on the
   epic holding the framing, and Sectile remembers it.
2. Given a Jira epic whose framing is already copied, when the owner saves a
   changed framing, then that same comment is rewritten; no second comment is
   created.
3. Given a save that leaves the copy body unchanged, then no tracker write is
   made.
4. Given a person edited the framing comment by hand on Jira, when the
   framing is next written, then the hand edit is replaced by the framing, and
   Sectile's framing never changed because of it.
5. Given a person deleted the framing comment on Jira, when the framing is
   next written, then a new comment is created and remembered.
6. Given a Jira epic carrying other people's comments and the todos comment of
   #663, when the framing is written, then no other comment is read as the
   framing copy or changed, and the todos comment is untouched.
7. Given the framing is emptied and saved, when a framing comment exists,
   then it is rewritten to say the epic has no framing any more; when none
   exists, nothing is written (D6).
8. Given the framing is saved in Sectile, then the save succeeds and the
   panel shows the new framing at once, whatever the tracker does.
9. Given a description, horizon, priority, quarter, readiness or todos save
   that leaves the framing as it is, then no framing write is scheduled.

### US3 (P1) - The framing stays in Sectile where no epic can carry it

As the owner of a macro that is not a Jira epic of the project, I want to be
told the framing stays in Sectile, so that I do not look for it on the
tracker.

1. Given a GitHub milestone macro, a GitLab macro, a local project's macro or
   a Jira macro with a local `M-<n>` key, when the owner opens the framing
   section, then a line under it says **Reste dans Sectile** with the reason,
   and saving the framing makes no tracker call.
2. Given the epic of a declared roadmap project, whatever its project's
   `RoadmapAxisWrites` option, then the same line shows the ADR 0043 reason,
   and saving the framing makes no tracker call.
3. Given a bulk edit of macros (`bulk: true`, such as the title seeding),
   then no framing write is scheduled, even for a copied macro.

### US4 (P1) - Failures are visible and retried

As the owner, I want to know when the epic shows a framing Sectile no longer
holds, so that I never believe the tracker is current when it is not.

1. Given a copied macro, when the panel shows its framing section, then a line
   under it says one of: **Recopié sur KEY** with a link to the comment,
   **Publication en attente**, **Échec de publication : reason**.
2. Given a transient tracker failure (network, rate limit, server error),
   when the write is attempted, then it is retried a few times before it is
   reported.
3. Given a write that failed, when the owner looks at the activity, then a
   failed activity names the epic and the reason.
4. Given the acting person has no tracker token for the project, when the
   write runs, then it fails with the missing-token reason, the framing stays
   saved in Sectile, and the panel offers to add the token as the todos copy
   does (#645).
5. Given the copy is not up to date, when the owner uses **Republier** on the
   line, then a write is queued at once.
6. Given a failure, when the owner saves the framing again, then the write is
   attempted again.
7. Given the write succeeded, when the activity ends, then the line turns to
   **Recopié sur KEY** without a page reload.

## Functional requirements

### Timeline search

- **FR1** The ticket query sent to the server omits the header search on the
  views that filter their own rows, the Roadmap and the Timeline, named as one
  set. Every other view sends it as today.
- **FR2** The header search field, its value and its clear button are
  unchanged on the Timeline. Leaving the Timeline for a ticket view refetches
  with the search.
- **FR3** The parent filter and every other filter are unchanged on the
  Timeline.

### Framing copy

- **FR4** A save of the framing by a person, through the macro save of the
  panel, schedules the framing copy of that macro when it is a copied macro
  (FR6) and the save is not a bulk edit. Saving never waits on the tracker.
  A save that does not carry the framing schedules nothing.
- **FR5** Scheduling is debounced per macro as the todos copy is: the write
  starts a few seconds after the last save, renders the framing as it is when
  the write runs, and goes out as the person whose save scheduled it, through
  the tracker activity queue.
- **FR6** A macro is a copied macro when, and only when, its project is a
  Jira project, its key belongs to that project, it is not a local `M-<n>`
  key, and the tracker can write marked comments. Every other macro keeps its
  framing in Sectile, with its reason: a GitHub milestone (no comment), a
  GitLab macro, a local project, a local `M-<n>` key, a declared roadmap
  project's epic, or a tracker that cannot write comments.
- **FR7** On Jira, Sectile rewrites the comment it created for the framing and
  remembers it. When that comment no longer exists, it finds a comment of the
  epic carrying the framing marker, or else creates a new one. It never edits
  or deletes a comment it did not create for the framing, the todos comment
  included.
- **FR8** The copy body is, in this order: a heading naming Sectile and the
  framing, the framing Markdown as saved, then one line saying the framing is
  kept in Sectile and that an edit made on the tracker is replaced. An empty
  framing renders as the heading and a line saying there is no framing (D6).
- **FR9** The body fits the comment size limit of the todos copy: a longer
  framing is cut at the budget and ends with a line saying the full text is in
  Sectile.
- **FR10** Transient failures are retried within the write, three attempts in
  all, with a growing pause; any other failure is reported at once. The last
  failure is kept and shown until a write succeeds.
- **FR11** An empty framing never creates a comment: a write with no comment
  remembered, none found by its marker, and an empty framing makes no tracker
  write and reports that there is nothing to copy.

### Status and retry

- **FR12** The macro, as the server returns it to the web app and to MCP
  (`get_macro`), carries a framing copy status shaped as the todos copy
  status: the kind (Jira comment, or none with its reason), whether it is up
  to date, the last failure, the missing-token tracker if any, the time of the
  last successful write and the address of the comment.
- **FR13** The framing section of the panel shows the line of US4.1 or US3,
  and **Republier** whenever a copied macro is not up to date. Republier
  queues a write at once, forcing it even when the body is unchanged.
- **FR14** A republish request for a macro that keeps its framing in Sectile
  is refused with its reason, and nothing is queued.

## Non-functional requirements

- **NFR1** No tracker call runs inside the HTTP request that saves the
  framing.
- **NFR2** No new child process, so no new console window on Windows.
- **NFR3** Server-side strings (activity, refusals, reasons, comment body) are
  in French like the rest of the product's runtime text; web strings exist in
  French and English.
- **NFR4** The schema change is a numbered migration; existing SQLite and
  PostgreSQL databases keep working.

## Acceptance criteria

- **AC1** US1 to US4 pass as written.
- **AC2** With a header search that matches no ticket, the Timeline shows the
  same sprints, counters and backlog as with no search.
- **AC3** On a Jira epic, ten successive framing saves leave exactly one
  framing comment, showing the last framing, beside an untouched todos
  comment.
- **AC4** A GitHub, GitLab, local or declared roadmap project's macro shows
  **Reste dans Sectile** with its reason, and no tracker call is made for its
  framing.
- **AC5** `CHANGELOG.md` has lines under `## [Unreleased]`: under `Added`,
  the framing of a Jira epic published as a comment; under `Fixed`, the
  header search no longer emptying the Timeline.

## Open points

None. The owner answered both product questions in round 2. D4 to D6 are
facts or reversible choices taken at specification time and are reported to
the owner with this stage.
