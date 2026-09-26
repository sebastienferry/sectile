# Tasks #507 - Dark/Light mode for Sectile Desktop

Ordered checklist. Each group leaves the desktop buildable. Before starting,
`git fetch origin` and integrate `origin/main` (merge, the branch is pushed).

## 1. Main process (FR-1, FR-2, FR-4, FR-8) - `feat(desktop)`

- [x] T1.1 `desktop/electron/appearance.cjs`: `APPEARANCES`,
  `normalizeAppearance`, `windowColors`.
- [x] T1.2 `desktop/tests/appearance.test.cjs`: normalisation of valid,
  missing and unknown values; dark colours equal today's.
- [x] T1.3 `main.cjs`: apply the stored preference before creating the window,
  window colours from `nativeTheme.shouldUseDarkColors`, repaint on
  `nativeTheme` `updated` (overlay outside macOS only).
- [x] T1.4 `set-appearance` IPC with the atomic settings write, and a
  read-only `appearance` IPC; `preload.cjs` exposes both.

## 2. Stylesheet tokens (FR-5, FR-7) - `feat(desktop)`

- [x] T2.1 Replace every colour literal of `style.css` with a role-named token;
  dark set in `:root` with today's values and `color-scheme: dark`.
- [x] T2.2 Light set under `@media (prefers-color-scheme: light)` with
  `color-scheme: light`.
- [x] T2.3 PR state colours in `pullRequests.mjs` become tokens.
- [x] T2.4 `desktop/tests/appearance.test.mjs`: no literal outside the token
  blocks; every used token defined; light redefines every dark token.

## 3. Terminal (FR-6) - `feat(desktop)`

- [x] T3.1 `desktop/src/appearance.mjs`: dark and light terminal themes,
  `terminalTheme`, `APPEARANCE_CHOICES`; tests in `appearance.test.mjs`.
- [x] T3.2 `main.js`: terminal created with the current theme, switched on the
  `prefers-color-scheme` change.

## 4. Settings category (FR-3) - `feat(desktop)`

- [x] T4.1 Appearance category after User profile, segmented System / Dark /
  Light, saved and applied on press.
- [x] T4.2 `settings-version.ui.cjs`: category list updated.
- [x] T4.3 `desktop/tests/appearance.ui.cjs`: default System, Light and Dark
  repaint and persist, emulated light scheme on System, light start from a
  stored choice.

## 5. Documentation and checks (FR-9)

- [x] T5.1 `CHANGELOG.md` `Added` line under `[Unreleased]`;
  `desktop/README.md` if it lists the settings.
- [x] T5.2 `npx vite build`, `npm test`, `npm run test:ui` in `desktop/`;
  restore `internal/webui/dist/.gitkeep` if a web build deleted it.
