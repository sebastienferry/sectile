# #286 — Implementation checklist

Ordered implementation checklist ensuring each layer is verified before building on top of it: graphic asset creation first, packager and runtime wiring second, HTML renderer integration third, and automated verification tests fourth.

---

## 1. Icon Assets Creation (`desktop/assets/`)

- [x] **T1** Create `desktop/assets/icon.svg`:
  - Author vector SVG (viewBox `0 0 36 36` or `0 0 64 64`) with rounded badge background gradient (`#1e1b4b` → `#0f172a` → `#020617`), indigo border, dynamic flow streams (`#6366f1` / `#8b5cf6` / `#06b6d4` / `#38bdf8`), and glowing accent nodes, mirroring `web/public/favicon.svg`.
- [x] **T2** Generate `desktop/assets/icon.png`:
  - Produce master 512×512 RGBA PNG with alpha transparency around rounded badge corners.
- [x] **T3** Generate `desktop/assets/icon.icns`:
  - Produce standard macOS Apple Icon Image containing multi-resolution mipmaps (16×16 through 1024×1024 with @2x Retina scales).
- [x] **T4** Generate `desktop/assets/icon.ico`:
  - Produce standard Windows Icon containing directory entries for 16×16, 24×24, 32×32, 48×48, 64×64, 128×128, and 256×256.

---

## 2. Electron Packager Configuration (`desktop/electron/`)

- [x] **T5** In `desktop/electron/package.cjs`:
  - Add `icon: path.resolve(__dirname, '../assets/icon')` to the `packager({ ... })` configuration options.
  - Verify that the path omits the file extension so `@electron/packager` automatically resolves `.icns` on macOS, `.ico` on Windows, and `.png` on Linux.

---

## 3. Electron Runtime Window & macOS Dock Integration (`desktop/electron/`)

- [x] **T6** In `desktop/electron/main.cjs` (`openWindow`):
  - Pass `icon: path.join(__dirname, '../assets/icon.png')` into `new BrowserWindow({ ... })`.
- [x] **T7** In `desktop/electron/main.cjs` (application initialization):
  - Add platform check `if (process.platform === 'darwin' && app.dock) app.dock.setIcon(path.join(__dirname, '../assets/icon.png'))` to set the macOS Dock icon during development runs.

---

## 4. Desktop HTML Renderer Template (`desktop/`)

- [x] **T8** In `desktop/index.html`:
  - Add `<link rel="icon" type="image/svg+xml" href="./assets/icon.svg">` inside `<head>`.
  - Preserve the text-only header `<strong id="app-title">Sectile Desktop</strong>` in `desktop/src/main.js` without adding an in-app visual icon.

---

## 5. Verification & Quality Gates

- [x] **T9** In `desktop/tests/icon.test.cjs`:
  - Create automated tests verifying:
    - Existence of `desktop/assets/icon.svg`, `icon.png`, `icon.icns`, and `icon.ico`.
    - Binary header validation for PNG (`\x89PNG\r\n\x1a\n`), ICNS (`icns`), and ICO (`\x00\x00\x01\x00`).
    - Presence of `icon` option in `desktop/electron/package.cjs`.
    - Presence of `icon` option in `desktop/electron/main.cjs` for `BrowserWindow`.
    - Presence of `app.dock.setIcon` call for `darwin` platform in `desktop/electron/main.cjs`.
    - Presence of `<link rel="icon">` in `desktop/index.html`.
- [x] **T10** Run full desktop test suite:
  ```bash
  cd desktop && npm test
  ```
- [x] **T11** Verify acceptance criteria in `spec.md` (US1 through US5).
