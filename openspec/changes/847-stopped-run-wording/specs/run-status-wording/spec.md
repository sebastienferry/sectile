## ADDED Requirements

### Requirement: Consistent stopped execution outcome
Every web and desktop surface that presents a canceled execution outcome SHALL use stopped wording in its existing language: “Stopped” in English, and “Arrêtée” or “Arrêtées” in French with grammatical agreement. This includes badges, tooltips, notifications, announcements, statistics, skill outcome labels, generated console-unavailable messages, and stop success feedback.

#### Scenario: English execution outcome
- **GIVEN** an execution has ended with the canceled outcome
- **WHEN** its status is presented in English on web or desktop
- **THEN** its status label reads “Stopped” and associated outcome text uses stopped wording
- **AND** the desktop skill outcome reads “Skill stopped” and its shared notification announcement says “was stopped”.

#### Scenario: French execution outcome
- **GIVEN** canceled executions are displayed on a French surface
- **WHEN** the outcome, tooltip, success feedback, or statistics are rendered
- **THEN** the wording uses arrêtée with grammatical agreement, including “Arrêtées” for execution counts
- **AND** the surface retains its current language.

### Requirement: Compatible history and lifecycle
The display rename SHALL preserve existing stop behavior, authorization, history visibility, glyphs and colors. Stored, API and MCP status values SHALL remain canceled. Existing structured historical records SHALL receive the new wording when rendered, without rewriting historical free-text logs. Workflow stages, tracker-defined statuses and unrelated cancellation flows SHALL retain their current wording and behavior.

#### Scenario: Historical execution
- **GIVEN** a historical execution stores canceled and contains an old cancellation sentence in its log
- **WHEN** its history entry is displayed
- **THEN** its structured outcome displays stopped wording
- **AND** its stored status and free-text log remain unchanged.

#### Scenario: Stop completes through existing clients
- **GIVEN** an authorized caller stops an execution through the existing interface
- **WHEN** the stop completes
- **THEN** the same cancellation lifecycle and canceled protocol value apply
- **AND** status visibility, icon and color remain unchanged.

#### Scenario: Unrelated cancellation
- **GIVEN** tracker statuses, OAuth, synchronization, or dismissed dialogs contain cancellation wording
- **WHEN** those surfaces render after this change
- **THEN** their wording and behavior remain unchanged.

### Requirement: Desktop conversation stop action
The desktop action currently named “Stop execution” SHALL be named “Stop conversation”, including its accessible name and tooltip. Its current visibility, enabled conditions, target and behavior SHALL be preserved.

#### Scenario: Stop a desktop conversation
- **GIVEN** the selected desktop execution currently permits the stop action
- **WHEN** the user locates the action visually, by tooltip or through assistive technology
- **THEN** it is named “Stop conversation”
- **AND** activating it requests the same stop for the same selected execution as before.

#### Scenario: Action unavailable
- **GIVEN** the current desktop state does not permit stopping
- **WHEN** the toolbar renders
- **THEN** the renamed action retains the previous hidden or disabled state
- **AND** the separate Stop agent action remains unchanged.

### Requirement: Release communication
The implementation SHALL add an English user-facing entry under CHANGELOG.md Unreleased describing stopped execution wording and the desktop conversation stop action.

#### Scenario: Prepare release notes
- **GIVEN** the implementation is complete
- **WHEN** Unreleased is read
- **THEN** one entry describes the visible wording changes for Sectile users.
