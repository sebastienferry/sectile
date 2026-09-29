---
name: refine-macro
description: "Interactively clarify macro framing text with the user and break it down into structured todos and Sectile tickets."
---
# Refine Macro

Interactive: the user answers in the terminal.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use the Sectile server URL the plugin was installed with (its server_url setting) as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the macro is identified, and before doing the work, rename the current session to `<macro ID> - <macro title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
- While the skill works, the title carries no status emoji: the host already shows whether the session is running. Add one at these moments, and at no others:
  - `❓` waiting for the user: right before asking a question the skill cannot continue without, or when the skill stops with open questions for the owner. Remove it when the work resumes.
  - `✅` done: as the very last action, once the skill reached its goal.
  - `❌` blocked: when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill.

## Session links
- Right after renaming the session, write one line in the conversation that links the macro: `Macro: [<macro ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the macro has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the macro ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
- **Next step.** When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): `/pickup-issue <task ID>` on the first card of the breakdown. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
- **Notification.** When the title gets `❓`, `✅` or `❌`, send one desktop notification, under 200 characters, leading with what the owner has to do (for example `#47 waits for your answer: 2 product questions`). Send none for routine progress; the host drops it anyway when the owner is watching.
- When the skill runs nested in pickup-issue or pickup-issues, do none of the above: the outer skill owns the group, the chapters, the pane, the next step and the notifications.

## Goal
Transform high-level macro framing text into an actionable, structured todo list and concrete Sectile tickets, interactively clarifying ambiguities with the user when framing text is vague.

## Read first
- The macro title and framing description.
- The active project SDD framework (SpecKit or OpenSpec).
- Existing macro todos and child tasks to avoid duplicating completed work.

## Steps
1. Inspect the macro title and high-level framing description.
2. **Evaluate framing completeness**:
   - If the framing description is empty, under 2 sentences, or lacks clear technical boundaries/acceptance criteria, formulate 3 to 5 numbered clarification questions and ask the user directly in this interactive terminal session before generating tasks.
3. **Decompose & Break Down**:
   - Once answered or if framing text is detailed, group action items according to the selected SDD framework:
     - **SpecKit SDD**: Group into User Stories ([US-x]) and Feature Modules ([FEAT-x]).
     - **OpenSpec SDD**: Group into Capabilities ([CAP-x]) and Change Proposals ([CHANGE-x]).
4. Output the generated checklist of actionable todos AND proposed Sectile tickets (Title, IssueType: Story/Task/Bug, Description) for bulk ticket creation.

## Do not
- Do not generate tasks blindly when framing text is vague without asking clarification questions.
- Do not overwrite existing todos or tasks without user confirmation in the UI.
- Do not mutate external tracker issues directly without user trigger.

## Report
- Clarification Q&A summary (if framing was vague).
- Structured list of proposed MacroTodo items.
- Proposed Sectile tickets breakdown (Title, IssueType, Description).
- Rationale behind the task breakdown.

## Macro run
A macro has no stage: this skill declares none, and moves nothing. `SECTILE_MACRO_KEY` and `SECTILE_MACRO_PROJECT_ID` name the macro when Sectile launched the session; the key given as argument wins over them. Invoked by hand, find the project ID with `list_projects`. The ticket variables (`SECTILE_TASK_*`) are not set here.
- **Remote execution indicator**: before doing work, call `start_run` with `projectId`, `macroKey` and the skill name, never a `taskKey`. If `SECTILE_RUN_ID` or a launch runId is supplied, pass it as `runId` to reuse that run. Keep the returned activity ID as runId, and call `finish_run` with the same `projectId` and `macroKey`, the runId, a status (completed, failed or canceled) and a note when the whole invocation ends, including errors or stopping for user input.
- Never touch the macro's title, labels, horizon or priority, and never move its tickets. Never delete anything remote, and never merge. Report faithfully: a file you could not write with the error it gave, a skipped step stated as skipped.
