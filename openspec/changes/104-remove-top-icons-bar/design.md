# Design

Remove the view-switcher JSX from `web/src/components/Header.tsx`, its unused Lucide imports and context bindings, and obsolete comments beside Quick Add. Keep the existing right-side layout and Quick Add handler.

The sidebar already routes to all seven destinations and retains activity and synchronization indicators. Hide-only CSS was rejected because it leaves redundant controls and dependencies in the header. Shared navigation-state changes are unnecessary.

Use the existing web tests, TypeScript build, lint, and project test target. This small presentation deletion does not warrant a new source-shape test; verify the rendered header and existing sidebar/Quick Add interactions where browser tooling permits.

Clarification: the entire duplicate switcher is removed at every viewport size. No unresolved requirements or external dependencies remain. The assigned branch is `feat/104`; existing local skill/config edits are excluded.
