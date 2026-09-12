## Why

Refining high-level roadmap macros into actionable engineering tasks currently relies on a static, non-interactive line-by-line parser. When macro framing text is vague, ambiguous, or incomplete, headless parsing produces low-quality or irrelevant TODO checklist items without soliciting user feedback. Adding an interactive LLM workflow ("Raffiner la macro") enables users to engage in a live dialogue with an AI agent to clarify vague macro framing text and decompose the macro into concrete TaskFlow tickets ready for sprint planning.

## What Changes

- Transform the "Raffiner la macro (AI)" action in `RoadmapView.tsx` to launch an interactive agent session (`/refine-macro <KEY>`) in the terminal/TTY panel (`InteractiveTerminal.tsx`).
- Enhance the `refine-macro` skill (`.agents/skills/refine-macro/SKILL.md`) to evaluate framing text completeness, ask 3–5 interactive clarification questions when framing is vague, and output a structured task breakdown upon alignment.
- Extend backend macro handlers (`internal/db/macros.go`, `internal/handlers/handlers.go`) to support interactive skill execution on macros and bulk task creation for generated macro tasks.
- Add a "Générer les tickets TaskFlow" action button in `RoadmapView.tsx` to convert proposed macro breakdown items directly into TaskFlow task cards linked via `parentKey`.

## Capabilities

### New Capabilities
- `interactive-macro-refiner`: Interactive LLM workflow allowing users to clarify vague macro framing text through live TTY dialogue and automatically break down refined macros into concrete TaskFlow tickets.

### Modified Capabilities
- `macro-refiner`: Update macro refinement from static line parsing to interactive skill execution and ticket generation.

## Impact

- **Backend**: `internal/db/macros.go`, `internal/handlers/handlers.go`, `internal/models/models.go`, `internal/db/skilltemplates.go`.
- **Frontend**: `web/src/components/RoadmapView.tsx`, `web/src/components/InteractiveTerminal.tsx`, `web/src/context/AppContext.tsx`.
- **Prebuilt Skill**: `.agents/skills/refine-macro/SKILL.md` rendered across `.skills/`, `.claude/`, `.gemini/`, `.agy/`.
- **APIs**: `POST /api/projects/:pid/macros/:key/refine`, `POST /api/tasks` (bulk creation).
