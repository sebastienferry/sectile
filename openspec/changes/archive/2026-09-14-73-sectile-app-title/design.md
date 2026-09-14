# Design

Change the existing title literals in desktop/index.html, desktop/electron/main.cjs and desktop/src/main.js together. Keep the native initial title aligned with the loaded document title. Change the adjacent TF badge to S and the desktop README heading to Sectile Desktop.

The web document and translated app headings already use Sectile and require no edits. A broad desktop rebrand and renaming the packaged application are rejected for this ticket because they extend beyond the app title and may affect platform identity and stored configuration.

Validation uses the existing desktop build and Electron integration suite, plus repository tests and static checks. No new test file is needed for a reversible text correction; existing runtime checks will be used to verify title and header output during validation.
