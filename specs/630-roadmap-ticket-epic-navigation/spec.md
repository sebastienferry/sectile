# Specification #630 - Roadmap: go from a ticket to its epic and back

- Ticket: https://github.com/sebastienferry/sectile/issues/630 (milestone M-11,
  Roadmap)
- Branch: `claude/clarify-issue-630`
- Clarification: `docs/clarifications/630.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

A ticket that has a parent opens the roadmap on that epic, selected and with
its panel shown, whatever the roadmap was showing. From the epic's panel, the
user goes back to the ticket view they came from, filtered on that epic. An
epic the roadmap does not hold is refused with a message naming it and its
cause. The roadmap stops applying the ticket views' parent filter, so this
round trip never empties the other epics.

## Scope

In scope: the board card menu, the list row actions, the ticket detail
header, the roadmap panel and its arrival behaviour, the roadmap's ticket
query, the header filter chips, the French and English strings, and the
changelog.

Out of scope:

- Showing the epics of other tracker projects on the roadmap (#632).
- A navigation history beyond one step.
- Changing what a click on the parent key does today: it filters the ticket
  views on a card, and opens the tracker in the detail.
- Restoring "all projects" or a multi-project view on the way back.
- Any server change.

## Vocabulary

- **Ticket views**: the board, the list, the triage and the timeline.
- **Parent key**: the `parentKey` a ticket carries (a Jira epic key, a GitHub
  milestone `M-<n>`, a local macro key).
- **Held epic**: an epic for which the roadmap of the ticket's project shows a
  row, once its tickets and its epics are loaded, before any roadmap filter.
- **Origin view**: the last ticket view shown before the user entered the
  roadmap, by any route, during the current page session.
- **Parent filter**: the existing ticket filter on a parent key, shown as a
  header chip.

## User stories

### US1 (P1) - Open a ticket's epic in the roadmap

As someone reading a ticket, I reach its epic's framing without looking for
the epic key myself.

1. Given a board card whose ticket has a parent, when I open its menu, then
   it offers "Open the epic in the roadmap" beside "Filter by parent".
2. Given a list row whose ticket has a parent, then its action column offers
   an "Open the epic in the roadmap" icon beside pin and clone.
3. Given a ticket detail whose ticket has a parent, then a button beside the
   parent key opens the epic in the roadmap.
4. Given a ticket without a parent key, then none of the three is offered.
5. Given a ticket whose project does not enable the roadmap view, then none
   of the three is offered, whatever scope is selected.
6. Given the parent key on a card, when I click it, then it filters the
   ticket views as today; in the detail header it opens the tracker as today.
7. Given I use the entry from a card or a list row, then the ticket detail
   does not open. Given I use it from the detail, then the detail closes.

### US2 (P1) - The epic is selected and visible on arrival

1. Given the roadmap was left on NOW, when I open a ticket's epic whose
   horizon is LATER, then the roadmap opens on the LATER tab, with that epic
   selected and its panel shown.
2. Given an epic without a horizon, then the Unclassified tab opens; given a
   hidden epic, then the Hidden tab opens.
3. Given a closed epic and closed epics hidden, then closed epics are shown.
4. Given a roadmap search, label filter, priority filter or "only issues"
   filter that would hide the epic, then each of them is cleared.
5. Given the details panel hidden, then it is shown.
6. Given the roadmap's grouping, sort, row shape, display mode and expanded
   panel, then they stay as they were.
7. Given the epic opened, then it is the roadmap's remembered selection for
   that project, as if I had clicked it.
8. Given the epic opened, when I later change a roadmap filter or tab, then
   the roadmap does not jump back to that epic.

### US3 (P1) - An epic the roadmap does not hold is refused

1. Given a ticket whose parent key is not a held epic of its project's
   roadmap (for example a Jira parent in another tracker project), when I open
   its epic, then I am back on the view I used the entry from, and an error
   names the key and says the epic is not on this project's roadmap: re-read
   the epics with a sync, or check that its tracker project is declared.
2. Given that refusal, then the roadmap's selection, tab and filters are
   unchanged, and no other epic's panel is opened.

### US4 (P1) - Go from an epic to its tickets

1. Given an epic panel whose epic has at least one ticket, open or closed,
   then it offers "Open its tickets" beside "Open in tracker".
2. Given an epic without any ticket, then the button is not offered.
3. Given I reached the roadmap from the list, when I use "Open its tickets",
   then the list opens filtered on that epic, and the parent filter chip names
   it.
4. Given I reached the roadmap from the board, the triage or the timeline,
   then that view opens, filtered on the epic.
5. Given no ticket view was shown before the roadmap in this page session, or
   the origin view is not enabled on the project, then the board opens,
   filtered on the epic.
6. Given other ticket filters were active (status, assignee, sprint), then
   they stay active.
7. Given the parent filter chip, when I clear it, then it clears as today.
8. Given I enter the roadmap again from another ticket view, then that view
   becomes the one the button returns to. Only one step is remembered.

### US5 (P1) - A ticket under "all projects" or a multi-project view

1. Given "all projects" or a multi-project view selected, when I open a
   ticket's epic, then the selected project becomes the ticket's project and
   its roadmap opens on the epic, as in US2.
2. Given that switch, then the ticket filters remembered for that project are
   the ones restored, as when choosing the project by hand.
3. Given I then use "Open its tickets", then the origin ticket view opens on
   that project, filtered on the epic. "All projects" or the multi-project
   view is not restored.

### US6 (P1) - The parent filter no longer reaches the roadmap

1. Given a parent filter active on the ticket views, when I open the roadmap,
   then every epic shows all its tickets, as if no parent filter were set.
2. Given a parent filter active, while the roadmap is shown, then the header
   does not show the parent filter chip.
3. Given I go back to a ticket view, then the parent filter still applies and
   its chip shows again.
4. Given the other ticket filters (status, priority, label, sprint, team,
   assignee, "my tasks", tracker status, issue type, pinned), then they keep
   reaching the roadmap as today.

## Functional requirements

- **FR1** The entry "Open the epic in the roadmap" is offered in the board
  card menu, in the list row actions and in the ticket detail header, only for
  a ticket with a non-empty parent key whose project enables the roadmap view.
- **FR2** The parent key keeps its current click on the card, the list and the
  detail.
- **FR3** Using the entry switches the selected project to the ticket's
  project when the current scope is "all projects", a multi-project view or
  another project, through the same path as choosing the project by hand, then
  shows the roadmap.
- **FR4** Once the roadmap of that project has loaded its tickets and epics,
  it honours the request exactly once. If the parent key is a held epic, it
  selects it through the remembered selection, moves to the tab of its horizon
  (Unclassified when none), shows closed epics when the epic is closed, clears
  the roadmap search, label, priority and "only issues" filters, and shows a
  hidden panel. Nothing else of the roadmap view changes.
- **FR5** If the parent key is not a held epic, the request is dropped, the
  view the entry was used from is shown again, and an error toast names the
  key and its cause. The roadmap's selection, tab and filters are untouched.
- **FR6** The epic panel shows "Open its tickets" when the epic has at least
  one ticket. It sets the parent filter to the epic key and shows the origin
  view, or the board when there is none or it is not enabled on the project.
  The other ticket filters are kept.
- **FR7** The origin view is the last ticket view shown before entering the
  roadmap, by any route, held in memory for the page session only.
- **FR8** The roadmap's ticket query never carries the parent filter; every
  other ticket filter it carries today is kept.
- **FR9** The parent filter chip is not shown while the roadmap is displayed.
- **FR10** Every new string exists in French and English.
- **FR11** `CHANGELOG.md` gets one `Added` line under `[Unreleased]` for the
  ticket-to-epic navigation and back, and one `Changed` line saying the
  roadmap no longer applies the ticket views' parent filter.

## Success criteria

- From any ticket with a parent on a project with the roadmap, one click
  shows that epic's panel, whatever the roadmap was showing.
- From that panel, one click shows the ticket view the user came from,
  filtered on the epic.
- With a parent filter set, the roadmap shows the same epics and ticket
  counts as without it.

## Assumptions

- A held epic is decided from the rows the roadmap builds for the project
  (tickets carrying the key, and epics known without tickets), before any
  roadmap filter. An epic hidden by the ticket filters that still reach the
  roadmap (for example an assignee filter leaving none of its tickets) and not
  known to the epic list counts as not held, and is refused with the same
  message.
- The triage and timeline views have no card menu of their own; their
  tickets reach the entry through the ticket detail. The command palette and
  the triage parent picker are unchanged.
- A ticket detail opened from inside the roadmap (a ticket of the epic panel)
  offers the entry too. Used there, the roadmap moves to that epic in place,
  and a refusal leaves the user on the roadmap; the origin view is unchanged,
  since the user did not come from a ticket view.

## Open points

None. The list row entry, left implicit by the clarification (list rows have
no menu), was settled with the owner during specification: an icon in the
action column.
