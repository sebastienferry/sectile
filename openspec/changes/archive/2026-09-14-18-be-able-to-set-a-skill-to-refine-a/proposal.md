## Why

TaskFlow allows product teams to frame high-level roadmap items ("macros") with initial framing text. Transforming raw framing text into executable action plans and todo items currently requires manual effort. Introducing a macro-level refinement skill (`refine-macro`) enables users to configure macro skills in the **Skills** management view and invoke refinement directly from **Roadmap** screens. The generated action plan dynamically adheres to the project's configured Specification Driven Development (SSD) framework (**SpecKit** or **OpenSpec**), organizing framing text into actionable `MacroTodo` items.

## What Changes

- Register a new built-in macro-level skill `refine_macro` (`/refine-macro`) in backend skill definitions (`internal/models/models.go`, `internal/db/skilltemplates.go`, `internal/db/projectskills.go`).
- Ship a prebuilt skill definition asset in `.agents/skills/refine-macro/SKILL.md` and sync it across agent directories (`.skills/`, `.claude/`, `.gemini/`, `.agy/`).
- Extend the **Skills** management view (`web/src/components/SkillsView.tsx`) to display and configure macro-level skills alongside task stage skills.
- Add a "Refine Macro" action trigger and interactive preview panel in **Roadmap** screens (`web/src/components/RoadmapView.tsx`) for framing macros.
- Extend backend endpoints (`POST /api/macros/refine` or `/api/skills/run`) to process macro framing text using the project's selected `specFramework` (`speckit` vs `openspec`).
- Convert generated action plans into structured `MacroTodo` items that can be saved directly to the macro metadata (`MacroMeta`).

## Capabilities

### New Capabilities
- `macro-refiner`: Skill and UI workflow enabling configuration of macro-level skills in the Skills view and execution of macro refinement on Roadmap screens to convert framing text into structured todo action plans formatted for SpecKit or OpenSpec.

### Modified Capabilities

## Impact

- **Backend**: `internal/models/models.go`, `internal/db/skilltemplates.go`, `internal/db/projectskills.go`, `internal/db/macros.go`, `internal/handlers/handlers.go`, `internal/runner/runner.go`.
- **Frontend**: `web/src/components/SkillsView.tsx`, `web/src/components/RoadmapView.tsx`, `web/src/context/AppContext.tsx`, `web/src/types/index.ts`.
- **Prebuilt Skill**: `.agents/skills/refine-macro/SKILL.md` rendered across agent directories.
- **APIs**: Macro refinement endpoint `POST /api/macros/:key/refine` / `POST /api/skills/run` and macro save endpoint `POST /api/macros`.
