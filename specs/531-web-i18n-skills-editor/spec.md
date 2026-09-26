# Spec #531 - Web i18n: workflow skill editor and execution-mode help

Clarification: `docs/clarifications/531.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - Skill editor chrome in the UI language

- **Given** English, **when** I open Skills without a project, **then** the
  empty state is English.
- **Given** English and a project, **when** I open a default skill, a custom
  skill, a missing one and a divergent one, **then** the indicators, their
  tooltips, the macro skill labels, the editor placeholder, the reset, save,
  regenerate and import actions and their feedback, and the update metadata
  ("Updated … by …", date in the UI language) are English.
- **Given** English, **when** I open the execution-mode choice, **then**
  "Project default", "Interactive", "Headless" and their help are English.

### US2 (P1) - Content is untouched

- **Given** any language, **when** I view or save a skill, **then** its
  Markdown, prompt, command, skill ID and the generated repository
  documentation are exactly what they were.

## Functional requirements

- FR1 Every Sectile-owned string of `SkillsView.tsx` and
  `CommandModePreview.tsx` (and French fallbacks of `lib/workflow.ts` /
  `lib/commandPresets.ts` shown in these screens) comes from the catalog.
- FR2 Skill content, commands, IDs and stored modes never change.
- FR3 Dates use the shared formatters.
- FR4 French values keep today's wording (D8).
