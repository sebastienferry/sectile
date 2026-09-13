# Design

Reuse browseTasks and the existing project-scoped serverTasks(projectID, query, true) bridge. Add a separate project-row icon so collapse, configuration and new-task behavior remain independent. Keep the action keyboard reachable and visible on focus; show it on devices without hover.

Remove the empty-query early return. Load open tasks on dialog entry and after search, including a cleared query. Use a generation counter and connected-dialog check so stale successes and failures cannot replace newer results or another project's dialog. Keep search available during requests to permit correcting a query; disable obsolete result actions by replacing the list with a loading state.

Retain the existing unfinished guard, server-provided skill choices, explicit Launch button, custom instructions and missing repository guard. Prefer pickup when the server supplies that skill. A failed request exposes retry through the Search button; it does not masquerade as an empty list.

Rejected alternatives: a new endpoint duplicates existing project-scoped task access; a separate popover duplicates launcher controls and focus handling. The existing modal supplies Escape dismissal and focus confinement. No architectural change warrants an ADR.
