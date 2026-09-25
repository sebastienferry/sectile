# Aligned run indicators in the Desktop sidebar

Scope restated from the clarification (`docs/clarifications/446.md`, confirmed
by the owner after two rounds). The implementation choices live in `plan.md`.

## User stories

### P1: The run states form one column

As someone scanning the Desktop sidebar, I want the run-state glyph of every
execution row to sit at the same horizontal position, whatever the length of the
task number and whether the row is a free console, so that I can read the states
of all my runs down one column, as the tickets pane already lets me.

### P2: The titles form one column too

As the same reader, I want the titles of the rows to start at the same
horizontal position, so that the list reads as a table rather than as ragged
lines.

## Functional requirements

1. **Order.** In every Desktop sidebar execution row, the elements appear in this
   order: run-state glyph, task number, title, skill-result badge, then the
   existing trailing controls (pull-request indicator, archive, actions menu),
   which are unchanged.
2. **State column.** The run-state glyph is the first element of the row. In
   rows of the same project group, all glyphs start at the same x position,
   including free-console rows, macro-run rows and rows whose task number is
   long.
3. **Key column.** The task number occupies a column of one common minimum
   width, rendered with tabular (fixed-width) figures, so that the titles of the
   rows start at the same x position whenever every key fits that width.
4. **Long keys.** A task number wider than the common width is shown in full: it
   is never truncated nor ellipsised. It pushes its own row's title to the right;
   the other rows keep their alignment.
5. **Free console.** A free-console row, which has no task number, reserves the
   same empty width, so that its glyph and its title fall in the same columns as
   the other rows. It still shows no task-number control.
6. **Task-number behaviour unchanged.** The task number stays a separate control
   that opens the task in Sectile, stays disabled for a macro run, and keeps its
   tooltip ("Open task in Sectile") and its accessible name ("Open <key> in
   Sectile").
7. **Row behaviour unchanged.** Clicking the title, the skill badge or the
   run-state glyph still selects the run, as before this change. The glyph keeps
   its tooltip and accessible name `Process: <state label>`, and the row keeps
   its tooltip summarising title, run, state and execution count.
8. **Live updates unchanged.** The glyph of a row keeps following the run state
   reported by polling (running, waiting, queued, finished, failed, cancelled),
   including while the row order is held, and the running glyph keeps its pulse
   (and none under reduced motion).
9. **Out of scope, unchanged:** the toolbar header, the tickets pane, the glyph
   sizes, the colours, the pulse, the web board, and the server.
10. **Changelog.** `CHANGELOG.md` carries one line for this change under
    `## [Unreleased]`, in the section `Changed`.

## Acceptance scenarios

- **Given** a project group listing a run for `#42`, a run for `#446` and a run
  for macro `M-7`, **when** the sidebar is shown, **then** the three run-state
  glyphs have the same left x coordinate, and the three titles have the same
  left x coordinate.
- **Given** a free console listed with a task run in the same group, **then**
  both glyphs share one left x coordinate, both titles share one left x
  coordinate, and the free-console row contains no task-number control.
- **Given** a row whose task number is wider than the common key width (for
  example `#123456`), **then** its key is shown in full, its glyph is still
  aligned with the other rows, and only its own title starts further right.
- **Given** any execution row, **then** its first element is the run-state glyph,
  followed by the task number (or the reserved empty width), then the title and
  the skill-result badge.
- **Given** a task run row, **when** the user clicks its task number, **then**
  the task opens in Sectile and the run is not selected; **given** a macro-run
  row, **then** its task number is disabled.
- **Given** a row that is not selected, **when** the user clicks its run-state
  glyph, **then** the run becomes selected.
- **Given** a running execution whose status changes through polling to waiting,
  queued, finished, failed and cancelled, **then** the row glyph follows each
  state, and it pulses only while running and not under reduced motion.

## Out of scope

- The toolbar header and the tickets pane layouts.
- Glyph sizes, colours, the pulse animation, and which states pulse.
- The web board badges and any server-side run status.

## Open questions

None. The clarification settled every product choice. The exact value of the
common key width is an implementation choice (see `plan.md`).
