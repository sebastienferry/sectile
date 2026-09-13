---
name: rewrite-story
description: "Reformat a story or task description into structured markdown, optionally incorporating task comments."
---
# Rewrite Story



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

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call taskflow_start_run with the full task primary key and skill name. If TASKFLOW_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call taskflow_finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
