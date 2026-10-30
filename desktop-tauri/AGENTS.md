# Contributor instructions

- Write code, comments, tests, commit messages and documentation in English.
- Preserve French runtime UI and diagnostics unless translation is requested.
- This is an independent repository. Do not modify the sibling Sectile repository as a side effect.
- The board reflects remote tracker state. Never create a separate local task source of truth.
- Keep credentials in native code. Do not persist tokens in web storage, logs or source files.
- Keep native commands narrow. Do not expose arbitrary URLs, shell commands or file access to the renderer.
- Use supported provider APIs/CLIs for execution. Do not assume a desktop app exposes its session control API.
- Any future background subprocess must suppress console windows on Windows; document intentionally visible terminals.
- Use feat/, fix/, refactor/, perf/, docs/, test/ or chore/ branches.
- Update README and CHANGELOG for user-visible changes; record substantial tradeoffs in docs/adrs.
- Validate with npm run check, npm test, npm run test:ui, npm run build and npm run test:rust.
