# Ending a discussion, and readable run indicators

Scope restated from the clarification (`docs/clarifications/438.md`).

## User stories

### P1 — Ending a discussion reads as a normal ending

As someone who opened a discussion (the "Discussion (no skill)" launch) in
Sectile Desktop, I want stopping it or closing its console to report it as
finished, so that the normal way a discussion ends does not look like an
abnormal outcome.

### P2 — Each indicator says what it reports

As someone reading an execution row or the toolbar header, I want each
indicator to say whether it describes the process or the skill, and the running
state to be drawn as something that does not read as "done", so that I can tell
the two glyphs apart without having learned them.

### P3 — The state comes first

As someone scanning the sidebar, I want the run state before the title, as the
tickets pane already shows it, so that the state column lines up.

## Functional requirements

1. A discussion (`skill` `discuss`) ended by **Stop execution** (toolbar, or the
   "Stop and archive" action) or whose console has already closed when it is
   stopped ends **completed**: the local run and the run recorded by the server
   (`finish_run`) both say `completed`, and the Desktop shows "Finished" with the
   green check and announces "finished its turn".
2. A skill run stopped the same way still ends **canceled**, unchanged.
3. A discussion whose CLI exits with a non-zero code still ends **failed**, and
   one whose CLI exits with zero still ends **completed**.
4. A discussion that is stopped before its console ever started, or that the
   agent interrupts while shutting down, still ends **canceled**: nothing ran, or
   the user did not end it.
5. A discussion shows **no skill-result badge**, in the sidebar row and in the
   header, whatever the server recorded; a pending stop may still be reported,
   as for a free console.
6. The run-state indicator carries the tooltip `Process: <state label>` and the
   skill-result badge the tooltip `Skill: <result label>`, in the sidebar row
   and in the header. Their accessible names say the same.
7. The **Running** state is drawn as a solid blue dot. It pulses slowly on the
   Desktop, and stays still when reduced motion is requested. The Desktop
   notification for a running session uses the same dot. Web surfaces that draw
   the shared run state show the same dot.
8. The run-state indicator comes **before the title**: `[#key] [state] [title]
   [skill badge]` in the sidebar row, `[state + label] [title] [skill badge]` in
   the header. The skill badge stays after the title.
9. No new run status: `finish_run`, the activity statuses, the web history and
   the other run-state glyphs are unchanged.
10. `CHANGELOG.md` carries the change under `[Unreleased]`.

## Acceptance scenarios

- **Given** a running discussion, **when** the user clicks Stop execution,
  **then** its row shows the green check with the tooltip "Process: Finished",
  no skill badge, and the server activity is `completed`.
- **Given** a running discussion whose console has vanished, **when** the user
  clicks Stop execution, **then** the run ends `completed`.
- **Given** a running `implement` run, **when** the user clicks Stop execution,
  **then** it ends `canceled`, as today.
- **Given** a discussion whose CLI exits with code 1, **then** it ends `failed`.
- **Given** a queued discussion, **when** the user stops it before it starts,
  **then** it ends `canceled`.
- **Given** a completed skill run with a recorded verdict, **when** the user
  hovers its badge, **then** the tooltip reads "Skill: Skill completed".
- **Given** a running execution, **then** its state is a blue dot placed before
  the title in the row and in the header; it pulses, and it stops pulsing when
  reduced motion is requested.

## Out of scope

- The outcome of skill runs stopped by the user.
- A dedicated "closed" status, or any change to `finish_run`.
- The layout of the web activity history.
- Wording of the badge labels themselves beyond the `Skill:` prefix.

## Open requirements

None.
