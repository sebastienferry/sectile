# Surface which agent session is waiting for user input

## Why
Several agent sessions run in parallel across worktrees and desktop tabs. When one blocks on a
permission prompt or an `AskUserQuestion`, nothing says so: the human has to open each session in
turn to find the one that stopped. The bottleneck is not the agent, it is the human noticing. The
upstream requests for a native signal (anthropics/claude-code#36885 and #36850) were both closed
without a solution, and `ccd_session_mgmt.list_sessions` reports `isRunning: false` for a finished
turn and for a blocked one alike, so it cannot answer the question either.

## What Changes
- Ship two Claude Code hook scripts, `Notification` and `Stop`, that read the hook JSON on stdin,
  name the session by the basename of its `cwd`, and raise a macOS notification with a distinct
  sound per event so "needs input" and "turn finished" are told apart without looking at a screen.
- Install those hooks at user level in `~/.claude/settings.json`, merged into the existing user
  settings rather than scaffolded per project, and keep the merge idempotent and reversible.
- Report the waiting state back to Sectile: the `Notification` hook posts to the local agent
  loopback, which marks the corresponding run as waiting; the `Stop` hook clears it.
- Render an explicit waiting state on a run in the desktop and web UI, distinct from running and
  finished, with the time spent waiting, so the sessions list answers the question even when no
  notification was seen.
- Degrade silently: a hook always exits 0, Sectile carries no macOS-only dependency, and an
  unreachable loopback is a no-op.

## Impact
- New: hook scripts and their installer in `internal/agentconfig`, a loopback route and a server
  sub-action for the waiting state, a `waitingSince` field on the run activity.
- Changed: `internal/agent/agent.go` and `agent_console.go` (run identity in the session
  environment), `internal/models/models.go`, `web/src/types/index.ts`,
  `web/src/lib/remoteRunIndicator.ts`, `web/src/components/RemoteRunBadge.tsx`.
- Unchanged: the run lifecycle, `start_run`/`finish_run`, stage transitions and the tracker.

## Non-goals
- SSH-tunnelled notifications to a remote workstation.
- Third-party bridges (`cc-clip`, VS Code extensions) mentioned in the upstream thread.
- Polling `ccd_session_mgmt` to infer the state.
- Reporting a waiting state for agent sessions Sectile did not launch: those are covered by the
  desktop notification alone.
