## Implementation
- [ ] Move `#next-step` and `#retry-next-step` from `footer#task-status` to `#toolbar`, placed immediately before `#stop`, leaving `#next-step-status` in the footer.
- [ ] Adjust `#toolbar` and `#task-status` styles so the moved buttons align with the existing toolbar controls and the footer keeps its status line.
- [ ] Replace the `#close-dialog` `×` glyph with the shared cross SVG in an `.icon-button`, coloured `#ffb7c3` on a transparent background, keeping `aria-label="Close"` and the SVG `aria-hidden`.
- [ ] Update `desktop/tests/next-step.ui.cjs` and `desktop/tests/workflow.ui.cjs` to the new container, without changing their behavioural assertions.
- [ ] Run `openspec validate 157-desktop-app-ux --strict`, `npx vite build` and the desktop test target, then review the full branch diff.
