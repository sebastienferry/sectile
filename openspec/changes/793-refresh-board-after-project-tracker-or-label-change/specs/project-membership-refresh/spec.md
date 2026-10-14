## ADDED Requirements

### Requirement: Refresh tasks and facets when a saved project's trackers or label change
When a project save (`PUT /api/projects/:id`) carries `trackers` or `label` and the updated project belongs to the active view scope (the selected project, the `'all'` scope, or the active saved board view), the web client SHALL re-fetch the tasks (`GET /api/tasks`) and the task facets (`GET /api/tasks/facets`) after refreshing the project list, without requiring a page reload.

#### Scenario: Adding, removing, or reordering trackers on the active project
- **GIVEN** a project is open on the board or list view
- **WHEN** the user saves the project with an updated `trackers` selection or `label`
- **THEN** the client refreshes the project list, re-fetches the tasks and task facets for the active scope, and displays the updated tickets and facets without a page reload.

#### Scenario: Saving a project included in the all-projects or saved board view scope
- **GIVEN** the user is viewing the `'all'` scope or a saved board view whose `projectIds` include the edited project
- **WHEN** the user saves the project with `trackers` or `label` in the payload
- **THEN** the client re-fetches the tasks and task facets for the active scope.

#### Scenario: Saving unrelated project fields or an out-of-scope project
- **GIVEN** a project save payload that carries neither `trackers` nor `label` (such as a sprint update), or a save of a project not in the active single-project or saved-view scope
- **WHEN** `updateProject` completes
- **THEN** the client refreshes the project list without triggering an unnecessary task or facet reload.

### Requirement: Retain multi-project tickets in client project scoping
Client-side project scoping (`tasksInProject`) SHALL consider a task a member of a project whenever `task.projectIds` (or `[task.projectId]` when `projectIds` is absent or empty) contains the project.

#### Scenario: Ticket belonging to two projects in single-project or bookmarked all scope
- **GIVEN** a ticket whose `projectIds` lists two projects `['p1', 'p2']` and whose primary `projectId` is `'p1'`
- **WHEN** `tasksInProject` filters for `'p2'` or for `'all'` with only `'p2'` bookmarked
- **THEN** the ticket is retained in the filtered list.
