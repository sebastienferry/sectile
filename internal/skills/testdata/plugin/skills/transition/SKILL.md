---
name: transition
description: "Record by hand that a ticket completed a stage: gather the stage's evidence, show it, and call transition_stage once the user confirms."
---
# Record a stage transition



## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use the Sectile server URL the plugin was installed with (its server_url setting) as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- A run Sectile launched reports through `start_run`/`finish_run`, and records its stage when its skill moves one, like any other run; never forge launch or completion status or use another endpoint to evade validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Goal
Record by hand that a ticket completed a stage, with the evidence the server checks. This is a standalone tool, not a step of the workflow: it does no stage work and starts no run. The server validates the transition and has the last word.

## Read first
- The invocation text: the full ticket key, then the stage to record (`clarified`, `specified`, `implemented`, `reviewed` or `finished`). A command that substitutes its arguments gives them under `## Ticket` below; otherwise take them from the text the skill was invoked with. When either is missing or ambiguous, ask the user for it before doing anything else.
- The task, with `get_task`, and its project, with `get_project_context`: the current stage, the assigned branch, the recorded pull requests and the repositories the task changed.
- The task's worktree when there is one: its actual branch (`git branch --show-current`) and whether its commits are pushed.

## Steps
1. Resolve the task: verify its full ID, project and external URL, since a bare key such as #47 can match another project's ticket. Read its current stage and check that the requested stage is the next one in new → clarified → specified → implemented → reviewed → finished. When it is not, say so and ask the user how to proceed.
2. Gather the evidence of the stage: a structured note of what was done (from the task's comments, its clarification or specification files and its commits; ask the user for what you cannot find), the actual branch, and the pull request of every repository the task changed (`prUrl` for the primary repository, `prUrls` for the others), or `noRepositoryChange: true` when the work changed no repository.
3. Check the stage's exit condition below against what you found. When it is not met, say what is missing and stop without recording anything.
4. Show the user the evidence exactly as it will be sent: task key, completed stage, note, branch, and `prUrl`/`prUrls` or `noRepositoryChange`. Ask for confirmation; apply the corrections the user gives and show the result again.
5. Once the user confirms, call `transition_stage` once with that evidence and check its result. On a refusal, report the server's message as it is and do not retry with altered evidence.

### Evidence
A task holds an ordered set of pull requests, one or more per repository it changed, `prUrl` being its primary repository's current one. A pull request on a branch the task already used in that repository is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence. A pull request opened outside a transition is recorded with `record_pull_request`, once per repository.
A task whose work changed no repository (a configuration made through an API, a review, a follow-up) has no pull request to give: pass `noRepositoryChange: true` to `transition_stage` instead of `prUrl`, and say in the note what was done instead. The server refuses the statement when the task records a pull request on its branch or a repository prepared with `prepare_repository_worktree`; then give those pull requests.
Use `add_comment` for an authorized ticket discussion update. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.

### Exit condition per stage
Record a stage only when its exit condition is met:
- `clarified` (from `new`, Clarify Issue): the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never transition new → clarified while any product question or decision remains open.
- `specified` (from `clarified`, Specify Issue): this step is complete.
- `implemented` (from `specified`, Implement Code): this step is complete.
- `reviewed` (from `implemented`, Adjust Existing Pull Request): the pull request is verified and the branch is not behind the remote default branch.
- `finished` (from `reviewed`, Handoff and Close): the merge is confirmed. Do not clean up while the merge is unconfirmed.
- `reviewed` (from `new`, Pickup Issue (Auto-Pilot to PR)): the test suite passes. Do not push or open a PR while it is failing.
- `reviewed` (from `new`, Batch Pickup Issues (Single Worktree & Combined PR)): this step is complete.

## Do not
- Do not call `start_run`, `finish_run` or `report_waiting`: a hand transition is not a run.
- Do not forge, guess or borrow evidence: no pull request from an unrelated branch, no branch other than the actual one, no `noRepositoryChange` for a task that changed a repository.
- Do not call `transition_stage` before the user confirmed the evidence shown, and never record more than one stage per invocation.
- Do not edit labels in the remote tracker: recording the stage is the transition.
- Do not do the stage's work, merge, or delete anything remote.

## Report
- The task key and the stage recorded, with the evidence sent.
- Or why nothing was recorded: a missing argument, an unmet exit condition, the user declining, or the server's refusal with its message.
