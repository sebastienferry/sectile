# Plan #699 - Animate the conversation status indicator

## Stack

Sectile Desktop renderer only: `desktop/src/conversation.js` and the
stylesheet `desktop/src/style.css`, tested by the Playwright suites
`desktop/tests/conversation.ui.cjs` and `desktop/tests/run-folders.ui.cjs`
(they load `dist/index.html`, so `npx vite build` runs first). No server, web
or agent change, no new dependency.

## Design

### 1. A status kind beside the status text (FR4, FR5, FR6, FR8)

`.conversation-status` (`conversation.js`, line 30 markup, line 32 lookup) is
written in six places. Replace each `status.textContent=...` by one local
helper, defined next to the other helpers in the factory:

```js
// kind is working, asking or idle; the stylesheet draws the indicator from it.
function showStatus(text,kind='idle'){status.textContent=text;status.dataset.kind=kind}
```

| Line | Text | Kind |
| --- | --- | --- |
| 142 | notice (while `noticeUntil` runs) | `idle` |
| 142 | `Read-only history` | `idle` |
| 142 | `Claude is asking you a question` | `asking` |
| 142 | `Waiting for your approval` | `asking` |
| 142 | `Claude Code is working…` | `working` |
| 142 | `Ready` | `idle` |
| 160 | folder notice | `idle` |
| 167 | `Stopping the answer…` | `working` |
| 329 | poll error | `idle` |
| 351 | `Running in the shell…`, `Checking MCP servers…`, `Claude Code is working…` | `working` |
| 359 | `Loading conversation…` | `working` |

Line 142 keeps its precedence chain; it becomes a small `if` ladder (or a
pair of ternaries) that picks the text and the kind together, so the two can
never disagree. `onError` belongs to the caller and does not write the
status.

The indicator is a `::before` pseudo-element, not a child node: every write
uses `textContent`, which would wipe a child, and the existing
`toHaveText(...)` assertions keep matching the bare sentence (AC5).
A pseudo-element is not part of the accessible name, so the live region
announces the same text as today (FR8).

### 2. Stylesheet (FR1, FR2, FR3, FR7, FR9)

After `.conversation-status` (`style.css`, line 591):

```css
.conversation-status:is([data-kind=working],[data-kind=asking])::before{content:'';display:inline-block;width:6px;height:6px;margin-right:6px;border-radius:50%;vertical-align:middle;background:currentColor}
.conversation-status[data-kind=working]::before{background:var(--accent);animation:run-state-pulse 2s ease-in-out infinite}
.conversation-status[data-kind=asking]{color:var(--waiting)}
@media(prefers-reduced-motion:reduce){.conversation-status[data-kind=working]::before{animation:none}}
```

- Reuses the `run-state-pulse` keyframes (line 341) for the shared rhythm;
  `transform` applies to the inline-block pseudo-element.
- `--accent` and `--waiting` exist in both the dark (lines 38, 53) and light
  (lines 124, 139) token blocks, so no new token is needed.
- `idle` and a missing attribute draw nothing: the faint text of today.
- The `overflow:hidden;text-overflow:ellipsis` of the status still truncates
  long text; the dot comes first so it is never cut.

### 3. Changelog (FR10)

Under `## [Unreleased]`, a `### Changed` section (created, since only
`### Added` exists today) with one line, for example:

> **The Desktop conversation shows when Claude is working.** A dot pulses
> before the composer status while Claude Code works, and the status turns
> amber with a still dot when Claude waits for your answer or approval. (#699)

## Rejected alternatives

- **A child `<span>` for the dot**: every write would have to rebuild it, and
  `toHaveText` would still pass but the six writes become two-node updates.
- **Deriving the kind from the text in CSS or JS**: couples the look to the
  wording; a reworded status would silently lose its indicator.
- **New keyframes**: the owner chose the run-state rhythm.

## Tests

- `desktop/tests/conversation.ui.cjs`:
  - after the first send (fake agent sets `busy=true`), the status has
    `data-kind="working"` and the `::before` computed `animation-name` is
    `run-state-pulse` (AC1);
  - at the existing `Waiting for your approval` (line 159) and
    `Claude is asking you a question` (line 175) checkpoints, `data-kind` is
    `asking`, the computed `color` equals the `--waiting` value, and the
    `::before` `animation-name` is `none` (AC2);
  - at `Read-only history` (line 211), `data-kind` is `idle` and the
    `::before` `content` is `none` (AC3);
  - `page.emulateMedia({reducedMotion:'reduce'})`, then a working status has
    `animation-name` `none` (AC4).
- `desktop/tests/run-folders.ui.cjs`: `Ready` is `idle`, `Claude Code is
  working…` is `working` (line 73), and a folder notice shown while busy is
  `idle` (FR6).
- Every existing `toHaveText` on the status stays unchanged (AC5).

Read pseudo-element styles with
`page.locator('.conversation-status').evaluate(el=>getComputedStyle(el,'::before').animationName)`.

## Risks

- UI tests need a fresh `npx vite build` and run unsandboxed (keychain), see
  the project memory.
- `vite build` deletes `webui/.gitkeep`; restore it before committing.
