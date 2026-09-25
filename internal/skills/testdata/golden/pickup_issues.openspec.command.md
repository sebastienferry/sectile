---
description: "Batch process a list of selected board tickets sequentially in autonomy inside a single dedicated worktree, producing one combined Pull Request covered by tests and lints."
argument-hint: <TICKET-KEY> [contexte]
---
# Batch Pickup Issues (Single Worktree & Combined PR)

Stage: new -> reviewed.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- For a batch, name the session after the first ticket followed by the remaining count, for example `#47 (+2) - Remove the parallelism setting`.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Goal
Autonomously process a batch of tickets selected from the board sequentially in the exact order provided inside a single dedicated batch worktree.

## Read first
- The list of tickets in the batch.
- The project's code and existing patterns.
- The project SDD framework.

## Steps
1. Inspect the current ticket state AND existing artifacts. Reuse assigned branches, specifications, checklist progress and PRs. Verify completed work before skipping it.
2. Use one dedicated worktree and branch for the ordered batch. Run clarification, specification and implementation for each ticket in order. If one blocks, preserve the batch and report completed tickets and the next action; never include unfinished work as completed.
3. Once all tickets are implemented, run adjustment and final checks across the whole batch and update the ONE combined PR created during the configured earlier stage.
Stop before merge. Stage-local boundaries apply while that stage is active; after its requirements are met, continue to the next stage without asking for routine confirmation.

### Clarify Issue
- The ticket: title, description, comments via get_task, parent epic if present.
- The existing clarification report if one exists: docs/clarifications/<n>.md on the assigned work branch.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

1. Re-read the assigned branch and worktree. If docs/clarifications/<n>.md already exists,
   this run continues an existing clarification into Round N. If not, this is Round 1.
2. In Round 1:
   a. Restate the request in two sentences, including what is out of scope.
   b. List ambiguities, worst first. Only list an ambiguity if two readings lead to different code.
   c. Name critical dependencies: other services, migrations, missing data, third-party limits.
   d. Resolve reversible technical choices using existing code and project conventions.
   e. Formulate essential product questions that alter acceptance criteria, with your recommended option.
   f. Write docs/clarifications/<n>.md, commit with docs(spec): clarify #<n> (round 1).
   g. Ask the questions (interactively in-session if the owner is present; as a ticket discussion
      comment via add_comment when unattended).
3. In Round N (follow-up after owner answers):
   a. Read the owner's answers from the interactive prompt or ticket comments via get_task.
   b. Append a dated section: "## Round N - answers from the owner (<date>)" to docs/clarifications/<n>.md.
   c. Explicitly record settled choices and any reversed prior assumptions.
   d. Address newly surfaced ambiguities or dependencies.
   e. Commit updates with docs(spec): clarify #<n> (round N).
   f. If follow-up product questions remain, ask them and stop without transitioning.
4. Exit condition:
   Rounds continue until the owner confirms that the clarification is satisfactory (or zero open
   product questions remain in unattended pickup). Never transition new → clarified while product
   questions remain open.
5. Persist the settled scope, decisions, and assumptions in the report before concluding.

- Do not transition new → clarified while any product question or decision remains open.
- Do not invent answers to essential product questions in unattended runs; record them and ask.
- Do not write production code or start the technical specification at this stage.
- Do not discard previous round sections when writing Round N; append each round chronologically.
- Do not switch branches or create a new branch: reuse the assigned feat/<n> branch.

Report and persist before continuing:
- The report path: docs/clarifications/<n>.md.
- Current round number and whether the exit condition was met.
- Settled decisions and reversed assumptions.
- Numbered open questions (if any) and who is expected to answer them.
- Stage transition status (applied or blocked awaiting answers).

### Specify Issue
- Project-configured SDD framework: openspec. Use it unless the invocation explicitly overrides it.
- The clarification outcome on the ticket: the decisions are already made, apply them.
- Select the SDD framework in order: explicit {sdd_framework} or --framework=<name>,
  then the project-configured framework, then repository detection:
  - If `openspec/` exists -> use OpenSpec SDD.
  - If `.specify/` or `specs/` exists -> use Spec Kit SDD.
- Ensure the project SDD directory is initialized before writing specifications.

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

- Do not decide what the clarification left open. Mark it as open and say so.
- Do not describe implementation inside the behaviour file.
- Do not start implementing, even the easy part.

Report and persist before continuing:
- The files written, with their paths.
- The work branch.
- Requirements that are still open, and what they block.

### Implement Code
- The specification and its task checklist. It is the contract, follow its order.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.

1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. On a multi-repo project, `$SECTILE_REPOSITORIES` lists the task's folders. Work in the
   primary worktree; the other repositories are read-only context. To change one, call
   `prepare_repository_worktree` for it first and work in the worktree it returns: each
   changed repository then needs its own pull request, given to `transition_stage` in `prUrls`.
3. Work through the checklist in small steps, each one leaving the tree buildable.
4. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
5. Run build, static analysis and tests. Fix until green, and quote the real output.
6. Re-read your own diff before finishing, as a reviewer would.

- Repair routine technical issues and update design/tasks when the implementation
  needs to change while preserving acceptance criteria. Continue after documenting why.
- Establish whether a failing test predates the change. Fix failures in scope; report
  unrelated failures with baseline evidence. Never hide them or mark checks green.
- Stop only for an essential product decision, an unavailable dependency after
  bounded recovery attempts, or work that materially expands the requested scope.
- Preserve the work branch, completed checklist items and remaining next action so
  a retry can resume instead of starting over.

Report and persist before continuing:
- What changed, file by file, and why.
- The real output of build, linters and tests, remaining failures included.
- What you deliberately left out, and what it would take to finish it.

### Adjust Existing Pull Request
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

1. Verify a matching PR exists for the task repository and branch before changing files: open, or already merged by the human. Record its URL. If missing, stop and recover through the configured creation owner (specify or implement). Never create a PR during adjustment, and never push onto a merged PR — review the merged state and report it. Read available PR feedback; retrieval failure is a blocker, not absence of feedback.
   A task that changed several repositories (`$SECTILE_REPOSITORIES` role `changed`) has one PR per repository: verify, review, push and update each of them in its own worktree, the same way.
   Fetch the remote (`git fetch origin`) and compare the work branch with the
   remote default branch (normally `origin/main`; use the repository's configured default when different).
   Integrate missing base commits before the final review: prefer rebase when the branch is private, or merge when
   repository policy or shared-branch state requires it. Resolve conflicts and do not continue until the working tree is clean.
2. Review the complete resulting diff against the specification for correctness, side effects, security, and edge cases with no test. Address actionable feedback and record dispositions. No human feedback is required.
3. Update documentation affected by the change. Fix what the review finds, now. A known defect belongs in the code, not in the
   description of the merge request.
4. Re-run build, static analysis and tests after integrating the default branch and on the final state.
5. Commit with a conventional message: type, scope, and why the change exists.
6. Push the branch and update the same existing merge request: summary, test plan, and the specific
   places where you want a reviewer's eyes.
   Run `git fetch origin`, then choose the push from the state of `origin/<branch>`:
   - `origin/<branch>` does not exist (first publication): run `git push -u origin <branch>`. Never force a branch the remote does not have.
   - `git merge-base --is-ancestor origin/<branch> HEAD` succeeds (fast-forward): run a plain `git push`.
   - Otherwise an authorized rebase rewrote published history: run `git push --force-with-lease`.
   If the push is refused because commits landed on `origin/<branch>` in between (stale lease or non-fast-forward), run `git fetch origin`,
   replay the local commits with `git rebase origin/<branch>` so the remote commits are kept (merge instead if the conflicts cannot be resolved safely),
   re-run the checks if new commits came in, and retry once with the same rule. If it is refused again, or for another cause
   (branch protection, permissions, authentication), stop, keep the work and report the blocker. Never run an unguarded `git push --force`.
7. Verify the same PR is open and contains the pushed final commit, update its description and check evidence, then mark it ready. If any check, feedback retrieval, push or readiness verification fails, preserve work and report the blocker. If the repository has no remote, stop.

- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not create a PR. Do not mark a PR ready on a red build. Report the failure instead.
- Do not complete adjustment on a branch known to be behind the remote default branch.

Report and persist before continuing:
- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.


## Do not
- Do not create separate branches or PRs per ticket.
- Do not merge into default branch (merging is reserved for human user).

## Report
- The created Pull Request URL.
- Summary of processed tickets and test results.

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. A batch tracks each task separately. Never start a run merely to read a task.
- **Waiting for the user (standalone only)**: Right before asking the user a question you cannot continue without, call report_waiting with taskKey, runId and waiting true, so the board and the owner's desktop show the run as waiting. Your next Sectile call ends the wait; call report_waiting with waiting false if you resume without one. A headless run is left unmarked, which the result says.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After verifying each completed step, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Record clarified, specified and implemented after each corresponding step. After PR verification, record reviewed with the PR URL. For a batch, use the same actual branch and combined PR URL for every completed ticket; never mark unfinished work reviewed.
A task holds an ordered set of pull requests, `prUrl` being its current one. A pull request on a branch the task already used is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Ticket
$ARGUMENTS
