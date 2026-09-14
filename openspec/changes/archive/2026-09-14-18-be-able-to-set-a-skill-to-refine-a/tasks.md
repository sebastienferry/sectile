## 1. Backend & Prebuilt Skill Definition

- [x] 1.1 Add `refine_macro` / `refine-macro` mappings to `internal/models/models.go` (`SkillDirNames`) and skill scope metadata.
- [x] 1.2 Add `refine-macro` skill template to `internal/db/skilltemplates.go` supporting dynamic SSD framework context (`SpecKit` vs `OpenSpec`).
- [x] 1.3 Update skill generator functions in `internal/db/projectskills.go` to ensure `refine-macro/SKILL.md` is generated across agent directories (`.skills/`, `.claude/`, `.gemini/`, `.agy/`).
- [x] 1.4 Add `POST /api/macros/:key/refine` endpoint in `internal/handlers/handlers.go` and `internal/db/macros.go` to run AI macro refinement and return structured `MacroTodo` plan.
- [x] 1.5 Create prebuilt skill definition file in `.agents/skills/refine-macro/SKILL.md`.

## 2. Frontend UI Implementation

- [x] 2.1 Update `web/src/types/index.ts` and `web/src/components/SkillsView.tsx` to display macro-level skills with a distinct "MACRO" badge and scope label.
- [x] 2.2 Add "Raffiner la macro (AI)" action button and loading state to `web/src/components/RoadmapView.tsx` framing pane.
- [x] 2.3 Add interactive refinement preview modal in `RoadmapView.tsx` displaying generated `MacroTodo` checklist.
- [x] 2.4 Add "Appliquer les TODOs" action in `RoadmapView.tsx` calling `saveMacroMeta` to persist generated action plan.
- [x] 2.5 Add toast notifications and error handling for missing framing text or failed refinement requests.

## 3. Testing & Validation

- [x] 3.1 Write Go unit tests for `refine-macro` skill template rendering and macro refinement endpoint (`internal/db/macros_test.go`, `internal/handlers/handlers_test.go`).
- [x] 3.2 Add frontend test cases or component validation for macro refinement UI in `RoadmapView` and `SkillsView`.
- [x] 3.3 Run `openspec validate 18-be-able-to-set-a-skill-to-refine-a --strict`.
- [x] 3.4 Execute project build (`make build` or `go build` / `npm run build`) and test suite (`go test ./...`) to verify 100% green status.
