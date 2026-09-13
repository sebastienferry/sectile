---
name: specify-issue
description: "Write the executable specification of a ticket in the project's Spec-Driven Design framework, before any code."
---
# Specify Issue (OpenSpec SDD)

Stage: clarified -> specified.

## Goal
Produce a specification another engineer could implement without asking you
anything. Behaviour and acceptance criteria first, implementation choices second,
and the two kept in separate files.

## Read first
- Project-configured SDD framework: openspec. Use it unless the invocation explicitly overrides it.
- The clarification outcome on the ticket: the decisions are already made, apply them.
- Select the SDD framework in order: explicit {sdd_framework} or --framework=<name>,
  then the project-configured framework, then repository detection:
  - If `openspec/` exists -> use OpenSpec SDD.
  - If `.specify/` or `specs/` exists -> use Spec Kit SDD.
- Ensure the project SDD directory is initialized before writing specifications.

## Steps
1. Reuse the assigned worktree and branch (including a shared batch branch). Only create <KEY>-<title-slug> when no work branch is assigned. Preserve existing work; never write on the default branch.
2. Select the SDD framework from {sdd_framework} argument, flag, or project detection:

   **If using OpenSpec SDD:**
   - Create change directory `openspec/changes/<KEY>-<title-slug>/`
   - Write `proposal.md` (problem, value, in/out scope)
   - Write `design.md` (technical decisions, rejected alternatives)
   - Write `tasks.md` (ordered implementation checklist)
   - Write `specs/<capability>/spec.md` (requirements with Given/When/Then)
   - Validate with `openspec validate <change-id> --strict`

   **If using Spec Kit SDD:**
   - Write `specs/<KEY>-<title-slug>/spec.md` (prioritised user stories, functional requirements, Given/When/Then)
   - Write `plan.md` (stack, architecture, data contracts, target files)
   - Write `tasks.md` (ordered implementation checklist with test plan)
   - Use `/speckit.specify`, `/speckit.plan`, `/speckit.tasks` if available.

## Do not
- Do not decide what the clarification left open. Mark it as open and say so.
- Do not describe implementation inside the behaviour file.
- Do not start implementing, even the easy part.

## Report
- The files written, with their paths.
- The work branch.
- Requirements that are still open, and what they block.

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Standalone invocation**: After verifying each completed step, use the local handler below. Check its exit status and response. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Transition clarified → specified only when this step is complete.
```bash
# Route via local agent if available, fallback to http://localhost:8090/api/tasks/stage
ENDPOINT="${TASKFLOW_AGENT_URL:-${TASKFLOW_SERVER_URL:-http://localhost:8090}}/api/tasks/stage"
curl --fail-with-body --silent --show-error -X POST "${ENDPOINT}" \
  ${TASKFLOW_AGENT_TOKEN:+-H "Authorization: Bearer $TASKFLOW_AGENT_TOKEN"} \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"specified","note":"<REPORT_NOTE>","branch":"<ACTUAL_BRANCH>"}' || \
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"specified","note":"<REPORT_NOTE>","branch":"<ACTUAL_BRANCH>"}'
```
This POST calls the local TaskFlow handler directly. Confirm HTTP success before continuing. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
