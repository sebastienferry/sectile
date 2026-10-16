---
name: realign-macro
description: "Bring the macro's specification back in line with its slicing: add what was added, rename what was renamed, mark what disappeared. Never rewrite the body of an entry that already exists."
---
# Realign Macro (OpenSpec SDD)

Interactive: the user answers in the terminal.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- A run Sectile launched reports through `start_run`/`finish_run`, and records its stage when its skill moves one, like any other run; never forge launch or completion status or use another endpoint to evade validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the macro is identified, and before doing the work, rename the current session to `<macro ID> - <macro title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
- The status emoji mirrors the run state the skill reports to Sectile, at the same moment and at no other. While the skill works the title carries none: the host already shows whether the session is running.
  - `❓` with report_waiting true, right before a question the skill cannot continue without, and when the skill stops with open questions for the owner. Remove it with report_waiting false, when the work resumes.
  - `✅` with finish_run completed, once the skill reached its goal.
  - `❌` with finish_run failed, when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill. A host that cannot rename, such as a Sectile Desktop console, shows the same state from the Sectile calls themselves.

## Launched by Sectile
A run is launched by Sectile when the invocation supplies a launch runId, or when the environment carries `SECTILE_RUN_ID` (read it with a plain shell command such as `printenv SECTILE_RUN_ID`). Sectile Desktop then already shows the ticket, the pull request, the next step and its own notifications, so the items marked "not when launched by Sectile" below are skipped: they would only repeat what Desktop shows.

## Session links
Not when launched by Sectile.
- Right after renaming the session, write one line in the conversation that links the macro: `Macro: [<macro ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the macro has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the macro ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
- **Next step** (not when launched by Sectile). When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): `/pickup-issue <task ID>` on the next card to deliver. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
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
Make the macro's specification say what the team now believes, and nothing more.

The slicing travelled one way only: the specification proposed lines, they were imported into the macro, and then the real decisions happened on them. A line was renamed, split in two, dropped, or typed by hand while looking at the code. Nothing brought that back, so the specification kept saying something nobody defends any more. You are closing that loop, surgically: what you must not do matters more here than what you do.

## Read first
- Where to write, on which branch, and what to align on: call the `prepare_macro_worktree` MCP tool with the project ID (`SECTILE_MACRO_PROJECT_ID`, else find it with `list_projects`) and the macro key (the argument, else `SECTILE_MACRO_KEY`). It prepares, or reuses as it is, the macro's own checkout, and answers with:
  - `path`, the checkout to write the specification in, and `branch`, the branch already checked out there. When `worktree` is `true`, it is this macro's own worktree, so another macro specified at the same time cannot touch your files. Read `warning` and repeat it in your report.
  - `todos`, the macro's slicing lines. That list is the truth to align on.
- Do NOT create the branch, do NOT switch branch, and do NOT run `git checkout`: switching would carry your untracked files onto another branch. When `worktree` is `false`, check that the checkout at `path` is on `branch`; if it is not, stop and say so.
- When `branch` is empty, `path` is a plain folder, not a Git repository: write in `path` directly, skip the branch check, and run no `git` command at all. Nothing is committed or pushed.
- The session's own directory is the code repository, which may not hold the specifications: work in `path`, never relative to the current directory.
- The origin of each line, which tells you where to look and what to do:
  - `sourceKind` is `tasks` or `spec`: the artefact the line was imported from, and therefore the file to align. Lines from `tasks` align the groups of `tasks.md`, lines from `spec` the entries of `spec.md`; never compare a line with the other file.
  - `sourceEntry` is the entry's title as the file writes it, before Sectile cleaned it. That is how you find the entry: `text` has lost the group prefix, the story key and the trailing reference.
  - No `sourceKind` means the line was typed by hand. It is an addition, not an orphan. Add it to the file the other lines came from (`tasks.md` when the slicing mixes both).
  - `sourceKind: stories` means the line was taken back from an existing story. It points at a ticket, not at an entry: leave it alone and report it.
  - Any other `sourceKind` is an origin this skill does not know: leave the line alone and report it.
- The change folder of this macro, `openspec/changes/<MACRO-KEY>-<slug>/` in `path`: its `tasks.md` carries the groups, and its `specs/<capability>/spec.md` the requirements.
- If no such change folder exists in `path` (or on the macro branch), say so and stop: there is nothing to realign, and writing one from the slicing alone would be a specification done badly.

## Steps
1. Start the run (see "Macro run" below), then call `prepare_macro_worktree` for `path`, `branch` and `todos`. Never write on the default branch; an empty `branch` means a plain folder, written in place.
2. Read the slicing and the macro's folder. An entry is a level-two group heading of `tasks.md`, or a `### Requirement:` of a capability's spec delta.
3. Compare, line by line, within the file each line came from, and classify. Each case has one answer:
   - A line whose `sourceEntry` matches an entry, with the same text: nothing to do.
   - A line whose `sourceEntry` matches an entry, with a different text: rename the entry's title, and leave its body untouched.
   - A line whose `sourceEntry` matches no entry any more: leave it, and report it as unmatched. Do not guess which entry it was.
   - A line with no `sourceKind`: add an entry for it, with a title and the minimum body OpenSpec requires. Say in the report that its body is a stub.
   - An entry of that file that no line points at any more: mark it, do not delete it. An entry of the file the slicing was not imported from is never an orphan.
4. Mark an orphan entry by appending ` (to be removed: no longer in the slicing)` to its title. Do not remove its requirements, do not empty its group.
5. Run `openspec validate <change-id> --strict` and fix only what it reports about what you wrote. A failure that predates your edit is reported, not fixed.
6. When you wrote something, commit only the macro's folder in `path` and push the macro branch:

       git -C "<path>" add -- openspec/changes/<MACRO-KEY>-<slug>
       git -C "<path>" commit -m "<MACRO-KEY>: realign the specification with the slicing"
       git -C "<path>" push -u origin HEAD

   Never `git add -A`: with worktrees off, `path` is a working checkout that may hold other changes. Push nothing when you wrote nothing, and never push the default branch. Opening the pull request is the human's gesture: say the branch is pushed and stop there.

   When `branch` is empty, skip this step entirely: the folder is not a Git repository, so there is no branch to commit on and nothing to push. Say so in the report.

## Do not
- Do not rewrite the body of an entry that exists. Its scenarios, its acceptance criteria and its prose are not in the slicing, and regenerating them from a one-line todo would destroy them. A renamed line changes a title, not a body.
- Do not delete an entry. Mark it, so a human decides: the line may have been dropped from the slicing for this iteration and still be worth specifying.
- Do not regenerate the file, and do not renumber it.
- Do not write the macro's todos. They are your input, and the call that writes them replaces the whole list.
- Do not create stories, and do not open a pull request.
- Do not switch a checkout's branch, and do not write on the default branch.
- On a plain folder (empty `branch`), do not run `git`: no branch check, no commit, no push.

## Report
- What you added, one line each, with the entry you created and a note that its body is a stub.
- What you renamed, old title then new one.
- What you marked as to be removed, and why you did not delete it.
- What you deliberately left alone, and why: an entry whose body you kept, a line whose entry no longer exists (unmatched), a line taken back from a story, an origin you do not know.
- The files touched, with their paths, the branch they are on, and whether it was pushed. On a plain folder (empty `branch`), say it is not a Git repository and nothing was committed.
- The `warning` of `prepare_macro_worktree`, if it gave one.

## Macro run
A macro has no stage: this skill declares none, and moves nothing. `SECTILE_MACRO_KEY` and `SECTILE_MACRO_PROJECT_ID` name the macro when Sectile launched the session; the key given as argument wins over them. Invoked by hand, find the project ID with `list_projects`. The ticket variables (`SECTILE_TASK_*`) are not set here.
- **Remote execution indicator**: before doing work, call `start_run` with `projectId`, `macroKey` and the skill name, never a `taskKey`. If `SECTILE_RUN_ID` or a launch runId is supplied, pass it as `runId` to reuse that run. Keep the returned activity ID as runId, and call `finish_run` with the same `projectId` and `macroKey`, the runId, a status (completed, failed or canceled) and a note when the whole invocation ends, including errors or stopping for user input.
- **Waiting for the user**: right before asking the user a question you cannot continue without, call `report_waiting` with the same `projectId` and `macroKey`, the runId and `waiting: true`, never a `taskKey`. Your next Sectile call ends the wait; call it with `waiting: false` if you resume without one.
- Never touch the macro's title, labels, horizon or priority, and never move its tickets. Never delete anything remote, and never merge. Report faithfully: a file you could not write with the error it gave, a skipped step stated as skipped.
