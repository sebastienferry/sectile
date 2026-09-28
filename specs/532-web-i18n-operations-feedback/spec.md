# Spec #532 - Web i18n: activity, synchronization and operation feedback

Clarification: `docs/clarifications/532.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - Notifications in one language

- **Given** English, **when** a read fails or a connection drops, **then**
  the banner and the toast are English, title and body alike ("Connection
  error", "Tracker statuses", "Tracker boards"), and the server's raw detail
  is quoted unchanged.
- **Given** English, **when** a sync or a project operation completes or
  fails, **then** its notification is English and interpolates project,
  tracker and task names unchanged.

### US2 (P1) - Activities view

- **Given** English, **when** I open an activity, **then** "Created at",
  "Started at", "Total duration", "Select an activity", the running and
  empty-output messages are English, and dates follow the UI language.
- **Given** an activity created by the server with a known template (for
  example `Exécution de clarify sur l'agent local` or
  `Tâche ciblée : #526 - Title`), **when** I read it in English, **then** I
  see `Running clarify on the local agent` and `Target task: #526 - Title`;
  in French I see the original text.
- **Given** an activity text no template matches (agent output, an external
  error, a custom command), **when** I read it in either language, **then** it
  is shown verbatim.
- **Given** an activity persisted before this change, **when** I read it,
  **then** the same rendering applies.

### US3 (P2) - Sync view

- **Given** English, **when** I open the Sync view, **then** its cards,
  options and results are English.

## Functional requirements

- FR1 Every Sectile-owned string of `ActivitiesView.tsx`, `SyncView.tsx`,
  `DegradedReadBanner.tsx`, `ToastContainer.tsx`, `RunStateGlyph.tsx` and the
  notifications and operation messages built in `AppContext.tsx` comes from
  the catalog; a notification never mixes languages.
- FR2 `web/src/lib/activityText.ts` recognizes the server's activity
  templates (action, summary, steps) and renders them in the viewer's locale;
  unmatched text is returned unchanged.
- FR3 The recognizer is anchored (whole-string match) and captures parameters
  verbatim.
- FR4 No API, storage or server change; ADR 0035 records the decision.
- FR5 French values keep today's wording (D8).
