## Purpose

Provides an interactive macro refinement workflow (`interactive-macro-refiner`) that allows product managers and engineers to clarify vague macro framing text via interactive LLM dialogue and decompose macros into concrete TaskFlow tickets.

## ADDED Requirements

### Requirement: Interactive Macro Refinement TTY Session
The Roadmap view SHALL trigger an interactive terminal session running `/refine-macro <KEY>` when the user clicks "Raffiner la macro (AI)".

#### Scenario: Launching interactive refinement
- **WHEN** the user selects a macro in the Roadmap framing pane and clicks "Raffiner la macro (AI)"
- **THEN** the application opens the interactive terminal panel and executes `/refine-macro <KEY>`, displaying live LLM output.

### Requirement: Interactive Framing Clarification
The `refine-macro` skill SHALL evaluate macro framing text and ask 3 to 5 clarification questions if the description is vague or incomplete.

#### Scenario: Vague framing text
- **WHEN** the macro framing description is empty, under 2 sentences, or lacks technical scope
- **THEN** the LLM asks 3 to 5 numbered clarification questions in the interactive terminal and waits for user response before generating tasks.

#### Scenario: Detailed framing text
- **WHEN** the macro framing description is complete and detailed
- **THEN** the LLM synthesizes the scope, updates macro TODO items, and proposes a structured breakdown of tasks.

### Requirement: Bulk Task Creation from Macro Breakdown
The Roadmap view SHALL display proposed breakdown tasks with a "Générer les tickets TaskFlow" action button to create TaskFlow task cards linked to the macro.

#### Scenario: Bulk creating macro tickets
- **WHEN** the user reviews proposed tasks in the framing pane and clicks "Générer les tickets TaskFlow"
- **THEN** the application creates TaskFlow task cards with `parentKey` set to the macro key and updates the Roadmap view.
