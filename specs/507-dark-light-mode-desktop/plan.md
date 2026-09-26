# Implementation Plan: Dark/Light mode for Sectile Desktop

**Spec**: [spec.md](spec.md) | **Branch**: `feat/507`

## Stack

Desktop only: Electron main process (`desktop/electron/*.cjs`), vanilla JS
renderer built by Vite (`desktop/src`), `@xterm/xterm` 6. No server, database,
tracker or web change.

## Architecture

```
settings.json ──appearance──▶ main process ──nativeTheme.themeSource──▶ renderer
                                  │                                     │
                    backgroundColor, titleBarOverlay        prefers-color-scheme
                    (at creation and on 'updated')          ├─ style.css tokens
                                                            └─ xterm theme
```

### 1. The preference and the window colours (main process)

New module `desktop/electron/appearance.cjs`, pure and unit-tested:

- `APPEARANCES = ['system', 'dark', 'light']`;
- `normalizeAppearance(value)` returns the value when it is one of them,
  `'system'` otherwise;
- `windowColors(dark)` returns `{background, symbol}`: `#11151c` / `#d8e0ec`
  for dark (today's values), `#f5f6f8` / `#111315` for light (the web light
  palette's primary background and text).

`desktop/electron/main.cjs`:

- `applyAppearance(value)` sets `nativeTheme.themeSource` to the normalised
  value. Before `openWindow()` creates the `BrowserWindow`, the stored
  preference is applied so `nativeTheme.shouldUseDarkColors` is already right,
  and the window is created with `backgroundColor` and `titleBarOverlay` from
  `windowColors(nativeTheme.shouldUseDarkColors)`.
- `nativeTheme.on('updated')` repaints the open window:
  `setBackgroundColor`, and `setTitleBarOverlay` outside macOS (the method
  only exists on Windows and Linux; macOS keeps its traffic lights).
- New IPC `set-appearance`: normalises the value, merges it into
  `settings.json` through the same atomic write as `save-settings`, applies it
  and returns the stored value. `settings` already returns the whole file, so
  the renderer reads the current value from it.
- `preload.cjs` exposes `setAppearance(value)`.

`nativeTheme.themeSource` drives `prefers-color-scheme` in the renderer and the
native widgets, so the renderer never needs to be told the resolved mode.

### 2. Colour tokens in the stylesheet

`desktop/src/style.css` holds about 80 distinct colour literals. Each becomes
a custom property named by role (`--bg`, `--surface`, `--border`, `--text`,
`--text-muted`, `--accent`, `--danger`, `--diff-add-bg`, ...), used as
`var(--token)` in the rules:

- `:root { color-scheme: dark; ... }` defines the dark set with today's
  values, literal for literal, so dark mode is unchanged (FR-7);
- `@media (prefers-color-scheme: light) { :root { color-scheme: light; ... } }`
  redefines every token for light;
- the two token blocks sit at the top of the file and are the only place a
  colour literal may appear. The rules that were already variable-based
  (`var(--muted,#8892a4)`) are folded into the tokens.

Light values reuse the web light palette (`web/src/index.css`, `.light`):
`#f5f6f8` background, `#ffffff` surfaces, `#eceef1` raised surfaces, `#e1e4e8`
borders, `#111315` text, `#48515d` secondary text, `#626b77` muted text. The
teal accent `#62d3be` is too light on white and becomes `#0f766e`, with white
text on accent buttons; status tones are darkened the same way (red
`#b42318`, amber `#a35f00`, blue `#0049a8`, green `#145c3f`, purple
`#7e3bb5`).

The PR state colours are inline styles set by `desktop/src/pullRequests.mjs`;
they become `var(--pr-open)`, `var(--pr-merged)`, ... so they follow the mode
through the same tokens.

### 3. The terminal

New module `desktop/src/appearance.mjs`:

- `TERMINAL_THEMES = {dark, light}`, each with `background`, `foreground`,
  `cursor`, `cursorAccent`, `selectionBackground` and the 16 ANSI keys
  (`black` ... `brightWhite`). The dark theme keeps `#11151c` / `#d8e0ec`.
- `terminalTheme(dark)` returns one of them.
- `APPEARANCE_CHOICES` lists `{value, label}` for the segmented control.

`desktop/src/main.js` creates the terminal with
`terminalTheme(darkQuery.matches)`, where
`darkQuery = matchMedia('(prefers-color-scheme: dark)')`, and on its `change`
event assigns `terminal.options.theme`.

### 4. The Appearance settings category

`SETTINGS_CATEGORIES` gains `{id: 'Appearance', label: 'Appearance'}` right
after User profile. Its panel holds one `settingRow('Theme', ...)` with a
`.segmented` group (`role=group`, `aria-label="Appearance"`) of three buttons
System / Dark / Light; `aria-pressed` marks the current value, read from
`api.settings()`. Pressing a button calls `api.setAppearance(value)` and
updates `aria-pressed` from the returned value; an error goes through the
dialog's usual `error()` path.

## Data contracts

- `settings.json`: new optional key `appearance: "system" | "dark" | "light"`.
- IPC `set-appearance(value: string) -> "system" | "dark" | "light"`.

## Target files

| File | Change |
| --- | --- |
| `desktop/electron/appearance.cjs` | new: normalisation and window colours |
| `desktop/electron/main.cjs` | apply at start, repaint on `updated`, `set-appearance` IPC |
| `desktop/electron/preload.cjs` | expose `setAppearance` |
| `desktop/src/appearance.mjs` | new: terminal themes, choices |
| `desktop/src/main.js` | terminal theme, live switch, Appearance category |
| `desktop/src/style.css` | colour tokens, dark and light sets |
| `desktop/src/pullRequests.mjs` | PR colours as tokens |
| `desktop/tests/appearance.test.cjs` | new: main-process helpers |
| `desktop/tests/appearance.test.mjs` | new: terminal themes, stylesheet tokens |
| `desktop/tests/appearance.ui.cjs` | new: settings, live switch, persistence |
| `desktop/tests/settings-version.ui.cjs` | category list gains Appearance |
| `desktop/README.md` | mention the setting if the README lists settings |
| `CHANGELOG.md` | `Added` line |

## Rejected alternatives

- **A `.light` class on `<html>` toggled by the renderer**, as the web does:
  it would leave native widgets and the title bar overlay on the OS
  appearance, and the renderer would have to learn the OS appearance itself.
  `nativeTheme.themeSource` gives all three from one switch.
- **`localStorage` for the preference**: the main process needs the value
  before the window exists to avoid a dark flash on a light start.
- **Duplicating every rule under a light selector**: two copies of 240 lines
  of CSS drift; tokens keep one set of rules.

## Test plan

- Unit (`node --test`): `normalizeAppearance`, `windowColors`; both terminal
  themes define the 16 ANSI colours and the dark one keeps today's colours;
  the stylesheet has no colour literal outside the two token blocks, every
  `var(--x)` it uses is defined in the dark set, and the light set redefines
  every dark token.
- UI (Playwright on Electron, after `npx vite build`): the Appearance category
  shows System pressed by default; Light repaints the body and the terminal
  and saves `appearance: "light"`; Dark repaints back; System with an emulated
  light scheme is light; a restart with `appearance: "light"` stored creates a
  light window.
- Existing suites: `npm test` and `npm run test:ui` in `desktop/`.
