# Design

Reuse the assigned `feat/101` branch. Render a native anchor inside the existing connection status container, with underlined styling and visible keyboard focus. Update the anchor in place during polling so focus survives refreshes. Remove it when the server disconnects or the local agent stops.

Expose a dedicated argument-free `openBoard` preload action. The Electron main process reads current agent status and validates a connected HTTP(S) server URL without embedded credentials before calling `shell.openExternal`. Preserve the configured base path, remove task selection and fragment, and retain other query parameters, following the existing task-link behavior. Report errors through the existing alert.

Rejected alternatives: renderer navigation would replace the console; reusing `openPR` would accept a renderer-supplied destination and misname the operation; linking to a guessed `/board` route would break the existing root-hosted web UI.

Validation: OpenSpec strict validation, desktop build and runtime tests, Electron UI coverage for mouse/keyboard opening, focus persistence, changed server URL, disconnected/stopped states, invalid URLs, and opening errors. Run the repository build and static/test checks before publication.
