## Why

Task descriptions imported from external issue trackers (Jira, GitHub, Linear) or written quickly by team members are often unstructured, missing context, or scattered across issue comments. Users spend manual effort reformatting task descriptions to conform to standard user story structures and acceptance criteria. Adding a dedicated `rewrite-story` skill (shipped as a prebuilt skill in `.agents/skills/rewrite-story/SKILL.md`) and UI action enables users and AI agents to quickly reformat task descriptions into clean GitHub-Flavored Markdown (GFM) with optional inclusion of existing task comments.

## What Changes

- Ship a prebuilt workflow skill `rewrite-story` (`/rewrite-story <KEY> [--with-comments]`) in `.agents/skills/rewrite-story/SKILL.md` and register it across TaskFlow skill directories (`.skills/`, `.claude/`, `.gemini/`, `.agy/`).
- Extend TaskFlow Go backend (`internal/db/skilltemplates.go`, `internal/models/models.go`) to define `rewrite-story` as an available prebuilt skill with template rendering and slash command generation.
- Add UI controls in `TaskDetailModal.tsx`:
  - "Rewrite Story" action button in the description section header and Skills tab.
  - "Include task comments" toggle checkbox allowing optional comment context aggregation.
  - Interactive preview modal/panel allowing users to inspect the generated markdown before applying it to the task description.
- Connect description updates to `PUT /api/tasks/:id` for persisting rewritten descriptions back to TaskFlow database and synced tracker items.

## Capabilities

### New Capabilities
- `story-rewriter`: Skill and UI workflow to reformat task descriptions into structured GFM markdown (User Story, Context, Acceptance Criteria, Notes) with optional comment integration, prebuilt skill definition (`.agents/skills/rewrite-story/SKILL.md`), and preview confirmation.

### Modified Capabilities

## Impact

- **Backend**: `internal/models/models.go`, `internal/db/skilltemplates.go`, `internal/db/projectskills.go`, `internal/handlers/handlers.go`.
- **Frontend**: `web/src/components/TaskDetailModal.tsx`, `web/src/types/index.ts`, `web/src/context/AppContext.tsx`.
- **Agent Skills**: Prebuilt `SKILL.md` shipped at `.agents/skills/rewrite-story/SKILL.md` and rendered across `.skills/`, `.claude/`, `.gemini/`, `.agy/`.
- **APIs**: Task update endpoint `PUT /api/tasks/:id` and skill execution API `POST /api/skills/run`.
