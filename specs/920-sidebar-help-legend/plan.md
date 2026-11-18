# Implementation plan — #920

## Baseline and boundaries

React 19, TypeScript, Vite, Tauri; existing CSS and Testing Library/Vitest. No new dependency, backend endpoint, native command, persistence key, data migration or execution API. Reuse feat/920 and preserve accepted clarification artifacts. No code is implemented by this specification.

## Verified architecture and contracts

- desktop-tauri/src/App.tsx owns showProjectSidebar, sidebar-scroll, SidebarRunButton, TicketExecutionNavigation and ConversationNavigation.
- Skill rows use `conversation-indicator skill-${skill.kind}`; non-skill rows use live, session, needs-input or idle. Identity comes from row text/tooltips, not distinct skill glyphs.
- desktop-tauri/src/domain/skill-status.ts defines SkillStatusKind: working, waiting, done, verify, failed, stopping, stopped. Its reconciliation remains authoritative and unchanged.
- desktop-tauri/src/App.css owns conversation-indicator and skill-status-mark geometry, colors and pulse/reduced-motion rules. The direct-child conversation selector currently supplies base geometry: ensure legend marks receive equivalent dimensions/border without relying on that parent selector.
- Current labels include Working, Needs your answer, Done, Needs verification, Could not finish, Stopping and Stopped. Keep contextual descriptions distinct from raw process state.

## Proposed components and integration

1. Add desktop-tauri/src/components/SidebarHelpDialog.tsx: presentational dialog with static entries grouped by ticket skill and conversation/fallback context; no Run or Task fetches or mutations. Local visibility belongs to App; closing uses a callback.
2. Add a real button showing ? with accessible name "Sidebar status help" in the sidebar footer outside sidebar-scroll. It must not be nested in a navigation/run button. Keep it visible even with no projects and preserve existing footer content.
3. Mount help independently of selected run/transcript so opening it does not remount those surfaces. Use a labeled modal dialog, initial focus on Close, focus containment, Escape and trigger restoration. A native showModal dialog or an existing equivalent accessible mechanism is acceptable; a bare `dialog open` is not sufficient to guarantee modal focus behavior. Close help on sidebar unmount and avoid restoring focus to a removed trigger.
4. Reuse existing skill-status-mark classes for skill samples and production conversation-indicator classes for conversation samples, adding scoped base sizing for the latter. Preserve pulse keyframes and reduced-motion suppression. Decorative samples are aria-hidden; text supplies meaning.
5. Add scoped dialog/footer styling in App.css: viewport-constrained width/height, wrapping descriptions, scrollable body, persistent reachable Close and visible focus. Use existing theme variables; do not duplicate status color values into a new palette.

## Target files

- desktop-tauri/src/App.tsx: trigger, visibility and integration.
- desktop-tauri/src/components/SidebarHelpDialog.tsx: legend content and accessible lifecycle.
- desktop-tauri/src/App.css: layout and shared symbol selector support.
- desktop-tauri/tests/sidebar-help.ui.test.tsx: UI coverage, following existing UI mocks.
- desktop-tauri/tests/skill-status.ui.test.tsx: reuse existing status fixtures where helpful; preserve reconciler regression coverage.
- docs/USER_GUIDE.md: Tauri sidebar help, required action and completion caveat.
- CHANGELOG.md: one Added entry at implementation.
- desktop-tauri/README.md: update only if feature documentation there is materially affected; no installation/configuration changes.

## Alternatives

A hover-only tooltip cannot provide reliable keyboard access or scrolling for the full inventory. A navigation page would disrupt the selected context. A new status engine, glyph catalogue or Electron changes would exceed the accepted scope. No ADR is needed for this local presentational addition.

## Validation

Run npm run check, npm test, npm run test:ui, npm run build and npm run test:rust from desktop-tauri as required by its AGENTS.md. Add focused tests for observable dialog behavior and integration, not a second implementation of reconciliation. Use a real browser/native UI to verify focus behavior, actual shapes/colors, narrow layout at 320 CSS pixels, long-content overflow, reduced motion and execution continuity; jsdom cannot prove visual fit or native dialog behavior. Record any unavailable environment check accurately.
