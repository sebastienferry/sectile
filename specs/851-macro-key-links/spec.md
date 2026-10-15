# Spec #851 - Macro keys link to the tracker

Ticket: https://github.com/sebastienferry/sectile/issues/851
Clarification: `docs/clarifications/851.md` (rounds 1 and 2, settled)
Branch: `feat/851`

## Problem

In the macro view (the roadmap), a macro's key is plain text. To open the
macro on its tracker (Jira epic, GitHub milestone, GitLab epic) the user has
to select the macro and press the side panel's separate "Lien" button. On the
task cards, the task key itself already opens the ticket on its tracker.

## Scope

In: every display of a macro key in the macro view of the web client, which
Sectile Desktop inherits through the shared components: the full row, the
condensed row, the side panel header and the priority/quarter seed table. The
removal of the panel's "Lien" button. The changelog line.

Out: the data model and the server (the tracker URL is already delivered);
the macro badge on task cards, which keeps its in-app navigation (#630); any
desktop-only surface; macros the tracker gives no page for.

## User stories

### US1 - Open a macro from its key (P1)

- **US1.1** Given a macro whose tracker gives a page, when I click its key in
  a full row of the macro view, then the macro's tracker page opens in a new
  tab, and the row is neither selected nor added to or removed from a
  multi-selection.
- **US1.2** The same holds for the key of a condensed row.
- **US1.3** The same holds for the key in the side panel header.
- **US1.4** The same holds for the key in the priority/quarter seed table;
  following it changes no checkbox of the table.
- **US1.5** Hovering the key shows the tooltip "Ouvrir {key} dans le tracker
  distant" (English: "Open {key} in the remote tracker") and underlines the
  key. Its colour, size and font are unchanged.
- **US1.6** A click anywhere else on a row still selects the macro, and a
  Shift or Cmd/Ctrl click still extends the selection, as today.
- **US1.7** Dragging a row between horizons or sections works the same when
  the drag starts on the key.

### US2 - A macro without a tracker page (P1)

- **US2.1** Given a macro the tracker gives no page for, then its key is plain
  text in all four places, and clicking it behaves as a click on the rest of
  the row (or the panel, or the table) does today.
- **US2.2** Such a macro offers no way to open it on a tracker, as today;
  "Copier" still copies its key and title.

### US3 - One way to open the macro from the panel (P2)

- **US3.1** The side panel header no longer shows the "Lien" button. The
  "Copier" button and every other header control stay.
- **US3.2** "Copier" behaves as today: it copies the tracker URL when there is
  one, the key and title otherwise.

## Functional requirements

- **FR1** In the macro view, each of the four displays of a macro key is a
  link to the macro's tracker page when the macro has one, and plain text
  otherwise.
- **FR2** The link opens in a new browsing context, without sending a
  referrer.
- **FR3** Activating the link does not reach the handlers of its container:
  no selection change, no multi-selection change, no checkbox change.
- **FR4** A drag that starts on the link drags the row, not the URL.
- **FR5** The link carries the localised "open in the remote tracker" title,
  in French and English.
- **FR6** The panel's "Lien" button is removed, together with its label
  string, in both languages. The "open in the remote tracker" string stays.
- **FR7** `CHANGELOG.md` gains one `Changed` line under `## [Unreleased]`
  naming both changes: macro keys open the macro on its tracker, and the
  panel's separate link button is gone.

## Acceptance criteria

- In the full row, the condensed row, the panel header and the seed table, a
  macro with a tracker URL shows its key as a link to that URL, opening in a
  new tab, with the localised tooltip.
- Clicking that key changes neither the selected macro nor the
  multi-selection nor a seed checkbox.
- A macro without a tracker URL shows a plain-text key everywhere.
- Dragging a row by its key still moves it between horizons.
- The panel header has no "Lien" button; "Copier" still works.
- The `link` string is gone from both locale catalogs; `openRemoteTitle`
  remains.
- `CHANGELOG.md` has the `Changed` line.

## Open points

None. Both product questions were answered in round 2 of the clarification.
