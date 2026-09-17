## Implementation
- [x] Replace the `stop` entry of `iconPaths` in `desktop/src/main.js` with a stroked cross.
- [x] Replace the `shutdown` entry of `iconPaths` with a disconnect mark distinct from the cross.
- [x] Verify both controls keep their accessible names, tooltips, tints, sizes, enable and disable behaviour, and that stopping an execution and stopping the agent still work.
- [x] Run `openspec validate 154-desktop-close-button --strict` and the project test target, then review the full branch diff.
