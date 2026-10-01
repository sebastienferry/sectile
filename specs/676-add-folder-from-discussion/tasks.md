# Tasks #676 - Add a folder from a discussion

Ordered; each task lists its tests. Specification: `spec.md`; design:
`plan.md`.

## 0. Base

- [x] T0 Merge `origin/main` into `feat/676` (no rebase). Check
  `git diff HEAD origin/main -- internal/agent desktop/src` shows only the
  branch's own changes.

## 1. Project folders (US2, US4)

- [x] T1 Add `projectFolderMap` in `internal/agent/repositories.go`.
  - Test (`repositories_test.go`): a project with one context repository
    mapped, a specifications folder and two attached folders (one missing)
    yields a map whose `folderMapDirs` lists the repository, the
    specifications folder and the present attached folder, never the
    directory itself nor the missing folder.

## 2. Conversation turns (US2, FR4)

- [x] T2 `claudeConversationCommand` takes `dirs` and `env`; emits one
  `--add-dir=<path>` per dir after the existing options.
  - Test (`agent_conversation_test.go`): argv contains `--add-dir=/a` and
    `--add-dir=/b c` as two single arguments; no dirs, argv unchanged;
    `SECTILE_REPOSITORIES` is in `cmd.Env` when given.
- [x] T3 `conversationTurn` reads the project folders per turn; on failure it
  writes the notice and runs without dirs.
  - Test: with a fake config fetch, a folder attached between two turns is
    present on the second command only; a failing fetch produces the
    notice and still starts the turn.
  - Keep `agent_conversation_windows_test.go` green (`CREATE_NO_WINDOW` on
    the command).

## 3. Launch lines (US4, FR7, FR8)

- [x] T4 `live()` in `dispatchCommand` appends `addDirArgs(provider, AddDirs)`.
  - Test (`agent_config_test.go`): `discuss` with provider `claude` and two
    dirs ends with two `--add-dir='…'` options; `codex` likewise; `agy` and
    `custom` unchanged; no dirs, unchanged; `open_terminal` without skill
    behaves like `discuss`.
- [x] T5 Free console PTY launch: `--add-dir` for built-in claude/codex,
  `{addDirs}` filled for a template, `SECTILE_REPOSITORIES` in the env.
  - Test (`agent_console_test.go`): the queued command and env for a project
    with attached folders; none for `agy`.

## 4. Attach from a run (US1, US3, FR2, FR3, FR5, FR6)

- [x] T6 `Manager.WaitQuiet` in `internal/terminal/run.go` wrapping
  `waitQuiet`.
  - Test (`internal/terminal`): returns after the quiet period on a silent
    session, and at the cap on a session that keeps printing.
- [x] T7 `controlledRun.interactiveProvider`, set at the discussion launch
  in `agent.go`. Implementation note: it records the provider the live
  launch opens (`liveProvider`), so a custom engine records `custom`, which
  is never typed into, rather than `""`.
  - Test: a `discuss` launch with provider `claude` records `claude`; with
    a custom template records `""`.
- [x] T8 `typeablePath` and `claudePromptPath` in
  `internal/agent/agent_run_folders.go`.
  - Test: `/a/b` as is; `/a/b c` → `"/a/b c"`; `"` and `\` escaped; a path
    with `\n`, `\t` or `\x7f` is not typeable.
- [x] T9 `POST /desktop/run-folder` and the `run-folders` capability.
  Implementation note: the line is typed as text, then Enter as a carriage
  return after a short pause, since Claude Code reads its prompt in raw mode
  (`InjectLine`'s newline would not submit it). A discussion moved to a
  native terminal is attached to but not typed into.
  - Tests (`agent_run_folders_test.go`, httptest, sandbox off):
    - conversation run → folder attached in the settings, answer
      `{typed:false, appliesAt:"next-turn"}`;
    - running `discuss` run with `claude` → one `/add-dir <path>\n` written
      to the session (fake terminal manager or a real PTY session running
      `cat`), answer `{typed:true, appliesAt:"now"}`;
    - `discuss` with `codex` → nothing written, `appliesAt:"next-launch"`;
    - refused attach (duplicate, local repository, specifications folder)
      → attachFolder's status and message, nothing written;
    - checkout of a declared repository → `mappedAs` set and the line typed;
    - unknown run 404; finished run 409; a skill run (not `discuss`) 409;
    - non-typeable path → attached, `typed:false`;
    - `GET /desktop/status` lists `run-folders`.

## 5. Desktop (US1, US3, FR1, FR3, FR9)

- [x] T10 IPC `add-run-folder` in `desktop/electron/main.cjs` with the
  capability check, and `addRunFolder` in `preload.cjs`.
- [x] T11 Composer button in `desktop/src/conversation.js` (`canAddFolder`
  option, enabled while busy, disabled read-only, status notice kept over
  polling).
- [x] T12 Toolbar button and status in `desktop/src/main.js` for a running
  ticket discussion; hidden otherwise and without the capability.
- [x] T13 Styles in `desktop/src/style.css`.
  - UI tests (build first: `npx vite build`; run unsandboxed):
    - extend `desktop/tests/conversation.ui.cjs`: the button is present,
      picking a folder (stubbed `chooseRepository`) calls
      `addRunFolder(id, path)` and shows "Attached …"; a refusal shows the
      agent's message; a read-only conversation disables it; an agent
      without `run-folders` hides it;
    - extend `desktop/tests/attached-folders.ui.cjs` (or a new
      `run-folders.ui.cjs`): the toolbar button shows on a running
      `discuss` run and not on a skill run, a finished run, or a
      conversation; each `appliesAt` outcome renders its text.
  - Restore `internal/webui/dist/.gitkeep` if the build removes it.

## 6. Documentation

- [x] T14 `CHANGELOG.md` `## [Unreleased]`: the `Added` and `Changed` lines
  of plan §6.
- [x] T15 `docs/experiments/desktop-conversation.md`: folders per turn and
  the composer button.

## 7. Verification

- [x] T16 `go test ./internal/agent/... ./internal/terminal/... ./internal/runner/...`
  (outside the sandbox for httptest; `GOCACHE` under `$TMPDIR`), `go vet`,
  desktop UI tests, `oxlint` on the touched JS.
- [ ] T17 Manual check with the installed Claude Code: in a ticket
  discussion, add a folder whose path contains a space; confirm Claude Code
  accepts the typed `/add-dir "<path>"` (plan, "Risks"). If it does not,
  apply the fallback (type nothing for such a path) and adjust T8's test.
- [ ] T18 Manual check: a conversation lists a file of a folder attached
  from its composer on the next message (AC1).

## Status

T0 to T16 are done. T17 and T18 need the installed Claude Code and are left
to the owner: the automated tests cover the typed bytes and the turn's argv,
not Claude Code's own reading of them.
