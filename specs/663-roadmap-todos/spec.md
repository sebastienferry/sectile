# Specification #663 - Roadmap: ordered epic todos, mirrored on the tracker

- Ticket: https://github.com/sebastienferry/sectile/issues/663
- Branch: `feat/663`
- Clarification: `docs/clarifications/663.md` (rounds 1 and 2, confirmed by
  the owner), plus one decision taken at specification time (D5 below)
- Framework: Spec Kit

## Summary

A macro (an epic on the roadmap) already carries a slicing checklist: one-line
todos, each a pre-ticket for a story to come. With this change the owner can
reword a todo and reorder the list, the order being the intended order of
execution. Sectile mirrors the list on the tracker, one way: a comment on the
Jira epic, a block at the end of the GitHub milestone description. The list can
also be read and saved through the Sectile MCP server, and the `refine-macro`
skill saves the todos the owner confirmed instead of only printing them.

## Decisions this specification applies

- **D1 Sectile is the source of truth.** The tracker copy is a mirror Sectile
  rewrites. A hand edit of it on the tracker is overwritten at the next write
  and is never read back.
- **D2 Where the mirror lives.** Jira: one comment on the epic. GitHub: a
  marker-delimited block at the end of the milestone description, since a
  milestone takes no comments.
- **D3 When it is written.** Automatically after each save of the list,
  debounced and queued, retried and reported on failure.
- **D4 Rewording an attached line** changes the line only. Its story keeps its
  title.
- **D5 No mirror on GitLab** (decided by the owner during specification, on
  2026-10-01). A GitLab macro is a pair of labels with a local `M-<n>` key
  (ADR 0030), so nothing exists on GitLab that could carry the comment. On
  GitLab the list stays in Sectile, and the panel says so.

## Scope

In scope: rewording and reordering todos in the macro's panel, the one-way
mirror on Jira epics and GitHub milestones, the mirror status in the panel,
the MCP tools `get_macro` and `update_macro_todos`, the `refine-macro` skill
saving the confirmed list, French and English strings, a changelog line and an
ADR.

Out of scope:

- How a todo becomes a story, one by one or as a batch (#634): unchanged.
- The readiness and the axes of an epic (#633, #635).
- The heuristic **Refine** button of the panel and its preview.
- Reading the tracker copy back into Sectile, in any form (D1).
- Any write on an epic of a declared roadmap project (ADR 0043).
- A mirror on GitLab (D5), on local projects, and on a macro with no tracker
  object behind it.
- Renaming a story when its line is reworded (D4).
- Moving the mirror when a macro is migrated to another project: the target
  gets its own mirror at the next save of its list.

## Definitions

- **Todo**: one line of a macro's slicing checklist (`MacroTodo`), with its
  text, its `done` state, and, once attached, its story key.
- **Order of execution**: the order of the todos in the list, top first. It is
  the stored order; no separate rank exists.
- **Mirror**: the tracker copy of a macro's todos, written by Sectile only.
- **Mirrored macro**: a macro the mirror applies to (FR12). Every other macro
  is **local only**, for a reason the panel states.
- **Mirror body**: the rendered list as the mirror shows it (FR14). Two saves
  that produce the same body make no second tracker write.
- **Up to date**: the last body successfully written on the tracker equals the
  body of the current list.

## User stories

### US1 (P1) - Reword a todo

As the owner of a macro, I want to change the wording of a todo in place, so
that the list says what the team now means without deleting and retyping it.

1. Given a todo in the panel, when the owner activates its text (click, or
   Enter on the focused line), then the text becomes an editable field holding
   the current wording, with the cursor at its end.
2. Given a todo being edited, when the owner presses Enter or leaves the field
   with a changed, non-blank text, then the new wording is saved, trimmed, and
   the line keeps its place, its `done` state, its target and its story key.
3. Given a todo being edited, when the owner presses Escape, or leaves the
   field with the text unchanged, then nothing is saved and the former wording
   is shown.
4. Given a todo being edited, when the owner empties it and confirms, then
   nothing is saved, the former wording comes back, and the line is not
   deleted: deleting stays the line's delete button.
5. Given a todo attached to a story, when the owner rewords it, then the line
   shows the new wording and the story, on the board and on the tracker, keeps
   its title.
6. Given a batch of story creation running on the macro, when the owner looks
   at the todos, then rewording is disabled, as every other edit of the list is
   (#634, US4.5).

### US2 (P1) - Reorder the todos

As the owner, I want to put the todos in the order they should be done, so
that the list reads as a plan.

1. Given a list of several todos, when the owner drags a todo by its handle
   and drops it between two others, then it is saved at that place and the
   others keep their relative order.
2. Given a focused todo, when the owner uses **Monter** or **Descendre** (its
   buttons, or Alt+Up and Alt+Down), then it swaps with its neighbour, is
   saved, and keeps the focus.
3. Given the first todo, then **Monter** is disabled; given the last, then
   **Descendre** is disabled; given a single todo, then no handle and no move
   control is offered.
4. Given a reordered list, when the panel is reloaded, or another person opens
   the macro, then the todos appear in the saved order.
5. Given a batch of story creation running on the macro, then dragging and
   moving are disabled.
6. Given a reorder, then no todo gains or loses its story key, its `done`
   state or its target.

### US3 (P1) - The list is mirrored on the tracker

As the owner, I want the epic on the tracker to show the current todos, so
that people who never open Sectile see the plan.

1. Given a mirrored Jira epic with no mirror yet, when the owner saves its
   list, then within a few seconds one comment appears on the epic with the
   todos in order, and Sectile remembers it.
2. Given a Jira epic already mirrored, when the owner rewords, adds, removes,
   ticks or reorders a todo, then that same comment is rewritten; no second
   comment is created.
3. Given a GitHub milestone macro, when the owner saves its list, then the
   milestone description ends with a block holding the todos in order, and
   the text before the block is left as it was.
4. Given several saves in quick succession (a drag, then a rewording, then a
   tick), when they settle, then the tracker shows the last state, and fewer
   writes are made than saves.
5. Given a save that changes nothing the mirror shows (a todo's target project
   only, for example), then no tracker write is made.
6. Given a person edited the comment or the block by hand on the tracker, when
   the list is next written, then the hand edit is replaced by the list, and
   Sectile's list never changed because of it.
7. Given a person deleted the mirror comment on Jira, when the list is next
   written, then a new comment is created and remembered.
8. Given a Jira epic carrying other people's comments, when the mirror is
   written, then no other comment is read as the mirror or changed.
9. Given a story created from a todo (one or a batch), when its key is
   recorded on the line, then the mirror is rewritten and shows that key on
   the line.
10. Given a todo list imported from tasks.md, spec.md or the macro's stories,
    when the import saves it, then the mirror is rewritten as for any save.
11. Given the list becomes empty, then on Jira the comment says there is no
    todo, and on GitHub the block is removed from the description.

### US4 (P1) - The mirror and the description share a milestone on GitHub

As the owner of a GitHub project, I want the description I write in Sectile
and the todo block to live side by side, so that editing one never erases the
other.

1. Given a milestone carrying a todo block, when the owner edits the macro's
   description in Sectile, then the milestone description becomes the new
   description followed by the block of the current list.
2. Given a milestone carrying a todo block, when Sectile reads the milestone
   (first import of a macro), then the block is not part of the macro's
   description field.
3. Given an empty description in Sectile and a non-empty list, then the
   milestone description holds the block only.

### US5 (P1) - Failures are visible and retried

As the owner, I want to know when the tracker copy is behind, so that I never
believe the epic shows a list it does not.

1. Given a save, when the panel shows the macro, then a status line under the
   todos says one of: **Recopiée sur KEY** with a link to the comment or the
   milestone, **Publication en attente**, **Échec de publication: reason**, or
   **Reste dans Sectile: reason** for a local-only macro.
2. Given a transient tracker failure (network, rate limit, server error),
   when the write is attempted, then it is retried a few times before it is
   reported.
3. Given a write that failed, when the owner looks at the activity, then a
   failed activity names the macro and the reason, as an epic label write
   does.
4. Given the acting person has no tracker token for the project, when the
   write runs, then it fails with the missing-token reason, the list stays
   saved in Sectile, and the panel offers to add the token as other tracker
   writes do (#645).
5. Given the mirror is not up to date, when the owner uses **Republier** on
   the status line, then a write is queued at once.
6. Given a failure, when the owner saves the list again, then the write is
   attempted again.
7. Given a save while the tracker is unreachable, then the save itself
   succeeds and the panel shows the new list immediately.

### US6 (P1) - Read and save the todos through MCP

As an agent working on a macro, I want to read its todos and save an ordered
list, so that a skill can propose and record a slicing without a person
retyping it.

1. Given a project id and a macro key, when an agent calls `get_macro`, then
   it receives the macro's title, description, framing comment, horizon, its
   todos in order (id, text, done, story key, target, origin) and its mirror
   status.
2. Given an unknown project or an unknown macro, then `get_macro` and
   `update_macro_todos` are refused with a sentence naming which.
3. Given a full ordered list, when an agent calls `update_macro_todos`, then
   the macro's todos become that list in that order: a line carrying a known
   id keeps that line's story key and origin and takes the given text, `done`
   and target; a line without an id is created; a stored line the list omits
   is removed, unless it is linked to a story: then the call is refused as a
   whole, names that line, and nothing is saved (added by #647, decided by the
   owner: only the macro's panel removes a line linked to a story).
4. Given a list with a blank text, an id the macro does not have, or the same
   id twice, then the call is refused as a whole and nothing is saved.
5. Given a line carrying a story key in the call, then that key is ignored:
   only story creation attaches a line.
6. Given a call that names no user (an anonymous MCP key), then the call is
   refused as other MCP writes are.
7. Given a successful call, then it answers with the saved macro and the
   mirror status, and the mirror is scheduled as after a panel save.
8. Given an agent saves while the owner has the panel open, when the panel
   next refreshes the macro, then it shows the agent's list.

### US7 (P2) - `refine-macro` saves the confirmed list

As the owner running `refine-macro`, I want the todos I approved to land on
the macro, so that the skill's output is not lost in the session.

1. Given the skill proposed todos, when the owner confirms them in the
   session, then the skill saves them with `update_macro_todos`, keeping the
   existing todos (with their ids) unless the owner asked to drop some.
2. Given the owner did not confirm, then nothing is saved.
3. Given the save is refused, then the skill reports the refusal and the
   proposed list, and does not retry through another route.
4. Given the save succeeded, then the skill's report gives the saved order and
   the mirror status the tool returned.

## Functional requirements

### List editing (web)

- **FR1** A todo's text is editable in place (US1). A confirmed change saves
  the whole list through the existing macro save, with every other field of
  every line unchanged. A blank or unchanged text saves nothing.
- **FR2** Rewording never writes the attached story (D4).
- **FR3** The list is reorderable by drag and drop on a handle and by
  keyboard (Alt+Up, Alt+Down) or move buttons (US2). A move saves the list in
  its new order. The order of the stored list is the order of execution.
- **FR4** Rewording and reordering are disabled while a batch of story
  creation runs on the macro, like every other edit of its list.
- **FR5** A save that races a story-key record keeps the key, as today
  (#634, FR7b).

### Mirror

- **FR6** Every save of a macro's todos, from the panel, an import, a story
  creation that records a key, or MCP, schedules the mirror of that macro if
  it is a mirrored macro (FR12). Saving never waits on the tracker.
- **FR7** Scheduling is debounced per macro: the write starts a few seconds
  after the last save, and renders the list as it is when the write runs, not
  as it was when it was scheduled.
- **FR8** The write is queued through the tracker activity queue, as the
  epic's label writes are, and goes out as the person whose save scheduled it.
  It records an activity.
- **FR9** A write whose body equals the last body written on the tracker
  makes no tracker call and records nothing as a change.
- **FR10** On Jira, Sectile updates the one comment it created for the macro
  and remembers it. When that comment no longer exists, it finds a comment it
  created for that macro, or else creates a new one. It never edits or deletes
  a comment it did not create.
- **FR11** On GitHub, Sectile replaces only the marker-delimited block at the
  end of the milestone description, reading the description first. Text
  outside the block is left as the tracker has it. A missing block is
  appended after a blank line.
- **FR12** A macro is mirrored when, and only when:
  - its project's macros are GitHub milestones and its key is a milestone
    the tracker lists: GitHub description block;
  - its project is a Jira project and its key belongs to that project:
    Jira comment.

  Every other macro is local only, with its reason: a declared roadmap
  project's epic (ADR 0043), a GitLab macro (D5), a local project, a Jira
  macro with a local `M-<n>` key, a GitHub macro whose milestone does not
  exist, or a tracker that cannot write comments.
- **FR13** Transient failures (network error, timeout, HTTP 429 or 5xx) are
  retried within the write, three attempts in all, with a growing pause. Any
  other failure is reported at once. The last failure is kept and shown until
  a write succeeds.
- **FR14** The mirror body is, in this order: a heading naming Sectile and the
  todos, the todos as an ordered list in the order of execution, each with its
  `done` state as a checkbox, its text, and its story key when attached, then
  one line saying the list is maintained in Sectile and that an edit made here
  is replaced. On GitHub the body sits between an opening and a closing
  marker. An empty list renders as "no todo" on Jira and as no block on GitHub
  (US3.11).
- **FR15** The body fits the tracker's size limit: when the full list would
  exceed it, the body keeps the first todos that fit and ends with a line
  giving how many more are in Sectile.
- **FR16** On GitHub, a description push from Sectile writes the local
  description followed by the block of the current list (US4.1), and the
  description read from a milestone has the block removed (US4.2).

### Status and retry

- **FR17** The macro, as the server returns it to the web app and to MCP,
  carries a mirror status: the mirror kind (Jira comment, GitHub description,
  or none with its reason), whether it is up to date, the last failure if any,
  the time of the last successful write and the address of the mirror.
- **FR18** The panel shows the status line of US5.1 under the todos, and a
  **Republier** action whenever a mirrored macro is not up to date. The
  action queues a write at once, without debounce.

### MCP

- **FR19** `get_macro` takes `projectId` and `macroKey` and answers with the
  macro and its mirror status (US6.1). It writes nothing.
- **FR20** `update_macro_todos` takes `projectId`, `macroKey` and `todos`, the
  full ordered list; each item takes `id` (optional), `text`, `done`
  (optional, false by default), `targetProjectId` and `targetTrackerProject`
  (optional). It validates the whole list before saving anything (US6.4),
  saves through the same path as the panel, and answers as US6.7.
- **FR21** Both tools refuse an unknown project or macro, and
  `update_macro_todos` refuses an anonymous caller.

### Skill

- **FR22** The `refine-macro` skill reads the macro with `get_macro`, and,
  after the owner's confirmation in the session, saves the merged list with
  `update_macro_todos` (US7). Its guard no longer forbids saving; it forbids
  saving without the owner's confirmation and dropping existing todos the
  owner did not ask to drop.

## Non-functional requirements

- **NFR1** No tracker call runs inside an HTTP request that saves the list.
- **NFR2** No new console window on Windows: the change adds no child
  process.
- **NFR3** The server-side strings (activity, refusals, status reasons) are in
  French like the rest of the product's runtime text; the web strings exist in
  French and English.
- **NFR4** The schema change is a numbered migration; existing databases and
  both SQLite and PostgreSQL keep working.

## Acceptance criteria

- **AC1** US1 to US7 pass as written, on a Jira project and a GitHub project.
- **AC2** On a Jira epic, ten successive saves leave exactly one Sectile
  comment, showing the last list.
- **AC3** On a GitHub milestone, a description edit in Sectile and a list save
  in any order leave the description and the current block, both intact.
- **AC4** A GitLab macro, a local project's macro and a declared roadmap
  project's epic show **Reste dans Sectile** with their reason, and no tracker
  call is made for them.
- **AC5** An MCP `update_macro_todos` call with an unknown id saves nothing.
- **AC6** `CHANGELOG.md` has one line under `## [Unreleased]` / `Added`
  describing rewording, reordering, the tracker mirror and the MCP tools.

## Open points

None. The GitLab case the clarification had answered on a wrong premise is
settled by D5.
