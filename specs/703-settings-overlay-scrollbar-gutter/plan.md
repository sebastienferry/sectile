# Plan #703 - Keep settings controls clear of the macOS overlay scrollbar

## Stack

Sectile Desktop renderer only: the stylesheet `desktop/src/style.css` and the
Playwright Electron suite `desktop/tests/workstation-settings.ui.cjs`. No
JavaScript change in `desktop/src/main.js`, no server, agent, web or migration
change.

## Branch base

`feat/703` merged `origin/main` (`7d4f2867`) at specification time. Line
numbers below are those of that merge.

## Where the scrollbar is today

Both settings views are rendered inside the Configuration page
(`showConfiguration`, `desktop/src/main.js:958`), so the
`.configuration-body` rules apply to both:

| View | Built at | Scrolling element | Right clearance today |
| --- | --- | --- | --- |
| Workstation settings | `openSettings`, `main.js:1719`, `.settings-content.stretch` | each `section#settings-panel-<id>` but Logs (`style.css:411`) | 0 px |
| Project settings | `openProject`, `main.js:2099`, plain `.settings-content` | `.settings-content` itself | 24 px (`style.css:230`, `!important`) |

In the workstation view the outer `.settings-content` has 24 px of padding
(`style.css:230`) but does not scroll: it is a flex column whose panel section
scrolls instead (`style.css:409-411`). The overlay scrollbar is drawn at the
section's right edge, where the `.setting-control` rows end
(`justify-content:flex-end`, `style.css:375`), so it covers the reset buttons.
That matches the measurement in the ticket: `elementFromPoint` returns
`#settings-panel-AgentCli` from x+12 of the 25 px reset button.

### Correction to the clarification

The clarification read `.settings-content{padding-right:6px}` (`style.css:374`)
and planned a 16 px gutter on the project view as well. That rule is
overridden by `.configuration-body .settings-content{padding:24px!important}`,
which already clears an overlay scrollbar. The project view therefore gets no
change (FR4); a 16 px rule there would lose to the `!important` anyway and
would only add dead CSS.

## Design

### 1. The gutter (FR1, FR2, FR3, FR5, FR6)

Extend the existing rule at `desktop/src/style.css:411`:

```css
.settings-content.stretch>section:not(#settings-panel-Logs){overflow-y:auto;padding-right:16px}
```

- 16 px: one overlay scrollbar (about 15 px) plus one pixel. Padding on the
  scrolling element is inside its scroll container, so the overlay scrollbar
  is painted over that padding.
- The outer `.settings-content.stretch` keeps its 24 px: it is the distance
  between the panel and the page edge, not a scrollbar gutter, and it does not
  scroll, so nothing stacks on the panel's gutter (FR3).
- `#settings-panel-Logs` is excluded by the selector already (FR5).
- The rule is outside any media query, so the narrow layout keeps it (FR6).
- Comment the rule in English, in the style of the block above it: say the
  padding is for an overlay scrollbar, which takes no layout space, and why
  `scrollbar-gutter` does not do it.

### 2. Rejected alternatives

- `scrollbar-gutter: stable`: reserves space for classic scrollbars only; the
  CSS spec excludes overlay scrollbars, so macOS "Automatic" is not fixed.
- Restyling with `::-webkit-scrollbar` to force a classic scrollbar: out of
  scope, and changes the platform's look.
- Moving `overflow-y:auto` to the outer `.settings-content`: breaks the log
  reader's stretch, the reason the panels scroll individually.
- A positioned click in the test (`position:{x:2,y:...}`): hides the defect
  instead of fixing it.

### 3. Test (acceptance criteria)

In `desktop/tests/workstation-settings.ui.cjs`, test "the skill settings save
through the agent and reset to their defaults" (around line 190):

- Keep the plain `.click()` calls: on macOS they are the end-to-end check.
- Before the first reset click, add a geometry check that holds on every OS.
  Scroll the scrolling ancestor of the reset button to its bottom, then, for
  both "Reset custom project skills win to default" and "Reset installed
  skills source to default", evaluate in the page:

  ```js
  const scroller=button.closest('section[id^="settings-panel-"]')
  const panel=scroller.getBoundingClientRect(),box=button.getBoundingClientRect()
  const contentRight=panel.left+scroller.clientLeft+scroller.clientWidth
  // gap: contentRight-box.right >= 15
  // hit: document.elementFromPoint(box.right-1,box.top+box.height/2) is button or inside it
  ```

  `clientWidth` excludes a classic scrollbar, so the 15 px gap is measured
  against the content edge on every platform; with an overlay scrollbar it is
  the panel's own edge. Assert the gap with a message naming the button and
  the measured value.
- The panel must be scrolled for the check to mean anything: assert
  `scroller.scrollHeight>scroller.clientHeight` first. The test window is the
  one the suite already opens; if the panel happens to fit, scroll is a no-op
  and the gap assertion still holds (FR1 keeps the gutter without a scroll).
  Do not make the test depend on scrolling: drop the `scrollHeight` assertion
  if it is false on CI, and say so in the commit.

Run with the desktop build first (`npx vite build` in `desktop/`, then restore
`internal/webui/dist/.gitkeep` if the build removed it) and outside the sandbox
(Electron cannot read keychain trust settings under it).

### 4. Changelog (FR8)

Under `## [Unreleased]` → `### Fixed` of `CHANGELOG.md`, one line, for users:

> **Desktop settings controls stay clickable next to the macOS scrollbar.**
> In a settings panel that scrolls, such as Execution defaults, the controls
> on the right, the reset buttons first, no longer sit under the overlay
> scrollbar macOS shows while scrolling. (#703)

## Target files

| File | Change |
| --- | --- |
| `desktop/src/style.css` | `padding-right:16px` on the scrolling workstation panels, with a comment |
| `desktop/tests/workstation-settings.ui.cjs` | geometry and hit-test assertions on the two reset buttons |
| `CHANGELOG.md` | one `Fixed` line |

## Risks

- A panel whose content relies on reaching the right edge (a full-width
  textarea, the command preview) loses 16 px of width. Acceptable: the panels
  are capped at 900 px and these controls use `width:100%` of the content box.
- Other UI suites asserting exact widths of settings controls: none found
  (`grep clientWidth desktop/tests/*.cjs` shows no settings test); run the
  suites that open settings (`workstation-settings`, `settings-version`,
  `sidebar-shortcut`) to confirm.
