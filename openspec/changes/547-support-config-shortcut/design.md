# Design

Use a capture-phase desktop renderer keydown handler so the supported chord is consumed before xterm processes it. A small pure shortcut helper decides whether to open, consume without reopening, or ignore. It accepts platform, modifier, repeat, modal, and configuration state, making exact-chord behavior testable without Electron.

The handler calls the existing `openSettings('Profile')` action only when configuration is closed. When configuration is open it prevents the event from reaching an input or terminal but leaves the page intact. An unrelated modal leaves the chord untouched. The Settings button advertises the platform-specific shortcut through its tooltip and `aria-keyshortcuts`.

No native menu accelerator is added because the renderer already owns keyboard shortcuts and the full-page configuration state. No global shortcut is registered because the feature applies only to the focused desktop window.
