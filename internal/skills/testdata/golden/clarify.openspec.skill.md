---
name: clarify-issue
description: "Analyse a ticket against the code, iterate in clarification rounds, and resolve open questions until the owner confirms satisfaction."
---
# Clarify Issue

Stage: new -> clarified.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

## Sectile reporting
- Task access, the run indicator, stage transitions, pull request links and ticket comments follow `/report-stage`: read `report-stage/SKILL.md` in the same skills directory as this skill (`.claude/commands/report-stage.md` for Claude Code). Load it before the first ticket read.
- If it cannot be loaded, these rules still hold. Standalone: call start_run before work, reusing a supplied SECTILE_RUN_ID or launch runId, and call finish_run when the entire invocation ends, including errors or stopping for user input; a nested skill never finishes the outer run. Managed run: submit only through the supplied result contract, and call no transition or comment tool.
- Gate: transition new → clarified only when the exit condition is met: the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never transition new → clarified while any product question or decision remains open.

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

## Do not
- Do not transition new → clarified while any product question or decision remains open.
- Do not invent answers to essential product questions in unattended runs; record them and ask.
- Do not write production code or start the technical specification at this stage.
- Do not discard previous round sections when writing Round N; append each round chronologically.
- Do not switch branches or create a new branch: reuse the assigned feat/<n> branch.

## Report
- The report path: docs/clarifications/<n>.md.
- Current round number and whether the exit condition was met.
- Settled decisions and reversed assumptions.
- Numbered open questions (if any) and who is expected to answer them.
- Stage transition status (applied or blocked awaiting answers).
