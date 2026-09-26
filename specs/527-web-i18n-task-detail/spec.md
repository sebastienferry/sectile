# Spec #527 - Web i18n: task details, editing and specification actions

Clarification: `docs/clarifications/527.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - Task detail in the UI language, in both presentations

- **Given** English and the drawer presentation, **when** I open a task,
  **then** field labels, lookup hints (assignee, sprint, team), the type
  fallback ("Type: Default"), PR actions and workflow launch controls are
  English; the same holds in the centered modal.
- **Given** English, **when** I copy the identifier, **then** the feedback is
  English and names the task key unchanged.
- **Given** a task moved to another tracker, **when** the confirmation asks,
  **then** it is English and interpolates the project and provider names
  unchanged.

### US2 (P1) - Specification and rewrite views

- **Given** a task with a specification, **when** I display it and open the
  full screen view, **then** "Technical specification", "Copy the full spec",
  "Recommended step" are English; the specification Markdown is unchanged.
- **Given** the rewrite preview, **when** it opens, **then** "Apply to the
  description" and its states are English; the proposed text is unchanged.

### US3 (P2) - Comments, clone dialog, copy-skill menu

- **Given** English, **when** I read comments, clone a task or open the
  copy-skill menu, **then** their chrome, empty states, validation and
  feedback are English, and dates are formatted in the UI language.

## Functional requirements

- FR1 Every Sectile-owned string of `TaskDetailModal.tsx`,
  `TaskComments.tsx`, `CloneTaskModal.tsx`, `CopyTaskSkillMenu.tsx`,
  `LookupField.tsx`, `PrioritySelect.tsx`, `PullRequestStateIcon.tsx` and
  `Markdown.tsx` (copy button feedback) comes from the catalog.
- FR2 Task keys, titles, descriptions, comments, specification Markdown,
  issue type names, statuses, project and provider names are interpolated,
  never translated.
- FR3 Dates use the shared formatters of #534 in the UI locale.
- FR4 French values keep today's wording (D8).
