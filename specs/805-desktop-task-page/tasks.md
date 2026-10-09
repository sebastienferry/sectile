# Tasks #805 - Desktop: Add task / Edit task

Order matters: each step leaves the tree buildable and its tests green. Tests go
with the step they cover. `desktop/src/main.js` is edited in small hunks.

## 1. Editor spike and gate

- [x] T1.1 Add `@milkdown/kit` to `desktop/package.json` (lockfile updated),
      record `du -k desktop/dist/assets` of a `npx vite build` before and after.
- [x] T1.2 Write `desktop/src/markdown-editor.mjs` with the API of the plan:
      baseline comparison, source mode, original text kept while unchanged,
      link click handling, image placeholder, paste sanitizing, keymap.
- [x] T1.3 Add the slash menu, inline toolbar and block handle views, and the
      `.md-editor` styles (light and dark).
- Tests (`desktop/tests/markdown-editor.ui.cjs`, fixtures in
  `desktop/tests/fixtures/markdown/`: a GitHub issue body, a Jira-converted
  description, one with `<details>` and a footnote, one with tables and nested
  task lists, one with a remote and a `data:` image):
  - parse then serialize then parse gives an equal document; `changed()` is
    false right after loading, and `markdown()` returns the original text
    (US3.7, FR4);
  - an edit then its undo reads as unchanged (US3.7);
  - source mode keeps raw HTML and the footnote untouched (US3.6);
  - `/` menu inserts a task list; Mod-b bolds a selection; Mod-k sets a link
    (US3.1-US3.4);
  - pasted HTML with a script inserts no script (US3.9);
  - a click on a link does not navigate, Mod-click calls `openLink` for an
    external link only (US3.8);
  - a remote image renders the placeholder, a `data:` image renders (US3.10);
  - no `securitypolicyviolation` event fires during the suite (FR8).
- Gate: if the CSP or the round-trip tests cannot pass, switch to Tiptap 3 in
  this module only, and say so in the pull request.

## 2. Pure form rules

- [x] T2.1 Write `desktop/src/task-form.mjs` (validation, link set
      operations, `taskChanges`, `assigneeLookup`, `initialProject`,
      `leaveMessage`).
- Tests (`desktop/tests/task-form.test.mjs`): each refusal of US5.2; add,
  remove, move up and down at the edges, current link (US5.1-US5.3); loaded
  links keep `state` and `branch`; `taskChanges` returns only the changed keys,
  nothing for an untouched form, `prLinks: []` when every link is removed,
  `description` only when `descriptionChanged` (US2.5, FR4); title and
  description limits (FR5); lookup per provider (US4); initial project order
  (US1.3).

## 3. Agent routes

- [x] T3.1 Add `taskPageCapability` (`task-page`) to `/desktop/status`.
- [x] T3.2 ~~Add `trackers` to the `/desktop/projects` entries.~~ Not needed:
      `GET /desktop/project?id=` already answers the project configuration,
      whose `server.trackers` the page reads when a project is chosen, so the
      projects list keeps one server call instead of one per project.
- [x] T3.3 Add `GET` and `PUT /desktop/tasks/detail` and
      `GET /desktop/tasks/assignable`, with the project check, the field
      whitelist and the pull request URL validation.
- Tests (Go, next to the `desktopCreateTask` tests, with a fake server):
  capability listed; trackers listed; detail of a task of another project
  refused; update forwards only the given fields with the paired token and
  relays the server status; invalid or credentialed PR URL refused with nothing
  forwarded; unknown keys dropped; assignable relayed with `q`; the server
  answer's `prUrl` is the last link (FR6, FR7).
- [x] T3.4 Document the routes in `docs/contracts/server-agent-v1.md`.

## 4. Desktop IPC

- [x] T4.1 `preload.cjs` and `main.cjs`: `task`, `updateTask`, `assignable`
      IPC gated on `task-page` with the US7.1 message; `createTask` passes
      `trackerID`.
- [x] T4.2 `will-prevent-unload` handler with the native confirmation.
- [x] T4.3 `desktop/tests/fake-agent.cjs`: the capability (switchable off),
      trackers on projects, the three routes, a recorded request log, and
      injectable failures for create and update.

## 5. Task page

- [x] T5.1 Write `desktop/src/task-page.mjs`: creation and edition modes, the
      fields of US1.2 and US2.3, the assignee picker and free text (US4), the
      pull request list (US5), the unsaved mark (US6.1), the save sequences
      and their messages (US1.5-US1.8, US2.5-US2.7), the post-creation strip
      (US1.6), loading and retry (US2.4), the outdated-agent behaviour (US7).
- [x] T5.2 `main.js`: mount the page in the workspace, `confirmLeave()` on
      every navigation of US6.2, `beforeunload`, entry points (Cmd+N, palette,
      project create button, Tickets row **Edit task**, run header **Edit
      task**); remove `quickAdd` and the `.quick-add*` styles.
- Tests (`desktop/tests/task-page.ui.cjs`, against the fake agent):
  - Cmd+N, palette and create button open the creation page; no quick add
    dialog in the DOM (AC1);
  - creation sends `create-task` then one update with assignee and links; each
    post-creation action (AC2); failed creation keeps the fields; failed
    follow-up update leaves them unsaved (AC3);
  - tracker select shown only for a project with several trackers (US1.2);
  - Edit task from the Tickets row and from the run header loads fresh data
    (AC4);
  - title-only change sends only `title`; untouched description not sent
    (AC5);
  - assignee picker on a Jira task sends name and account id; free text on a
    GitHub task; disabled with its hint in creation on Jira (AC9, US4.3);
  - links added, reordered, removed, refused when invalid or duplicate; the
    last marked Current (AC10);
  - leaving with unsaved changes asks, Keep editing keeps them, Discard leaves;
    no question without changes (AC11);
  - agent without `task-page`: outdated message on edit, basic creation works
    (AC12).
- [x] T5.3 Move the existing UI tests that drove the quick add dialog
      (`grep -l "quick-add\|New task" desktop/tests/*.ui.cjs`) to the page.

## 6. Documentation and checks

- [x] T6.1 `docs/USER_GUIDE.md`: replace **Quick add task** with the task page
      (create, edit, Markdown toggle, unsaved changes).
- [x] T6.2 `CHANGELOG.md`, under `## [Unreleased]` / `### Added`: one line for
      the Desktop task page (create and edit, rich description, assignee, pull
      requests) with the pull request number.
- [x] T6.3 Run `go test ./internal/agent/...`, `cd desktop && npm test`,
      `npx vite build`, then `npm run test:ui` (unsandboxed, serially).
