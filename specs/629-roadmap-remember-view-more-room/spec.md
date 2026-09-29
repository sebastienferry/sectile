# Specification #629 - Roadmap: remember the view and give the list more room

- Ticket: https://github.com/sebastienferry/sectile/issues/629
- Branch: `claude/clarify-issue-gh-11a4f59c-d3f15d`
- Clarification: `docs/clarifications/629.md` (rounds 1 and 2, all questions
  answered by the owner)
- Framework: Spec Kit

## Summary

The roadmap forgets how the user left it: coming back from another view
reopens NOW on its first epic, with the panel collapsed back and every section
unfolded. With this change the roadmap reopens as it was left. The user can
also hide the details panel so the epic list takes the whole width, expand the
panel with nothing else on screen, write an epic's framing in a full-screen
editor with its preview beside it, and copy the epic's link. "Open in tracker"
opens the epic itself instead of one of its tickets.

## Scope

In scope: the roadmap view of the web app (also shown by Sectile Desktop), the
macro list the server returns to it, the shared Markdown editor (an opt-in
option), their French and English strings, and the changelog.

Out of scope:

- The maturity badge: no lagging count, no change to its rule (owner decision,
  round 2).
- Remembering the panel display mode (execution, framing, phases, goals) per
  tab.
- The ticket description editor of the ticket detail modal, the comment editor
  and the quick-add editor: they keep today's editor.
- Epic label badges (#626), the Timeline.

## User stories

### US1 (P1) - The roadmap reopens as I left it

As a roadmap user, I go to another view and come back without losing my place.

1. Given I am on the LATER tab, when I open the board and come back to the
   roadmap, then the LATER tab is active and the panel shows its default
   framing display.
2. Given I selected epic M-12 in project A, when I leave and come back, then
   M-12 is selected and its panel is open.
3. Given I selected M-12 in project A and M-30 in project B, when I switch from
   B back to A, then M-12 is selected.
4. Given the remembered epic is no longer on the active tab (closed, hidden by
   a filter, deleted, moved to another horizon), when the roadmap opens, then
   the first epic of the tab is shown, as today; when the remembered epic is
   visible again later, it is selected again.
5. Given I folded the Description section and left the Framing notes open,
   when I come back, then the Description is folded and the Framing notes are
   open, for whatever epic I select.
6. Given I expanded the panel, when I come back, then it is still expanded.
7. Given I reload the page, then all of the above still hold.
8. Given a search on the roadmap finds nothing on the remembered tab, then the
   roadmap still jumps to the tab that has a match, as today, and that tab is
   the one remembered from then on.
9. Given the browser refuses storage (private mode, quota), then the roadmap
   works as today and forgets on leaving.

### US2 (P1) - Hide the panel to give the list the full width

As a user comparing many epics, I put the details panel away and keep it away
while I browse.

1. Given an epic selected, when I click "Masquer le panneau" in the panel
   header, then the panel and its split handle disappear and the list takes
   the full width.
2. Given the panel hidden, when I select another epic, then the panel stays
   hidden and the row shows as selected.
3. Given the panel hidden, then a narrow rail on the right edge of the view
   offers "Afficher le panneau"; when I click it, the panel comes back at its
   remembered width, on the selected epic.
4. Given the panel expanded, when I hide it, then it is no longer expanded;
   given it hidden, when I bring it back, it opens at its normal width.
5. Given I hid the panel, when I leave and come back or reload, then it is
   still hidden.
6. Given no epic is shown on the tab, then neither the panel nor the rail
   appear, as today.

### US3 (P1) - An expanded panel gets the whole view

As a user working on one epic, I give its panel the whole view.

1. Given an epic selected, when I click "Agrandir", then the panel takes the
   full width, and the horizon tabs, the toolbar (display modes, filters,
   search, row shape, create, pushes), the filter chips and the sprint strip
   disappear with the list. The application sidebar stays.
2. Given the panel expanded, then its header still shows "Réduire", the copy
   and tracker buttons and the epic actions; when I click "Réduire", the list
   and the whole toolbar come back as they were (tab, search, filters
   unchanged).
3. Given the panel expanded, when no epic remains to show (the last one is
   closed or deleted), then the toolbar and the list come back.

### US4 (P1) - Write the framing in a full-screen editor

As a user framing an epic, I write its description and its framing notes with
room and see the result while I type.

1. Given the Description or the Framing notes section open, then its editor
   shows a maximize button.
2. When I click it, then a full-screen editor opens over the whole
   application, with the text on one side and its rendered preview on the
   other, updated as I type, and the formatting toolbar available.
3. When I press Escape or click "Réduire" in that editor, then it closes, the
   section shows the text I typed, and nothing else closes (not the panel, not
   a dialog behind it).
4. Given I typed in the full-screen editor, then the section's Save button
   appears as for any edit, and nothing is saved until I click it.
5. Given the comment editor, the quick-add editor or the ticket description
   editor, then none of them shows a maximize button.

### US5 (P1) - Open and copy the epic itself

As a user who shares an epic, I get its own address, not one of its tickets'.

1. Given an epic that has an address on its tracker (a GitHub milestone, a
   Jira epic), when I click "Ouvrir dans le tracker", then the epic's own page
   opens in a new tab.
2. Given an epic with no address (created locally, or its tracker gives none),
   then "Ouvrir dans le tracker" is not shown, even when the epic has tickets.
3. When I click "Copier le lien" on an epic with an address, then the clipboard
   holds that address, and a toast "Lien copié" shows it.
4. Given an epic with no address, when I click "Copier le lien", then the
   clipboard holds `<KEY>: <title>`, and a toast "Référence copiée" shows it;
   the button's tooltip says in advance that the reference will be copied.
5. Given the clipboard is refused (insecure origin, denied permission), when I
   click "Copier le lien", then an error toast says the copy failed and shows
   the text to copy by hand, long enough to be read.

## Functional requirements

- **FR1** The roadmap remembers, per browser, across views and reloads: the
  active tab, whether the panel is expanded, whether it is hidden, and the
  folded state of the Description and of the Framing notes sections.
- **FR2** The roadmap remembers the selected epic per project, per browser.
- **FR3** A remembered value that is unknown, unreadable or refers to nothing
  visible falls back to today's default (NOW, first visible epic, panel shown
  and not expanded, sections open) without being erased.
- **FR4** Every change of tab is remembered, whether it comes from a click or
  from the search jump; the display mode default of the new tab applies as
  today.
- **FR5** The panel header offers "Masquer le panneau" next to "Agrandir".
  Hiding removes the panel and its split handle; a rail on the right edge
  brings it back. Hidden and expanded are exclusive: hiding clears expanded,
  expanding or bringing the panel back clears hidden. Selecting an epic never
  changes the hidden state.
- **FR6** While the panel is expanded and an epic is shown, the view renders
  only the panel: no tabs, toolbar, filter chips, sprint strip, list or split
  handle.
- **FR7** The Description and the Framing notes editors of the epic panel
  offer a maximize button that opens a full-screen editor with the input and
  its live preview side by side; Escape or its Réduire button closes it and
  closes nothing else. The text is the same draft as the section's: editing
  there marks the section dirty and saves nothing by itself.
- **FR8** No other Markdown editor of the application changes.
- **FR9** The macro list returned by the server carries, for each epic, the
  address of the epic on its tracker when one is known, and none otherwise.
  No address is guessed for an epic the tracker does not know.
- **FR10** "Ouvrir dans le tracker" uses that address and is hidden without
  one.
- **FR11** "Copier le lien" copies the epic's address, or `<KEY>: <title>`
  without one, confirms which was copied, and on failure shows the text to
  copy by hand.
- **FR12** All new texts exist in French and English.
- **FR13** `CHANGELOG.md` gains one line under `[Unreleased]` for this change.

## Acceptance criteria

- AC1: US1 to US5 hold in the web app, in French and in English.
- AC2: After a reload, the tab, the selected epic of the current project, the
  hidden or expanded panel and the folded sections are as they were left.
- AC3: With the panel expanded, no horizon tab and no toolbar control is in
  the page.
- AC4: The maximize button appears on exactly two editors, the epic
  Description and Framing notes.
- AC5: On a GitHub project, "Ouvrir dans le tracker" of M-11 opens
  `https://github.com/<repo>/milestone/11`; a macro key the milestone list
  does not contain gets no address.
- AC6: The maturity badge is unchanged.
- AC7: The web type check, lint, unit tests, browser component tests and the
  Go tests of the changed packages pass.

## Open points

None. Every product question of the clarification is answered.
