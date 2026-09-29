---
description: "Analyse a ticket against the code, iterate in clarification rounds, and resolve open questions until the owner confirms satisfaction."
argument-hint: <TICKET-KEY> [contexte]
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
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
- While the skill works, the title carries no status emoji: the host already shows whether the session is running. Add one at these moments, and at no others:
  - `❓` waiting for the user: right before asking a question the skill cannot continue without, or when the skill stops with open questions for the owner. Remove it when the work resumes.
  - `✅` done: as the very last action, once the skill reached its goal.
  - `❌` blocked: when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill.

## Session links
- Right after renaming the session, write one line in the conversation that links the ticket: `Ticket: [<ticket ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the ticket has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the ticket ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
- **Next step.** When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): `/specify-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
- **Notification.** When the title gets `❓`, `✅` or `❌`, send one desktop notification, under 200 characters, leading with what the owner has to do (for example `#47 waits for your answer: 2 product questions`). Send none for routine progress; the host drops it anyway when the owner is watching.
- When the skill runs nested in pickup-issue or pickup-issues, do none of the above: the outer skill owns the group, the chapters, the pane, the next step and the notifications.

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
   g. Publish the round as described below, then ask any open questions interactively
      when the owner is present or in the ticket discussion when unattended.
3. In Round N (follow-up after owner answers):
   a. Read the owner's answers from the interactive prompt or ticket comments via get_task.
   b. Append a dated section: "## Round N - answers from the owner (<date>)" to docs/clarifications/<n>.md.
   c. Explicitly record settled choices and any reversed prior assumptions.
   d. Address newly surfaced ambiguities or dependencies.
   e. Commit updates with docs(spec): clarify #<n> (round N), unless the file is ignored by Git (step 6).
   f. Publish the round as described below. If follow-up product questions remain,
      ask them and stop without transitioning.
4. Exit condition:
   Rounds continue until the owner confirms that the clarification is satisfactory (or zero open
   product questions remain in unattended pickup). Never transition new → clarified while product
   questions remain open.
5. Persist the settled scope, decisions, and assumptions in the report before concluding.
6. Dropped artefacts: `<n>` is the task key without its leading `#` (`487` for `#487`). Before
   committing, run `git check-ignore -q docs/clarifications/<n>.md`. When it succeeds, the project
   drops its specification artefacts on this workstation: write and update the file in the worktree,
   never commit it, never force it with `git add -f`, and include the settled decisions
   in the round section used as the transition note, saying that the report file stays local to the worktree.

7. Optional stage publication: read `pushStageCommits` from `get_project_context`
   (or the supplied project configuration); missing or false means off. When true,
   after each stage commit push the actual assigned work branch with a plain push.
   Use `git push -u origin <branch>` on first publication and `git push origin <branch>`
   afterwards. Never force. Report a refused push and continue the stage; retain the
   commit for retry. Ignored artifacts are never committed or force-added, so writing
   them alone triggers no push. This setting does not replace required PR publication.

### Publish every clarification round

- Every round, interactive or unattended, publishes its section in full: Round 1
  uses the whole initial report; Round N uses only its newly appended section.
  Retain the Markdown file as the chronological history, even when it stays local.
  Include the report path, commit/local status, settled decisions, open questions
  and publication failures in that section; a path or summary alone is insufficient.
- Standalone intermediate rounds with open questions use `add_comment`. The final
  round uses its full section as the `transition_stage` note, with no separate
  `add_comment` for the same content. Verify each response before claiming publication.
- Managed runs call no comment or stage tool: put the full round section in the
  supplied result note and let Sectile publish it through its completion contract.
- If a section exceeds the tracker comment limit (GitHub 65,536 characters; Jira
  about 32,767), split at Markdown paragraph boundaries into numbered parts, reserving
  space for the server header and part numbering. Use at most 30,000 characters per
  part for either tracker, and split an oversized paragraph without dropping text.
  Standalone intermediate parts use `add_comment` in order. For a final round, post
  all preceding parts with `add_comment` and use only the last numbered part as the
  transition note, so each part appears once. Managed runs keep the complete section
  in the result note and report any completion-contract size limitation rather than
  bypassing the contract.


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
- Full content of this round (initial report or new section), publication result and any numbered parts.
- Optional stage push result, including refusals that did not block the stage.


## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. Never start a run merely to read a task.
- **Waiting for the user (standalone only)**: Right before asking the user a question you cannot continue without, call report_waiting with taskKey, runId and waiting true, so the board and the owner's desktop show the run as waiting. Your next Sectile call ends the wait; call report_waiting with waiting false if you resume without one. A headless run is left unmarked, which the result says.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After verifying each completed step, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition new → clarified only when the exit condition is met: the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never transition new → clarified while any product question or decision remains open.
A task holds an ordered set of pull requests, `prUrl` being its current one. A pull request on a branch the task already used is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Ticket
$ARGUMENTS
