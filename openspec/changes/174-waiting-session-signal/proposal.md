# Surface which agent session is waiting for user input

## Why
Several agent sessions run in parallel across worktrees and desktop tabs. When one blocks on a
permission prompt or an `AskUserQuestion`, nothing says so: the human has to open each session in
turn to find the one that stopped. The bottleneck is not the agent, it is the human noticing. The
upstream requests for a native signal (anthropics/claude-code#36885 and #36850) were both closed
without a solution, and `ccd_session_mgmt.list_sessions` reports `isRunning: false` for a finished
turn and for a blocked one alike, so it cannot answer the question either.

The signal has to arrive as a real operating-system notification, carrying the same icon that marks
the state in the task list. A banner the user cannot attribute to Sectile, and whose glyph says
nothing the list also says, is one more thing to decode rather than an answer.

## What Changes
- Ship two Claude Code hook scripts, `Notification` and `Stop`, that read the hook JSON on stdin and
  report the session state. The hooks raise no alert themselves: they are reporters.
- Install those hooks at user level in `~/.claude/settings.json`, merged into the existing user
  settings rather than scaffolded per project, and keep the merge idempotent and reversible.
- Report the state to the local agent loopback: a session Sectile launched is identified by the run
  it carries in its environment; a session it did not launch is identified by its working directory,
  authenticated with the agent connection file the workstation already publishes.
- Raise the alert from the desktop application, through the operating system's own notification
  facility, so the banner is attributed to Sectile, carries its icon, and works on macOS, Windows
  and Linux alike without any external binary.
- Draw the notification icon from one shared state vocabulary, so the glyph on the banner is the
  glyph on the task row: waiting, running, finished.
- Render an explicit waiting state on a run in the desktop and web UI, distinct from running and
  finished, with the time spent waiting, so the sessions list answers the question even when no
  notification was seen.
- Degrade silently: a hook always exits 0, Sectile carries no platform-specific dependency, an
  unreachable loopback is a no-op, and a workstation with notifications denied still shows the state
  in the list.

## Impact
- New: hook scripts and their installer in `internal/agentconfig`, a loopback route and a server
  sub-action for the waiting state, a session-level report for sessions Sectile did not launch, a
  `waitingSince` field on the run activity and on the desktop run, a shared run-state vocabulary
  consumed by the web badge and the desktop notification.
- Changed: `internal/agent/agent.go` and `agent_console.go` (run identity in the session
  environment), `internal/agent/agent_desktop.go`, `internal/models/models.go`,
  `desktop/electron/main.cjs`, `desktop/src/main.js`, `web/src/types/index.ts`,
  `web/src/lib/remoteRunIndicator.ts`, `web/src/components/RemoteRunBadge.tsx`.
- Unchanged: the run lifecycle, `start_run`/`finish_run`, stage transitions and the tracker.

## Non-goals
- SSH-tunnelled notifications to a remote workstation.
- Third-party bridges (`cc-clip`, VS Code extensions) mentioned in the upstream thread.
- Polling `ccd_session_mgmt` to infer the state.
- Notifying when the desktop application is not running. The state is still recorded and still shown
  in the list; the banner is the desktop's job, and a closed desktop raises none.
