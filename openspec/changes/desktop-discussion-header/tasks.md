# Tasks

## 1. Header layout
- [x] 1.1 Split the toolbar into `.toolbar-identity` and `.toolbar-actions` (`desktop/src/main.js:25`), with the title line, the run state, the skill result and the native-terminal badge in the identity block.
- [x] 1.2 Move `.execution-views` into the action row and drop its dedicated row and bottom border in `desktop/src/style.css`.
- [x] 1.3 Keep `#save-log` → `#stop` → `#next-step` adjacent, so the closing/continuing order of change 230 survives.

## 2. Run state beside the title
- [x] 2.1 Add `#run-state` to the title line and render it from `runStateOf` / `runStateLabel` / `runStateSvg`, glyph plus label.
- [x] 2.2 Hide it when no execution is selected.

## 3. Copyable worktree path
- [x] 3.1 Replace the `#directory` caption with a `#worktree` button wrapping the folder glyph and the `#directory` text.
- [x] 3.2 Route every directory write through `showDirectory`, which also hides the control when there is no path.
- [x] 3.3 Add the `copy-text` IPC (`desktop/electron/main.cjs`) and `copyText` (`desktop/electron/preload.cjs`), rejecting anything but a non-empty string.
- [x] 3.4 Show a `Copied` confirmation for two seconds through a `role="status"` element, cleared when the selection changes.
- [x] 3.5 Keep the path selectable (`user-select:text`) and give the control a visible focus outline.

## 4. Icon controls
- [x] 4.1 Add `rerun`, `save-log`, `view-console` and `view-changes` glyphs to `iconPaths`, each distinct from the agent controls already in the window header.
- [x] 4.2 Give each icon control its former wording as `aria-label` and `title`.
- [x] 4.3 Render `#selected-pr` as the shared pull-request glyph followed by its number.
- [x] 4.4 Leave `Next: <skill>`, `Mark reviewed`, `Retry` and `Launch anyway` labelled.

## 5. Skill result against the run state
- [x] 5.1 Return no result from `skillResult` (`desktop/src/skill-result.mjs`) for a run still in flight, for a run that failed or was cancelled without a server verdict, and for a free console.
- [x] 5.2 Keep the requested-stop report, and withdraw it once the run reaches a terminal status.
- [x] 5.3 Keep `Execution ended · skill completion unconfirmed`: the gap between a clean exit and a missing server record is the one thing the run state does not say.
- [x] 5.4 Clear a task-row badge in `renderTaskSkillStatuses` when there is no result, instead of leaving the previous glyph in place.

## 6. Tests
- [x] 6.1 Add `desktop/tests/discussion-header.ui.cjs` covering identity and state, the clipboard copy and its confirmation, the accessible names of the icon controls, the preserved pull-request number, and every control staying inside an 800px window and keyboard-reachable.
- [x] 6.2 Update `desktop/tests/skill-result.ui.cjs`, `task-header.ui.cjs`, `free-console.ui.cjs` and `run-result-missing.ui.cjs`, which asserted the echoed labels.
- [x] 6.3 Run `npx vite build`, then `npm run test:ui` and `npm test`.
