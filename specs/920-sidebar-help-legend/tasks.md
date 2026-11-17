# Implementation tasks — #920

All items remain pending; this invocation produces specification only.

1. [ ] Recheck the accepted clarification and current sidebar/status inventory against spec.md. Preserve the assigned branch, clarification and existing code; report any newly conflicting semantics before changing scope.
2. [ ] Add the persistent footer help button and locally controlled accessible dialog. Keep it outside sidebar-scroll and independent of selected conversation and execution lifecycle.
3. [ ] Add the seven skill and four conversation/fallback entries using production symbol classes. Explain identity versus state, session versus work, required response/checks, unknown reasons and skill completion versus ticket completion.
4. [ ] Implement title/accessible name, initial focus, Tab/Shift+Tab containment, explicit Close, Escape and focus restoration, including sidebar disappearance. Keep decorative symbols out of the accessibility tree.
5. [ ] Apply viewport-bounded wrapping/scrolling layout and existing reduced-motion behavior. Ensure footer access with an empty list and a long scrolling list.
6. [ ] Add focused UI tests: keyboard/click opening; all inventory meanings; Close/Escape; focus behavior; no selected-run/draft changes or execution mutation; empty/long lists; sidebar unmount. Reuse existing status fixtures without altering reconciliation expectations.
7. [ ] Perform browser/native verification: compare all symbols with production rows, normal/reduced motion, 320 CSS pixel viewport, overflowing content, keyboard traversal and restoration, active execution continuing while help is open. Record evidence and limitations.
8. [ ] Update docs/USER_GUIDE.md with access, contextual status explanations and completion caveat; add one Added entry under CHANGELOG.md [Unreleased]. Update other feature documentation only where affected.
9. [ ] Run the desktop-tauri required checks: npm run check, npm test, npm run test:ui, npm run build and npm run test:rust. Resolve failures attributable to the change and record external blockers truthfully.
10. [ ] At implemented stage only, follow configured creation-owner checks, commit/push policy and PR discovery/creation policy. Reuse an existing PR if present, preserve attained stage and never advance to reviewed as part of creation recovery. No PR is created during specification.

## Specification verification

- [x] Applied accepted clarification round 2; no open product decisions.
- [x] Rechecked all seven skill states and four conversation/fallback classes against App.tsx, App.css and skill-status.ts.
- [x] Kept observable behavior separate from implementation choices.
- [x] Defined target files and implementation validation.
- [x] Verified specification files are not ignored; deliver them in a local specification commit before reporting specified.
