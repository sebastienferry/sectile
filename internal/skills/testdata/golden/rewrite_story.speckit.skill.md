---
name: rewrite-story
description: "Reformat a story or task description into structured markdown, optionally incorporating task comments."
---
# Rewrite Story



## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Sectile reporting
- Task access, the run indicator, stage transitions, pull request links and ticket comments follow `/report-stage`: read `report-stage/SKILL.md` in the same skills directory as this skill (`.claude/commands/report-stage.md` for Claude Code). Load it before the first ticket read.
- If it cannot be loaded, these rules still hold. Standalone: call start_run before work, reusing a supplied SECTILE_RUN_ID or launch runId, and call finish_run when the entire invocation ends, including errors or stopping for user input; a nested skill never finishes the outer run. Managed run: submit only through the supplied result contract, and call no transition or comment tool.
- Gate: this skill does not change the workflow stage; never record a stage transition.

## Goal
Reformat a task's title, description, and optional comments into a clean GitHub-Flavored Markdown specification (User Story: As a..., I want..., So that... + Context + Acceptance Criteria + Notes).

## Read first
- The task: title, description, and task comments (if requested or passed as context).
- Standard GitHub-Flavored Markdown (GFM) formatting guidelines.

## Steps
1. Inspect the task title, raw description, and comments (if provided).
2. Extract the core intent, user value, technical context, and acceptance criteria.
3. Generate a structured GFM document containing:
   - **User Story**: As a <role>, I want <feature>, So that <benefit>.
   - **Context**: Problem background and technical overview.
   - **Acceptance Criteria**: Checkbox list (- [ ]) of verifiable functional & non-functional requirements.
   - **Notes**: Extra technical details or risks mentioned in comments.
4. Output the reformatted markdown directly for preview and user confirmation.

## Do not
- Do not mutate task title, status, priority, assignee, branch, or pull request.
- Do not delete or overwrite task comments.
- Do not invent artificial requirements not implied by the description or comments.

## Report
- The reformatted GFM description preview.
- List of comment points integrated into acceptance criteria (if any).
