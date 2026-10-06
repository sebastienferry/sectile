# Plan #778 - Desktop: the Claude settings tab draws the Claude logo

## Stack

Desktop renderer only (`desktop/src`, plain ES modules, no framework). No
agent, server, migration or API change.

## Design

- `desktop/src/claude-mark.mjs` gains `settingsCategoryIcon(document, category)`:
  it returns `claudeMark(document)` when the category has `mark: 'claude'`, and
  otherwise the stroked outline `<svg>` the five sites build today, with
  `category.icon` as its content. It lives next to `claudeMark` so it can be
  unit tested without Electron.
- `SETTINGS_CATEGORIES` and `PROJECT_SETTINGS_CATEGORIES` (`desktop/src/main.js`)
  give their `Sandbox` entry `mark: 'claude'` and drop the shield `icon`.
- The five tab-building sites (`configurationNavigation`, `openSettings` twice,
  the project settings page twice) replace their `innerHTML` string with
  `tab.append(settingsCategoryIcon(document, category))`.
- `desktop/src/style.css`: `--claude-brand: #D97757` on `:root` (shared by both
  themes) and `.claude-mark { color: var(--claude-brand) }`. The mark fills with
  `currentColor`, so the rule colours it without a `fill` attribute and wins
  over the tab's selected and hover colours, which are set on the button.
  `.settings-nav svg` already sizes it to 15 px.

### Rejected alternatives

- A `fill="#D97757"` attribute in `claudeMark()`: it would put the brand colour
  in the markup and differ from the web copy's `currentColor`.
- Inlining the Claude path in the category `icon` string: it would duplicate
  the path outside the test that guards it, and the stroked wrapper would draw
  it as an outlined blob.

## Target files

- `desktop/src/claude-mark.mjs`
- `desktop/src/main.js`
- `desktop/src/style.css`
- `desktop/tests/claude-mark.test.mjs`
- `desktop/tests/conversation-mode.ui.cjs`
- `CHANGELOG.md`
