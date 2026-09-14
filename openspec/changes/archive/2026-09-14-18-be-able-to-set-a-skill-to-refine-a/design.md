## Context

In TaskFlow, macros represent high-level roadmap items or epics. A macro holds framing text (`description`) and a list of shaping todos (`todos`). Breaking down high-level framing text into executable action items is currently a manual process. Moreover, TaskFlow uses Specification Driven Development (SSD) frameworks—**SpecKit** and **OpenSpec**—to structure specifications and action plans.

This feature adds a macro-level skill (`refine_macro` / `/refine-macro`) that users can view and configure in the **Skills** management view and trigger from **Roadmap** views when refining a macro.

## Goals / Non-Goals

**Goals:**
- Provide a prebuilt `refine-macro` skill (`/refine-macro`) shipped as `.agents/skills/refine-macro/SKILL.md` for agent runners.
- Extend the **Skills** management view to present macro-level skills cleanly with clear target scope ("Macro Refinement").
- Integrate a "Refine Macro" trigger in `RoadmapView.tsx` framing mode / side panel when inspecting a macro.
- Parse framing text and generate structured action plans composed of `MacroTodo` items.
- Adapt the prompt template and generated action plan layout based on the active SSD framework (**SpecKit** vs **OpenSpec**):
  - **SpecKit**: Formats plans as user story / feature module checklists (e.g. `[US-1] ...`, `[FEAT] ...`).
  - **OpenSpec**: Formats plans as capability / change proposal checklists (e.g. `[CAP-1] ...`, `[CHANGE] ...`).
- Provide an interactive preview modal/panel in Roadmap screens allowing users to review generated todos before saving them to `MacroMeta`.

**Non-Goals:**
- Automatically creating tracker tickets for generated todos without explicit user action ("Créer story").
- Modifying closed macros or external GitHub Milestone status without user interaction.

## Decisions

1. **Macro Skill Definition**:
   - Internal ID: `refine_macro` / `refine-macro`
   - Command: `/refine-macro`
   - Scope: `macro` (distinct from single-ticket stage skills)
   - Prebuilt Skill Path: `.agents/skills/refine-macro/SKILL.md`

2. **SSD Framework Integration**:
   - The backend checks `proj.SpecFramework` ("speckit" or "openspec") when rendering the prompt or executing the refinement handler.
   - When **SpecKit** is set, the prompt instructs the model to structure todos by user stories and feature modules.
   - When **OpenSpec** is set, the prompt instructs the model to structure todos by capabilities and change proposals.

3. **Roadmap UI Trigger & Preview**:
   - In `RoadmapView.tsx` framing pane, add a "Raffiner la macro (AI)" button next to the framing description editor.
   - Invoking this action sends `POST /api/macros/:key/refine` or runs the `refine_macro` skill.
   - The UI displays a preview modal/panel with proposed `MacroTodo` items.
   - Users can choose "Replace Todos", "Append Todos", or "Discard".

4. **Skill Editor Visibility**:
   - `SkillsView.tsx` is updated to handle both task-stage skills and macro-level skills. Macro skills are tagged with a "MACRO" badge and target scope labels.

## Risks / Trade-offs

- **Risk**: Automated refinement might overwrite existing manually written todos on the macro.
  - *Mitigation*: Render preview with options to either append to existing todos or replace them; require explicit confirmation before calling `SaveMacroMeta`.
- **Risk**: Missing framing text when invoking skill.
  - *Mitigation*: Display a validation message prompting the user to enter framing text before executing refinement.
