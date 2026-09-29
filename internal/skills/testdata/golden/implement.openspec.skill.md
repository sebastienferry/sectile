---
name: code-issue
description: "Implement the ticket from its specification and prove it works with the project's own build, linters and tests."
---
# Implement Code

Stage: specified -> implemented.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
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
- Right after renaming the session, write one line in the conversation that links the ticket: `Ticket: [<ticket ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the ticket has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the ticket ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
- **Changes.** When the skill ends after changing code, show the session's diff pane, provided it covers the worktree the skill worked in; otherwise name the worktree path in the reply instead.
- **Next step** (not when launched by Sectile). When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): `/adjust-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
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
Ship the change described by the specification, in code that reads like the code
already there, with the project's checks green.

## Read first
- The specification and its task checklist. It is the contract, follow its order.
  Its files may be ignored by Git (`git check-ignore -q` succeeds on them): the project drops its
  specification artefacts on this workstation. Read them from the worktree, never commit them and
  never force them with `git add -f`. When the project drops its artefacts (the launch prompt says so,
  or the paths are ignored) and the specification is missing from the worktree, stop and report that
  it is not available on this workstation: never rewrite it.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.

## Steps
1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. On a multi-repo project, `$SECTILE_REPOSITORIES` lists the task's folders. Work in the
   primary worktree; the other repositories are read-only context. To change one, call
   `prepare_repository_worktree` for it first and work in the worktree it returns: each
   changed repository then needs its own pull request, given to `transition_stage` in `prUrls`.
3. Work through the checklist in small steps, each one leaving the tree buildable. When the
   specification artefacts are ignored by Git, commit the code only and never force-add them.
4. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
5. Run build, static analysis and tests. Fix until green, and quote the real output.
6. Re-read your own diff before finishing, as a reviewer would.

## Recovery and blockers
- Repair routine technical issues and update design/tasks when the implementation
  needs to change while preserving acceptance criteria. Continue after documenting why.
- Establish whether a failing test predates the change. Fix failures in scope; report
  unrelated failures with baseline evidence. Never hide them or mark checks green.
- Stop only for an essential product decision, an unavailable dependency after
  bounded recovery attempts, or work that materially expands the requested scope.
- Preserve the work branch, completed checklist items and remaining next action so
  a retry can resume instead of starting over.

## Report
- What changed, file by file, and why.
- The real output of build, linters and tests, remaining failures included.
- What you deliberately left out, and what it would take to finish it.

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. Never start a run merely to read a task.
- **Waiting for the user (standalone only)**: Right before asking the user a question you cannot continue without, call report_waiting with taskKey, runId and waiting true, so the board and the owner's desktop show the run as waiting. Your next Sectile call ends the wait; call report_waiting with waiting false if you resume without one. A headless run is left unmarked, which the result says.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After verifying each completed step, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition specified → implemented only when this step is complete.
A task holds an ordered set of pull requests, `prUrl` being its current one. A pull request on a branch the task already used is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
