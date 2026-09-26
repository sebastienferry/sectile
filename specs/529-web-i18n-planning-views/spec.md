# Spec #529 - Web i18n: roadmap, triage and team workload views

Clarification: `docs/clarifications/529.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - Roadmap

- **Given** English, **when** I open the Roadmap, **then** the tabs
  ("Unclassified", "Hidden"), filters ("unassigned", "pinned only"), macro
  framing and slicing controls, realign button and feedback ("Framing
  required") are English; horizons read NOW, NEXT, FUTURE in both languages
  and their stored keys are unchanged.

### US2 (P1) - Triage

- **Given** English, **when** I open Triage, **then** "no team", "no
  assignee", the filter placeholder, "Done hidden", "Deselect all", "All
  sorted!", batch assignment forms and selection counts are English, with
  singular and plural forms.

### US3 (P1) - Team

- **Given** English, **when** I open Team, **then** the empty state, refresh
  tooltip, assignment summaries, "outside the team" and "Unassigned" are
  English, with dates in the UI language.

## Functional requirements

- FR1 Every Sectile-owned string of `RoadmapView.tsx`, `MacroTaskRow.tsx`,
  `MacroLabelGroups.tsx`, `MacroRealignButton.tsx`, `TriageView.tsx`,
  `CurationTable.tsx`, `TeamView.tsx` and the display text of
  `lib/roadmap.ts`, `lib/macroRuns.ts`, `lib/labelAxes.ts` comes from the
  catalog.
- FR2 Horizon keys, macro keys, label values, member and team names, task
  titles are never translated.
- FR3 Counts use `plural`, dates the shared formatters.
- FR4 French values keep today's wording (D8).
