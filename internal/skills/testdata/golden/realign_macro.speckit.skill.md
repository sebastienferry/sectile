---
name: realign-macro
description: "Bring the macro's specification back in line with its slicing: add what was added, rename what was renamed, mark what disappeared. Never rewrite the body of an entry that already exists."
---
# Realign Macro (Spec Kit SDD)

Interactive: the user answers in the terminal.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the macro is identified, and before doing the work, rename the current session to `<macro ID> - <macro title>`, for example `#47 - Remove the parallelism setting`. Keep that title for the whole run.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, skip the rename silently and continue. It never blocks, delays or replaces the work of the skill.

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
- The specification folder of this macro, `specs/<MACRO-KEY>-<slug>/` in `path`: `spec.md` carries the prioritised user stories, `tasks.md` the groups.
- If no such folder exists in `path` (or on the macro branch), say so and stop: there is nothing to realign, and writing one from the slicing alone would be a specification done badly.

## Steps
1. Start the run (see "Macro run" below), then call `prepare_macro_worktree` for `path`, `branch` and `todos`. Never write on the default branch; an empty `branch` means a plain folder, written in place.
2. Read the slicing and the macro's folder. An entry is a numbered user story under the user stories heading of `spec.md`, or a level-two group heading of `tasks.md`.
3. Compare, line by line, within the file each line came from, and classify. Each case has one answer:
   - A line whose `sourceEntry` matches an entry, with the same text: nothing to do.
   - A line whose `sourceEntry` matches an entry, with a different text: rename the entry's title, and leave its body untouched.
   - A line whose `sourceEntry` matches no entry any more: leave it, and report it as unmatched. Do not guess which entry it was.
   - A line with no `sourceKind`: add an entry for it, with a title and the minimum body Spec Kit requires. Say in the report that its body is a stub.
   - An entry of that file that no line points at any more: mark it, do not delete it. An entry of the file the slicing was not imported from is never an orphan.
4. Mark an orphan entry by appending ` (to be removed: no longer in the slicing)` to its title. Do not remove its acceptance criteria, do not empty its group.
5. Keep the numbering and the priorities consistent: an added story takes the next free number, and a marked one keeps the one it had. Never renumber the file: that would break every reference to it.
6. When you wrote something, commit only the macro's folder in `path` and push the macro branch:

       git -C "<path>" add -- specs/<MACRO-KEY>-<slug>
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
- Never touch the macro's title, labels, horizon or priority, and never move its tickets. Never delete anything remote, and never merge. Report faithfully: a file you could not write with the error it gave, a skipped step stated as skipped.
