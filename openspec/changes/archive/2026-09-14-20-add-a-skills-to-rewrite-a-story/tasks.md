## 1. Backend Skill Definition & Models

- [x] 1.1 Add `rewrite_story` / `rewrite-story` definition and mappings to `internal/models/models.go` (`SkillDirNames`).
- [x] 1.2 Add `rewrite-story` skill template to `internal/db/skilltemplates.go` with GFM prompt structure, goal, steps, and guardrails.
- [x] 1.3 Update skill generator functions in `internal/db/projectskills.go` to ensure `rewrite-story/SKILL.md` and `.claude/commands/rewrite-story.md` are written to disk across agent directories.
- [x] 1.4 Ensure backend skill execution endpoint (`/api/skills/run`) handles `withComments` flag and passes comment contents from `GetTaskComments` into prompt context.

## 2. Frontend UI Implementation

- [x] 2.1 Add "Rewrite Story" button and "Include task comments" checkbox to description section and Skills tab in `TaskDetailModal.tsx`.
- [x] 2.2 Implement skill execution state and preview renderer in `TaskDetailModal.tsx` showing generated GFM markdown.
- [x] 2.3 Add "Apply to Description" action button calling `updateTask(taskId, { description: newContent })`.
- [x] 2.4 Add toast notification on success/error and reset preview state.

## 3. Prebuilt Skill Asset & Documentation

- [x] 3.1 Ship prebuilt skill definition file in `.agents/skills/rewrite-story/SKILL.md`.
- [x] 3.2 Add unit and integration tests for backend skill template rendering and frontend component interactions.
- [x] 3.3 Run build, linter, and tests to confirm 100% pass rate.
