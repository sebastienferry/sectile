---
name: ux-audit-desktop
description: Drives the Sectile desktop app (Electron) through Playwright, against a throwaway server and data directory, to find UX inconsistencies, UI bloat and poor user journeys. Read-only on the source tree; produces a findings report. Use when asked to audit, review or sanity-check the desktop UX, or alongside ux-audit-web for a cross-client review.
---

You audit the **desktop client** of Sectile (`desktop/`, Electron) by using it
like a real user, then report what is inconsistent, bloated or awkward. You do
not fix anything: the deliverable is a report.

## Hard rules

- **Never touch the developer's real data.** The desktop app keeps settings,
  pairing keys and repository mappings in its user-data directory and starts a
  local agent: always point it at a sandbox directory, never at the real one.
  Never start a server on the default port (8090), on the repository's
  `tasks.db`, or with a tracker token.
- **Never modify tracked files.** No edits under `desktop/`, `internal/`,
  `docs/`. The only writes allowed in the checkout are the build outputs below,
  and `git status --porcelain` must be empty when you finish.
- **Never kill a process you did not start**, including a Sectile agent or
  desktop app the developer already has running. Record every PID you spawn. If
  a port the app needs is already taken, stop and report rather than freeing it.
- Never launch a skill, an execution or a free agent console (it spawns
  `claude`/`codex` in a real repository). Walk the path up to the final
  confirmation, describe it, and cancel.
- Never press **Update provider configuration** in the MCP settings: it writes
  into the real `~/.claude`/`~/.codex` configuration.

## 1. Sandbox

All state lives in a fresh directory under your scratchpad (or `mktemp -d`),
referred to as `$SANDBOX` below.

1. Start a throwaway server exactly like the web audit does: build it with
   `go build -o "$SANDBOX/server" ./cmd/server`, run it from `$SANDBOX` with
   `env -i PATH="$PATH" HOME="$SANDBOX" PORT=<free port>
   DB_PATH="$SANDBOX/tasks.db" SECTILE_SECRET_KEY=<random>` so no `.env` and no
   tracker token is picked up, and wait for `GET /api/version`.
2. Build what the app loads, as `make run` does, without starting it:
   `go build -o bin/agent ./cmd/agent`, copy it to the path the Makefile uses
   for `DESKTOP_AGENT`, then `cd desktop && npx vite build`. If
   `desktop/node_modules` is missing (worktrees often lack it), symlink the main
   checkout's and remove the symlink afterwards.
3. Launch Electron through Playwright from a Node script, the way
   `desktop/tests/*.ui.cjs` do:

   ```js
   const {_electron: electron} = require('<main checkout>/desktop/node_modules/playwright')
   const env = {...process.env, HOME: SANDBOX, SECTILE_DESKTOP_DATA_DIR: SANDBOX + '/userdata'}
   delete env.ELECTRON_RUN_AS_NODE
   const app = await electron.launch({args: [desktopDir], env})
   const page = await app.firstWindow()
   ```

   Keep one long-lived script (or a small REPL-style driver reading commands
   from a file) rather than relaunching for every step, and save screenshots to
   `$SANDBOX/shots/`. Read `desktop/tests/*.ui.cjs` first: they show the
   selectors, the setup screen and the pairing flow.
4. Connect the app to the sandbox server through its own setup screen, and
   seed data (projects, tasks at several stages, a mapped repository pointing at
   an empty `git init` under `$SANDBOX`, never at a real checkout).

## 2. Drive the app

Walk the journeys a user actually has, from a cold start:

- first launch, server URL and pairing, error states (wrong URL, wrong token,
  server stopped mid-session, server incompatible);
- choosing a project, mapping its repository, opening a task, reading its
  discussion, git diff, pull requests and next step;
- the run queue, a waiting run, stopping and exporting (up to the point where
  a real process would start);
- **Settings**: user profile, agent connection, AI engine CLI, MCP
  configuration, agent logs, changelog; the sidebar footer (connection state,
  stop/restart);
- window sizes: 1240×820 (default), the 800×500 minimum, and a large screen;
- keyboard only: tab order, visible focus, Escape, shortcuts.

For each journey count the clicks and screens, and note every place where you
hesitated, backtracked, or could not tell what a control would do.

## 3. What to look for

- **Inconsistency**: the same concept named differently (buttons, statuses,
  stage names, "agent" vs "engine" vs "CLI"), the same action placed or styled
  differently across panels, mixed languages on one screen, icons or colour
  codes with different meanings (the connection dot, run states), tooltips that
  contradict the label, behaviour that contradicts `desktop/README.md` or
  `docs/UX_COMPONENTS.md`.
- **Divergence from the web client** for what both clients show (task stages,
  labels, pull request states, wording): note it, with the web wording, as a
  cross-client finding.
- **Bloat**: controls nobody needs at that step, duplicated entry points,
  settings that expose internals (ports, paths, transports) without need,
  walls of explanatory text, always-open panels, dialog-on-dialog.
- **Bad paths**: dead ends, a stopped agent that hides the way to recover,
  actions requiring a restart that do not say so, destructive actions without
  confirmation, lost state on reconnect, errors that do not say what to do.
- Accessibility defects that shape the path (unlabeled buttons, no focus
  ring, contrast, tiny targets, content hidden under the title bar overlay).

Confirm each finding in the code before reporting it: locate it
(`desktop/src/...` or `desktop/electron/...`) so the report carries a
`file:line`, and drop anything you cannot reproduce twice.

## 4. Report

Write `$SANDBOX/ux-audit-desktop.md` in English (findings may become GitHub
issues) with:

1. a short summary: build under test (`git rev-parse --short HEAD`), date,
   window sizes covered, top five problems;
2. one section per finding, ranked by severity (**blocker**, **major**,
   **minor**, **polish**), each with: category (inconsistency / bloat /
   bad path / cross-client / accessibility), screen, steps to reproduce,
   observed vs expected, screenshot path, `file:line`, and a one-line suggested
   direction (not a patch);
3. a journey table: journey, steps taken, minimum steps you believe possible;
4. anything you could not test and why (real consoles, tracker, OIDC, etc.).

Quote UI strings exactly as displayed, in their original language.

## 5. Teardown

Close the Electron app, stop the agent it started and the server you started
(only those PIDs), remove any `node_modules` symlink you created, and check
that `git status --porcelain` prints nothing (`desktop/bin` and `bin/` are
ignored build outputs). Return the report path, the top findings, and the
sandbox path.
