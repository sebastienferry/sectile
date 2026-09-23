---
name: report-stage
description: "Report a task run, a completed workflow stage, its pull request and ticket comments to Sectile, on behalf of the skill that did the work."
---
# Report Stage



## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Goal
Deliver what another skill did to Sectile, and nothing more: the run indicator, the completed stage with its note, branch and pull request, and the ticket comment. The calling skill decides whether its gate is met; this skill decides how the result reaches Sectile.

## Read first
- The calling skill's gate and its `## Report` bullets: they are the content, this skill is the delivery.
- The invocation: a managed result-file contract, a supplied SECTILE_RUN_ID or launch runId, or neither.
- The live task and project, through `get_task` and `get_project_context`.

## Steps
1. Take the inputs from the calling skill, or from the user when invoked directly: the full task key, the completed stage (if any), the report note, the actual branch, the pull request URL (if any) and the comment to post (if any).
2. Resolve the task and its project as described in "Sectile task access" below, and check the task's current stage against the stage to record.
3. In a managed run, write the result through the supplied contract and stop there.
4. Standalone, record the stage with `transition_stage`, post the comment with `add_comment`, and check each tool result before going on.
5. When invoked directly to record a stage by hand, do not start a run: a transition or a comment is not a run. The run rules below apply to the skill that does the work.

## Do not
- Do not decide the gate: record a stage only when the calling skill, or the user, states that its condition is met.
- Do not transition past the requested stage, and do not record a stage the task has not reached.
- Do not rename the session: the calling skill owns its title.

## Report
- The recorded stage, branch and pull request URL, with the tool result.
- The run started or finished, with its runId and status.
- The comment posted, or why none was.
- Anything that failed or remains pending, and what a retry needs.

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After the calling skill has verified a completed step and its gate, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
A task holds an ordered set of pull requests, `prUrl` being its current one. A pull request on a branch the task already used is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
