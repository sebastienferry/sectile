# Specification #515 - Next-step and full-chain icon buttons in the desktop toolbar

- Ticket: https://github.com/sebastienferry/sectile/issues/515
- Branch: `feat/515`
- Clarification: `docs/clarifications/515.md` (rounds 1 and 2, confirmed by
  the owner), plus three decisions taken during specification (FR8, FR9,
  FR10)
- Builds on: `specs/500-next-button-in-desktop/` (the `Current:` / `Next:`
  label rules, unchanged here)
- Framework: Spec Kit

## Summary

In the desktop app, the console toolbar launches the selected task's work the
way the web task card does: a `>` icon button launches the next workflow step,
and a `>>` icon button launches the full `pickup` chain in one click. The text
that the next-step button carries today (`Next: Specify`, `Current: Pickup`)
moves into a small, non-clickable badge next to the two buttons. The closing
button (the circled check that ends the current execution) is kept as it is.

## Scope

In scope: the workflow controls of the desktop console toolbar.

Out of scope:

- The closing button (`Stop execution`, drawn as a circled check).
- The web client.
- The desktop Tickets pane: its per-row `Run: <label>` button and its
  "Pickup (full chain)" menu entry.
- The workflow itself: which step follows which stage.
- The rules of the `Current:` / `Next:` label settled by #500: which skill
  `Current:` names and what counts as running.
- The footer status line, "Mark reviewed", "Retry" and "Launch anyway", which
  keep their wording and behaviour.

## Definitions

The terms *selected task*, *active execution*, *launch in flight*, *stage
step* and *skill label* keep the meaning given in
`specs/500-next-button-in-desktop/spec.md`. In addition:

- **Next-step button (`>`)**: the icon-only button that launches the stage
  step.
- **Full-chain button (`>>`)**: the icon-only button that launches the
  project's `pickup` skill on the selected task.
- **Step badge**: the small, non-clickable text element next to the two
  buttons that reads `Next: <step label>` or `Current: <skill label>`.
- **Pickup available**: the project is configured on this workstation and its
  server skills include `pickup`.
- **Finished task**: a task whose workflow stage is `finished`.

## User stories (prioritised)

### US1 - Launch the next step from a `>` button (P1)

As a desktop user, I launch the task's next step from a `>` icon button, like
on the web task card, and a badge next to it tells me which step that is.

**Acceptance scenarios**

1. **Given** a task at `clarified` with no active execution, **when** I select
   it, **then** the toolbar shows an enabled `>` icon button whose accessible
   name and tooltip are `Next: Specify`, and the badge next to it reads
   `Next: Specify`.
2. **Given** that state, **when** I click `>`, **then** the `specify` step is
   launched exactly as the former `Next: Specify` text button did: freshness
   recheck, duplicate refusal and "Launch anyway" on refusal, selection of the
   new console.
3. **Given** a task with an active `specify` execution, **when** I select it,
   **then** `>` is visible and disabled, its accessible name and tooltip are
   `Current: Specify`, and the badge reads `Current: Specify`.
4. **Given** a task whose stage proposes no step (unconfigured project, next
   skill unavailable) and no active execution, **when** I select it, **then**
   `>` and the badge are hidden, as the text button is today.

### US2 - Launch the full chain from a `>>` button (P1)

As a desktop user, I launch the whole workflow of a task in one click, without
answering each step, from a `>>` icon button.

**Acceptance scenarios**

1. **Given** a task at `new` on a project with pickup available and no active
   execution, **when** I select it, **then** the toolbar shows an enabled
   `>>` icon button whose accessible name and tooltip are
   `Pickup (full chain)`.
2. **Given** that state, **when** I click `>>`, **then** one execution of the
   `pickup` skill is launched on the task in `autonomous` mode, and its
   console is selected once it appears.
3. **Given** I clicked `>>`, **when** the launch is submitting, is submitted,
   or its execution is running, **then** `>` and `>>` are visible and
   disabled and the badge reads `Current: Pickup`.
4. **Given** the server refuses the `>>` launch because an execution is
   already active on the task, **when** the refusal is reported, **then**
   "Launch anyway" appears, and clicking it re-sends the `pickup` launch in
   `autonomous` mode with the duplicate check waived.
5. **Given** a `>>` launch that fails for any other reason, **when** the
   failure is reported, **then** the footer shows the error and both buttons
   are enabled again.

### US3 - A toolbar that does not shift (P2)

As a desktop user, the toolbar keeps its layout when an execution starts or
ends, and shows nothing I cannot use.

**Acceptance scenarios**

1. **Given** `>>` is shown, **when** an execution of the task starts, from any
   surface, **then** `>>` stays visible and becomes disabled; it is enabled
   again when no execution of the task is active.
2. **Given** a project without a `pickup` skill, **when** I select one of its
   tasks, **then** `>>` is hidden and `>` and the badge behave as in US1.
3. **Given** a finished task with no active execution, **when** I select it,
   **then** `>`, `>>` and the badge are hidden.
4. **Given** a free agent console, a macro run, no selection, or a task whose
   workflow is loading or failed to load, **when** it is shown, **then** `>`,
   `>>` and the badge are hidden, and the footer shows what it shows today
   (including "Retry" on a load error).

## Functional requirements

- **FR1** The next-step button is icon-only, drawn as a single chevron in the
  toolbar's existing icon style. Its accessible name and tooltip carry the
  text the #500 text button carried: `Next: <step label>` when idle,
  `Current: <skill label>` while an execution of the task is active or a
  launch is in flight.
- **FR2** Its visibility, enablement and launch behaviour are those of the
  former text button, unchanged.
- **FR3** The step badge shows the same text as the next-step button's
  accessible name, next to the two buttons. It is not clickable, is not a
  focus stop, and is not announced by assistive technology (the button's name
  and the footer status already say it). It is visible exactly when the
  next-step button is visible.
- **FR4** The full-chain button is icon-only, drawn as a double chevron in
  the same icon style, and its accessible name and tooltip are
  `Pickup (full chain)`.
- **FR5** The full-chain button is visible when a task is selected, its
  workflow is loaded without error, the task is not finished, and pickup is
  available. It is hidden otherwise.
- **FR6** When visible, the full-chain button is disabled while the task has
  an active execution or a launch in flight (from either button), and enabled
  otherwise.
- **FR7** A click on the full-chain button launches the `pickup` skill on the
  task with the execution mode forced to `autonomous` and an empty prompt, and
  selects the new console once it appears. The next-step button keeps sending
  no mode.
- **FR8** Before launching, the full-chain button rechecks the task and the
  executions, as the next-step button does, and abandons the launch without
  an error when an execution of the task became active, the task became
  finished, or pickup is no longer available. A change of the stage step
  alone does not abandon it, since the chain starts from whatever stage the
  task is at. (Specification decision: the clarification asks for "the same
  freshness recheck"; the next-step recheck compares the stage step, which is
  irrelevant to a chain.)
- **FR9** A duplicate refusal of a full-chain launch reveals "Launch anyway",
  which re-sends the `pickup` launch in `autonomous` mode with the duplicate
  check waived. A duplicate refusal of a next-step launch keeps re-sending the
  next step. "Launch anyway" re-sends the launch that was refused, whichever
  button made it. (Specification decision: required for the clarification's
  "same duplicate / Launch anyway path" to mean something for `>>`.)
- **FR10** On a finished task with an active execution, the next-step button
  and the badge keep showing `Current: <skill label>` (disabled), as #500
  FR7 requires, while the full-chain button stays hidden. (Specification
  decision: the clarification's "on a finished task, both buttons and the
  badge are hidden" and its "#500 rules unchanged" meet in this case; the
  idle case follows the former, the running case the latter.)
- **FR11** The closing button, "Mark reviewed", "Retry", "Launch anyway"
  (apart from FR9), the footer status and the Tickets pane keep their current
  texts, visibility and enablement rules.
- **FR12** `CHANGELOG.md` gains one `Changed` line under `[Unreleased]`.

## Success criteria

- A user can launch the next step or the whole chain of the selected task from
  the console toolbar in one click, and read from the badge which step is next
  or which skill is running.
- The toolbar controls keep their position when an execution starts or ends.
- The desktop UI tests that look the next-step button up by its
  `Next: <label>` / `Current: <label>` name keep passing through its
  accessible name.

## Open questions

None. The clarification closed every product question. FR8, FR9 and FR10 are
the three decisions taken during specification, each derived from the
clarification's own criteria; the owner may overturn any of them at review.
