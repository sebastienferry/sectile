# Make an autonomous run's output visible while it runs

## Why
An autonomous (headless) launch with the AGY or the Claude provider executes, but nothing it
prints ever reaches the user. Two independent defects produce the same silence, and fixing
either one alone leaves the run invisible.

The command shape discards the stream. The AGY autonomous preset ends in
`jq -rs 'map(select(.event == "result"))[0].result.response'`: `-s` slurps the whole stream
before emitting a byte, so the 3-second flush in `superviseHeadlessRun` has nothing to post for
the entire run, and the filter throws away every event but the terminal one — a CLI that dies
mid-stream prints nothing at all. The Claude autonomous preset carries no `--output-format
stream-json`, so it emits one final answer and no progress.

The surface has no renderer. A headless run is registered with no PTY session
(`registerHeadlessRun`, `Headless: true`), so `needsConsoleNotice` sends the desktop console
pane to a static line — "its output is recorded on the task activity" — and the pane never shows
what the run printed. The transport between the two is sound: `postRunOutput` →
`HandleAgentRunOutput` → `AppendRemoteRunOutput` is already incremental.

## What Changes
- Rewrite the three autonomous presets (Claude, AGY, Codex) so each asks its CLI for a JSON
  event stream and renders it through a streaming `jq` filter: `--unbuffered`, never `-s`, one
  readable line per event, the final answer still standing out, and a non-JSON line passed
  through verbatim instead of aborting the pipeline.
- Keep the rendering in the preset, not in the agent. `superviseHeadlessRun` stays ignorant of
  every provider's event schema; a fourth CLI is a new preset and no Go change.
- Make `jq`'s absence a stated failure on the run instead of an empty panel: the agent refuses a
  headless launch whose command line needs `jq` when `jq` is not on the PATH, and a headless
  command line runs under `pipefail` so a broken pipeline cannot report success.
- Buffer a headless run's output on the agent and serve it to the desktop, so the console pane
  shows the run's accumulated output, live, read-only, instead of a static notice.
- Mirror every preset change in `web/src/lib/commandTemplate.ts` and
  `desktop/src/command-preview.mjs`, which mirror `internal/agent/agent_config.go` by contract.

## Impact
- Changed: `web/src/lib/commandPresets.ts`, `web/src/lib/commandTemplate.ts`,
  `desktop/src/command-preview.mjs`, `internal/agent/agent_config.go`,
  `internal/agent/agent_headless.go`, `internal/agent/agent.go` (one desktop route),
  `desktop/src/run-console.mjs`, `desktop/src/main.js`.
- Unchanged: the output transport, the activity record and its 256 KiB cap, the run lifecycle,
  the workflow stages, the settings screens, and every user-written custom command.

## Non-goals
- Attaching a PTY to an autonomous run. It has no terminal by definition.
- Rewriting a user's configured command. Only the shipped defaults change.
- Raising `RemoteRunOutputLimit`. An event stream is more verbose than a final answer and will
  reach the cap sooner; the truncation notice already says so. Tracked separately if it bites.
- A provider-aware renderer in the agent. Rejected on the owner's agnosticism argument; see
  `design.md` D1.
