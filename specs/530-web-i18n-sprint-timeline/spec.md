# Spec #530 - Web i18n: sprint timeline and sprint lifecycle dialogs

Clarification: `docs/clarifications/530.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - Timeline in the UI language

- **Given** English, **when** I open the Timeline, **then** its labels
  ("Time slicing & automatic cycle computation", "Start S1", "Show closed
  sprints", "Assign to a sprint…", "Unplanned backlog"), empty states and
  warnings are English, and dates are formatted `en-US`; in French they are
  today's labels and `fr-FR` dates.
- **Given** a sprint starting `2026-09-01`, **when** it is shown in any time
  zone, **then** it starts on 1 September.

### US2 (P1) - Sprint lifecycle dialogs

- **Given** English, **when** I create, edit, start, close, reopen or delete
  a sprint, **then** the dialog, the unfinished-task destinations, the
  confirmation and the feedback ("Sprints updated") are English, with
  singular and plural counts ("1 unfinished task", "3 unfinished tasks").
- **Given** any language, **when** I save, **then** the sprint name, the dates
  sent to the tracker and the tracker write are exactly as before.

## Functional requirements

- FR1 Every Sectile-owned string of `SprintTimelineView.tsx` comes from the
  catalog.
- FR2 `lib/sprints.ts` no longer formats with `fr-FR`: display formatting goes
  through the shared `formatDate` (#534) with the UI locale; `formatDateISO`
  and `formatDateInput` are unchanged.
- FR3 Sprint names, stored dates, time zones and tracker payloads unchanged.
- FR4 French values keep today's wording (D8).
