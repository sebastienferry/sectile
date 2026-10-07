# Specification #791 - Desktop: Merge request

- Ticket: https://github.com/sebastienferry/sectile/issues/791
- Branch: `feat/791`
- Clarification: `docs/clarifications/791.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-07)
- Framework: Spec Kit

## Summary

When the selected task holds pull requests or merge requests in several
repositories, the console toolbar of Sectile Desktop stops showing one button
per repository side by side. It shows the primary repository's pull request as
today, followed by a `+N` chevron that opens a menu listing every pull request
of the task. A task with a single pull request keeps today's toolbar.

## Scope

In scope:

- the pull request controls of the console toolbar of the selected task;
- the desktop UI test that covers a task with several repositories;
- the Desktop README and the changelog.

Out of scope:

- the sidebar run rows and the tickets table: they keep their state icon and
  their passive `+N` badge with its tooltip;
- superseded pull requests on the same repository (a follow-up after a merged
  one): they stay hidden, as today;
- the web client, the server and its `prLinks` model, how pull request states
  are fetched;
- the single pull request case, which keeps its current button.

## User stories

### US1 - Several pull requests collapse into one control (P1)

As a Desktop user working on a task that changed several repositories, I see
one compact pull request control in the toolbar instead of a row of buttons,
so the toolbar stays readable however many repositories the task touched.

- **Given** a selected task with pull requests in two or more repositories,
  **when** I look at the toolbar, **then** I see the primary repository's pull
  request button, labelled as today (`PR #79`, `MR !7`), followed by a chevron
  labelled `+N`, N being the number of the other repositories' pull requests.
- **Given** the same task, **then** no other pull request button is shown in
  the toolbar.
- **Given** the primary pull request button, **when** I activate it, **then**
  its pull request opens externally in one click, as today.
- **Given** the chevron, **then** it announces a menu (`aria-haspopup="menu"`,
  `aria-expanded`) and its accessible name says it lists the pull requests of
  the task.

### US2 - The menu lists and opens every pull request (P1)

- **Given** the chevron, **when** I activate it, **then** a menu opens below it
  listing one entry per repository, the primary repository first, then the
  others in the order the toolbar uses today.
- **Given** each entry, **then** it shows the pull request's state icon in its
  state colour, the repository name (the last segment of its identity) and the
  `PR #n` or `MR !n` label; its tooltip and accessible name give the state, as
  the buttons do today (for example `State unknown: no GitLab token`).
- **Given** the open menu, **when** I choose an entry, **then** that pull
  request opens externally, the menu closes and focus returns to the chevron.
- **Given** the menu, **then** each repository appears once, with its current
  pull request only.

### US3 - The menu behaves like the toolbar's other menu (P1)

- **Given** the open menu, **when** I press Escape, **then** it closes and
  focus returns to the chevron.
- **Given** the open menu, **when** I click outside it or the window loses
  focus, **then** it closes.
- **Given** the open menu, **when** I press ArrowDown, ArrowUp, Home or End,
  **then** focus moves between its entries, wrapping at both ends; Tab closes
  it.
- **Given** the focused chevron with the menu closed, **when** I press
  ArrowDown, **then** the menu opens with its first entry focused.
- **Given** the open menu, **when** another execution is selected or the task
  is left with fewer than two pull requests, **then** the menu closes.

### US4 - A single pull request keeps today's toolbar (P1)

- **Given** a selected task with one pull request, **then** the toolbar shows
  its button as today and no chevron.
- **Given** a selected task with no pull request, **then** the toolbar shows
  neither the button nor the chevron.

### US5 - The other surfaces are unchanged (P2)

- **Given** a task with pull requests in several repositories, **then** its
  sidebar run row and its tickets table row still show one state icon and a
  passive `+N` badge whose tooltip lists the other pull requests.

## Functional requirements

- **FR-001** The toolbar shows the chevron if and only if the selected task
  has two or more pull requests to show.
- **FR-002** The chevron's visible label is `+N`, N being the count of
  pull requests other than the primary one.
- **FR-003** The menu lists the current pull request of each repository of
  the task, primary first, as the toolbar ordered its buttons before this
  change. Superseded pull requests are not listed.
- **FR-004** Each entry shows the state icon and colour, the repository name
  and the `PR #n` / `MR !n` label, carries the state in its tooltip and
  accessible name, and opens its pull request externally when chosen.
- **FR-005** The menu closes on a choice, on Escape, on a pointer press
  outside it and the chevron, on window blur, on Tab, and when the selected
  execution changes or the task drops below two pull requests.
- **FR-006** The menu is built each time it opens, so a pull request added or a
  state refreshed while it is closed shows at the next opening and never moves
  under the pointer.
- **FR-007** The primary pull request button, the sidebar rows and the tickets
  table behave and render as before.

## Acceptance criteria

- **AC-1** With two repositories, the toolbar shows `PR #79` and `+1`, and no
  `#selected-pr-others` buttons.
- **AC-2** Opening the chevron lists two entries, `app PR #79` then
  `deploy MR !7`; the second's accessible name is the one today's button
  carries: `Open MR !7 in gitlab.com/example/deploy`, the indicator separator,
  then `State unknown: no GitLab token`.
- **AC-3** Choosing `deploy MR !7` opens its URL externally and closes the
  menu.
- **AC-4** Escape closes the open menu and focuses the chevron.
- **AC-5** A task with one pull request shows its button and no chevron.
- **AC-6** The sidebar `+1` badge and its tooltip are unchanged.
- **AC-7** `CHANGELOG.md` carries a `Changed` line under `## [Unreleased]`
  describing the toolbar menu.

## Open points

None: the three product questions were answered in round 2 of the
clarification, and every recommendation was kept.
