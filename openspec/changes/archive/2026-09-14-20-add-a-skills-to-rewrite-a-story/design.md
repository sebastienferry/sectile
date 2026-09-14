## Context

TaskFlow provides agentic skills (such as `clarify-issue`, `specify-issue`, `code-issue`, `create-pr`, `handoff-issue`, `pickup-issue`) that operate on tasks. Users often need to refine a task's initial description before specifying or coding it. Currently, there is no skill or UI button specifically tailored to reformat task descriptions into standard Agile/SDD structure (User Story: As a..., I want..., So that... + Context + Acceptance Criteria + Notes). Furthermore, task discussions in comments frequently contain refined requirements that should be folded back into the primary task description. The output of this ticket must include a prebuilt skill file shipped with the codebase.

## Goals / Non-Goals

**Goals:**
- Provide a prebuilt `rewrite-story` skill (`/rewrite-story <KEY> [--with-comments]`) shipped as `.agents/skills/rewrite-story/SKILL.md` for agent runners.
- Add a "Rewrite Story" trigger and "Include comments" toggle in `TaskDetailModal.tsx`.
- Generate structured GFM markdown containing User Story, Context, Acceptance Criteria, and Notes.
- Support preview and explicit user confirmation before replacing task description.
- Register `rewrite-story` in TaskFlow Go backend `StageSkills` and skill file generators.

**Non-Goals:**
- Automatic execution upon task creation.
- Editing, deleting, or mutating existing task comments or original ticket titles.
- Modifying task state, branch, PR, or assignee.
- Automatic commit or git branch execution.

## Decisions

1. **Prebuilt Skill Asset & Name**:
   - Internal ID: `rewrite_story` / `rewrite-story`
   - Slash Command: `/rewrite-story`
   - Prebuilt Skill Path: `.agents/skills/rewrite-story/SKILL.md`
   - Command Arguments: `<KEY> [--with-comments]`

2. **Comment Aggregation Option**:
   - Optional boolean flag `withComments` passed to skill runner.
   - When enabled, backend fetches task comments via `db.GetTaskComments(taskId)` and appends comment history to prompt context so the AI model synthesizes description + comments into a coherent story.

3. **Preview & Confirmation UI Flow**:
   - To prevent accidental overwrite of customized task descriptions, the UI presents the generated GFM markdown in a preview block with "Apply to Description" and "Discard" actions.
   - Upon clicking "Apply to Description", the backend handler updates the task description via `PUT /api/tasks/:id`.

4. **Template & Registration**:
   - Added to built-in skills in `internal/db/skilltemplates.go` so it is generated into all supported agent directories (`.skills`, `.claude`, `.agents`, `.gemini`, `.agy`).

## Risks / Trade-offs

- **Risk**: AI output might omit custom details present in raw description.
  - *Mitigation*: Render preview prior to saving; preserve original task description in activity history / cancel action.
- **Risk**: Large volume of comments exceeding context token window.
  - *Mitigation*: Limit comment inclusion to the top N latest/relevant comments or truncate cleanly if necessary.
