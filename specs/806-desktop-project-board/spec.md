# Spec #806 - Desktop project board

Ticket: https://github.com/sebastienferry/sectile/issues/806
Clarification: `docs/clarifications/806.md` (rounds 1 and 2, settled)
Branch: `feat/806`

## Problem

Sectile Desktop lists a project's tasks only as a table ("Open tasks"), which
hides finished tasks and shows the stage as a column. The web app shows the
same tasks as a board of workflow columns, where the stage of a task reads at
a glance and a card can be moved to another stage. Desktop users have to open
the web interface to get that view.

## Scope

In: a new full page in Desktop, the **project board**, showing one project's
tasks in the six workflow columns; its display options (finished column
hidden, card display), the macro colour bar, moving a card between columns by
drag and drop, the entries that open it, the agent data the board needs, and
the matching documentation and changelog line.

Out: the web board and its defaults, the web's tracker-status grouping
("Statuts"), the web's optional views (triage, roadmap, timeline), boards
spanning several projects, batch pickup selection, quick-add from a column,
launching a skill on drop, and any change to how the server stores tasks or
stages. The "Open tasks" list is unchanged.

## User stories

### US1 - Open the board (P1)

- **US1.1** Given a project in the sidebar, when I open its `…` menu, then
  an **Open board** entry is listed right after **Open tasks**.
- **US1.2** When I choose it, then the board of that project fills the
  workspace, titled **Board · &lt;project name&gt;**, with a **Close board**
  button that returns to the previous view and gives the focus back to the
  element that opened the board.
- **US1.3** Given the command palette, then it lists a **Project board**
  action. It opens the board of the selected project, or of the only project
  when there is one, and otherwise asks which project, as **Tasks list**
  does.
- **US1.4** Given the agent is not connected, when I ask for the board, then
  a dialog says the agent must be connected, as the tickets list does.
- **US1.5** Opening the board closes the tickets list if it is open, and
  opening the tickets list closes the board: one full page at a time.

### US2 - Workflow columns (P1)

- **US2.1** The board shows six columns, in this order: **New**,
  **Clarified**, **Specified**, **Implemented**, **Reviewed**, **Finished**.
  Each column header carries its name and its number of cards.
- **US2.2** Each column is tinted with the colour of its stage, the one the
  sidebar's stage keys use, in the light and the dark themes.
- **US2.3** Every task of the project appears in exactly one column, finished
  tasks included.
- **US2.4** A task sits in the column the web board shows it in: an explicit
  workflow label first (`#finished`, `#closed` or `#done` read as Finished),
  then the stage the project maps the task's tracker column to, then the
  status fallbacks Desktop already applies.
- **US2.5** Given the agent does not send the project's stage mapping (an
  older agent), then the board still opens and places tasks from their labels
  and status alone.
- **US2.6** Within a column, cards are ordered as the tickets list orders them
  by default: priority descending, then key.
- **US2.7** Given a column has no card, then it still shows, with a count of
  0 and an empty drop area.
- **US2.8** Given the project has no task at all, then the board shows its
  columns and a line saying the project has no tasks.
- **US2.9** Given loading fails, then the board says why and offers to retry,
  as the tickets list does.

### US3 - Search and refresh (P2)

- **US3.1** The board carries a search field (title or task key). Submitting
  it reloads the board with only the matching tasks, the column counts
  included.
- **US3.2** The board reloads when it opens and after an action started from
  one of its cards, as the tickets list does, and after a successful drop.

### US4 - Finished column hidden by default (P1)

- **US4.1** On first use, the Finished column is collapsed into a narrow
  strip that shows its name and its number of cards.
- **US4.2** Clicking the strip expands the column. A **Hide finished**
  control in the column header collapses it again.
- **US4.3** The choice is remembered on this workstation, once for every
  project, and survives a restart of Desktop.
- **US4.4** The web board's own setting and default are not affected.

### US5 - Card display (P1)

- **US5.1** A toggle in the board toolbar switches between **Condensed** and
  **Full** cards. Condensed is the default on first use.
- **US5.2** The choice is remembered on this workstation, once for every
  project, and survives a restart.
- **US5.3** A condensed card shows, on one line: the task key, its title
  (truncated, the full title in the tooltip), the run state badge of its
  latest execution when it has one, and the actions menu.
- **US5.4** A full card shows, in addition: the parent (macro) key when the
  task has one, its priority, its labels other than the workflow stage
  labels, its pull request link when it records one, and its assignee when
  it has one.

### US6 - Macro colour bar (P2)

- **US6.1** Given the project enables macro colours, then every card whose
  task has a parent carries a colour bar on its left edge, in both card
  displays.
- **US6.2** The colour of a parent key is the one the web board paints it
  with, so a macro keeps the same colour in both clients.
- **US6.3** A task without a parent has no bar. Given the project does not
  enable macro colours, or the agent does not say whether it does, then no
  card has a bar.

### US7 - Card actions (P1)

- **US7.1** Each card carries an actions menu offering the same entries as
  the matching row of the tickets list (next skill, other skills, pickup,
  discussions, custom instructions, **Launch…**, **Skip to Handoff…** on an
  implemented task), and they behave identically.
- **US7.2** Selecting a card's title opens the task as selecting a row of the
  tickets list does; it launches nothing.

### US8 - Move a card between stages (P1)

- **US8.1** Given the agent supports moving a stage, when I drag a card and
  drop it onto another column, then the task's stage becomes that column's:
  its workflow stage label is replaced by the target one, its internal status
  becomes the target stage's, and its tracker status follows the project's
  stage to column mapping when the project maps that stage. No skill is
  launched.
- **US8.2** The change is the one the web board makes for the same drop:
  same labels, same status, same tracker status.
- **US8.3** A drop onto any stage is accepted, backwards and forwards, and
  onto Implemented without a pull request. No stage report is recorded.
- **US8.4** Given the Finished column is collapsed, when I drop a card onto
  its strip, then the task moves to Finished and the column stays collapsed;
  the strip's count grows by one.
- **US8.5** While the move is in progress the card shows it is busy and
  cannot be dragged again. After a successful move the board reloads and the
  card sits in its new column.
- **US8.6** Given the move is refused or fails, then the card stays in (or
  returns to) its original column and the board shows the server's message.
- **US8.7** Dropping a card back onto its own column does nothing.
- **US8.8** Given the agent does not support moving a stage (an older agent),
  then cards cannot be dragged and nothing suggests they can; every other
  part of the board works.

## Functional requirements

- **FR1** The board reads the project's tasks including finished ones, and
  the project's display data: whether macro colours are enabled, its tracker
  columns and its stage to column mapping, per tracker it selects.
- **FR2** The stage of a task is resolved by one function, used by the board,
  the tickets list and the sidebar's grouping by stage, so they agree. With a
  stage mapping it gives the web board's answer.
- **FR3** The macro colour is computed by one implementation shared by the
  web and Desktop, from the parent key alone.
- **FR4** The stage move is computed by one implementation shared by the web
  and Desktop: labels, internal status and tracker status. The web's
  behaviour is unchanged.
- **FR5** The local agent exposes the stage move to Desktop, applying only
  the labels, status, tracker status and board project of the task, and
  advertises it as a capability.
- **FR6** The board's two options are stored in Desktop, independent of the
  web and of every project.
- **FR7** User-facing strings are English, as the rest of the Desktop
  renderer around them.

## Non-functional requirements

- **NFR1** No database migration and no new server endpoint.
- **NFR2** An agent older than this change keeps working with a newer Desktop
  (US2.5, US6.3, US8.8), and a newer agent keeps answering an older Desktop.
- **NFR3** The board stays usable with a few hundred tasks: one load per
  refresh, no request per card.
- **NFR4** Controls have accessible names; the column headers and the
  collapsed strip are reachable with the keyboard.

## Acceptance criteria

- **AC1** US1 to US8 hold in Desktop UI tests against the fake agent.
- **AC2** Shared stage resolution, macro colour and stage move have unit
  tests; the web's existing tests still pass unchanged, and a test pins that
  the web and Desktop paint a given parent key with the same colour.
- **AC3** The agent route and its capability have Go tests: allowed fields
  only, a missing project or task refused, the server's answer and status
  passed through.
- **AC4** `CHANGELOG.md` has one `Added` line under `[Unreleased]` naming the
  Desktop project board and moving a card between stages.
- **AC5** `docs/USER_GUIDE.md` describes how to open the board, its options
  and the drop.

## Open points

- **O1 Keyboard alternative to drag and drop.** The clarification settles
  moving by drag and drop and says nothing of a keyboard way (for instance a
  "Move to stage" entry in the card menu). Not decided here: the board ships
  with drag and drop only, and the owner may ask for the keyboard entry in
  review or in its own ticket. It blocks nothing.
