---
name: create-pr
description: "Create or reuse a pull request for the current task branch without advancing its workflow stage."
---
# Create PR



## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Goal
Publish the current task branch as a reviewable pull request. This is a standalone utility, outside the five-stage agentic workflow.

## Read first
- The task, specification, repository instructions, current branch, diff and existing pull requests.

## Steps
1. Reuse the assigned worktree and branch. Inspect the complete diff and verify the target repository and base branch.
2. Run git fetch origin and reconcile the remote default branch (for example origin/main). Do not publish while behind the remote default branch. Preserve shared history; prefer rebase when the branch is private, and use git push --force-with-lease only when an authorized private-branch rebase requires it. Run the repository's required build, lint and tests. Fix findings before publishing and record the results.
3. Commit and push the authorized changes. Look up the matching open PR for this branch before creating one; reuse it if present.
4. Create a draft PR if none exists, or update the existing PR description with the final scope and validation. Preserve its existing draft/ready state.
5. Verify the remote PR URL and head commit. Report the PR URL and evidence without transitioning the task.

## Do not
- Do not advance workflow stages, mark the task reviewed, merge, approve or clean up the worktree.
- Do not create duplicate PRs or publish with failing checks.

## Report
- PR URL and branch.
- Scope of the change and validation results.
- Confirmation that the task workflow stage was preserved.

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
