# #286 — Implementation plan

## Stack

- **Desktop Framework**: Electron 44.x (`desktop/electron/main.cjs`, `desktop/electron/preload.cjs`).
- **Packaging Engine**: `@electron/packager` 20.x (`desktop/electron/package.cjs`).
- **Bundler & Frontend**: Vite 8.x (`desktop/index.html`, `desktop/vite.config.mjs`).
- **Graphic Assets**:
  - `desktop/assets/icon.svg`: Scalable Vector Graphics (64×64 viewBox).
  - `desktop/assets/icon.png`: Portable Network Graphics (512×512 master raster).
  - `desktop/assets/icon.icns`: Apple Icon Image format (standard macOS icon bundle from 16×16 to 1024×1024).
  - `desktop/assets/icon.ico`: Windows Icon format (standard multi-resolution bundle from 16×16 to 256×256).
- **Test Runner**: Node.js built-in test runner (`node --test tests/*.test.cjs`).

---

## Architecture decisions

### D1 — Pre-Generated Versioned Icon Assets in `desktop/assets/`

To guarantee deterministic, zero-dependency builds across all platforms (macOS, Linux, Windows, CI/CD runners), binary and vector icon files are pre-generated and checked directly into the repository under `desktop/assets/`:

```
desktop/
├── assets/
│   ├── icon.icns    # macOS application bundle icon
│   ├── icon.ico     # Windows executable / taskbar icon
│   ├── icon.png     # Master 512x512 PNG (Linux, BrowserWindow, Dock)
│   └── icon.svg     # Vector icon (Vite HTML renderer)
├── electron/
│   ├── main.cjs     # BrowserWindow and Dock icon wiring
│   └── package.cjs  # @electron/packager icon configuration
└── index.html       # Favicon link in HTML head
```

#### Artwork Specification
The artwork mirrors the official Sectile stream logo defined in [`web/src/components/SectileLogo.tsx`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23286/web/src/components/SectileLogo.tsx) and [`web/public/favicon.svg`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23286/web/public/favicon.svg):
- **Background Badge**: Rounded rectangle (border radius ~25% of width) with linear gradient (`x1="0%" y1="0%" x2="100%" y2="100%"`):
  - 0%: `#1e1b4b` (deep indigo)
  - 50%: `#0f172a` (slate dark)
  - 100%: `#020617` (night black)
- **Badge Border**: 1px stroke with `#6366f1` at 40% opacity.
- **Dynamic Stream Lines**:
  - Stream 1: Linear gradient `#6366f1` → `#8b5cf6` → `#06b6d4`
  - Stream 2: Linear gradient `#38bdf8` → `#818cf8`
  - Stream 3: Linear gradient `#6366f1` → `#8b5cf6` → `#06b6d4`
- **Glowing Accent Nodes**:
  - Glowing circles positioned along streams using `#a5b4fc`, `#38bdf8`, `#a855f7`, and `#818cf8`.

#### File Formats & Resolutions
1. **`desktop/assets/icon.svg`**:
   - Vector format matching the viewBox (`0 0 36 36` or `0 0 64 64`).
2. **`desktop/assets/icon.png`**:
   - 512×512 high-resolution 32-bit RGBA PNG, rendered crisply with alpha transparency around the rounded badge corners.
3. **`desktop/assets/icon.icns`**:
   - Apple Icon Image containing representations for `icp4` (16×16), `icp5` (32×32), `icp6` (64×64), `ic07` (128×128), `ic08` (256×256), `ic09` (512×512), and `ic10` (1024×1024 / 512@2x).
4. **`desktop/assets/icon.ico`**:
   - Windows icon containing color directory entries for 16×16, 24×24, 32×32, 48×48, 64×64, 128×128, and 256×256 (PNG compressed sub-image for 256×256).

---

### D2 — `@electron/packager` Configuration (`desktop/electron/package.cjs`)

In `desktop/electron/package.cjs`, configure the `icon` option:

```javascript
packager({
  dir: path.resolve(__dirname, '..'),
  name: 'Sectile',
  out: path.resolve(__dirname, '../release'),
  overwrite: true,
  icon: path.resolve(__dirname, '../assets/icon'),
  extraResource: path.resolve(__dirname, '../bin', agentName()),
  ignore: /^\/(bin|tests|release[^/]*)(\/|$)/,
})
```

#### Note on Packager Platform Resolution
When `@electron/packager` receives an `icon` path without an extension:
- On macOS (`darwin`), it automatically appends `.icns`.
- On Windows (`win32`), it automatically appends `.ico`.
- On Linux (`linux`), it automatically appends `.png`.
Supplying the path without extension ensures seamless cross-platform packaging behavior.

---

### D3 — Runtime Window & macOS Dock Integration (`desktop/electron/main.cjs`)

1. **Window Icon Configuration**:
   In `desktop/electron/main.cjs`, update the `BrowserWindow` creation options inside `openWindow()`:
   ```javascript
   const iconPath = path.join(__dirname, '../assets/icon.png')
   window = new BrowserWindow({
     show: process.env.SECTILE_DESKTOP_TEST !== '1',
     width: 1240,
     height: 820,
     minWidth: 800,
     minHeight: 500,
     backgroundColor: '#11151c',
     title: 'Sectile Desktop',
     titleBarStyle: 'hidden',
     titleBarOverlay: { color: '#11151c', symbolColor: '#d8e0ec', height: 68 },
     trafficLightPosition: { x: 18, y: 25 },
     icon: iconPath,
     webPreferences: {
       preload: path.join(__dirname, 'preload.cjs'),
       nodeIntegration: false,
       contextIsolation: true,
       sandbox: true,
     },
   })
   ```

2. **macOS Dock Icon in Development**:
   In `desktop/electron/main.cjs`, invoke `app.dock.setIcon` when the application is ready on macOS:
   ```javascript
   if (process.platform === 'darwin' && app.dock) {
     app.dock.setIcon(path.join(__dirname, '../assets/icon.png'))
   }
   ```
   This ensures that running from source (`npm start` / `electron .`) immediately shows the Sectile badge in the macOS Dock instead of the default Electron icon.

---

### D4 — Vite Renderer Template Configuration (`desktop/index.html`)

In `desktop/index.html`, add the favicon `<link>` tag within the `<head>` section:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="Content-Security-Policy" content="default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'none'">
    <link rel="icon" type="image/svg+xml" href="./assets/icon.svg">
    <title>Sectile Desktop</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.js"></script>
  </body>
</html>
```

---

### D5 — Test Plan (`desktop/tests/icon.test.cjs`)

A dedicated unit test file `desktop/tests/icon.test.cjs` will be executed as part of `npm test`:

1. **Asset Integrity**:
   - Verify `desktop/assets/icon.svg` exists, parses as valid XML/SVG, and defines the Sectile stream gradients.
   - Verify `desktop/assets/icon.png` exists, is at least 1 KB, and starts with the PNG magic byte header `\x89PNG\r\n\x1a\n`.
   - Verify `desktop/assets/icon.icns` exists, is at least 1 KB, and starts with the Apple ICNS magic header `icns`.
   - Verify `desktop/assets/icon.ico` exists, is at least 1 KB, and starts with the Windows ICO header `\x00\x00\x01\x00`.
2. **Packager Configuration Verification**:
   - Inspect `desktop/electron/package.cjs` to assert that `icon` is specified and points to the `assets/icon` path.
3. **Electron Runtime Wiring Verification**:
   - Inspect `desktop/electron/main.cjs` to assert that `icon:` is passed to `BrowserWindow`.
   - Assert that `app.dock.setIcon` is called when running on `darwin`.
4. **Renderer HTML Verification**:
   - Inspect `desktop/index.html` to assert that `<link rel="icon"` is present with `type="image/svg+xml"`.
