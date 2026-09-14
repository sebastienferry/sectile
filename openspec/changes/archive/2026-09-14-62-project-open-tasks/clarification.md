# Clarification

Add an action on each desktop sidebar project row that opens the project's open server tasks immediately and lets the user pick a task to launch. Web navigation redesign, task creation and execution dispatch changes are outside this change.

Settled assumptions:
- The desktop is the intended surface: its sidebar groups projects with local executions and already offers a task launcher.
- Open tasks use the existing server launchable filter and renderer unfinished-task guard.
- Picking a task uses the existing skill selector and explicit Launch action; opening the list never launches work.
- Search narrows the list; clearing it restores all open tasks. Existing project collapse, configuration and new-task actions remain available.

Read: desktop/src/main.js, desktop/src/style.css, desktop/src/workflow.mjs, desktop/electron/preload.cjs, desktop/electron/main.cjs, desktop/README.md, desktop/tests/pr-display.ui.cjs, desktop/tests/next-step.ui.cjs and web/src/components/Sidebar.tsx. No unresolved product questions or new dependencies.

Workflow reporting resumed successfully through MCP in standalone run `33b97ea5-d830-450d-9974-56db571cec03`. No unresolved clarification decisions remain.
