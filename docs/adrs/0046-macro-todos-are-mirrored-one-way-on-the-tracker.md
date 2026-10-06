# ADR 0046: Macro todos are mirrored one way on the tracker

- Status: Accepted
- Date: 2026-10-01
- Issue: [#663](https://github.com/sebastienferry/sectile/issues/663)

## Context

A macro carries a slicing checklist, the todos that become its stories. It
lived in Sectile only: someone reading the epic on Jira or the milestone on
GitHub saw none of it. #663 makes the list reorderable, so its order becomes the
plan, and asks for that plan to be visible where the rest of the team reads the
epic.

Jira and GitHub can both carry a copy, in different places: a Jira epic takes
comments, a GitHub milestone takes none but has a description. GitLab macros
are a pair of labels with a local `M-<n>` key (ADR 0030), so nothing on GitLab
names the macro. A copy that people can edit on the tracker also raises the
question of which side wins.

## Decision

- **Sectile is the source of truth.** The tracker copy is a mirror: Sectile
  rewrites it after each save of the list and never reads it back. A hand edit
  on the tracker is replaced by the next write.
- **Jira: one comment Sectile owns on the epic.** Its id is remembered on the
  macro, and the comment carries a comment property, `sectile.macroTodos`, so it
  is found again when the id is lost. The marker is a property rather than text
  in the body because ADF keeps no HTML comment and a person editing the
  comment keeps the property. A comment without the property is never read as
  the mirror nor touched.
- **GitHub: a block at the end of the milestone description**, between
  `<!-- sectile:macro-todos -->` and `<!-- /sectile:macro-todos -->`. The job
  reads the description, replaces only the block and leaves the rest as GitHub
  has it. A description edit made in Sectile is sent with the block of the
  current list, and a milestone imported into Sectile has the block stripped,
  so the macro's description and the block never erase each other.
- **No mirror on GitLab, on local projects, on a macro without a tracker
  object, nor on a declared roadmap project's epic** (ADR 0043). Those macros
  keep their list in Sectile, and the panel gives the reason.
- **The write is debounced, queued and retried.** Each save schedules the copy
  a few seconds later on the instance that took the save, as the person who
  saved; the write goes through the tracker activity queue, renders the list as
  it is when it runs, retries network errors, timeouts, HTTP 429 and 5xx three
  times, and keeps the last failure on the macro until a write succeeds. A body
  equal to the last one written, compared by hash, makes no tracker call.
- **Nothing replays a lost write.** A restart drops the pending timers; the
  macro then reads as not up to date and the panel offers to publish it again.

## The framing of a Jira epic follows the same decision (#636)

The framing comment of a macro is copied on its Jira epic the same way: one
comment Sectile owns, rewritten after each save by a person and never read
back. It is a second comment, marked by its own property
(`sectile.macroFraming`) and remembered in its own `framing_mirror_*` columns,
so it never finds or rewrites the todos comment, and a ticked todo never
rewrites the framing text. Debounce, queue, retries and status are the todos
copy's.

- **Jira only.** A GitHub milestone takes no comment, and its description
  already carries the macro's description and the todo block: the framing
  stays in Sectile there, as on GitLab, local projects, local `M-<n>` keys and
  declared roadmap projects' epics, whatever `RoadmapAxisWrites` says.
- **A bulk edit never schedules it**, such as the title seeding.
- **An empty framing never creates a comment.** Emptied after a copy, the
  comment is rewritten to say there is no framing any more, never deleted.

## Consequences

- Each change of a list makes at most one tracker write per few seconds, and
  one activity a person can read when it fails.
- A comment on a Jira epic, or a block of a milestone description, says that it
  is maintained in Sectile and that an edit made there is replaced.
- Two instances may each queue a write after the same save; the hash makes the
  second one write nothing, so no cross-instance lock is needed.
- A hand edit on the tracker survives until the next change of the list or a
  republish, which writes whatever the hash says.
- Existing macros with todos read as not yet published until their list is
  saved again or published from the panel. The framings of a Jira project's
  epics can also be published all at once from the roadmap, in one activity
  signed by the person who asks (#691).
- Rejected: a two-way sync parsing the comment back (it would make the tracker
  a second source of truth), an HTML comment marker on Jira (ADF drops it), a
  GitLab carrier issue or group epic (nothing ties it to a label macro), a
  pending flag replayed at startup (it would sign a write for someone who may no
  longer be the one to make it, #482), and one write per save without debounce.
