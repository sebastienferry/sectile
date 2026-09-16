# Design — clean up task line in sidebar

## Context
The Sectile desktop application (`desktop/src/main.js` and `desktop/src/style.css`) renders a list of active and recent task executions grouped by project inside an `<aside>` sidebar. Each task entry represents a group of runs for a given task and displays identification, title, execution status, and actions.

Currently, if a task has an associated pull request or merge request URL, a `<button class="pr-indicator">` is appended directly to the project group container (`group.append(pr)`), rather than inside the `.local-task` flex container. This breaks the linear visual flow of tasks and creates an awkward multiline indentation.

## Decisions
1. **Single inline flex row**: Place all elements of a task inside the `.local-task` container:
   - `task-number` button: displays `run.taskKey || run.taskId`, clicks to `api.openTask(run.taskId)`.
   - `run` button: flexes to fill available width, displays the title (`strong`) with ellipsis and the status icon (`span.status`).
   - `pr-indicator` button: if a PR link is present, rendered inline right after the `run` button. Styled as a compact icon button with a Git Pull Request SVG icon (`<svg viewBox="0 0 24 24" ...>`), preserving the accessible name `Open ${prLabel(link)} for ${run.taskKey || run.taskId}` and clicking to `api.openPR(link)`.
   - `task-archive` button: hover/focus action to stop/archive.
   - `task-menu` button: hover/focus action opening the task menu dialog (`...`).
2. **Iconography and styling**:
   - The PR icon uses a standard Git pull request icon with clean SVG paths (`width="14" height="14" viewBox="0 0 24 24"`), colored with accent/pr link styling (`color: #a6c8ff`).
   - The PR indicator has `flex-shrink: 0`, transparent background on default, subtle hover highlight, and no block-level margins.
3. **Preserve accessible names and test compatibility**:
   - `desktop/tests/console.ui.cjs` verifies `page.getByRole('button', {name: 'Open PR #48 for #48', exact: true})`. Maintaining `aria-label="Open " + prLabel(link) + " for " + (run.taskKey || run.taskId)` guarantees seamless test compatibility and accessibility.
   - `page.getByRole('button', {name: 'Stop and archive Local review', exact: true})` is preserved for the archive button.

## Target Files
- `desktop/src/main.js`: restructure row appending, render PR button inside `.local-task` with SVG icon.
- `desktop/src/style.css`: update `.pr-indicator` rules for inline layout, flex alignment, and icon dimensions.

## Rejected Alternatives
- Retaining PR indicator outside the row: rejects user requirement for clean single-line sidebar tasks.
- Text-only PR badge inside the row: takes too much horizontal space in a narrow 280px sidebar, crowding the task title. An icon button with accessible tooltip and aria-label is cleaner and more space-efficient.
- Removing archive button completely: breaks existing automated UI tests and user ability to quickly archive stopped runs.

## Verification
1. Run `npm run build` in `desktop`.
2. Run `npm run test:ui` in `desktop` (`tests/console.ui.cjs` and `tests/server-check.ui.cjs`).
3. Verify all assertions pass 100%.
