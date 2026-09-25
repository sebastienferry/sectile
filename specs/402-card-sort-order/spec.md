# #402: Choose how Board and Backlog cards are sorted

Ticket: https://github.com/sebastienferry/sectile/issues/402
Branch: `feat/402`.
Clarification: [`docs/clarifications/402.md`](../../docs/clarifications/402.md).

## Context

Every Board column is sorted by priority only, and cards of equal priority keep
whatever order the task list arrived in. The Backlog has its own sort, local to
the page and forgotten on every visit. Viewers want to choose the order, in
particular to keep the tickets of one epic together, which a secondary sort by
epic after priority cannot do: the cards of one epic would stay scattered across
priority levels.

Out of scope: drag and drop (it changes a ticket's stage and never reorders a
column, so there is no manual order to preserve), the column set and the
Workflow / Status grouping, epic group headers, and any server, API or tracker
change.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled by the owner during the clarification (rounds 2 and 3).

- **D1: Four criteria.** Priority (the default), Epic, Key, Last updated.
- **D2: A direction toggle.** The viewer picks ascending or descending, as in
  the Backlog today, and the direction is remembered with the criterion.
- **D3: No epic header.** In Epic mode the cards of one epic are only kept
  contiguous.
- **D4: Board and Backlog.** One selector, in both toolbars. In the Backlog it
  replaces the "Priorité" button and drives the grouped view and the flat
  table.
- **D5: One preference.** The Board and the Backlog share one remembered sort:
  changing it in one view changes it in the other.
- **D6: Table headers stay.** In the Backlog flat table, clicking a column
  header still sorts the table, as a local override that is not remembered;
  changing the selector clears it.

Settled from the code during the clarification (rounds 1 and 2).

- **D7: Natural direction first.** Picking a criterion starts it in its natural
  direction: Priority highest first, Epic groups highest priority first, Key
  oldest first, Last updated most recent first. The toggle then flips it.
- **D8: The direction flips the criterion, nothing else.** In Epic mode it
  reverses the order of the groups, not the priority order inside a group.
  Tie-breaks never flip.
- **D9: Deterministic ties.** Cards equal on the chosen criterion fall back to
  priority (highest first), then to the key (oldest first).
- **D10: Per viewer.** The preference is kept in the browser, not per project
  and not on the server. A missing or unreadable preference means Priority,
  highest first.
- **D11: Nothing changes by default.** A viewer who never touches the selector
  sees the Board sorted by priority, as today, and the Backlog sorted by
  priority, highest first, as today.

Interpretations made while specifying, reversible, flagged for review.

- **D12: Missing values stay last.** Tickets without a parent in Epic mode, and
  tickets with no date at all in Last updated mode, come after the others in
  both directions, as tie-breaks do (D8).
- **D13: The creation date header does not exist.** The clarification lists
  "creation date" among the flat table's sortable headers. The table has no
  such column today (key, title, status, priority and due date are sortable);
  this ticket does not add one.

---

## Definitions

- **Priority order**: urgent, high, medium, low, from highest to lowest.
- **Key order**: numeric-aware, so `#9` comes before `#402` and `PROJ-9` before
  `PROJ-12`. Ascending is oldest first.
- **Epic** of a ticket: its parent key (a Jira epic, or a Sectile macro such as
  `M-7`). A ticket without a parent key has no epic.
- **Last update** of a ticket: its tracker update date, or, when it has none,
  its Sectile update date.

---

## User stories

### US1: Choose the sort criterion (P1)

As someone reading the Board or the Backlog, I want to choose how the cards are
ordered, so that the order serves what I am doing right now.

**Acceptance**

1. **Given** the Board, **then** its toolbar shows a sort selector next to the
   Workflow / Status toggle, offering Priority, Epic, Key and Last updated, and
   a direction button.
2. **Given** the Backlog, **then** its toolbar shows the same selector where the
   "Priorité" button was, and the "Priorité" button is gone.
3. **Given** the selector set to Priority, highest first, **then** every Board
   column lists urgent cards, then high, then medium, then low.
4. **Given** the selector set to Key, oldest first, **then** every column lists
   its cards by key, numeric-aware: `#9` above `#42` above `#402`.
5. **Given** the selector set to Last updated, most recent first, **then** every
   column lists the most recently updated ticket first, using its tracker
   update date when it has one and its Sectile update date otherwise.
6. **Given** a criterion just picked in the selector, **then** it starts in its
   natural direction (D7), whatever the direction of the previous criterion.
7. **Given** the Board in Workflow mode, in Status mode, and with the
   "unassigned" column shown, **then** every column follows the selector.

### US2: Keep the tickets of one epic together (P1)

As someone following several epics, I want the cards of one epic to sit
together in each column, so that I can read an epic's progress at a glance.

**Acceptance**

1. **Given** the selector set to Epic, **then** in every column the cards
   sharing a parent key are contiguous.
2. **Given** Epic, highest first, and a column holding epic A (one high card,
   one low card) and epic B (one urgent card), **then** the column shows B's
   urgent card, then A's high card, then A's low card.
3. **Given** two epics whose highest priority is the same, **then** the one with
   the smaller parent key (numeric-aware) comes first.
4. **Given** tickets without a parent, **then** they come after every epic, in
   priority order.
5. **Given** Epic, then the direction flipped, **then** the order of the epic
   groups reverses, the cards inside each group stay in priority order, highest
   first, and the tickets without a parent stay last.
6. **Given** Epic mode, **then** no header, separator or label is added to the
   column; the card's own epic marker, when the project shows it, is unchanged.

### US3: Flip the direction (P1)

**Acceptance**

1. **Given** Priority, highest first, **when** I press the direction button,
   **then** every column lists low cards first, and cards of equal priority
   keep key order, oldest first.
2. **Given** Key, oldest first, **when** I press the direction button, **then**
   the newest key comes first.
3. **Given** Last updated, **when** I press the direction button, **then** the
   least recently updated ticket comes first, and tickets with no date at all
   are still last.
4. **Given** any criterion, **then** the direction button shows which way the
   order runs and names it for assistive technology and in its tooltip.

### US4: One remembered choice for both views (P1)

**Acceptance**

1. **Given** I set Epic, highest first, on the Board, **when** I open the
   Backlog, **then** its selector shows Epic, highest first, and its lists
   follow it.
2. **Given** I change the sort in the Backlog, **when** I go back to the Board,
   **then** the Board follows the new sort.
3. **Given** a chosen sort, **when** I reload the page, **then** it is kept.
4. **Given** a browser that never stored a sort, an unreadable stored value, or
   no available storage, **then** the sort is Priority, highest first, and the
   app works as before.

### US5: The Backlog follows the selector (P1)

**Acceptance**

1. **Given** the Backlog grouped by stage or by status, **then** the rows of
   every group follow the selector, Epic included.
2. **Given** the Backlog flat table and no header clicked, **then** its rows
   follow the selector.
3. **Given** the flat table, **when** I click a sortable column header (key,
   title, status, priority, due date), **then** the table is sorted by that
   column: a click on the column the table is currently sorted by flips the
   direction, a click on another column sorts by it, priority highest first and
   the others ascending, as today.
4. **Given** a header-sorted flat table, **then** the selector still shows the
   remembered sort, and the Board is unaffected.
5. **Given** a header-sorted flat table, **when** I change the criterion or the
   direction in the selector, **then** the header sort is dropped and the table
   follows the selector.
6. **Given** a header-sorted flat table, **when** I reload the page, **then**
   the table follows the selector again.
7. **Given** a header-sorted flat table, **when** I switch to the grouped view,
   **then** the grouped view follows the selector.

### US6: Order is stable (P2)

**Acceptance**

1. **Given** the same tickets delivered in a different order (a refresh, a sync),
   **then** the cards are shown in the same order.
2. **Given** two tickets with the same key in two projects (a multi-project
   view), **then** their relative order does not change between renders.

### US7: Selection follows the screen (P2)

**Acceptance**

1. **Given** a chosen sort on the Board, **when** I select a range of cards or
   launch a batch on selected cards, **then** the cards are taken in the order
   the columns show them.
2. **Given** a chosen sort in the Backlog, **then** the batch order is the order
   of the rows on screen, header override included.

---

## Functional requirements

- **FR1** One sort selector component, used by the Board toolbar and the
  Backlog toolbar, offers Priority, Epic, Key and Last updated, and a direction
  toggle.
- **FR2** Priority, Key and Last updated sort on that value; Epic groups by
  parent key, orders groups by their highest priority then by parent key, and
  orders cards by priority inside a group.
- **FR3** The direction reverses the primary criterion only (for Epic, the group
  order); tie-breaks, the tickets without a parent and the tickets without a
  date are never reversed.
- **FR4** Ties fall back to priority, highest first, then key, oldest first,
  then an order that does not depend on how the list arrived.
- **FR5** Picking a criterion resets the direction to that criterion's natural
  one.
- **FR6** The criterion and the direction persist per browser in one preference
  shared by the Board and the Backlog; an absent, unreadable or invalid value
  means Priority, highest first.
- **FR7** The Board applies the sort to its workflow, tracker-status and
  unassigned columns; card selection and batch order follow the displayed
  order.
- **FR8** The Backlog applies the sort to its grouped view and its flat table;
  flat-table header clicks override it for that table only, are never
  persisted, and are cleared by any change in the selector.
- **FR9** The strings the user reads are in French and English through
  `translations.ts`.
- **FR10** `CHANGELOG.md` gets one line under `[Unreleased]` / `Added`.

## Success criteria

- Unit tests cover every comparator row of US1, US2, US3 and US6, and the
  preference load and save of US4.4.
- A web browser regression covers US1.1, US1.2, US2.2, US3.1, US4.1-US4.3 and
  US5.3-US5.5.
- The existing browser regressions of the Board and the Backlog pass unchanged.
- `npm run build`, `npm run lint` and `npm test` pass in `web/`.
