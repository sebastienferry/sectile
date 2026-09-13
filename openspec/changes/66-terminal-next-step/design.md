# Design

Reuse the desktop task metadata refresh and launchServerTask bridge. Store workflow metadata by project and task identity; keep selection-independent requests from overwriting another task's footer. Refresh selected metadata on selection and periodically, clearing stale actions on read failures. Re-read task, project and local runs before dispatch; if the recommended skill changes, update the button and require another click. Disable duplicate submissions and any task with queued, preparing or running executions, including historical-console selection.

Map new → clarify, clarified → specify, specified → implement, implemented → create_pr. Resolve normalized workflow labels before internal status aliases, consistently with the existing workflow conventions. Finished status always blocks launching. Do not infer a stage from a run's skill or a PR link.

A flex footer wraps at narrow widths and remains below the resizable terminal. Its status text is announced through a polite live region. Launch success refreshes executions and selects the new console when available. Errors remain visible and permit retry. No new IPC or backend endpoint is necessary. Injecting commands into a possibly completed TTY was rejected because the existing dispatcher owns execution creation and configuration.

Validation uses pure workflow tests and the existing Electron/mock-agent integration harness, plus desktop and web builds, web lint/tests, Go tests and vet. Existing generated skill changes are excluded from commits.
