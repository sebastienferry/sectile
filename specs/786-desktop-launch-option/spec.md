# Spec #786 - Desktop: Launch dialog with an AI engine

Ticket: https://github.com/sebastienferry/sectile/issues/786
Clarification: `docs/clarifications/786.md` (rounds 1 and 2, settled)
Branch: `feat/786`

## Problem

The Desktop execution toolbar offers **Relaunch** on a task execution: a
dialog that picks the skill, the instructions and the execution mode. It
cannot pick the AI engine, so running a task on another engine means
switching it first in the ticket table's Engine column. And a task with no
execution yet has no way to reach this dialog at all.

## Scope

In: the toolbar button `#rerun` and its dialog, an **AI engine** select in
that dialog, a **Launch…** entry in every ticket row's `…` menu that opens the
same dialog, the tests and the user documentation.

Out: the web app launch menu and its per-launch model picker, a per-launch
model choice, the Engine column, the row `Run:` button, the toolbar `Next:`
button, the "Custom instructions…" compose row, the Project prompt console,
the agent and the server.

## User stories

### US1 - Relaunch becomes Launch (P1)

- **US1.1** Given a task execution is selected, when I look at the execution
  toolbar, then the button formerly named **Relaunch** is named **Launch**
  (accessible name and tooltip).
- **US1.2** When I click it, then a dialog titled **Launch &lt;task key&gt;**
  opens, with **Skill**, **Instructions** and **Execution mode** prefilled
  from the selected execution, as before, and a submit button named
  **Launch**.
- **US1.3** Given a free console execution is selected, when I click
  **Launch**, then the Project prompt dialog opens, as before. Given a macro
  run is selected, then no **Launch** button is shown, as before.

### US2 - The dialog picks the AI engine (P1)

- **US2.1** Given the agent reports the `task-engines` capability, when the
  Launch dialog opens, then it shows an **AI engine** select listing the
  engine catalogue in its configured order, each option named as the Project
  prompt console names it, the project default marked.
- **US2.2** The select starts on the task's effective engine: its stored
  engine, else the project default.
- **US2.3** Given I pick another engine and submit, then the engine is stored
  as the task engine (`PUT /desktop/task-engines`) before the launch is
  submitted, and the launch request itself is unchanged.
- **US2.4** Given I pick the project default for a task switched to another
  engine, then the task's switch is cleared, as the Engine column does.
- **US2.5** Given I leave the select on the task's effective engine, then no
  engine is stored.
- **US2.6** Given storing the engine fails, then nothing is launched, the
  dialog stays open and says why, and the submit button is usable again.
- **US2.7** Given the agent does not report `task-engines`, then the dialog
  shows no engine select and launches as before.
- **US2.8** Given the ticket table of that project is open, then after a
  launch that changed the engine its Engine column shows the new engine.

### US3 - Launch a task from the ticket row (P1)

- **US3.1** Given the ticket table, when I open a row's `…` menu, then it
  offers **Launch…**, disabled when the project has no configured local
  repository, like the other entries.
- **US3.2** When I choose it, then the same Launch dialog opens for that task,
  whether or not it has an execution yet.
- **US3.3** The dialog preselects the task's next workflow skill when there is
  one, else **Discussion (no skill)**, with empty instructions and the default
  execution mode.

### US4 - Documentation (P2)

- **US4.1** `CHANGELOG.md` `[Unreleased]` has one line naming the Launch
  dialog, its engine choice and the row entry.
- **US4.2** `docs/CAPABILITIES.md` no longer names a Relaunch dialog.

## Functional requirements

- **FR1** One dialog function serves both entry points; it takes the project,
  the task id and key, and the prefilled skill and instructions.
- **FR2** The engine to store is decided by a pure helper: nothing when the
  choice equals the task's effective engine, else the chosen id.
- **FR3** The engine is stored through `api.setTaskEngine`, and the launch
  through `api.launchServerTask` with the same arguments as today.
- **FR4** Accessible names: `Launch skill`, `Launch instructions`,
  `Launch execution mode`, `Launch AI engine`.

## Success criteria

- Desktop unit tests and the UI suites touching the dialog pass.
- No string `Relaunch` remains in `desktop/src` or `desktop/tests`.
