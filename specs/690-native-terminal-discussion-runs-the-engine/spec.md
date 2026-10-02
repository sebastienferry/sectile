# Specification #690 - "Discussion in native terminal" runs the ticket discussion

- Ticket: https://github.com/sebastienferry/sectile/issues/690
- Branch: `feat/batch-675-690-693` (batch #675, #690, #693)
- Clarification: `docs/clarifications/690.md` (round 1, no product question)
- Framework: Spec Kit
- Type: bug

## Summary

From the Sectile Desktop tickets pane, "Discussion in native terminal" opens a
native terminal window in which the task's engine is running the ticket
discussion, with the project's folders, exactly as "Discussion (no skill)"
starts it inside the app.

## Scope

In scope: the agent endpoint behind the action
(`POST /desktop/tasks/terminal-external`), its test and the changelog.

Out of scope: the conversation view, "Detach to native terminal", the web
interface, the menu itself and every other launch.

## User stories

### US1 (P1) - The discussion starts in the native terminal

1. Given a task whose engine is Claude Code, when I choose "Discussion in
   native terminal", then a native terminal opens on the run's console and
   Claude Code is running in it, in the task's checkout.
2. Given the same launch, then the engine is the one the in-app discussion
   would use for that task (the task's engine, else the project's, else the
   workstation default).
3. Given the engine exits, then the run ends, as an in-app discussion does.

### US2 (P1) - The session sees the project's folders

1. Given a folder attached to the project on this workstation, when the
   discussion starts, then the engine line carries `--add-dir` for it (for an
   engine whose option is attested, Claude Code and Codex).
2. Given the same launch, then the session's environment carries
   `SECTILE_REPOSITORIES` with the folder map.
3. Given the project's Claude Code sandbox settings, then the line carries
   them, as the in-app launch does.

## Functional requirements

- FR1. The endpoint resolves the task's configuration with the workstation
  overrides, as the dispatch of an in-app discussion does.
- FR2. It builds the line of skill `discuss` with the task's folder map and
  the Claude settings, through the same function as the in-app discussion.
- FR3. It types the line, wrapped so the agent sees the engine exit, into the
  run's console before opening the native terminal on it.
- FR4. The console's environment carries `SECTILE_REPOSITORIES` when the map
  is not empty, besides the variables it carries today.
- FR5. The run records the discussion's engine, so "Add folder" can reach it.
- FR6. A failure to build or start the line releases the run and answers an
  error, and no native terminal opens.
- FR7. The answer keeps its shape: `success`, `runId`, `terminal`.
- FR8. `CHANGELOG.md` gets one `Fixed` line under `## [Unreleased]`.

## Acceptance criteria

- [ ] From the tickets pane, "Discussion in native terminal" opens a terminal
  where the configured engine runs the ticket discussion.
- [ ] That session sees the project's attached folders.
