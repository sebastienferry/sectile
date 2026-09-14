## Purpose

Provides a macro-level refinement skill (`refine-macro`) and UI workflow that transforms high-level framing text into an organized action plan of todos (`MacroTodo`), configurable in the Skills section and executable on Roadmap screens according to the project's active SSD framework (SpecKit or OpenSpec).

## ADDED Requirements

### Requirement: Macro-Level Skill Configuration in Skills Section
The Skills management view SHALL display macro-level skills alongside workflow stage skills, allowing users to view, edit, reset, and re-import macro refinement skill configurations.

#### Scenario: Prebuilt macro skill file presence
- **WHEN** a user or agent inspects the available skills in `.agents/skills/`
- **THEN** `.agents/skills/refine-macro/SKILL.md` exists with frontmatter `name: refine-macro` and description designated for macro refinement operations.

#### Scenario: Displaying macro skills in Skills view
- **WHEN** the user navigates to the Skills view (`SkillsView`)
- **THEN** macro-level skills (such as `refine-macro`) are listed with a "MACRO" scope badge and macro refinement details.

#### Scenario: Editing and saving macro skill content
- **WHEN** the user modifies the skill template for `refine-macro` and clicks "Enregistrer"
- **THEN** the application saves the custom template and regenerates `SKILL.md` across all supported agent directories (`.skills/`, `.claude/`, `.gemini/`, `.agy/`).

### Requirement: Roadmap UI Integration and Skill Execution
The Roadmap view SHALL provide an executable "Refine Macro" action button when viewing or framing a macro, allowing users to trigger automated refinement.

#### Scenario: User triggers macro refinement from Roadmap framing pane
- **WHEN** the user selects a macro in the Roadmap view framing pane and clicks "Raffiner la macro"
- **THEN** the application invokes the `refine-macro` skill using the macro title, framing text, and project SSD framework context, displaying a loading state.

#### Scenario: Validation on missing framing text
- **WHEN** the user clicks "Raffiner la macro" on a macro that has an empty description / framing text
- **THEN** the application prompts the user to add framing text before executing refinement.

### Requirement: Framing Text Transformation into Structured Todos
Executing the macro refinement skill SHALL convert macro framing text into an organized list of actionable todo items (`MacroTodo`).

#### Scenario: Generated todo preview display
- **WHEN** macro refinement completes successfully
- **THEN** the Roadmap view displays a preview modal or panel listing the generated action plan items formatted as todo checkboxes.

#### Scenario: Applying generated todos to macro
- **WHEN** the user clicks "Appliquer à la macro" or "Remplacer les TODOs" on the preview panel
- **THEN** the application updates the macro's `todos` array via API and persists the changes to database / tracker.

### Requirement: SSD Framework Specific Formatting (SpecKit vs OpenSpec)
Plan generation SHALL dynamically adapt to the project's selected SSD framework (**SpecKit** or **OpenSpec**).

#### Scenario: Plan generation using SpecKit conventions
- **WHEN** the project's active SSD framework is configured as `speckit`
- **THEN** the refinement skill prompt and generated output structure todos according to SpecKit conventions (user stories, feature modules, functional breakdown).

#### Scenario: Plan generation using OpenSpec conventions
- **WHEN** the project's active SSD framework is configured as `openspec`
- **THEN** the refinement skill prompt and generated output structure todos according to OpenSpec conventions (capabilities, proposal requirements, delta changes).
