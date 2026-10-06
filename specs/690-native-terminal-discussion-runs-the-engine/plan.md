# Plan #690 - "Discussion in native terminal" runs the ticket discussion

## 1. Placement

Go agent only, `internal/agent/agent_desktop.go`,
`desktopTasksTerminalExternal`. Desktop and server are unchanged.

## 2. Flow

1. Unchanged: decode, `fetchConfig`, read the task, refuse a finished task or
   another project's.
2. `localProjectRoot` as today, then `config = agentconfig.ResolveTask(config,
   overrides, task.ID)` (FR1).
3. Unchanged: `primaryRoot`, work directory and branch.
4. `folders := d.taskFolderMap(ctx, config, task, workDir)`,
   `claudeSettings := d.launchClaudeSettings(config)`, then
   `dispatchCommand(config, task.ID, input.SkillID, "", "", "", "", "",
   agentCommandContext{Task, Branch, Directory, Tracker, Repo, AddDirs:
   folderMapDirs(folders), ClaudeSettings})` (FR2). Built before the run is
   registered, so a failure leaves nothing to release.
5. Register the run as today, with `Status: "preparing"` and
   `interactiveProvider = discussionProvider(config, input.SkillID)` (FR5).
6. `wrapRun(task.ID, runID, line)`, which needs the run `preparing`.
7. Environment: today's map plus `SECTILE_REPOSITORIES` (FR4).
8. `runInPty(runID, workDir, env, wrapped)` replaces `GetOrCreateSession` and
   `tapConsole` (FR3); then the run moves to `running` unless it already
   reported its exit.
9. `launchExternalTerminal` as today.

A failure at 6 or 8 closes the session, closes `run.exited`, removes the run
from the queue and the store, and answers 500, as the PTY failure does today
(FR6).

## 3. Tests

`internal/agent/agent_desktop_test.go`, a new test beside the existing one,
POSIX only:

- a fake `claude` first in `PATH` writes its arguments and
  `$SECTILE_REPOSITORIES` to a file, then sleeps;
- workstation settings: the project's checkout, an attached folder, Claude as
  the default engine;
- the agent daemon runs its loopback, so `agent-exec` can report to it
  (`TestMain` already serves `agent-exec`);
- after the POST: 200, the native terminal launched on the run, the fake
  received `--add-dir=<folder>` and `SECTILE_REPOSITORIES` names the folder,
  and the run is `running` with the discussion's engine recorded.

The existing test keeps checking the checkout, the branch and the terminal.

## 4. Changelog

`Fixed`: "**Discussion in native terminal starts the ticket discussion.** From
the Desktop tickets pane, it opened a bare shell; it now runs the task's
engine in the native terminal, with the project's folders, as **Discussion
(no skill)** does in the app. Upgrade the agent along with the desktop. (#690)"
