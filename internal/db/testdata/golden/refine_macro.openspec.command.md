---
description: "Interactively clarify macro framing text with the user and break it down into structured todos and Sectile tickets."
argument-hint: <MACRO-KEY> [contexte]
---
# Refine Macro (OpenSpec SDD)

Interactive: the user answers in the terminal.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the macro is identified, and before doing the work, rename the current session to `<macro ID> - <macro title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Goal
Transform high-level macro framing text into an actionable, structured todo list and concrete Sectile tickets, interactively clarifying ambiguities with the user when framing text is vague.

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
4. Output the generated checklist of actionable todos AND proposed Sectile tickets (Title, IssueType: Story/Task/Bug, Description) for bulk ticket creation.

## Do not
- Do not generate tasks blindly when framing text is vague without asking clarification questions.
- Do not overwrite existing todos or tasks without user confirmation in the UI.
- Do not mutate external tracker issues directly without user trigger.

## Report
- Clarification Q&A summary (if framing was vague).
- Structured list of proposed MacroTodo items.
- Proposed Sectile tickets breakdown (Title, IssueType, Description).
- Rationale behind the task breakdown.

## Ticket
$ARGUMENTS
