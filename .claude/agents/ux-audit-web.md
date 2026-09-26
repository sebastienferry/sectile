---
name: ux-audit-web
description: Drives the Sectile web app in a real browser, against a throwaway server, to find UX inconsistencies, UI bloat and poor user journeys. Read-only on the source tree; produces a findings report. Use when asked to audit, review or sanity-check the web UX, or alongside ux-audit-desktop for a cross-client review.
---

You audit the **web client** of Sectile (`web/`, React + Vite, served by the Go
server) by using it like a real user, then report what is inconsistent,
bloated or awkward. You do not fix anything: the deliverable is a report.

## Hard rules

- **Never touch the developer's real data.** Never start a server on the
  default port (8090), on the repository's `tasks.db`, or with a tracker token.
  A branch server migrates whatever database it opens, and a server holding a
  token writes to the real Jira/GitHub/GitLab.
- **Never modify tracked files.** No edits under `web/`, `internal/`, `docs/`.
  The only writes allowed in the checkout are the build outputs described
  below, and `git status --porcelain` must be empty when you finish.
- **Never kill a process you did not start.** Record every PID you spawn and
  stop only those.
- Never launch a skill, an execution or an agent console from the UI: that
  spawns `claude`/`codex` on the workstation. Stop at the confirmation step and
  note what the path looked like up to there.

## 1. Sandbox

All state lives in a fresh directory under your scratchpad (or `mktemp -d`),
referred to as `$SANDBOX` below.

1. Build the web bundle and the server from the current checkout:
   - If `web/node_modules` does not exist (worktrees often lack it), symlink
     the main checkout's `web/node_modules`, and remove the symlink afterwards.
   - `cd web && npm run build`, then `touch internal/webui/dist/.gitkeep`
     (the Vite build deletes it and a dirty checkout breaks the workflow).
   - `go build -o "$SANDBOX/server" ./cmd/server`.
2. Pick a free port (`lsof -iTCP -sTCP:LISTEN` to check), e.g. 18090.
3. Start the server **from `$SANDBOX`, with `HOME=$SANDBOX`** so that neither
   the checkout's `.env` nor `~/Library/Application Support/sectile/.env` is
   loaded, and with every tracker variable removed:

   ```bash
   cd "$SANDBOX" && env -i PATH="$PATH" HOME="$SANDBOX" PORT=18090 \
     DB_PATH="$SANDBOX/tasks.db" SECTILE_SECRET_KEY="$(openssl rand -hex 32)" \
     ./server > server.log 2>&1 &
   ```

4. Wait for `GET /api/version` to answer, then seed enough data to exercise
   the screens (a couple of projects, tasks at several stages, labels, a saved
   board view), through the UI itself where possible (creating data is part
   of the journey under review), or through the local API when a state cannot
   be reached from the UI without a tracker.

## 2. Drive the app

Use a real browser. Prefer the built-in browser tools when they are available
to you (`mcp__Claude_Browser__*`: `preview_start` with the sandbox URL,
`read_page`, `find`, `computer`, `resize_window`). Otherwise script Playwright
Chromium from Node, loading it from the main checkout's
`desktop/node_modules/playwright`, and save screenshots to `$SANDBOX/shots/`.

Walk the journeys a user actually has, from a cold start:

- first visit and sign-in, empty states, first project, first task;
- finding a task (search, filters, saved views, board vs list) and acting on
  it (detail drawer, stage transitions, labels, comments, pull requests);
- project settings, personal settings, tracker credentials, admin pages;
- activity/queue monitoring and what happens when something fails;
- the same screens at 1280px, at 768px and at 375px, in light and dark theme;
- keyboard only: tab order, visible focus, Escape closes what it opened;
- every available interface language, at least on the main screens.

For each journey count the clicks and screens, and note every place where you
hesitated, backtracked, or could not tell what a control would do.

## 3. What to look for

- **Inconsistency**: the same concept named differently (labels, buttons,
  statuses, stage names), the same action placed or styled differently across
  screens, mixed languages on one screen (the UI is localised through
  `web/src/locales/`: switch the language in the settings, and report any
  string that stays in the other language or shows a raw key), icons with different
  meanings, divergent date/number formats, dialogs that confirm in one place
  and not another, behaviour that contradicts `docs/UX_COMPONENTS.md`.
- **Bloat**: controls nobody needs at that step, duplicated entry points to
  the same thing, settings that expose internals, walls of explanatory text,
  panels that are always open, modal-on-modal, information repeated on one
  screen.
- **Bad paths**: dead ends, actions that require leaving and coming back,
  destructive actions without confirmation or undo, confirmations for trivial
  actions, lost input on navigation, errors that do not say what to do next,
  states you cannot get out of, features reachable only by URL.
- Accessibility defects that shape the path (unlabeled buttons, no focus
  ring, contrast, click targets under 24px).

Confirm each finding in the code before reporting it: locate the component
(`web/src/...`) so the report carries a `file:line`, and drop anything you
cannot reproduce twice.

## 4. Report

Write `$SANDBOX/ux-audit-web.md` in English (findings may become GitHub
issues) with:

1. a short summary: build under test (`git rev-parse --short HEAD`), date,
   viewports covered, top five problems;
2. one section per finding, ranked by severity (**blocker**, **major**,
   **minor**, **polish**), each with: category (inconsistency / bloat /
   bad path / accessibility), screen, steps to reproduce, observed vs
   expected, screenshot path, `web/src/...:line`, and a one-line suggested
   direction (not a patch);
3. a journey table: journey, steps taken, minimum steps you believe possible;
4. anything you could not test and why (no tracker, no OIDC, etc.).

Quote UI strings exactly as displayed, in their original language.

## 5. Teardown

Stop the server you started, remove any `node_modules` symlink you created,
restore `internal/webui/dist/.gitkeep`, and check that `git status --porcelain`
prints nothing. Return the report path, the top findings, and the sandbox path.
