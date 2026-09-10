# Design — condensed board cards

## Context
React/TypeScript with Tailwind and CSS variables. `TaskCard.tsx` already reads settings and owns the action portal, activity border, tracker link and drag handler. `BoardView.tsx` renders this component in three paths. `AppContext.tsx` applies and persists density, while `index.css` controls existing density scales. No new state or data contract is required.

## Decisions
1. Derive condensed rendering in TaskCard from `settings.density === 'compact'`. Undefined density retains the existing detailed fallback. Reuse the same component and handlers; avoid a second independently maintained action implementation.
2. Render a small header containing the non-truncated public key and Actions button, followed by a single-line title. Use compact padding with no metadata/footer placeholders. Keep the existing activity and dragging border classes. This two-row layout gives the title the column width while preserving its single-line requirement.
3. Use a constrained title container with ellipsis and the complete title as the hover text and accessible name. Provide a native title button for detail opening in condensed mode, with visible focus, so Enter/Space works without turning a container holding links/buttons into a nested button. Keep card body click behavior. Key link, menu and menu actions stop event propagation; title keyboard activation must not cause duplicate opening.
4. Keep the public-key link and its existing external destination. A local ticket without a URL retains plain key text. Hide the parent reference, priority/type badges, description, branch/PR links, labels, assignee, dates and action footer in condensed mode.
5. Reuse the existing portal and menu positioning. In condensed mode, add missing equivalents for pin/unpin, advance one step, advance automatically, parent filter toggle when a parent exists, and opening an existing PR/MR. Reuse existing detail, terminal, diff, editor, clone, copy, sprint-removal, workflow and deletion entries. Parent filtering and PR navigation are included because clarification requires retaining operations exposed by hidden metadata.
6. Advance entries call the existing `handleAdvance(false/true)` and keep the `advancing !== null || isFinishedTask` guard. Skill actions retain their `isSkillRunning` guard and current conditional availability. Pin label reflects current pin state. Preserve existing confirmations, destinations and task arguments. Do not reinterpret any existing workflow or merge behavior.
7. Keep the menu trigger available without hover. Ensure keyboard reachability of controls, Escape dismissal and focus return to the trigger after keyboard dismissal. Maintain clipping avoidance at top/bottom of scrollable columns and at current UI zoom settings.

## Target Files
- `web/src/components/TaskCard.tsx`: conditional presentation, title accessibility and missing compact-menu actions.
- `web/src/components/BoardView.tsx`: verify all three card paths; change only if necessary for shared rendering.
- `web/src/index.css`: only scoped compact card styles if utility classes are insufficient; do not alter global density or column rules.
- `web/src/locales/translations.ts`: use existing translations or add any necessary labels in both supported languages.
Existing settings, Task types and backend APIs remain unchanged.

## Rejected Alternatives
- Separate per-board/per-project toggle: contradicts the adopted existing-density decision and creates new persistence.
- CSS-only hiding: removes access to shortcuts and does not supply keyboard detail navigation.
- Multiline title or retained metadata rows: defeats the agreed predictable compact height.
- Duplicated card/menu implementation: risks diverging operation guards and destinations.

## Verification
Run `npm run lint` and `npm run build` in `web`. There is no frontend test script in the current package; use the acceptance scenarios as a recorded browser verification checklist rather than introducing a new test framework for this UI change. Test a rich ticket, a local minimal ticket, a long title, parent/PR actions, finished/running/queued states, both themes, all density values and the three board paths. Verify operation requests occur once and retain existing arguments. Use controlled test tickets for mutating actions. No production implementation or execution tests are part of this specification-only stage.

## Open Questions
None. No additional service, migration or team dependency identified.
