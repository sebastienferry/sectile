---
description: "Refine a macro framing text into a structured action plan of todos respecting the project SDD framework."
argument-hint: <TICKET-KEY> [contexte]
---
# Refine Macro (OpenSpec SDD)



## Goal
Transform high-level macro framing text into an actionable, structured todo list aligned with the active Spec-Driven Design framework (SpecKit or OpenSpec).

## Read first
- The macro title and framing description.
- The active project SDD framework (SpecKit or OpenSpec).
- Existing macro todos to avoid duplicating completed work.

## Steps
1. Read the macro title and high-level framing description.
2. Structure the action plan according to the selected SDD framework:

   **If using SpecKit SDD:**
   - Group action items into User Stories and Feature Modules.
   - Format each todo item clearly with functional scope (e.g. "[US-1] User Story description" or "[FEAT] Feature item").

   **If using OpenSpec SDD:**
   - Group action items into Capabilities and Change Proposals.
   - Format each todo item clearly with delta scope (e.g. "[CAP-1] Capability requirement" or "[CHANGE] Proposal change").

3. Output the generated checklist of actionable todos for preview before applying to the macro.

## Do not
- Do not overwrite existing todos without user confirmation in the UI.
- Do not generate unstructured or generic todo items.
- Do not mutate tracker issues or milestones directly without user trigger.

## Report
- Structured list of proposed MacroTodo items.
- Rationale behind the task breakdown.

## Ticket
$ARGUMENTS
