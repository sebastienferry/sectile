---
description: "Pick a ticket and autonomously execute all development steps up to Pull Request creation."
argument-hint: <TICKET-KEY> [contexte]
---
# Pickup Issue (Auto-Pilot to PR)

Stage: new -> reviewed.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Sectile reporting
- Task access, the run indicator, stage transitions, pull request links and ticket comments follow `/report-stage`: read `report-stage/SKILL.md` in the same skills directory as this skill, for example `~/.claude/skills/report-stage/SKILL.md`. Load it before the first ticket read.
- If it cannot be loaded, these rules still hold. Standalone: call start_run before work, reusing a supplied SECTILE_RUN_ID or launch runId, and call finish_run when the entire invocation ends, including errors or stopping for user input; a nested skill never finishes the outer run. Managed run: submit only through the supplied result contract, and call no transition or comment tool.
- Gate: record clarified, specified and implemented after each corresponding step. After PR verification, record reviewed with the PR URL. For a batch, use the same actual branch and combined PR URL for every completed ticket; never mark unfinished work reviewed.

## Goal
Autonomously take a ticket from its current stage through clarification, specification,
implementation, and testing, all the way to opening a clean Pull Request, updating each stage via Sectile.

## Read first
- The ticket: key, title, description, parent macro, and tracker comments.
- The project's code and existing patterns.
- The project SDD framework (OpenSpec or Spec Kit).

## Steps
1. Inspect the current ticket state AND existing artifacts. Reuse assigned branches, specifications, checklist progress and PRs. Verify completed work before skipping it.
2. Reuse or create a dedicated worktree and work branch. Continue through the stages below from the first incomplete stage to a verified PR.
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

### Adjust Existing Pull Request
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

1. Verify a matching PR exists for the task repository and branch before changing files: open, or already merged by the human. Record its URL. If missing, stop and recover through the configured creation owner (specify or implement). Never create a PR during adjustment, and never push onto a merged PR — review the merged state and report it. Read available PR feedback; retrieval failure is a blocker, not absence of feedback.
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
   If rebasing an already-pushed branch, use `git push --force-with-lease`, never an unguarded force push.
7. Verify the same PR is open and contains the pushed final commit, update its description and check evidence, then mark it ready. If any check, feedback retrieval, push or readiness verification fails, preserve work and report the blocker. If the repository has no remote, stop.

- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not create a PR. Do not mark a PR ready on a red build. Report the failure instead.
- Do not complete adjustment on a branch known to be behind the remote default branch.

Report and persist before continuing:
- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.


## Do not
- Do not merge into the default branch (merging is reserved for the human user).
- Do not push or open a PR if the test suite is failing.
- Follow the managed or standalone transition contract for the invocation.

## Report
- The created Pull Request URL.
- The work branch and files modified.
- The test results demonstrating that build, lint, and tests pass.
- Summary of settled scope and key architectural decisions.

## Ticket
$ARGUMENTS
