# #286 — Change icon for Desktop App

## Context

The Sectile Desktop application (`desktop/`) is an Electron-based companion application that provides local agent connectivity, terminal execution consoles, and workflow controls. Currently, the desktop application packaging configuration (`desktop/electron/package.cjs`), runtime window instantiation (`desktop/electron/main.cjs`), and HTML renderer template (`desktop/index.html`) do not specify custom application icons. As a consequence, packaged application bundles (macOS `.app`, Windows `.exe`, Linux binaries), development window frames, and the macOS Dock display the default Electron framework icon rather than official Sectile branding.

During the clarification phase (recorded in [`docs/clarifications/286.md`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23286/docs/clarifications/286.md)), the project owner approved adopting and adapting the official Sectile stream logo (`web/src/components/SectileLogo.tsx` / `web/public/favicon.svg`) into standard multi-resolution desktop icon assets (`.icns`, `.ico`, `.png`, `.svg`) placed in `desktop/assets/`. The owner confirmed that the icon update must be confined strictly to the operating system application bundle, taskbar/dock, window chrome, and HTML favicon, keeping the in-app desktop header text-only without embedded visual logos.

This specification defines the user-facing behaviour and acceptance criteria. Technical decisions, asset parameters, and file wiring are detailed in [`plan.md`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23286/specs/286-change-icon-for-desktop-app/plan.md), and the ordered implementation checklist is in [`tasks.md`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23286/specs/286-change-icon-for-desktop-app/tasks.md).

---

## Decisions being specified

1. **Official Sectile Stream Logo Artwork**:
   - The desktop application icon is derived from the official Sectile visual identity: a dark gradient rounded badge (`#1e1b4b` → `#0f172a` → `#020617`), subtle indigo border (`#6366f1` at 40% opacity), dynamic violet/cyan stream curves (`#6366f1`, `#8b5cf6`, `#06b6d4`, `#38bdf8`), and glowing accent nodes (`#a5b4fc`, `#38bdf8`, `#a855f7`, `#818cf8`).
   - The design is packaged into multi-resolution formats covering all target desktop operating systems:
     - `icon.icns` for macOS application bundles.
     - `icon.ico` for Windows executables and taskbar integration.
     - `icon.png` for Linux desktop launchers, window chrome, and master runtime.
     - `icon.svg` for web/renderer HTML favicon.

2. **Packaged Application Bundle Icon**:
   - Application bundles produced by `@electron/packager` display the Sectile icon on macOS Finder, Windows File Explorer, and Linux file managers instead of the default Electron logo.

3. **Runtime Window Chrome Icon**:
   - When the desktop application window opens, the window frame and taskbar entry display the Sectile icon across operating systems.

4. **Development macOS Dock Icon**:
   - When launching the application from source in development on macOS (`npm start` or `electron .`), the macOS Dock displays the Sectile icon instead of the Electron framework icon.

5. **Desktop Renderer Favicon**:
   - The Vite HTML entry template (`desktop/index.html`) declares the Sectile icon as its favicon.

6. **In-App Header Scope Boundary**:
   - The desktop application top navigation header bar remains text-only (`<strong id="app-title">Sectile Desktop</strong>`). No image or SVG logo is inserted into the in-app header UI.

---

## User stories

### US1 — Branded application icon for packaged distributions (P1)

**As a** user installing or launching Sectile Desktop on macOS, Windows, or Linux,  
**I want** the application bundle, executable, and system launcher to display the Sectile logo,  
**So that** I can easily identify Sectile among other installed applications.

- **Given** the desktop application package build is executed (`make desktop-package` / `npm run package`) on macOS
- **When** the packaged application bundle (`release/Sectile-darwin-*/Sectile.app`) is inspected
- **Then** the application bundle icon displays the Sectile branded stream logo rather than the default Electron icon.

- **Given** the desktop application package build is executed on Windows
- **When** the packaged executable (`release/Sectile-win32-*/Sectile.exe`) is inspected
- **Then** the executable icon and application properties display the Sectile branded icon.

- **Given** the desktop application package build is executed on Linux
- **When** the packaged application directory (`release/Sectile-linux-*/`) is inspected
- **Then** the Linux package includes the Sectile branded icon.

---

### US2 — Branded window icon at runtime (P1)

**As a** user running Sectile Desktop on Windows or Linux,  
**I want** the window title bar and taskbar entry to display the Sectile icon,  
**So that** Sectile is visually distinguishable in my running application list and window manager.

- **Given** the desktop application is running
- **When** the main application window is created and displayed
- **Then** the operating system window manager displays the Sectile branded icon on the window and in the task switcher.

---

### US3 — Branded macOS Dock icon in development mode (P1)

**As a** developer running the desktop app from source on macOS (`npm start` or `make run`),  
**I want** the macOS Dock to display the Sectile icon rather than the generic Electron icon,  
**So that** the running development instance is branded and easy to locate on the Dock.

- **Given** the desktop application is started from source on macOS (`darwin`)
- **When** the Electron application reaches the ready state and creates the main window
- **Then** the macOS Dock tile icon is updated to the Sectile branded icon.

- **Given** the desktop application is started on a non-macOS platform (e.g. Linux or Windows)
- **When** the application starts up
- **Then** the macOS Dock setup call is safely bypassed without throwing an exception or crashing the runtime.

---

### US4 — Branded favicon in desktop renderer HTML (P2)

**As a** developer or user inspecting the desktop web renderer or running in a browser/dev environment,  
**I want** the HTML document to reference the Sectile icon as a favicon,  
**So that** the page metadata is complete and properly branded.

- **Given** the desktop application `index.html` is loaded in the Electron renderer or browser
- **When** the document `<head>` is parsed
- **Then** a `<link rel="icon">` element is present referencing the Sectile icon asset.

---

### US5 — Preservation of text-only in-app header (P2)

**As a** user interacting with the desktop application UI,  
**I want** the top header to maintain its clean, text-only layout without unexpected layout shifts,  
**So that** the interface remains compact and consistent with current designs.

- **Given** the main application window is displayed
- **When** the header element (`#app-title`) is rendered
- **Then** it continues to render the text "Sectile Desktop" without prepending or appending an in-app icon image.
