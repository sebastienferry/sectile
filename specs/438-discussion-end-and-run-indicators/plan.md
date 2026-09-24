# Plan — ending a discussion, and readable run indicators

## Stack

- Local agent, Go (`internal/agent`). Gate: `go build ./...`, `go vet ./...`,
  `go test ./internal/agent/...`.
- Desktop, Electron + plain JS (`desktop/src`), unit tests `node --test`
  (`npm test` in `desktop/`), UI tests `tests/*.ui.cjs` against a Vite build.
- Shared run-state definition `shared/runStates.ts`, read by the Desktop and by
  the web (`web/src/components/RunStateGlyph.tsx`). Web gate: `npm test`,
  `npx tsc --noEmit`, `npx oxlint src`.

## How a stopped run ends today

Stop sets `controlledRun.canceled`. Three places turn that flag into a status:

1. `handleRunControl` (`agent_run.go`): the supervised `agent-exec` reports its
   exit, and `canceled` overrides it.
2. `/desktop/stop` (`agent_desktop.go`): calls `finishDesktopRun(..., "canceled")`
   once the exit is confirmed, or after `recoverOrphanedPTYRun` found the
   terminal gone.
3. `recoverOrphanedPTYRun`: writes `run.desktop.Status = "canceled"`.

Launch failures (`agent.go` deferred `!launched` block, `agent_console.go`) and
shutdown also map `canceled` to `canceled`; they stay as they are (FR 4).

## Design

- `stoppedStatus(skill string) string` in `agent_run.go`: `completed` for
  `models.NormalizeSkillID(skill) == "discuss"`, `canceled` otherwise. Used at
  the three places above, reading `run.desktop.Skill` under the queue lock.
- The `finish_run` note for a discussion reads "Discussion ended" (and "…after
  its local terminal closed") instead of "Execution canceled".
- `skillResult` (`desktop/src/skill-result.mjs`): a run whose `skill` is
  `discuss` is treated like a free console — only a pending stop is reported.
  `refreshSkillResult` skips fetching the result for it, like `freeConsole`.
- Tooltips (`desktop/src/main.js`): `renderRunState` and `renderHeaderState` set
  `title`/`aria-label` to `Process: <label>`; `renderTaskSkillStatuses` and the
  header badge set `Skill: <result label>`. The header keeps the visible label
  without the prefix.
- Running glyph (`shared/runStates.ts`): `running.icon` becomes a filled circle
  `['circle', {cx:12, cy:12, r:6, fill:'currentColor'}]`; `spins` is renamed
  `pulses`. `runStateSvg` adds `color="<state colour>"` on the root so
  `currentColor` resolves to the state's colour in standalone markup (the
  notification icon). `RunStateGlyph` inherits the badge colour as today.
- Desktop CSS: the spin keyframes become a slow opacity/scale pulse on
  `[data-run-state=running] svg`, disabled under `prefers-reduced-motion`.
- Web: `RemoteRunBadge` and `ActivitiesView` apply `animate-spin` to the running
  glyph; a spinning dot shows nothing, so they use `motion-safe:animate-pulse`.
- Placement: the sidebar row appends `state, title, status`; in the header the
  `#run-state` span moves before `#title` in the static markup.

Rejected: a new `closed` status (server model, filters and every terminal
check change for a wording gain — settled in the clarification). Overriding the
running glyph in the Desktop only: it would break the single definition the
notification and the web read.

## Target files

- `internal/agent/agent_run.go`, `agent_desktop.go`; tests in
  `agent_run_test.go`, `agent_desktop_test.go`
- `shared/runStates.ts`, `web/tests/runStates.test.mjs`
- `desktop/src/skill-result.mjs`, `desktop/src/main.js`, `desktop/src/style.css`
- `desktop/tests/skill-result.ui.cjs`, `run-state-animation.ui.cjs`,
  `tooltips.ui.cjs` / `task-header.ui.cjs` as affected
- `web/src/components/RemoteRunBadge.tsx`, `ActivitiesView.tsx`
- `CHANGELOG.md`
