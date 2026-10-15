# Specification #809 - Web UI Roadmap: Clickable macro ID

- Ticket: https://github.com/sebastienferry/sectile/issues/809
- Branch: `feat/809`
- Clarification: `docs/clarifications/809.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-09)
- Framework: Spec Kit

## Summary

In the web client's Roadmap and Timeline views, a macro (epic) key is shown the
way the ticket detail shows a ticket key: a link that opens the macro on its
tracker, followed by a small copy button that copies the key alone. A macro the
tracker gives no address (a local `M-<n>` macro, a GitLab macro) shows its key
as plain text, still followed by the copy button.

## Scope

In scope:

- the Roadmap's expanded macro row, condensed macro row and side panel header;
- the Timeline's ticket rows and cards that show the macro title chip today;
- the removal of the side panel's "Lien" button, now redundant;
- the browser tests of these surfaces and the changelog.

Out of scope:

- the Roadmap's attach-candidate list and the axes seed modal, which keep the
  key as plain text;
- the Timeline's chip display mode, which shows no macro title today;
- the ticket views (board, list, ticket detail), whose parent key already has a
  link and a copy button;
- the Sectile Desktop app, which renders no roadmap or timeline;
- any server, API, data or tracker change.

## User stories

### US1 - The macro key of a roadmap row opens the macro (P1)

As a web user scanning the Roadmap, I can open a macro on its tracker from its
key, without opening the side panel first.

- **Given** a macro row (expanded or condensed) whose macro has a tracker
  address, **then** its key is shown as a link to that address, followed by an
  external-link icon, and the link opens in a new tab.
- **Given** a macro row whose macro has no tracker address, **then** its key is
  shown as plain text, as today, with no external-link icon.
- **Given** the key link, **when** I hover it, **then** its tooltip says it
  opens the macro on the remote tracker (the wording of the former "Lien"
  button: `Ouvrir {key} dans le tracker distant` / `Open {key} in the remote
  tracker`).

### US2 - The macro key can be copied alone (P1)

As a web user, I can copy a macro key in one click to paste it elsewhere, as I
can for a ticket key.

- **Given** any macro key covered by this specification, with or without an
  address, **then** a copy button follows it, with the tooltip and accessible
  name `Copier {key}` / `Copy {key}`.
- **Given** the copy button, **when** I activate it, **then** the clipboard
  holds the key alone (for example `PE-123` or `M-7`), with no title and no
  URL.
- **Given** a successful copy, **then** the button shows a check icon for about
  two seconds and a success toast says the key was copied, with the same title
  and text as the ticket detail's key copy.
- **Given** a clipboard that is missing or refuses the write, **then** an error
  toast says the copy failed, with the ticket detail's wording, and no check
  icon is shown.

### US3 - Acting on the key leaves the row alone (P1)

- **Given** a roadmap row, **when** I click its key link or its copy button,
  with or without Ctrl, Cmd or Shift, **then** the row is neither selected in
  the side panel nor added to or removed from the multi-selection.
- **Given** a grouped roadmap tab, where rows are draggable, **when** I press
  and drag from the key link or the copy button, **then** no row drag starts.
- **Given** a Timeline row or card, **when** I click the macro key link or its
  copy button, **then** the ticket detail does not open, its checkbox is not
  toggled, and dragging from them starts no card drag.
- **Given** any other part of the row or card, **then** clicking, picking and
  dragging behave as today.

### US4 - The side panel header carries the link (P1)

- **Given** the side panel of a selected macro, **then** the key in its header
  renders as in US1 and US2: a link when the macro has an address, plain text
  otherwise, followed by the copy button.
- **Given** the same header, **then** the "Lien" / "Link" button is gone.
- **Given** the same header, **then** the "Copier" / "Copy" button is still
  there and behaves as today: it copies the macro's link, or `KEY: title` when
  the macro has no address.

### US5 - The Timeline shows the macro key of a ticket (P1)

As a web user planning sprints, I see which macro a ticket belongs to by its
key, and can open or copy it from there.

- **Given** a ticket with a macro (a non-empty parent key), in a Timeline
  layout that shows the macro title chip today (sprint list rows, sprint cards,
  backlog cards), **then** the macro key is shown right before that chip,
  rendered as in US1 and US2.
- **Given** such a ticket whose macro has no title, **then** the macro key is
  still shown where the chip would be.
- **Given** a ticket whose macro has a tracker address known to the server
  (a Jira epic, a GitHub milestone), **then** the key links to that address.
- **Given** a ticket whose macro has no address (a local `M-<n>`, a GitLab
  macro), or whose macro the server does not list, **then** the key is plain
  text with the copy button only. An address is never guessed from the key.
- **Given** the macro list has not loaded yet or failed to load, **then** the
  key is shown as plain text with its copy button, and becomes a link once the
  list arrives.
- **Given** a ticket with no macro, **then** nothing new is shown.

## Functional requirements

- **FR-001** A macro key in the five places of this scope (Roadmap expanded
  row, condensed row, side panel header; Timeline sprint list row, sprint card,
  backlog card) renders through one shared presentation: the key, a link to the
  macro's address with an external-link icon when it has one, then a copy
  button.
- **FR-002** The link's address is the `externalUrl` the server gives for the
  macro. When it is empty or unknown, there is no link.
- **FR-003** The link opens in a new tab and carries `rel="noreferrer"`.
- **FR-004** The copy button copies the key alone, confirms it with a check
  icon for about two seconds and a success toast, and reports a failed write
  with an error toast.
- **FR-005** Clicks on the link or the copy button do not reach the enclosing
  row or card, and neither element starts a drag of the row or card.
- **FR-006** The side panel header no longer shows the "Lien" button; its
  "Copier" button is unchanged.
- **FR-007** The Timeline reads macro addresses from the project's macro list
  the server already exposes, fetched once per project holding at least one
  shown ticket with a macro, and fetched again when that set of projects
  changes. No new endpoint, field
  or request per ticket is added.
- **FR-008** Two projects may hold the same macro key; the Timeline resolves a
  ticket's macro address within the ticket's own project.
- **FR-009** Tooltips and accessible names exist in French and English, and
  reuse the existing strings: the "open on the remote tracker" title of the
  former "Lien" button for the link, the ticket detail's `copyKey` for the
  button, the ticket detail's copy toasts.
- **FR-010** The attach-candidate list, the axes seed modal, the Timeline's
  chip mode and the ticket detail render as before.

## Acceptance criteria

- **AC-1** On the Roadmap, a row for a macro with
  `externalUrl: https://github.com/acme/app/milestone/3` shows its key `M-3` as
  a link with that `href` and `target="_blank"`; a row for a macro without
  `externalUrl` shows `M-1` with no link inside the row's key area.
- **AC-2** Each of these rows has a button named `Copier M-3` / `Copier M-1`;
  activating it writes exactly `M-3` / `M-1` to the clipboard and shows the
  `Identifiant copié` toast.
- **AC-3** A clipboard that rejects the write shows the `Copie impossible`
  error toast and no check icon.
- **AC-4** Clicking the copy button or the key link of a row, plain or with
  Ctrl/Cmd or Shift, leaves the selected macro and the multi-selection as they
  were; a `dragstart` from either does not carry `DRAG_EPIC_KEYS`.
- **AC-5** The side panel header of `M-3` shows the `M-3` link and its copy
  button, shows no `Lien` button, and its `Copier` button still copies
  `https://github.com/acme/app/milestone/3` with the `Lien copié` toast.
- **AC-6** On the Timeline, a ticket whose `parentKey` is `M-3` in a project
  whose macro list gives `M-3` that address shows the `M-3` link before its
  macro title chip, in the list rows, the sprint cards and the backlog cards; a
  ticket whose macro has no address shows `M-1` as plain text with its copy
  button; clicking either does not open the ticket detail.
- **AC-7** The Timeline issues one macro-list request per project shown, not
  one per ticket, and none when no ticket has a macro.
- **AC-8** `CHANGELOG.md` carries an `Added` line under `## [Unreleased]`
  describing the clickable, copyable macro key in the Roadmap and Timeline.

## Open points

None: the four product questions were answered in round 2 of the
clarification, and every recommendation was kept.
