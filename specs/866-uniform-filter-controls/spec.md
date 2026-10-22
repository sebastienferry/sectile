# Spec #866 - Uniform filter controls across all views

Ticket: https://github.com/sebastienferry/sectile/issues/866
Clarification: `docs/clarifications/866.md` (rounds 1 and 2, settled)
Branch: `feat/866`

## Problem

Each view of the web client filters its content with controls built on its
own: a panel on the board and the list, toolbar popovers and chips on the
roadmap, native drop-downs elsewhere, three differently styled search fields.
The same dimension looks and behaves differently from one view to the next,
active filters are visible on the roadmap only, and a way to clear them exists
in two places only.

## Scope

In: the filter controls of seven views of the web client, which Sectile
Desktop inherits through the shared components: Board, List, Roadmap, Triage,
Sprint timeline, Team, Activities.

Out:

- Where each filter's state lives and what the server receives: a filter that
  is global today stays global, a local one stays local (round 2, decision 1).
- The cardinality of each filter: tracker status, issue type, epic labels and
  epic origins stay multi-select; sprint, team, assignee, parent and priority
  stay single-select (round 2, decision 3).
- Controls that are not filters: the per-row stage and status editor of the
  list, the target sprint of a batch move in the timeline, the per-row "add to
  project" action of the tracker backlog, the roadmap's sort selector.
- The global command palette, the header search, the admin panels.

## Vocabulary

- **Toggle**: an on/off filter, such as "pinned only" or "active only".
- **Search**: a free-text filter on the view's content.
- **Picker**: a filter choosing one value (single) or several values (multi)
  from a list, such as sprint, assignee or epic labels.
- **Filter bar**: the part of a view's toolbar holding its filters.
- **Filters panel**: the menu behind the filter bar's "Filters" button.
- **Chip row**: the list of the active filters of a view, one chip each.

## User stories

### US1 - One filter bar on every view (P1)

- **US1.1** Given any of the seven views, its filters sit in its toolbar, in
  one filter bar: toggles and search inline, pickers in the filters panel.
- **US1.2** The Board and the List keep their current inline controls (pinned,
  active, priority dots) and their current panel content.
- **US1.3** A picker whose view never shows an empty value (the Team view's
  team) stays inline, as the view's subject, and is not counted as a filter.
- **US1.4** The "Filters" button shows how many pickers of its panel are set,
  and nothing when none is.
- **US1.5** The panel closes on a press outside it and on Escape, and stays
  open while a value is being picked inside it.
- **US1.6** A view whose panel would be empty shows no "Filters" button.

### US2 - The same control for the same dimension (P1)

- **US2.1** Every picker opens the same searchable list, with the same option
  layout (label, optional secondary line with a count), the same empty-result
  wording and the same "clear" entry.
- **US2.2** A multi-select picker shows a check on each chosen value and keeps
  its list open between picks; a single-select picker closes on a pick.
- **US2.3** A dimension offered in several views (sprint, parent, assignee,
  team, priority) is offered with the same label, the same values for the same
  data, and the same "none" entry wording (for example "No sprint", "No
  macro") wherever it appears.
- **US2.4** Every search field looks the same, filters as the user types, and
  offers a clear button once it holds text.
- **US2.5** Every toggle and the Activities status selector use the same
  pressed and released look.

### US3 - See and clear active filters (P1)

- **US3.1** Given a view with at least one active filter, a chip row under its
  filter bar lists each active filter as one chip naming its value; a
  multi-select picker gives one chip per chosen value.
- **US3.2** Pressing a chip clears that filter (or that value) only.
- **US3.3** The chip row ends with "Clear all", which clears every filter the
  row lists, and only those.
- **US3.4** A view with no active filter shows no chip row.
- **US3.5** The roadmap's existing chips keep their meaning: the origin chip
  resets to the project's own key, as today.

### US4 - Behaviour unchanged (P1)

- **US4.1** For a given set of filter values, every view shows exactly the
  content it shows today, and the server receives the same requests.
- **US4.2** A filter global today keeps following the user from one view to
  another; a local one keeps resetting as it does today.

## Functional requirements

- **FR1** One shared set of filter controls (toggle, search, single picker,
  multi picker, segmented selector, filter bar with its panel, chip row) is the
  only way the seven views render a filter.
- **FR2** No native `<select>` remains as a filter in the seven views.
- **FR3** `EpicLabelFilter`, `EpicOriginFilter`, the timeline parent drop-down
  and the native filter drop-downs of Triage, Team and Activities are replaced
  by the shared controls; their inputs and the state they write are unchanged.
- **FR4** Every new or moved user-facing string exists in French and English.
- **FR5** The controls are reachable and operable with the keyboard: Tab to
  the control, Enter or Space to open, arrows to move, Enter to pick, Escape
  to close; each picker and chip has an accessible name.
- **FR6** `CHANGELOG.md` gains one `Changed` line under `## [Unreleased]`.

## Acceptance criteria

- On each of the seven views, filters sit in one filter bar: toggles and
  search inline, pickers in the "Filters" panel, the Team view's team inline.
- The "Filters" button counts the set pickers; no button when the panel would
  be empty.
- Setting any filter shows its chip; pressing the chip clears it; "Clear all"
  clears the row; no row without an active filter.
- Sprint, parent, assignee, team and priority read the same in every view that
  offers them.
- No native `<select>` is used as a filter in the seven views; the non-filter
  selects listed out of scope are untouched.
- For the same filter values, each view lists the same items as before the
  change, and the board and list send the same query parameters.
- The panel closes on Escape and on a press outside, not on a pick inside.
- `CHANGELOG.md` has the `Changed` line.

## Open points

None. The four product questions were answered in round 2 of the
clarification. How each view splits its controls between inline and panel
(US1.1 to US1.3) applies decision 2 (the `TaskFilters` model: toggles inline,
value filters in the panel).
