---
name: refine-macro
description: "Interactively clarify macro framing text with the user and break it down into structured todos and TaskFlow tickets."
---
# Refine Macro (OpenSpec SDD)

Interactive: the user answers in the terminal.

## Goal
Transform high-level macro framing text into an actionable, structured todo list and concrete TaskFlow tickets, interactively clarifying ambiguities with the user when framing text is vague.

## Read first
- The macro title and framing description.
- The active project SDD framework (SpecKit or OpenSpec).
- Existing macro todos and child tasks to avoid duplicating completed work.

## Steps
1. Inspect the macro title and high-level framing description.
2. **Evaluate framing completeness**:
   - If the framing description is empty, under 2 sentences, or lacks clear technical boundaries/acceptance criteria, formulate 3 to 5 numbered clarification questions and ask the user directly in this interactive terminal session before generating tasks.
3. **Decompose & Break Down**:
   - Once answered or if framing text is detailed, group action items according to the selected SDD framework:
     - **SpecKit SDD**: Group into User Stories ([US-x]) and Feature Modules ([FEAT-x]).
     - **OpenSpec SDD**: Group into Capabilities ([CAP-x]) and Change Proposals ([CHANGE-x]).
4. Output the generated checklist of actionable todos AND proposed TaskFlow tickets (Title, IssueType: Story/Task/Bug, Description) for bulk ticket creation.

## Do not
- Do not generate tasks blindly when framing text is vague without asking clarification questions.
- Do not overwrite existing todos or tasks without user confirmation in the UI.
- Do not mutate external tracker issues directly without user trigger.

## Report
- Clarification Q&A summary (if framing was vague).
- Structured list of proposed MacroTodo items.
- Proposed TaskFlow tickets breakdown (Title, IssueType, Description).
- Rationale behind the task breakdown.

