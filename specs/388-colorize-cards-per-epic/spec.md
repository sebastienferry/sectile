# Colour board cards per epic

Scope restated from the clarification (`docs/clarifications/388.md`): this is a
permanent behaviour, not an option. There is no toggle, no stored preference
and no default to choose, although the ticket title says "add an option".

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
3. Nothing is persisted: no new field, setting, API or migration.
4. Board, expanded card: a coloured bar on the card's left edge and a coloured
   dot in front of the parent key.
5. Board, condensed card: the coloured bar only (the condensed card does not
   show the parent key).
6. Backlog row: a coloured bar on the row's left edge and a coloured dot in
   front of the parent key badge.
7. Sprint timeline, chip form: a coloured dot in front of the task key.
   Sprint timeline, list form: a coloured bar on the row's left edge and a
   coloured dot in front of the parent title. This holds in the sprint blocks,
   in the planning view and in the unscheduled backlog pane.
8. Roadmap: a coloured bar on the left edge of each macro row, expanded and
   condensed; the tasks listed under a macro get no per-row colour.
9. The card and row border and background are unchanged, so the running,
   queued, selected, checked and dragging states keep their current rendering.
10. Colour is never the only carrier of information: wherever the parent key or
    title is shown today it stays shown, and the dot carries the epic key in
    its accessible name.
11. A task without a parent renders exactly as it does today on every surface.

## Acceptance scenarios

- Given two board cards whose parent key is `#12`, when the board renders,
  then both carry the same bar colour, and a card whose parent is `#40` carries
  the colour computed for `#40`.
- Given a card without a parent key, when the board renders, then it has no
  bar and no dot, and its markup is the same as before the change.
- Given an expanded card with parent `#12`, when it renders, then a dot of
  the `#12` colour precedes the `#12` link and the link still filters by parent.
- Given a condensed card with parent `#12`, when it renders, then its left
  edge carries the `#12` colour and no parent key text is added.
- Given a running, queued or selected card with a parent, when it renders,
  then its border and ring are the ones it had before the change.
- Given the task `#7` with parent `#12` in the Backlog, the sprint timeline
  and the Board, when each view renders, then `#12`'s colour is the same in
  all three, and the Roadmap macro row `#12` carries that colour too.
- Given a key with surrounding whitespace, `" #12 "`, when its colour is
  computed, then it is the colour of `#12`.

## Non-goals

No colour picker, no per-epic stored colour, no toggle or preference, no change
to parent import, filtering or editing, no backend, schema or dependency
change.

The change is visible to users, so it carries a `CHANGELOG.md` entry under
`[Unreleased]`.

## Open requirements

None. Every product question was settled in the clarification rounds 2 and 3.
