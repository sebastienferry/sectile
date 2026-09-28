# Spec #526 - Web i18n: navigation, project selection, board and backlog controls

Clarification: `docs/clarifications/526.md` (batch decisions D1 to D9).

## User stories

### US1 (P1) - The shell follows the UI language

As an English user, the sidebar, the project picker, the header, the pinned
bar, the command palette, the display scale menu and the status bar are in
English, tooltips and accessible names included; in French they are in French.

- **Given** English, **when** I open the project picker, **then** I read
  "Project spaces", "Remove from favorites", "Configure this project" and
  "New project…", not their French versions.
- **Given** English, **when** I hover or focus a navigation entry (Skills,
  Team, views), **then** its tooltip and accessible name are English.
- **Given** I switch to French, **when** the shell re-renders, **then** no
  English label remains, and none of the French labels changed wording.

### US2 (P1) - Board and backlog controls follow the UI language

- **Given** English, **when** I open the Board, **then** column actions
  ("Collapse", "Hide Done", "Add a task"), Sectile's fallback headings
  ("Unclassified") and counts are English, while tracker column names and
  statuses are shown as the tracker names them.
- **Given** English, **when** I open a card's action menu or the backlog
  selection, **then** "Select for a batch", "Open the issue", "Unpin" and the
  other actions are English, as accessible names too.
- **Given** 1 and 3 selected tasks, **when** the selection count shows,
  **then** it uses the singular and the plural form of the language.

### US3 (P2) - Filters

- **Given** English, **when** I open the filters, **then** placeholders,
  tooltips, empty states and chips are English.

## Functional requirements

- FR1 Every Sectile-owned string of `Sidebar.tsx`, `Header.tsx`,
  `PinnedBar.tsx`, `BoardView.tsx`, `ListView.tsx`, `TaskCard.tsx`,
  `TaskFilters.tsx`, `CommandPalette.tsx`, `DisplayScaleMenu.tsx`,
  `StatusBar.tsx` (and the small board helpers they render:
  `BoardGroupingToggle.tsx`, `BoardSortSelect.tsx`, `EpicMarker.tsx`,
  `RemoteRunBadge.tsx`, `BatchPickupModal.tsx`, `BoardViewModal.tsx`) comes
  from the catalog: labels, `title`, `aria-label`, `placeholder`, confirmations,
  empty and loading states, toasts.
- FR2 Counts use `plural` (D3); interpolations use `format` (D2).
- FR3 Tracker column names, statuses, labels, task titles and keys are never
  passed through the catalog.
- FR4 French values keep today's wording (D8).
- FR5 The Skills navigation tooltip (Sidebar) is translated here for #531.

## Out of scope

Quick-add (#455), task detail (#527), native desktop UI.
