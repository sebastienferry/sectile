# ADR 0012: The waiting state is a timestamp on the run, reported through the loopback

Status: Accepted, then revised: the hook transport is withdrawn (#260), and an MCP tool is the reporter (#318)

## Context

Several agent sessions run in parallel across worktrees and desktop tabs. When
one blocks on a permission prompt or a question, nothing says so: the human has
to open each session in turn to find the one that stopped. The bottleneck is not
the agent, it is the human noticing.

The upstream requests for a native signal (anthropics/claude-code#36885 and
\#36850) were both closed without a solution. The desktop app's
`ccd_session_mgmt.list_sessions` reports `isRunning: false` for a finished turn
and for a blocked one alike, so it cannot answer the question either, and PTY
inactivity would flag a long build as waiting.

What does answer it is the `Notification` hook, which fires exactly when the
agent asks for input, and the `Stop` hook for the symmetric case.

## Decision

**The transport is the agent loopback, not a new MCP tool.** A launched session
is already given `SECTILE_RUN_ID`, `SECTILE_LOOPBACK_URL` and
`SECTILE_AGENT_TOKEN`, and a hook is a child process that inherits them. It
therefore knows which run it belongs to without any `cwd` correlation, which
would have been ambiguous the moment two worktrees shared a basename. The
loopback route relays the report to `POST /api/activities/{id}/waiting`.

`finish_run` goes through MCP because it is part of the contract a model calls;
this report is made by a shell script with no MCP client, and a two-line `curl`
is the whole client.

The route authenticates with the workstation API key rather than the per-run
token of `/control/runs/{id}`: that token exists only for a wrapped dispatched
run, and a free console has a run identifier but no such token. The key is what
every launched session is given, console included.

**The state is `waitingSince *time.Time`, not a new `ActivityStatus`.**
`ActivityStatus` is shared by the server, the persisted activities,
`ActivityStats` and every filter; a new enum value would have to be handled by
each of them, and by every activity written before the change. A nullable
timestamp is additive, needs no backfill, and carries the "waiting for 4 min"
the UI wants, which a flat status cannot express. A waiting run keeps the status
`running`, so it stays inside the fast polling window the client already applies
to live runs.

`waitingSince` is set by the waiting report, cleared by the resumed report, and
cleared by every path that gives a run a terminal status, so a session killed
while blocked cannot leave a run waiting forever.

**The hooks are installed once per workstation.** `ResolveLocations` already
established that Claude configuration is user-level, because the per-project
layout it replaced left four dead copies per project. The hooks follow that
rule, which makes Sectile a writer of `~/.claude/settings.json` — a file it
previously only read, in `checkExternalMCPPolicies`. The file is therefore
merged, never replaced: only the Sectile-owned entries are added or updated, and
a file that cannot be parsed is left alone with the failure reported rather than
the setup aborted.

## Consequences

- A run has two live presentations, running and waiting, derived in one place
  (`deriveRunIndicator`) so every surface inherits the same precedence.
- Sectile owns two more managed files, and they are in the whitelist, so an
  uninstall removes exactly what it wrote.
- The desktop alert is macOS-only, by way of `osascript`. Nothing else in the
  feature is: the state is stored and rendered identically on any platform, and
  the alert degrades to a silent no-op where the command is missing.
- The Claude Code hook payload is an external contract with no version we
  control. The hooks read only `cwd`, treat its absence as a fallback name, and
  never fail the session.
- A free console now exports its own `SECTILE_RUN_ID`, where it previously
  exported an empty value. It is a local run the server does not record, so its
  report is dropped at the relay; the desktop alert still fires.

## Revision: the banner moved out of the hook

The first implementation raised the alert from the hook itself, with
`osascript -e 'display notification …'`. It was withdrawn.

`display notification` is a real Notification Center banner, but macOS
attributes it to **Script Editor**, not to Sectile, and the command accepts no
icon at all. So the banner could not be recognised as Sectile's, and — more
importantly — it could not carry the glyph that marks the same state in the task
list, which is half of what this change exists for. It was also macOS-only,
which the change's own degradation rule forbids.

The banner is therefore raised by the desktop application. Electron's
notification API is a thin binding over each platform's own facility, so the
result is a real system notification, attributed to the packaged application,
carrying an icon we choose, on macOS, Windows and Linux alike, with no
`terminal-notifier`, no Homebrew and no external binary. The desktop already
polls the run list, so the transition — not the state — is what notifies.

Alongside it, `shared/runStates.ts` became the single definition of a run
state's label, colour and glyph. The web badge and the desktop notification both
read it, so their icons cannot drift apart; adding a state is one entry, and
neither surface can show a state the other does not know.

Attribution was measured rather than assumed. On a packaged build Launch
Services reports `CFBundleIdentifier=com.electron.sectile` and
`LSDisplayName=Sectile` — the identity macOS attributes a notification to —
where a build run from source reports `com.github.Electron`. So the banner is
Sectile's once packaged, and only once packaged; a development run still shows
Electron, which is a property of the build and not of this code. The bundle is
ad-hoc signed and its code-signature identifier is still `Electron`; that does
not affect attribution, which keys off the bundle, and `Notification.permission`
comes back `granted` with no prompt.

Two consequences follow, both accepted:

- No desktop application running means no banner. The waiting state is still
  recorded and still rendered, so the list answers the question either way.
- A session Sectile did not launch has no run to hang a report on. Rather than
  dropping the coverage that `osascript` gave for free — and that the ticket
  explicitly asks for, naming "several Claude Code desktop tabs" — such a
  session reports itself by the basename of its working directory, through the
  connection file the agent already publishes for its companions. Those reports
  are held in a bounded list and drained by the desktop's existing poll.

## Alternatives rejected

- **Polling `ccd_session_mgmt`**: undocumented, unversioned, and it cannot tell
  a finished turn from a blocked one, which is the whole question.
- **Deriving the state from PTY inactivity**: a false positive on every slow
  tool call.
- **A new `ActivityStatus` value**: see above; and it could not carry a duration.
- **Scaffolding the hooks per project**: the flaw `locations.go` documents having
  fixed.
- **The `PreToolUse` workaround** circulated in the upstream issue: it fires on
  every single tool call.
- **`terminal-notifier` / `notify-send` from the hook**: they do carry an icon,
  but they are external binaries the workstation may not have, one per platform,
  and they would put the icon vocabulary in a second place where it would drift
  from the interface's.
- **The browser Notification API from the web UI**: a permission prompt per
  browser, the tab has to be open, and a background tab is throttled.

## Revision: the wait is bracketed from both sides

The first implementation set the wait on `Notification` and cleared it on
`Stop`, and nothing else. In use, the indicator was wrong about as often as it
was right, in both directions:

- **A hand while the agent worked.** A permission granted, a question answered
  or a prompt typed after an idle notification resumes the agent, and no hook
  fired on any of those. The wait stayed set for the whole turn that followed,
  sometimes many minutes of tool calls under a raised hand.
- **A spinner while the agent waited.** `Stop` was read as "resumed", so the
  end of a turn showed the session as running while it sat at the prompt. The
  `idle_prompt` notification corrected that, but Claude Code sends it sixty
  seconds after the turn ends.
- **Spurious hands.** `Notification` fires for more than prompts: a sign-in, a
  quota notice, a finished sub-agent. Each set the wait.

The fix is a change of reading, not of transport. **A wait opens on
`Notification` and on `Stop`**, because both leave the agent awaiting the user,
and **closes on `UserPromptSubmit`, `PreToolUse` and `PostToolUse`**, because
each of those only fires while the agent works. `PreToolUse` fires before the
permission prompt for the same tool and `PostToolUse` after it ran, so a granted
permission clears the wait as soon as the tool has done its work, and a denied
one clears it on the agent's next tool call or at the end of its turn.
`Notification` is filtered on its `notification_type`: only a prompt, an idle
prompt or an elicitation is a wait; an absent type is an older payload and is
read as a prompt, which is what every payload was before the field existed.

Two consequences on the agent side:

- **An autonomous run never waits.** Its `Stop` hook fires as the process ends,
  and a wait set there would raise a false "waiting for you" banner in the poll
  before the exit is observed. The loopback accepts the report and changes
  nothing for a headless run; the exit is the only thing that reports on it.
- **Only a transition is relayed.** Every tool call now reports "working" again,
  and each relay is a row update plus a `task_updated` event on every client.
  The agent keeps the state, relays it when it changes, keeps the stamp of the
  first waiting report so the duration stays honest, and serialises relays per
  run so that a prompt asked and granted within a few milliseconds cannot reach
  the server as two requests in the wrong order.

The two per-event scripts became one, `sectile-hook.sh`, registered on the five
events and reading the event from its payload. One script is the point: the
split is how the first version came to clear the wait in one place only. The
retired names stay recognised so an upgrade retires the files through the
manifest and drops their registrations from `~/.claude/settings.json` rather
than leaving two dead entries that would fail on every turn.

The cost is one `sh` plus one `curl` to the loopback per tool call, measured
under 50 ms on a 200 KB payload. It was preferred to an `async` registration, which would let a
"working" report land after the "waiting" one it is meant to precede, and to a
marker file that the script would have to keep in step with the agent.

## Revision (#260): the hooks are withdrawn

The Claude Code hook was the reporter of this ADR's transport, and it is gone.
Sectile installs no hook any more, writes nothing new to
`~/.claude/settings.json`, and the agent no longer exposes
`/control/runs/{id}/waiting` or `/desktop/session-alert`.

#260 began as a Windows defect: the registered command was the path of a `.sh`
file, which `cmd.exe` resolves through its file association, so the hook had
never run there. The first answer was to port it into the agent binary as a
subcommand. The decision taken instead was to remove it, for two reasons that
the port would not have changed:

- **It was intrusive.** The hook made Sectile a writer of a user file it had no
  other business writing, and it ran a process on every tool call of every
  Claude Code session on the workstation — launched by Sectile or not, since
  the registration is user-level by the rule this ADR itself adopted.
- **It was not needed.** The desktop already raises a banner when a run reaches
  a terminal status, from the run list it polls; the agent observes that exit
  itself. What the hook added was the waiting mark and the banner for sessions
  Sectile did not launch, and neither was judged worth the cost above.

What stays is the model, deliberately: `waitingSince` on the run, the server
sub-action that sets and clears it, the board and activities rendering of a
waiting run, and the desktop's banner on a waiting transition. They are
additive and idle. A future reporter that is not a per-tool-call hook can feed
them without touching the interface; until then a blocked session shows as
running, which is what it showed before #174.

The retirement is the part that must keep working: a workstation upgraded from
any release that installed a hook has the script removed through the managed
manifest and its registrations dropped from the settings file, on the next
project setup and for whichever provider that project uses, leaving third-party
hooks, every other key, and an unparseable file exactly as they were.

## Revision (#318): the model declares its wait over MCP

The model kept by #260 has a reporter again, and it is the one this ADR first
set aside: an MCP tool, `report_waiting`. The reason given for the loopback no
longer holds. The reporter was a shell script with no MCP client; now it is the
model itself, which already speaks MCP for `start_run` and `finish_run`, and is
the only party that knows it is about to block. Nothing runs per tool call, and
nothing is written outside Sectile.

- **Declared by the model, ended by the server.** A model that forgets to clear
  its wait must not leave a run waiting for hours, so the server ends the wait on
  the declaring session's next tool call. A ping does not count: the stdio bridge
  pings every minute whatever the model is doing. `waiting: false`, any terminal
  status, and the end of the session clear it as well.
- **Ownership follows `finish_run`**, and the first mark wins, so the elapsed time
  shown is the real one. A headless run is never marked.
- **The desktop is fed from the server.** The wait is declared on the server, and
  the desktop reads the agent's run list, so the server pushes a `run_waiting`
  message to the owner's agent, which restores `waitingSince` on its run.
  Nothing is sent to the tracker.
- **What is not covered.** A tool permission prompt is raised by the client, not
  asked by the model, and is not detected. The question text is not stored: the
  mark stays a timestamp and needs no migration.
