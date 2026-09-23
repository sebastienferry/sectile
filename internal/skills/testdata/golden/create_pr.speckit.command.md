---
description: "Create or reuse a pull request for the current task branch without advancing its workflow stage."
argument-hint: <TICKET-KEY> [contexte]
---
# Create PR



## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Sectile reporting
- Task access, the run indicator, stage transitions, pull request links and ticket comments follow `/report-stage`: read `report-stage/SKILL.md` in the same skills directory as this skill, for example `~/.claude/skills/report-stage/SKILL.md`. Load it before the first ticket read.
- If it cannot be loaded, these rules still hold. Standalone: call start_run before work, reusing a supplied SECTILE_RUN_ID or launch runId, and call finish_run when the entire invocation ends, including errors or stopping for user input; a nested skill never finishes the outer run. Managed run: submit only through the supplied result contract, and call no transition or comment tool.
- Gate: this skill does not change the workflow stage; never record a stage transition.

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

## Ticket
$ARGUMENTS
