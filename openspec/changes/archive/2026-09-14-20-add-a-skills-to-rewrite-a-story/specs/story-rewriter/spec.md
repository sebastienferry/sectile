## Purpose

Provides a specialized skill and UI workflow (`story-rewriter`) that reformats task descriptions into structured GFM markdown (User Story, Context, Acceptance Criteria, Notes) with optional integration of task comments, prebuilt skill asset shipping, and preview confirmation.

## ADDED Requirements

### Requirement: Prebuilt Skill Asset Shipping and Skill Registration
The project SHALL ship a prebuilt skill file at `.agents/skills/rewrite-story/SKILL.md` and register the `rewrite-story` skill (`/rewrite-story`) across agent directories (`.skills/rewrite-story/SKILL.md`, `.claude/skills/rewrite-story/SKILL.md`, `.gemini/skills/rewrite-story/SKILL.md`, `.agy/skills/rewrite-story/SKILL.md`).

#### Scenario: Prebuilt skill file presence
- **WHEN** a user or agent inspects the available skills directory `.agents/skills/`
- **THEN** `.agents/skills/rewrite-story/SKILL.md` exists with frontmatter `name: rewrite-story` and `description: Reformat a story or task description into structured markdown, optionally incorporating task comments.`

#### Scenario: Agent skill lookup
- **WHEN** an agent or user invokes `/rewrite-story <KEY>`
- **THEN** the system executes the `rewrite-story` skill prompt instructing the model to reformat the task description for `<KEY>` into GFM format containing User Story, Context, Acceptance Criteria, and Notes.

#### Scenario: Slash command with comments flag
- **WHEN** an agent or user invokes `/rewrite-story <KEY> --with-comments`
- **THEN** the system fetches existing comments for task `<KEY>` and includes comment text in the prompt context sent to the AI model.

### Requirement: UI Trigger and Preview in Task Detail Modal
The task detail modal SHALL present a "Rewrite Story" action button and an optional "Include task comments" toggle checkbox near the description section and in the Skills tab.

#### Scenario: User triggers story rewrite without comments
- **WHEN** the user clicks "Rewrite Story" with "Include task comments" unchecked
- **THEN** the application runs the `rewrite-story` skill using the task title and current description as prompt context, and displays the generated GFM markdown in a preview panel.

#### Scenario: User triggers story rewrite with comments
- **WHEN** the user checks "Include task comments" and clicks "Rewrite Story"
- **THEN** the application includes task comments in the skill request payload, generates a consolidated story description, and displays the result in a preview panel.

#### Scenario: User applies preview to task description
- **WHEN** the user clicks "Apply to Description" on the preview panel
- **THEN** the application updates the task description via `PUT /api/tasks/:id` with the generated GFM content and closes the preview.

#### Scenario: User discards preview
- **WHEN** the user clicks "Discard" on the preview panel
- **THEN** the existing task description remains unchanged and the preview panel closes.
