# Specification #500 - Current and Next labels on the desktop workflow button

- Ticket: https://github.com/sebastienferry/sectile/issues/500
- Branch: `feat/500`
- Clarification: `docs/clarifications/500.md` (rounds 1 and 2, confirmed by
  the owner), plus two decisions taken during specification (FR6, FR7)
- Framework: Spec Kit

## Summary

In the desktop app, the workflow button of the console toolbar tells what the
selected task is doing. While an execution of the task is active, or being
submitted, it reads `Current: <skill>` and stays disabled. Once no execution of
the task is active, it reads `Next: <step>` and launches that step, as it does
today.

## Scope

In scope: the workflow button of the desktop console toolbar.

Out of scope:

- The web client.
- The per-row `Run: <label>` button of the desktop Tickets pane.
- The workflow itself: which step follows which stage.
- The launch mechanics behind the button: the freshness recheck, the
  duplicate guard and "Launch anyway".
- The footer status line, "Mark reviewed", "Retry" and "Launch anyway", which
  keep their current wording and behaviour.

## Definitions

- **Selected task**: the task of the execution selected in the sidebar or in
  the execution history. A free agent console and a macro run have no task
  and show no workflow button, as today.
- **Active execution**: an execution of the selected task whose status is
  `running`, `queued`, `preparing` or `waiting`. Which console is selected
  does not matter: an older, finished console of a task that has an active
  execution still counts as a task with an active execution.
- **Launch in flight**: the time between a click on the button and the
  appearance of the execution it created, made of the *submitting* state
  (the request is being sent) and the *submitted* state (the server accepted
  it, its console has not appeared yet).
- **Stage step**: the step the task's workflow stage proposes (`new` ->
  Clarify, `clarified` -> Specify, `specified` -> Implement, `implemented` ->
  Adjust or Create PR, `reviewed` -> Handoff).
- **Skill label**: the display name of a launched skill id. The ids the
  workflow knows read as the workflow does (`clarify` -> `Clarify`, `specify`
  -> `Specify`, `implement` -> `Implement`, `adjust` -> `Adjust`, `handoff` ->
  `Handoff`, `create_pr` -> `Create PR`); any other id is shown with its first
  letter capitalized (`pickup` -> `Pickup`, `discuss` -> `Discuss`, `custom`
  -> `Custom`).

## User stories (prioritised)

### US1 - See what is running on the task (P1)

As a desktop user, when I look at a task while one of its executions is
running, the toolbar button tells me which skill is running instead of
proposing a step I cannot launch yet.

**Acceptance scenarios**

1. **Given** a task at `clarified` with a running `specify` execution,
   **when** I select it, **then** the button reads `Current: Specify` and is
   disabled.
2. **Given** a task at `new` with a running `pickup` execution, **when** I
   select it, **then** the button reads `Current: Pickup`, not
   `Current: Clarify`.
3. **Given** a task at `implemented` with a running `adjust` launched from the
   Tickets pane menu, **when** I select it, **then** the button reads
   `Current: Adjust`.
4. **Given** a task with a `queued`, `preparing` or `waiting` execution,
   **when** I select it, **then** the button reads `Current: <skill label>` of
   that execution and is disabled.
5. **Given** a task with an active execution, **when** I select an older,
   finished console of the same task in the execution history, **then** the
   button still reads `Current: <skill label>` of the active execution.
6. **Given** a finished task (no stage step) with an active `discuss`
   execution, **when** I select it, **then** the button is shown and reads
   `Current: Discuss`.

### US2 - No flicker while a launch is in flight (P1)

As a desktop user, when I click `Next: <step>`, the button switches to
`Current:` at once and stays there until the execution ends.

**Acceptance scenarios**

1. **Given** the button reads `Next: Specify`, **when** I click it, **then**
   it reads `Current: Specify` and is disabled while the launch is submitting,
   while it is submitted, and once the new execution has appeared.
2. **Given** the button reads `Next: Create PR` on a project that creates its
   pull request at implementation, **when** I click it, **then** it reads
   `Current: Implement`, the skill actually launched, from the click until
   the execution ends.
3. **Given** a launch that fails or is refused, **when** the failure is
   reported, **then** the button reads `Next: <step>` again and is enabled,
   exactly as today.

### US3 - Next step once the execution ends (P1)

As a desktop user, when the task's execution ends, the button proposes the
step that follows from the task's refreshed stage.

**Acceptance scenarios**

1. **Given** the button reads `Current: Specify`, **when** the execution
   completes and the task moves to `specified`, **then** the button reads
   `Next: Implement` and is enabled.
2. **Given** the button reads `Current: Implement`, **when** the execution
   fails or is canceled and the stage has not moved, **then** the button reads
   `Next: <stage step>` (here `Next: Implement`) and is enabled.
3. **Given** a task with no active execution, **when** I select it, **then**
   the button reads `Next: <stage step>` and behaves exactly as today,
   including being hidden when the stage proposes no step.

## Functional requirements

- **FR1** While the selected task has an active execution or a launch in
  flight, the workflow button is visible, disabled, and its text (hence its
  accessible name) is `Current: ` followed by a skill label.
- **FR2** With active executions, the skill label is that of the most recently
  created active execution of the task (several can coexist through "Launch
  anyway").
- **FR3** During a launch in flight with no active execution yet, the skill
  label is that of the skill being launched.
- **FR4** Without any active execution or launch in flight, the button reads
  `Next: <stage step label>`, is enabled, and is hidden when the stage
  proposes no step, exactly as today.
- **FR5** The label follows the executions without a manual refresh: it
  changes on the same refresh cycle that already updates the run list and the
  task stage.
- **FR6** While the task workflow is loading or failed to load, the button
  stays hidden and the footer shows its loading or error message with
  "Retry", as today, even if an execution is active. (Specification decision:
  it follows from the clarification's "footer and Retry behave as today",
  since the `Current:` label must not hide the Retry path.)
- **FR7** `Current:` is shown even when the stage proposes no step (finished
  task, unconfigured project, unavailable skill), as long as an execution is
  active. (Specification decision: it applies the acceptance criterion
  "visible while any execution is active" to those stages.)
- **FR8** The footer status, "Mark reviewed", "Retry", "Launch anyway" and the
  Tickets pane keep their current texts, visibility and enablement rules.
- **FR9** `CHANGELOG.md` gains one `Changed` line under `[Unreleased]`.

## Success criteria

- A user can tell from the toolbar alone which skill is running on the
  selected task, including a pickup chain and a manually launched skill.
- No state shows `Next:` on a disabled button because of an active execution
  or a launch in flight.
- The existing desktop UI tests that assert `Next: <label>` on idle tasks pass
  unchanged.

## Open questions

None. The clarification closed every product question; FR6 and FR7 are the
two decisions taken during specification, both derived from its acceptance
criteria.
