## 1. Backend & Skill Definition

- [ ] 1.1 Update `refine-macro` skill template in `internal/db/skilltemplates.go` to include interactive Q&A framing steps and task breakdown instructions.
- [ ] 1.2 Update prebuilt skill asset `.agents/skills/refine-macro/SKILL.md` and sync across `.skills/`, `.claude/`, `.gemini/`, `.agy/`.
- [ ] 1.3 Update `/api/projects/:pid/macros/:key/refine` endpoint in `internal/handlers/handlers.go` and `internal/db/macros.go` to accept interactive prompt parameters and return proposed tasks.

## 2. Frontend UI Implementation

- [ ] 2.1 Update `RoadmapView.tsx` "Raffiner la macro (AI)" button to launch interactive TTY session (`/refine-macro <KEY>`).
- [ ] 2.2 Add bulk task creation handler (`createMacroTasks`) to `RoadmapView.tsx` and `AppContext.tsx`.
- [ ] 2.3 Add "Générer les tickets TaskFlow" action button and task breakdown list in `RoadmapView.tsx` framing pane.
- [ ] 2.4 Display generated proposed tasks with issue type badges (Story, Bug, Task) and checkable selection.

## 3. Validation & Testing

- [ ] 3.1 Write Go unit tests in `internal/db/` for interactive macro skill rendering and task breakdown parsing.
- [ ] 3.2 Verify frontend build, linter (`make test`), and OpenSpec validation (`openspec validate 25-interactive-macro-refinement --strict`).
