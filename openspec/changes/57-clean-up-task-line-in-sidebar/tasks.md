# Implementation checklist

- [x] 1. Write and validate OpenSpec specification files under `openspec/changes/57-clean-up-task-line-in-sidebar/`.
- [x] 2. Update `desktop/src/main.js` to render the PR icon button inline within `.local-task` with an SVG pull request icon.
- [x] 3. Update `desktop/src/style.css` to style `.pr-indicator` as an inline icon button without block margins.
- [x] 4. Build the desktop app with `npm --prefix desktop run build`.
- [x] 5. Run the desktop UI test suite with `npm --prefix desktop run test:ui` and ensure 100% green.
- [x] 6. Review diff, commit changes, and prepare for pull request.
