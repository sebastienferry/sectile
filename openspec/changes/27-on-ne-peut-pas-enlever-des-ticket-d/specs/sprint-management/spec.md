## Purpose

Functional specification and acceptance criteria for unassigning individual tasks or batch tasks from a sprint to the backlog in Sectile UI and API.

## ADDED Requirements

### Requirement: Individual Task Sprint Unassignment from Sprint View and Context Menu
The application SHALL provide a "Retirer du sprint" (or "Déplacer vers le backlog") action accessible from every task card in Sprint View (in compact card mode, compact chip mode, and planning mode) as well as within the `TaskCard` contextual action menu.

#### Scenario: Unassigning a task via TaskCard context menu
- **GIVEN** a task assigned to an active or future sprint (`task.sprint` is not empty)
- **WHEN** the user opens the contextual action menu (`...`) on the task card and clicks "Retirer du sprint"
- **THEN** the system SHALL reset the task's `sprint` attribute to an empty value (`""`)
- **AND** the task SHALL immediately leave the sprint and appear in the backlog list without requiring a manual page refresh
- **AND** a toast notification SHALL confirm that the task was sent back to the backlog.

#### Scenario: Unassigning a task in compact card view
- **GIVEN** the Sprint view displayed in compact mode with "Cartes" layout
- **WHEN** the user clicks the "Retirer du sprint" action button on a task item within a sprint
- **THEN** the sprint association SHALL be reset immediately and the card SHALL disappear from the sprint container.

#### Scenario: Unassigning a task in compact chip view
- **GIVEN** the Sprint view displayed in compact mode with "Chips" layout
- **WHEN** the user clicks the remove icon on a task chip inside a sprint
- **THEN** the task SHALL be unassigned from the sprint and the chip SHALL be removed from the displayed sprint.

---

### Requirement: Batch Task Sprint Unassignment from Sprint View
The application SHALL allow selecting multiple cards within the Sprint view and provide a "Retirer du sprint" / "Renvoyer au backlog" action in the batch action bar.

#### Scenario: Batch unassignment of multiple tasks from a sprint to the backlog
- **GIVEN** multiple tasks checked within one or more sprints in the Sprint view
- **WHEN** the user clicks the "Retirer du sprint" button (or selects "Renvoyer au backlog") in the batch action bar
- **THEN** the system SHALL reset the sprint association (`sprint = ""`) for all selected tasks via `POST /api/projects/{id}/sprint-move`
- **AND** all selected tasks SHALL immediately leave their sprint columns and be counted in the backlog
- **AND** the checkbox selections SHALL be reset.

---

### Requirement: Backend Persistence and Tracker Sync for Sprint Unassignment
The Sectile backend server SHALL persist sprint removal in the local database (SQLite) and queue the corresponding synchronization operation with the configured tracker (GitHub, Linear, or Jira).

#### Scenario: Backend processing of sprint unassignment
- **GIVEN** an HTTP request `POST /api/tasks/{id}/sprint` sent with payload `{"sprintId": "", "sprintName": ""}`
- **WHEN** the server handler processes the request
- **THEN** the SQLite database SHALL update the task row with `sprint = ""`
- **AND** a `TrackerOpSetSprint` activity targeted at "backlog" SHALL be enqueued in the sync activity queue for external tracker updates.
