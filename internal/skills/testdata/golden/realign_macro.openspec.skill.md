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
- Where to write, and on which branch:
  - `SECTILE_SPEC_REPO` is the checkout to write the specification in, and `SECTILE_SPEC_BRANCH` the branch already checked out there. When `SECTILE_SPEC_WORKTREE` is `true`, it is this macro's own worktree, so another macro specified at the same time cannot touch your files.
  - When they are not set (the skill was invoked by hand), call the `prepare_macro_worktree` MCP tool with the project ID and the macro key, and use the `path` and `branch` it returns.
  - Do NOT create the branch, do NOT switch branch, and do NOT run `git checkout`: switching would carry your untracked files onto another branch. When the answer says `worktree: false`, check that the checkout is on that branch; if it is not, stop and say so.
  - The session's own directory is the code repository, which may not hold the specifications: work from `SECTILE_SPEC_REPO`, never relative to the current directory.
- The macro, read by its key, and its `todos`. That list is the truth to align on:

      curl -sS -H "Authorization: Bearer ${SECTILE_AGENT_TOKEN}" "${SECTILE_SERVER_URL:-http://localhost:8090}/api/projects/${SECTILE_MACRO_PROJECT_ID}/macros" | jq --arg key "${SECTILE_MACRO_KEY}" '.[] | select(.key | ascii_upcase == ($key | ascii_upcase))'

- The origin of each line, which tells you where to look and what to do:
  - `sourceKind` is `tasks` or `spec`: the artefact the line was imported from, and therefore the file to align.
  - `sourceEntry` is the entry's title as the file writes it, before Sectile cleaned it. That is how you find the entry: `text` has lost the group prefix, the story key and the trailing reference.
  - No `sourceKind` means the line was typed by hand. It is an addition, not an orphan.
  - `sourceKind: stories` means the line was taken back from an existing story. It points at a ticket, not at an entry: leave it alone and report it.
  - Any other `sourceKind` is an origin this skill does not know: leave the line alone and report it.
- The change folder of this macro, `openspec/changes/<MACRO-KEY>-<slug>/` in `SECTILE_SPEC_REPO`: its `tasks.md` carries the groups, and its `specs/<capability>/spec.md` the requirements.
- If no such change folder exists on the macro branch, say so and stop: there is nothing to realign, and writing one from the slicing alone would be a specification done badly.

## Steps
1. Start the run (see "Macro run" below), then resolve where to write: `SECTILE_SPEC_REPO` and `SECTILE_SPEC_BRANCH`, or the answer of `prepare_macro_worktree`. Never write on the default branch.
2. Read the slicing and the change folder. An entry is a level-two group heading of `tasks.md`, or a `### Requirement:` of a capability's spec delta.
3. Compare, line by line, and classify. There are exactly four cases, and each one has one answer:
   - A line whose `sourceEntry` matches an entry, with the same text: nothing to do.
   - A line whose `sourceEntry` matches an entry, with a different text: rename the entry's title, and leave its body untouched.
   - A line with no `sourceKind`: add an entry for it, with a title and the minimum body OpenSpec requires. Say in the report that its body is a stub.
   - An entry no line points at any more: mark it, do not delete it.
4. Mark an orphan entry by appending ` (to be removed: no longer in the slicing)` to its title. Do not remove its requirements, do not empty its group.
5. Run `openspec validate <change-id> --strict` and fix only what it reports about what you wrote. A failure that predates your edit is reported, not fixed.
6. When you wrote something, commit it in `SECTILE_SPEC_REPO` and push the macro branch:

       git -C "${SECTILE_SPEC_REPO}" add -A
       git -C "${SECTILE_SPEC_REPO}" commit -m "<MACRO-KEY>: realign the specification with the slicing"
       git -C "${SECTILE_SPEC_REPO}" push -u origin HEAD

   Push nothing when you wrote nothing, and never push the default branch. Opening the pull request is the human's gesture: say the branch is pushed and stop there.

## Do not
- Do not rewrite the body of an entry that exists. Its scenarios, its acceptance criteria and its prose are not in the slicing, and regenerating them from a one-line todo would destroy them. A renamed line changes a title, not a body.
- Do not delete an entry. Mark it, so a human decides: the line may have been dropped from the slicing for this iteration and still be worth specifying.
- Do not regenerate the file, and do not renumber it.
- Do not write the macro's todos. They are your input, and the call that writes them replaces the whole list.
- Do not create stories, and do not open a pull request.
- Do not switch a checkout's branch, and do not write on the default branch.

## Report
- What you added, one line each, with the entry you created and a note that its body is a stub.
- What you renamed, old title then new one.
- What you marked as to be removed, and why you did not delete it.
- What you deliberately left alone, and why: an entry whose body you kept, a line you could not match to any entry, a line taken back from a story, an origin you do not know.
- The files touched, with their paths, the branch they are on, and whether it was pushed.

## Macro run
A macro has no stage: this skill declares none, and moves nothing. `SECTILE_MACRO_KEY` and `SECTILE_MACRO_PROJECT_ID` name the macro when Sectile launched the session; the key given as argument wins over them. The ticket variables (`SECTILE_TASK_*`) are not set here.
- **Remote execution indicator**: before doing work, call `start_run` with `projectId`, `macroKey` and the skill name, never a `taskKey`. If `SECTILE_RUN_ID` or a launch runId is supplied, pass it as `runId` to reuse that run. Keep the returned activity ID as runId, and call `finish_run` with the same `projectId` and `macroKey`, the runId, a status (completed, failed or canceled) and a note when the whole invocation ends, including errors or stopping for user input.
- Never touch the macro's title, labels, horizon or priority, and never move its tickets. Never delete anything remote, and never merge. Report faithfully: a file you could not write with the error it gave, a skipped step stated as skipped.
