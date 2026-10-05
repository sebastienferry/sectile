---
name: specify-issue
description: "Write the executable specification of a ticket in the project's Spec-Driven Design framework, before any code."
---
# Specify Issue (Spec Kit SDD)

Stage: clarified -> specified.

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
- **Next step** (not when launched by Sectile). When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): `/implement-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
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

## Specifications workspace
The task's clarification report (`docs/clarifications/<n>.md`) and specification (`specs/<KEY>-<slug>/` or `openspec/changes/<KEY>-<slug>/`) are its issue artefacts. They live in the specifications workspace, which may be another repository than the code: the project's Issue specifications folder on this workstation.
- **Where**: `SECTILE_SPEC_REPO` is its path, `SECTILE_SPEC_BRANCH` its branch and `SECTILE_SPEC_WORKTREE` whether it is a dedicated worktree, when Sectile launched the session. Otherwise call the `prepare_task_spec_worktree` MCP tool with the task key: it answers `path`, `branch`, `worktree`, `distinct` and `warning`, and reuses what a launch prepared. When neither is available, the task worktree is the specifications workspace.
- Read and write the issue artefacts under that path, never relative to the session directory, and repeat any warning in your report. Do not create, switch or check out a branch there: when it is not a dedicated worktree, check that the path is on its branch, and stop and say so if it is not.
- **Distinct workspace** (`distinct` true, or `SECTILE_SPEC_REPO` is not the task worktree): the issue artefacts are committed there, never in the code branch. Stage only the task's files (`git -C "<path>" add <files>`, never `git add -A`), commit with the stage's message, and push with `git -C "<path>" push -u origin <branch>` the first time and a plain push afterwards. Never force, never push the default branch; a refused push is reported and does not block the stage. Opening a pull request in that repository is the human's gesture: say the branch is pushed. The dropped artefacts check (`git check-ignore -q`) runs in that path.
- With a distinct workspace the code branch carries no artefact: open no pull request at the clarified or specified stage, even when the project creates it there, say so in the report, and create the draft after implementation. The code pull request description names the specifications repository and branch.
- **Plain folder** (empty branch): write in place and run no `git` command there; nothing is committed or pushed. Say so in the report.

## Goal
Produce a specification another engineer could implement without asking you
anything. Behaviour and acceptance criteria first, implementation choices second,
and the two kept in separate files.

## Read first
- Project-configured SDD framework: speckit. Use it unless the invocation explicitly overrides it.
- The clarification outcome on the ticket: the decisions are already made, apply them. When the
  clarification report is ignored by Git, docs/clarifications/<n>.md may be missing from this
  worktree; the rounds published on the ticket carry the same content.
- Select the SDD framework in order: explicit {sdd_framework} or --framework=<name>,
  then the project-configured framework, then repository detection:
  - If `openspec/` exists -> use OpenSpec SDD.
  - If `.specify/` or `specs/` exists -> use Spec Kit SDD.
- Ensure the project SDD directory is initialized before writing specifications.

## Steps
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
3. Dropped artefacts: before committing the specification, run `git check-ignore -q` on one of its
   files (`specs/<KEY>-<title-slug>/spec.md` or `openspec/changes/<KEY>-<title-slug>/proposal.md`).
   When it succeeds, the project drops its specification artefacts on this workstation: write the
   files in the worktree, never commit them, never force them with `git add -f`, and put the
   requirements and the open points in the transition note, saying that the files stay local to the
   worktree. Open no pull request at this stage then, even when the project creates it after
   specification: say in the report that it is deferred to the implemented stage.

4. Optional stage publication: read `pushStageCommits` from `get_project_context`
   (or the supplied project configuration); missing or false means off. When true,
   after each stage commit push the actual assigned work branch with a plain push.
   Use `git push -u origin <branch>` on first publication and `git push origin <branch>`
   afterwards. Never force. Report a refused push and continue the stage; retain the
   commit for retry. Ignored artifacts are never committed or force-added, so writing
   them alone triggers no push. This setting does not replace required PR publication.


## Do not
- Do not decide what the clarification left open. Mark it as open and say so.
- Do not describe implementation inside the behaviour file.
- Do not start implementing, even the easy part.

## Report
- The files written, with their paths, and whether they are committed or local to the worktree (ignored by Git).
- The work branch.
- Requirements that are still open, and what they block.

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. Never start a run merely to read a task.
- **Waiting for the user (standalone only)**: Right before asking the user a question you cannot continue without, call report_waiting with taskKey, runId and waiting true, so the board and the owner's desktop show the run as waiting. Your next Sectile call ends the wait; call report_waiting with waiting false if you resume without one. A headless run is left unmarked, which the result says.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After verifying each completed step, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Transition clarified → specified only when this step is complete.
A task holds an ordered set of pull requests, one or more per repository it changed, `prUrl` being its primary repository's current one. A pull request on a branch the task already used in that repository is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence. A pull request opened outside a transition is recorded with `record_pull_request`, once per repository.
A task whose work changed no repository (a configuration made through an API, a review, a follow-up) has no pull request to give: pass `noRepositoryChange: true` to `transition_stage` instead of `prUrl`, and say in the note what was done instead. The server refuses the statement when the task records a pull request on its branch or a repository prepared with `prepare_repository_worktree`; then give those pull requests.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
