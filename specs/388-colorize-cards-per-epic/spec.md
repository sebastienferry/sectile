# Colour board cards per epic

Scope restated from the clarification (`docs/clarifications/388.md`), then
revised after the first review (see its "Review revision" section): the colour
is a per-project setting, off by default, shown as a thin full-height bar
that follows the card's rounded corners.

## User stories

### P1 — Read a board column as a set of epics

As a board reader, I want every card that belongs to an epic to carry that
epic's colour, so that I can tell at a glance which cards of a column belong
together.

### P2 — Recognise the same epic across views

As a planner moving between the Backlog, the sprint timeline and the Roadmap,
I want an epic to keep one colour everywhere, so that the colour I learnt on
the board still means the same epic elsewhere.

## Functional requirements

1. A task whose `parentKey` is non-empty (after trimming) is given one colour
   of the existing accent palette. A task without a parent key gets none.
2. The colour depends on the parent key only. The same key gets the same
   colour in every view, for every user, across reloads, whatever other epics
   are present. Two epics may share a colour; that is accepted.
3. Each project has one setting, "Couleur par épic", in its settings under
   General. It is off by default, including on every existing project. It is
   stored as `projects.epic_colors`, added by numbered migration 3. With it
   off, every surface renders as it did before this change.
4. A task is painted according to the setting of its own project, so a view
   listing several projects paints each task by its project. A task that does
   not name its project follows the project on screen.
5. The colour is a 3px bar along the full height of the left edge, inside the
   border. It follows the curve of the card's rounded corners, whatever their
   radius, and never overflows them.
6. It is shown on the board cards (expanded and condensed), the Backlog rows,
   the sprint timeline chips and list rows, and the Roadmap macro rows.
7. The sprint timeline rules hold in the sprint blocks, in the planning view
   and in the unscheduled backlog pane.
8. On the Roadmap, the setting read is that of the project on screen; the tasks
   listed under a macro get no per-row colour.
9. The card and row border and background are unchanged, so the running,
   queued, selected, checked and dragging states keep their current rendering.
10. Colour is never the only carrier of information: wherever the parent key or
    title is shown today it stays shown, with its text.
11. A task without a parent renders exactly as it does today on every surface.

## Acceptance scenarios

- Given a project with the setting off, when any view renders, then no card,
  row, chip or macro carries an epic colour.
- Given a project with the setting on and two board cards whose parent key is
  `#12`, when the board renders, then both carry a bar of the same colour that
  follows their rounded corners, and a card whose parent is `#40` carries
  `#40`'s.
- Given a condensed card with parent `#12` on such a project, when it renders,
  then it carries the `#12` bar, no parent key text is added and its text is
  not covered.
- Given an expanded card with parent `#12`, when its key is clicked, then the
  board still filters by parent.
- Given a running, queued or selected card with a parent, when it renders,
  then its border and ring are the ones it had before the change.
- Given "all projects" selected, one project with the setting on and one with
  it off, when the board renders, then only the first project's cards carry
  epic colours.
- Given the task `#7` with parent `#12` in the Backlog, the sprint timeline
  and the Board, when each view renders, then `#12`'s colour is the same in
  all three, and the Roadmap macro row `#12` carries that colour too.
- Given a key with surrounding whitespace, `" #12 "`, when its colour is
  computed, then it is the colour of `#12`.
- Given a database migrated before this change, when the server starts, then
  migration 3 adds `epic_colors` with every project off.

## Non-goals

No colour picker, no per-epic stored colour, no per-user preference, no change
to parent import, filtering or editing, no dependency change.

The change is visible to users, so it carries a `CHANGELOG.md` entry under
`[Unreleased]`.

## Open requirements

None.
