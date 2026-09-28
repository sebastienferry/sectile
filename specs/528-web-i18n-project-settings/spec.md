# Spec #528 - Web i18n: project settings and tracker column configuration

Clarification: `docs/clarifications/528.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - Every settings tab in one language

- **Given** English, **when** I open the settings of a project, **then** the
  title ("Settings: Sectile"), the tabs (General, Tracker, Agentic workflow,
  Skills & SDD), field labels ("Project title", "Git repository", "Workspace
  views", "Color by epic") and actions ("Delete", "Update") are English.
- **Given** English, **when** I open a new-project form, **then** it is
  English too, validation messages included.
- **Given** French, **when** I open the same tabs, **then** the wording is
  today's.

### US2 (P1) - Providers are described correctly

- **Given** the Tracker tab, **when** I read GitHub, GitLab or Jira,
  **then** the description says Sectile talks to the tracker's HTTP API,
  reflects workflow stages as labels and syncs in the background, in the UI
  language, with no claim about a CLI.

### US3 (P2) - Column editor

- **Given** English, **when** I edit board columns, **then** "Free statuses",
  "Move up", "Move down", "Remove" and the help text are English; the column
  names and status values are unchanged.

## Functional requirements

- FR1 Every Sectile-owned string of `ProjectModal.tsx` and
  `BoardColumnsEditor.tsx` (and `TrackerSetup.tsx` provider text if shared)
  comes from the catalog: tabs, labels, help, placeholders, provider and
  capability descriptions, optional views, execution policy, deletion and
  validation feedback, `title` and `aria-label`.
- FR2 Repository URLs, tracker settings, workflow mappings, column names and
  status values are never translated or rewritten.
- FR3 Provider descriptions describe the HTTP API integration in both
  languages.
- FR4 French values keep today's wording (D8), except the provider
  descriptions rewritten by FR3.
