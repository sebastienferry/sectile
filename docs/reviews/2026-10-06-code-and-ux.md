# Code and UX review — 2026-10-06

## Scope

This review prioritizes loss of user input, cross-project state, asynchronous
responses and misleading failure states. It examines the web creation,
comments and skill-editing flows, samples Desktop actions and agent process
launches, and runs the Go, web and Desktop unit suites. The follow-up extends this to server lifecycle, account administration and
agent launch acknowledgement, plus a Desktop visual sweep. It is not a complete
security audit or a live tracker integration test.

## Corrected findings

| Priority | Trigger and former behavior | Correction |
| --- | --- | --- |
| High | Switching projects in the skill editor retains the selected skill ID, so the new project's entries can appear beside the old project's draft. Saving could write that text into the new project. | Remount editor state by project and ignore reads after unmount. No old content remains editable while a new project loads. |
| Medium | Changing a skill's execution mode replaces an unsaved draft with the server's saved content. Selecting another skill also drops the draft. | Preserve drafts per skill within the current editor and leave content untouched by mode-only updates. Disable editing and selection while a write is pending. |
| Medium | Refreshing project data while Quick Add is open resets the whole form. Closing and reopening during creation lets an old response affect a new form. | Snapshot defaults only on opening; prevent dismissal and edits during creation, including the global Escape shortcut. A failed creation retains the draft. |
| Medium | Switching tickets while comments load can display the previous ticket's response and retain its draft on the new ticket. | Scope comment state to the ticket and reject superseded reads. |
| Medium | A successful comment publication clears text typed during the request; an older read can then overwrite the returned comments. | Clear only the submitted draft if unchanged; invalidate reads when posting starts. |
| Medium | A failed comments read returns an empty list and replaces previously visible comments with an empty-conversation message. | Return a distinct failure result, retain the last successful list, and display an inline error with the existing refresh action. |

The regression fixture uses real React components and controlled asynchronous
responses, including out-of-order completion. The existing Quick Add browser
suite additionally checks wide and narrow layouts and the full AppContext flow.

## Backend follow-up

The backend pass inspected launch acknowledgement and connection-bound responses,
run ownership, queue admission and shutdown, account roles and sessions, credential
unlock revocation, OAuth state consumption, and workspace archive guards.

| Priority | Confirmed defect | Correction and evidence |
| --- | --- | --- |
| High | Two concurrent demotions, blocks or deletions could both pass the last-admin count check and leave no active admin. | Count and mutation now share the existing singleton transaction lock across stores. Tests reproduce all three races with two database handles. Bootstrap uses the same lock. |
| High | A role value such as ` member ` passed validation but bypassed the last-admin comparison before being normalized on write. | Normalize before authorization checks; regression fails before the change. |
| Medium | An already blocked admin could not be demoted or deleted when only one other active admin remained. | Only the removal of an active admin can reduce the protected count. |
| High | A disconnect after delivery of a launch was treated as a definitive failure, closing a run that the workstation could still be executing. | Classify it as `ErrLaunchUnconfirmed`, already preserved by the cluster relay. An HTTP/socket regression verifies that the run remains running and accepts its owner's eventual completion. |
| Medium | Closing a store left its queue consumer blocked forever, retaining the store. | Stop admission, wait for admitted senders, close and join the queue consumer, then drain work and close the connection. Concurrent close calls share one result. |

Blocking an account also now commits its blocked flag, session revocations and
credential-unlock deletion in one transaction. The queue shutdown retains the
existing five-second bound on in-flight work; it is not durable job recovery or
cancellation of every long-running task.

The full race-enabled run exposed an independent data race in the
`configurationTask` test fixture: background postbacks shared an ordinary lookup
counter. It now uses an atomic, and assertions about synchronous lookups run
before transitions can enqueue independent postbacks.

## Desktop visual follow-up

The review launches the actual Electron renderer with an isolated profile and a
local fixture agent. It combines visual inspection of screenshots with the full
Desktop interaction suite. Settings are captured in dark mode at 1240 × 820 and
light mode at the 800 × 650 minimum window size, including the bottoms of scrollable
panels. The fixture asserts the requested color scheme and checks horizontal
overflow and renderer errors. Screenshots are evidence for manual review, not
pixel comparison baselines.

| Surface | Coverage |
| --- | --- |
| Workstation settings | Profile, appearance, connection, execution defaults, AI engines and their editor, Claude settings, deployment, logs, changelog. |
| Project settings | General/removal, folders, execution, Claude settings; save footer and long-page scrolling. |
| Execution workspace | Terminal, autonomous activity, conversation and composer, task header, history, PR state, next-step actions, free console, file changes and rendered Markdown. |
| Navigation and actions | Sidebar, project menus, ticket list, creation chooser, command palette, keyboard focus, tooltips and archive action. |
| Failure and recovery | Missing pairing, rejected credentials, disconnected agent, failed reads and launches, unsupported capabilities and superseded responses through the interaction suite. |

### Corrected visual defects

- **Model-list alignment:** the stacked parent section also applied its block
  layout to nested setting rows, putting reset buttons below their inputs. Scope
  that rule to direct children so nested rows keep their normal alignment.

- **Narrow Changes view:** two side-by-side panels left too little space to read
  a diff when the sidebar was visible. Below 680 pixels of execution-area width,
  the console and changes now stack vertically. The divider follows that axis
  for pointer and keyboard resizing, exposes the matching accessible orientation,
  and returns to side-by-side when space becomes available. The console stays
  attached throughout the transition.
- **Incomplete conversation labels:** conversations without a skill name showed
  `undefined` in the sidebar and a trailing separator in the header. The sidebar
  now uses the existing Claude Code fallback; the header omits empty segments.

The ticket-list test also contained an obsolete archive label and fixture API.
It now exercises the current archive capability and checks the project and task
sent to the archive endpoint; the production archive behavior was not weakened.

### Design observations

The settings panels fit the tested widths without horizontal page scrolling.
Long sandbox and deployment pages need vertical scrolling; their controls remain
reachable, and project saves remain in the footer. Engine names wrap in narrow
rows. Empty logs and unavailable configuration have explicit messages.

Some improvements would benefit from a separate product pass: project sandbox policy
controls use a different label alignment from the execution rows; deployment
and sandbox explanations are dense. These are consistency and discoverability
issues, not confirmed data-loss or blocked-action defects. The review preserves
the current information architecture rather than redesigning those workflows.

The fixture's unsupported-provider preview and deliberately failed tool/socket
messages are test states, not evidence of failures in a configured installation.
This pass does not establish Windows/Linux rendering parity, screen-reader
usability, or behavior with real engine and tracker accounts.

## Structural follow-ups

- `web/src/context/AppContext.tsx` combines requests, application state and
  keyboard routing in roughly 4,500 lines. `desktop/src/main.js` combines
  roughly 3,600 lines of UI and application coordination. Extract cohesive
  features incrementally with behavioral tests; file size alone does not
  establish a runtime defect and does not justify a wholesale rewrite.
- The production web build emits a roughly 1.45 MB JavaScript bundle
  (367 KB gzip). Evaluate route-level lazy loading with cold-start measurements
  before claiming a performance benefit. This review has not measured real
  device startup latency.
- Browser tests are separate from `npm test` in `web/package.json` and Desktop
  UI tests are separate from its unit suite. The new regression must be run
  explicitly. A dedicated browser CI job would make these failures harder to
  miss; it needs browser provisioning and a deliberate suite/runtime budget.
- The lint command succeeds but reports warnings, including effect dependencies
  and state changes in effects. These need behavioral triage; automatically
  adding provider functions to dependencies can introduce repeated requests.
- Draft preservation is scoped to the mounted editor, not durable browser
  storage. Closing the editor or switching projects does not promise draft
  recovery. Navigation-wide unsaved-change handling remains a broader UX decision.

## Validation

- `go test ./...`: passed. The full `go test -race ./...` run passed every
  package except the independent DB fixture race described above; after its fix,
  `go test -race ./internal/db -count=1` passed. No external PostgreSQL or live
  tracker environment was configured.
- Web unit suite: 695 passed.
- Desktop unit suite: 243 passed after the final build.
- Full Electron interaction suite: 105/106 passed. The remaining ticket-list
  test needed the archive fixture update and an awaited asynchronous assertion;
  its corrected rerun passed. A focused rerun covers the final renderer changes
  (appearance, conversation, Changes, workstation settings and visual sweep):
  all 12 passed.
- Web TypeScript and production build: passed, with the bundle-size warning.
- Desktop production build: passed.
- Web lint: completed successfully with warnings.
- `web/tests/draft-safety.browser.mjs`: passed in Chrome.
- `web/tests/quick-add.browser.mjs`: passed in Chrome.

Browser test invocation from the repository root, using the Desktop installation
of Playwright:

```sh
PLAYWRIGHT_MODULE="$PWD/desktop/node_modules/playwright/index.mjs" \
  node web/tests/draft-safety.browser.mjs
PLAYWRIGHT_MODULE="$PWD/desktop/node_modules/playwright/index.mjs" \
  node web/tests/quick-add.browser.mjs
```

To reproduce the Desktop sweep:

```sh
npm run build --prefix desktop
npm run test:ui --prefix desktop
SECTILE_VISUAL_REVIEW_DIR=/tmp/sectile-desktop-visual-review \
  node --test desktop/tests/visual-layout.ui.cjs
```

The isolated worktree reuses the existing local Node dependencies. These runs
are not a fresh lockfile installation or a cross-platform release build.
