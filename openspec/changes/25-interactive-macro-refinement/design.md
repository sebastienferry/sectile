## Context

TaskFlow macros represent high-level roadmap epics or initiatives. Previously, the "Raffiner la macro" action executed a static helper `GenerateMacroTodosFromFraming` that split description lines into `MacroTodo` items without AI reasoning or interactive dialogue. Users need an interactive session where an AI agent can detect ambiguities in macro framing, ask clarification questions, and generate concrete tasks (User Stories, Technical Tasks, Bugs) attached to the macro.

## Goals / Non-Goals

**Goals:**
- Connect "Raffiner la macro" in `RoadmapView.tsx` to an interactive TTY session running `/refine-macro <KEY>`.
- Prompt the LLM to inspect macro framing text; if vague, ask 3–5 clarification questions in the interactive terminal.
- Provide a bulk task creation handler ("Générer les tickets TaskFlow") in the UI that creates TaskFlow cards with `parentKey` matching the macro key.
- Persist refined `MacroTodo` items into macro metadata (`MacroMeta`).

**Non-Goals:**
- Automatically deleting or modifying pre-existing tasks under the macro.
- Executing git commits or branch checkouts automatically during macro refinement.
- Forcing interactive session if the user explicitly prefers headless background generation.

## Decisions

1. **Interactive Session Binding**:
   - The "Raffiner la macro (AI)" button invokes `injectTaskSkill` / `launchTaskTerminal` with command `/refine-macro <KEY>`.
   - The interactive terminal panel (`InteractiveTerminal.tsx`) opens alongside the framing pane, displaying live agent output and allowing user input.

2. **Framing Completeness & Q&A Protocol**:
   - The `/refine-macro` prompt instructs the model:
     - If `description` is empty or vague (< 2 sentences, lacking scope or criteria), formulate 3-5 numbered clarification questions.
     - Once answered or if framing is complete, output GFM JSON/Markdown block containing `todos` and `proposedTasks` (`title`, `issueType`, `description`).

3. **Bulk Task Creation API**:
   - Add frontend context helper `createMacroTasks(macroKey, tasks)` or extend `POST /api/tasks` bulk handler to instantiate cards with `projectId`, `parentKey`, `title`, `description`, `issueType`.

4. **Backward Compatibility**:
   - Retain headless refinement API `/api/projects/:pid/macros/:key/refine` for automated scripts or non-interactive fallbacks.

## Risks / Trade-offs

- **Risk**: User closes terminal panel before refinement completes.
  - *Mitigation*: Terminal session runs in background; session state persists in `TaskSessionID`.
- **Risk**: Duplicate task creation if clicked multiple times.
  - *Mitigation*: UI disables bulk creation button once tasks are generated for a macro session or highlights existing tasks.
