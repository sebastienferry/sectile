---
name: clarify-issue
description: "Analyse a ticket against the code, iterate in clarification rounds, and resolve open questions until the owner confirms satisfaction."
---
# Clarify Issue

Stage: new -> clarified.

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
Turn an ambiguous ticket into a settled specification baseline. Clarification is an
iterative dialogue with the work item's owner: execute in rounds until the owner
explicitly confirms that the clarification is satisfactory.

## Read first
- The ticket: title, description, comments via get_task, parent epic if present.
- The existing clarification report if one exists: docs/clarifications/<n>.md on the assigned work branch.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

## Steps
1. Re-read the assigned branch and worktree. If docs/clarifications/<n>.md already exists,
   this run continues an existing clarification into Round N. If not, this is Round 1.
2. In Round 1:
   a. Restate the request in two sentences, including what is out of scope.
   b. List ambiguities, worst first. Only list an ambiguity if two readings lead to different code.
   c. Name critical dependencies: other services, migrations, missing data, third-party limits.
   d. Resolve reversible technical choices using existing code and project conventions.
   e. Formulate essential product questions that alter acceptance criteria, with your recommended option.
   f. Write docs/clarifications/<n>.md, commit with docs(spec): clarify #<n> (round 1), unless the file
      is ignored by Git (step 6).
   g. Ask the questions (interactively in-session if the owner is present; as a ticket discussion
      comment via add_comment when unattended).
3. In Round N (follow-up after owner answers):
   a. Read the owner's answers from the interactive prompt or ticket comments via get_task.
   b. Append a dated section: "## Round N - answers from the owner (<date>)" to docs/clarifications/<n>.md.
   c. Explicitly record settled choices and any reversed prior assumptions.
   d. Address newly surfaced ambiguities or dependencies.
   e. Commit updates with docs(spec): clarify #<n> (round N), unless the file is ignored by Git (step 6).
   f. If follow-up product questions remain, ask them and stop without transitioning.
4. Exit condition:
   Rounds continue until the owner confirms that the clarification is satisfactory (or zero open
   product questions remain in unattended pickup). Never transition new → clarified while product
   questions remain open.
5. Persist the settled scope, decisions, and assumptions in the report before concluding.
6. Dropped artefacts: `<n>` is the task key without its leading `#` (`487` for `#487`). Before
   committing, run `git check-ignore -q docs/clarifications/<n>.md`. When it succeeds, the project
   drops its specification artefacts on this workstation: write and update the file in the worktree,
   never commit it, never force it with `git add -f`, and put the settled decisions in full in the
   transition note, saying that the report file stays local to the worktree.

## Do not
- Do not transition new → clarified while any product question or decision remains open.
- Do not invent answers to essential product questions in unattended runs; record them and ask.
- Do not write production code or start the technical specification at this stage.
- Do not discard previous round sections when writing Round N; append each round chronologically.
- Do not switch branches or create a new branch: reuse the assigned feat/<n> branch.

## Report
- The report path: docs/clarifications/<n>.md, and whether it is committed or local to the worktree (ignored by Git).
- Current round number and whether the exit condition was met.
- Settled decisions and reversed assumptions.
- Numbered open questions (if any) and who is expected to answer them.
- Stage transition status (applied or blocked awaiting answers).

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. Never start a run merely to read a task.
- **Waiting for the user (standalone only)**: Right before asking the user a question you cannot continue without, call report_waiting with taskKey, runId and waiting true, so the board and the owner's desktop show the run as waiting. Your next Sectile call ends the wait; call report_waiting with waiting false if you resume without one. A headless run is left unmarked, which the result says.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After verifying each completed step, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition new → clarified only when the exit condition is met: the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never transition new → clarified while any product question or decision remains open.
A task holds an ordered set of pull requests, `prUrl` being its current one. A pull request on a branch the task already used is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
