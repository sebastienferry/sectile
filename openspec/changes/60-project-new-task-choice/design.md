# Design

Reuse the assigned branch feat/60. Add a small project-scoped choice dialog in desktop/src/main.js using the existing dialog and button conventions. Its actions call browseTasks(projectID) and quickAdd(projectID). Give quickAdd an optional project argument defaulting to the current selection so the command palette remains compatible and the project choice is captured before its asynchronous project refresh.

Reuse the current task creation success screen and launch skill picker. Avoid a new API or an automatic create-and-execute operation, which would change the existing explicit launch semantics. No architectural trade-off warrants a separate ADR.

Extend desktop/tests/console.ui.cjs to exercise both options, a project different from the selected execution, dismissal without side effects, and creation followed by launch. Retain command palette coverage. Update README.md.

Validation: openspec validate 60-project-new-task-choice --strict; desktop build and test:ui; web build, lint and tests; Go build, vet and internal tests. Preserve unrelated pre-existing generated skill changes outside commits.

Validation adjustment: parallel Electron test files caused an application window to close during the console regression. Sequential execution passed all four tests. The desktop test:ui command now uses --test-concurrency=1 to avoid competing native app lifecycles. This is a test runner adjustment; production behavior is unchanged.
