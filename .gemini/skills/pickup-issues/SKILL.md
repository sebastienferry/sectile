---
name: pickup-issues
description: "Batch process a list of selected board tickets sequentially in autonomy inside a single dedicated worktree, producing one combined Pull Request covered by tests and lints."
---
# Batch Pickup Issues (Single Worktree & Combined PR)

Stage: new -> reviewed.

## Goal
Autonomously process a batch of tickets selected from the board sequentially in the exact order provided inside a single dedicated batch worktree.

## Read first
- The list of tickets in the batch.
- The project's code and existing patterns.
- The project SDD framework.

## Steps
1. Inspect the current ticket state AND existing artifacts. Reuse assigned branches, specifications, checklist progress and PRs. Verify completed work before skipping it.
2. Use one dedicated worktree and branch for the ordered batch. Run clarification, specification and implementation for each ticket in order. If one blocks, preserve the batch and report completed tickets and the next action; never include unfinished work as completed.
3. Once all tickets are implemented, run review and final checks across the whole batch and create or update ONE combined PR.
Stop before merge. Stage-local boundaries apply while that stage is active; after its requirements are met, continue to the next stage without asking for routine confirmation.

### Clarify Issue
- The ticket: title, description, comments, parent epic if there is one.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

1. Restate the request in two sentences, including what you believe is out of scope.
2. List the ambiguities you found, worst first. An ambiguity is worth listing only
   if two readings lead to different code.
3. Name the critical dependencies: other services, other teams, migrations, data
   you do not have.
4. Resolve reversible choices using existing code and project conventions. Record
   the choice and rationale; do not ask questions merely to fill a quota.
   Ask only when an essential product decision changes acceptance criteria or an
   unavailable dependency prevents progress. In an unattended run, report the
   concrete blocker and the decision needed; do not invent settled requirements.
   A TTY alone does not make a run interactive: follow the invocation's mode.
5. Persist the settled scope and assumptions in the report for specification.

- Do not write production code at this stage, and do not start the specification.
- Do not invent an answer to your own question and move on without stating your assumption.
- Do not pad the list to reach five questions.

Report and persist before continuing:
- Restated request and scope.
- Ambiguities, worst first.
- Critical dependencies.
- Numbered questions with your recommended option.
- Settled scope and assumptions.

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
2. Work through the checklist in small steps, each one leaving the tree buildable.
3. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
4. Run build, static analysis and tests. Fix until green, and quote the real output.
5. Re-read your own diff before finishing, as a reviewer would.

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

### Review and Pull Request
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

1. Fetch the remote (`git fetch origin`) and compare the work branch with the
   remote default branch (normally `origin/main`; use the repository's configured default when different).
   Integrate missing base commits before the final review: prefer rebase when the branch is private, or merge when
   repository policy or shared-branch state requires it. Resolve conflicts and do not continue until the working tree is clean.
2. Review the resulting diff for correctness, side effects, security, and edge cases with no test.
3. Update documentation affected by the change. Fix what the review finds, now. A known defect belongs in the code, not in the
   description of the merge request.
4. Re-run build, static analysis and tests after integrating the default branch and on the final state.
5. Commit with a conventional message: type, scope, and why the change exists.
6. Push the branch and create or update its existing merge request: summary, test plan, and the specific
   places where you want a reviewer's eyes.
   If rebasing an already-pushed branch, use `git push --force-with-lease`, never an unguarded force push.
7. If the repository has no remote, say so and stop rather than merging locally.

- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not open a merge request on a red build. Report the failure instead.
- Do not open a merge request from a branch known to be behind the remote default branch.

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
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Standalone invocation**: After verifying each completed step, use the local handler below. Check its exit status and response. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
For each ticket key in the batch, record clarified, specified and implemented after that ticket's corresponding step. Use the SAME actual batch branch for every ticket. After the combined PR is verified, record reviewed and the SAME PR URL for every implemented ticket. Never mark an unfinished ticket reviewed.
```bash
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"clarified","note":"<settled scope and assumptions>"}'
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"specified","note":"<spec paths>","branch":"<ACTUAL_BRANCH>"}'
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"implemented","note":"<check results>","branch":"<ACTUAL_BRANCH>"}'
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"reviewed","note":"<review summary>","prUrl":"<PR_URL>"}'
```
This POST calls the local TaskFlow handler directly. Confirm HTTP success before continuing. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
