# Specification #626 - Roadmap: keep and show an epic's labels

- Ticket: https://github.com/sebastienferry/sectile/issues/626
- Branch: `claude/clarify-issue-gh-11a4f59c-855f76`
- Clarification: `docs/clarifications/626.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit
- Reference: Taskativ `openspec/changes/taskativ-38-macro-labels-in-roadmap`
  (keep, show, filter); editing is Sectile's own addition.

## Summary

The roadmap sync reads each tracker epic's labels and throws them away once it
has found the horizon. With this change Sectile keeps them. The roadmap row
shows an epic's free labels as badges, the toolbar filters the roadmap on them,
and the epic's panel lets the user add and remove them. The labels the roadmap
owns for its own axes, `roadmap:<horizon>` today, are never shown as free
labels and cannot be edited there. The later classification axes (#627, #635)
build on this.

## Scope

In scope: projects whose tracker reads epics, which is Jira today. That covers
the sync of the epic labels, the roadmap row, the roadmap toolbar, the epic's
panel, the French and English strings, and the changelog.

Out of scope:

- GitHub projects, whose macros are milestones and carry no labels, and GitLab
  projects, whose epics are not read. There, badges, filter and editor are
  absent. Reading GitLab epics would be a separate ticket.
- The priority and quarter axes (#627), per-project axis prefixes (#635), the
  maturity counter on the row (#629).
- Renaming a label across several epics, or deleting it from the tracker.
- The application-wide label filter of the board, which keeps filtering
  tickets as it does today.
- The child tickets' `phase:` and `goal:` groupings in the panel.

## Definitions

- **Epic labels**: the labels the tracker returns on the epic work item, in
  the order it returns them.
- **Axis label**: an epic label under a prefix the roadmap owns. Today the only
  prefix is `roadmap:`. The match ignores case and a leading `#`.
- **Free label**: an epic label that is not an axis label.

## User stories

### US1 (P1) - The sync keeps an epic's labels

As a roadmap user on a Jira project, I want Sectile to remember the labels my
epics carry on the tracker, so that the roadmap can show them and filter on
them.

1. Given a Jira epic carrying `domain-billing`, `client-acme` and
   `roadmap:next`, when the project syncs, then Sectile records all three on
   that epic, in the tracker's order, the horizon label included.
2. Given an epic whose labels are known, when its horizon, description,
   framing or slicing is saved from Sectile, then its recorded labels are
   unchanged.
3. Given an epic whose `client-acme` label was removed on the tracker, when the
   project syncs again, then Sectile no longer records `client-acme` on it.
4. Given an epic whose every label was removed on the tracker, when the
   project syncs again, then Sectile records no label on it.
5. Given a database created before this change, when the server starts, then
   every existing macro reads as carrying no label until the next sync, and
   nothing else about it changes.
6. Given a macro moved to another project, when the move completes, then its
   recorded labels move with it.

### US2 (P1) - The roadmap row shows the free labels

As a roadmap user, I read an epic's free labels on its row without opening it.

1. Given an epic carrying free labels, when the roadmap shows its row in the
   unfolded shape, then each free label appears as a badge, in the recorded
   order.
2. Given the same epic in the condensed row shape, then at most the first two
   free labels appear as badges, followed by a `+n` counter when more exist;
   the counter's tooltip lists the hidden ones.
3. Given an epic carrying `roadmap:now`, then no badge shows `roadmap:now`:
   the tab already says it.
4. Given an epic with no label, or with only axis labels, then its row shows
   no badge and keeps its usual height.

### US3 (P1) - Filter the roadmap on epic labels

As a roadmap user, I narrow the roadmap to the epics carrying some labels.

1. Given epics carrying free labels, when I open the roadmap, then the toolbar
   offers a label filter listing every free label carried by the epics the
   other filters (closed epics, search) let through, across every horizon
   tab, each with the number of epics carrying it, sorted by label.
2. Given the filter, when I select one label, then the list shows only the
   epics carrying it, and the tab counters count only those.
3. Given two selected labels, then an epic carrying either of them stays
   visible.
4. Given selected labels, when I clear the filter, then the list shows every
   epic the other filters let through.
5. Given no epic of the view carrying a free label, then the filter does not
   appear in the toolbar.
6. Given a selected label, when another filter changes so that no epic of the
   view carries it any more, then that label stops being selected, rather
   than the list emptying with no visible reason.
7. Given a selected label, when I leave the roadmap and come back, then no
   label is selected.
8. Given selected labels, then each one appears among the active filter chips
   with a control that clears it.
9. Given the filter, then axis labels are never offered in it.

### US4 (P2) - Edit an epic's free labels

As a roadmap user on a Jira project, I add and remove an epic's free labels
from its panel, and the tracker is updated.

1. Given an epic of the project opened in the panel, then its free labels
   appear as chips, each with a remove control, followed by an input to add
   one.
2. Given the input, when I type, then it suggests the free labels already
   carried by the project's epics that match what I typed and that this epic
   does not carry yet.
3. Given the input, when I confirm `client-acme`, then a tracker write is
   queued, the activity reads "Labels de <key>" with what is added and
   removed, and once it succeeds the chip appears on the panel and the badge
   on the row.
4. Given a chip, when I remove it, then a tracker write removing only that
   label is queued, and once it succeeds the chip and the badge disappear.
5. Given a write that the tracker refuses or that cannot reach it, then the
   epic's recorded labels are unchanged, the failed activity shows the reason,
   the usual failure toast appears, and nothing is left to push again later.
6. Given an edit, then the other labels of the epic, axis labels included,
   are left as the tracker holds them.
7. Given a label under an axis prefix, such as `roadmap:later`, when I try to
   add it, then it is refused with a message saying that this label belongs
   to the roadmap and is set from the horizon tabs; no write is queued.
8. Given a label that is empty after trimming, or that contains a space, when
   I try to add it, then it is refused with a message saying why; no write is
   queued.
9. Given a label the epic already carries, ignoring case, when I add it, then
   nothing is queued.
10. Given a milestone macro, a local macro or an epic of another project, then
    its free labels, if any, are shown read-only, with the sentence Sectile
    already gives for why that macro cannot be labelled.
11. Given a request that bypasses the view and asks the server to add or
    remove an axis label, then the server refuses it the same way.

## Functional requirements

- **FR1** The epic sync records the full label list of each epic it reads, as
  the tracker returns it, replacing the previous list. An epic read with no
  label records an empty list.
- **FR2** Every other write of an epic's local data leaves its recorded labels
  unchanged.
- **FR3** The macro data returned to the clients carries the recorded labels,
  an empty list when there are none.
- **FR4** A macro moved to another project keeps its recorded labels.
- **FR5** The roadmap row shows the free labels as badges: all of them in the
  unfolded shape, at most two plus a `+n` counter in the condensed shape.
- **FR6** Axis labels are never shown as badges, never offered by the filter,
  and never accepted by the editor, on the client and on the server alike.
  The list of axis prefixes is defined once on the server and holds
  `roadmap:`.
- **FR7** The roadmap toolbar offers a multi-select label filter, local to the
  roadmap and not persisted, with OR semantics, fed by the free labels of the
  epics the other roadmap filters let through, each with its epic count. The
  tab counters follow it. It is hidden when there is nothing to offer, and a
  selected label that is no longer offered is dropped.
- **FR8** The epic's panel lists the free labels as removable chips and offers
  an input with suggestions from the project's epics' free labels.
- **FR9** An edit sends to the tracker only the labels added and removed,
  through a queued tracker activity, as the horizon label does.
- **FR10** The recorded labels change only once the tracker write succeeded.
  A failed write changes nothing locally and leaves nothing pending.
- **FR11** The server refuses, without queuing anything: an axis label, an
  empty label, a label containing whitespace, an edit with nothing to add or
  remove, and a macro that cannot be labelled (milestone, local key, epic of
  another project, tracker without label support), with a French sentence
  saying why.
- **FR12** Badges, filter and editor are absent for projects whose tracker
  does not read epics.
- **FR13** Every new string exists in French and English.
- **FR14** `CHANGELOG.md` carries one `Added` line under `Unreleased`.

## Acceptance criteria

- **AC1** US1.1 to US1.6 hold, checked by Go tests on SQLite and PostgreSQL.
- **AC2** US2 and US3 hold, checked by unit tests on the roadmap helpers and
  by a manual check on a Jira project.
- **AC3** US4.3 to US4.11 hold, checked by Go tests on the new endpoint and
  the new tracker activity against a fake tracker, and US4.1, US4.2 and
  US4.10 by a manual check.
- **AC4** A GitHub project's roadmap looks exactly as before.
- **AC5** `go test ./...`, the web type check, lint and `node --test` pass.
- **AC6** `CHANGELOG.md` has the `Added` line.

## Open points

None. Both product questions were settled in round 2 of the clarification.
