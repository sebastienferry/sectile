---
description: "Review the project's whole backlog and launch the autonomous chains of the tickets to work on now, within the free orchestrator slots, explaining every launch and deferral."
argument-hint: <PROJECT-ID> [contexte]
---
# Orchestrate Backlog



## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the project is identified, and before doing the work, rename the current session to `<project ID> - <project title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
- The status emoji mirrors the run state the skill reports to Sectile, at the same moment and at no other. While the skill works the title carries none: the host already shows whether the session is running.
  - `❓` with report_waiting true, right before a question the skill cannot continue without, and when the skill stops with open questions for the owner. Remove it with report_waiting false, when the work resumes.
  - `✅` with finish_run completed, once the skill reached its goal.
  - `❌` with finish_run failed, when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill. A host that cannot rename, such as a Sectile Desktop console, shows the same state from the Sectile calls themselves.

## Launched by Sectile
A run is launched by Sectile when the invocation supplies a launch runId or a result-file contract, or when the environment carries `SECTILE_RUN_ID` (read it with a plain shell command such as `printenv SECTILE_RUN_ID`). Sectile Desktop then already shows the ticket, the pull request, the next step and its own notifications, so the items marked "not when launched by Sectile" below are skipped: they would only repeat what Desktop shows.

## Session links
Not when launched by Sectile.
- Right after renaming the session, write one line in the conversation that links the project: `Project: [<project ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the project has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the project ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
- **Next step** (not when launched by Sectile). When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): follow the launched chains in Sectile and review the pull requests they open. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
- **Notification** (not when launched by Sectile). When the run reaches `❓`, `✅` or `❌`, send one desktop notification, under 200 characters, leading with what the owner has to do (for example `#47 waits for your answer: 2 product questions`). Send none for routine progress; the host drops it anyway when the owner is watching.
- When the skill runs nested in pickup-issue or pickup-issues, do none of the above: the outer skill owns the group, the chapters, the pane, the next step and the notifications.

## Status update
Wherever the skill runs, end every reply addressed to a person with this block, including the final report and a reply that stops on a question. Keep the three labels as written; write the items in the language of the conversation, and write "None" for an empty line. A headless run, with nobody to read it, writes none.

```markdown
### 📋 Status Update

- **Done**:
  - <what was done in this reply>
- **Remaining (Agent)**:
  - <what is left for the agent, or None>
- **Pending (User)**:
  - <what the user has to do or decide, or None>
```

When the skill runs nested in pickup-issue or pickup-issues, do not write the block: the outer skill writes one for the whole run.

## Goal
Review the project's backlog as a whole and launch the autonomous workflow chain of the tickets that should be worked on now, within the free orchestrator slots, following the orchestration policy given in the prompt. Explain every launch and every deferral, so a person reading the backlog understands why each ticket was or was not taken.


## Read first
- The orchestration policy in the prompt: the profile's Markdown file, or the project's shared policy. It is mandatory and outranks the defaults below.
- `get_orchestration_state`: free slots, the blocker labels the server refuses, the tickets already being worked on, and the latest decision on each ticket with whether the ticket changed since.
- `list_tasks` for the project, then `get_task` on the candidates you need to read in full.
- `get_project_context` for the project's workflow, specification framework and conventions.


## Steps
1. Read the orchestration state. With no free slot, finish the run as completed with a note saying so: there is nothing to launch.
2. List the candidates: tickets before `implemented`, not already being worked on, without a blocker label. Skip a ticket deferred earlier and unchanged since, unless the policy says otherwise; it keeps its earlier reason.
3. Judge each remaining candidate against the policy and against each other, with the whole list in view:
   - **Ready**: a clear, actionable goal, enough technical context to start, a bounded scope, and no unresolved product or architecture decision.
   - **Dependencies**: a ticket that needs another one first waits for it; prefer the one that unblocks others.
   - **Order**: the policy's priorities first, then the board's priority and order. Say why when you depart from the board order.
4. For as many tickets as there are free slots, best first, call `launch_task_chain` with a rationale of one or two sentences that every step of the chain will read. A refusal is final for this run: record it, then try the next candidate.
5. Call `defer_task` on every other candidate you judged, with a reason written for the people who read the backlog: what is missing, or what it waits for.
6. Finish the run with `finish_run`, its note listing the launched tickets and the deferred ones, each with its reason.


## Do not
- Do not edit, comment on, create, move or close any ticket, and do not run any other skill: launching and deferring are this skill's only writes.
- Do not exceed the free slots, and do not try to get around a refusal of `launch_task_chain`.
- Do not launch a ticket the policy excludes, or one whose readiness you doubt: defer it with the reason instead.
- Do not ask questions: this session runs headless and nobody answers. A doubt becomes a deferral.


## Report
- The tickets launched, each with its rationale.
- The tickets deferred, each with its reason.
- The free slots left, and anything the server refused with its reason.


## Orchestrator session
This skill works on a whole project, not on a ticket or a macro: it declares no stage and moves none. `SECTILE_PROJECT_ID` names the project when Sectile launched the session; the project given as argument wins over it.
- **Remote execution indicator**: Sectile launched this run and reports its end itself. Do not call `start_run`. When the whole invocation ends, call `finish_run` with `projectId`, the runId (`SECTILE_RUN_ID`), a status (completed, failed or canceled) and a note summarizing what was launched and what was deferred; never pass a `taskKey` or a `macroKey`.
- **Acting on tickets**: `launch_task_chain` and `defer_task` are the only writes this skill makes. They answer only to the project's running orchestrator session, which Sectile identifies from this console. Never edit, comment on, create, move or close a ticket, never start a skill on a ticket by any other means, and never merge.
- Report faithfully: a refusal from `launch_task_chain` is a decision the server made, stated with its reason, never retried under another name.

## Ticket
$ARGUMENTS
