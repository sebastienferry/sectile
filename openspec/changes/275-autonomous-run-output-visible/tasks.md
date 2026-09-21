# Tasks

## 1. The autonomous presets
- [ ] 1.1 Record one real event stream per provider (`claude`, `agy`, `codex`) into a fixture
      under `web/src/lib/__fixtures__/` (a dozen lines: progress events, the terminal event, one
      plain-text warning). Confirm at the same time which flags each CLI needs for the stream —
      `--output-format stream-json` and whether `--verbose` is required with it for `claude`,
      the JSONL flag for `codex exec` (D3).
- [ ] 1.2 Write the three `jq` filters against those fixtures, obeying D2: `--unbuffered -Rr`,
      never `-s`, one readable line per event, the final answer set apart, `fromjson?` and
      `try … catch .` for non-JSON and unknown shapes.
- [ ] 1.3 Update the three autonomous entries in `web/src/lib/commandPresets.ts`, keeping every
      preset overridable by `SECTILE_PRESET_*` / `VITE_PRESET_*`.
- [ ] 1.4 Mirror the new default autonomous command lines in `commandPreview` in
      `web/src/lib/commandTemplate.ts` and in `desktop/src/command-preview.mjs`, and in the
      agent's own fallbacks in `internal/agent/agent_config.go`. These three mirror each other
      by contract; a preset that lands in one and not the others is the bug the doc comments
      warn about.
- [ ] 1.5 Test: a web test piping each fixture through its filter, asserting output appears for a
      stream truncated before its terminal event, that a plain-text line survives verbatim, and
      that the final answer is present in full. Skip cleanly when `jq` is absent so the suite
      stays runnable without it.
- [ ] 1.6 Test: assert no shipped preset contains a slurping `jq` invocation.

## 2. The agent's headless launch
- [ ] 2.1 In `startHeadlessRun`, preflight `jq` when the resolved command line references it, and
      finish the run as `failed` with a message naming `jq` before spawning `bash` (D4).
- [ ] 2.2 Prefix the headless line with `set -o pipefail;` (D5).
- [ ] 2.3 In `superviseHeadlessRun`, append each drained chunk to a transcript held on the
      `controlledRun`, under the same 256 KiB budget, truncating head-first with a stated marker
      (D6, D8). The existing flush-and-post path is unchanged.
- [ ] 2.4 Serve `GET /desktop/runs/{id}/output?offset=N` from `desktopHandler`: the bytes after
      `offset`, the new offset, the run's status, and whether the transcript was truncated. 404
      for an unknown run; the existing desktop token check applies.
- [ ] 2.5 Go tests: the `jq` preflight failing a run before launch; a pipeline whose first stage
      fails recorded as `failed`; the transcript endpoint returning only what follows an offset,
      and its truncation marker.

## 3. The desktop console pane
- [ ] 3.1 Add the transcript call to the desktop's agent API surface alongside `runResult`.
- [ ] 3.2 In `select()` (`desktop/src/main.js`), branch on `run.headless === true` before the
      notice: write the read-only banner, then poll the transcript and write each new chunk into
      the terminal, keeping the offset per run (D7).
- [ ] 3.3 Clear the poll on selection change, on the run reaching a terminal status after its
      last chunk, and on agent disconnection. Leave the terminal read-only: no `api.attach`, no
      input forwarding.
- [ ] 3.4 Update `desktop/src/run-console.mjs`: the headless branch of `consoleNotice` becomes
      the read-only banner plus the empty-so-far wording; the queued, preparing and
      missing-session branches are untouched.
- [ ] 3.5 Test: extend `desktop/tests/console.ui.cjs` (or a sibling UI test) over a headless run —
      output shown on selection, a later chunk appended without reprinting, a finished run
      showing its tail, an empty run showing the stated wording, and a queued run still showing
      its own notice. A build precedes the UI tests (`npx vite build`).
- [ ] 3.6 Unit-test `consoleNotice`/`needsConsoleNotice` for the branches that did not change.

## 4. Gates
- [ ] 4.1 `make test` (go, web tests, tsc, oxlint).
- [ ] 4.2 `npm test` in `desktop/`.
- [ ] 4.3 One real autonomous run per provider on a workstation with `jq`, and one with `jq`
      removed from the PATH, watched in the desktop pane.
- [ ] 4.4 Re-read the diff as a reviewer.
