## ADDED Requirements

### Requirement: Shared form order and assignment preservation
Desktop SHALL apply the new fields to both creation and editing. Visible fields SHALL follow Project, Tracker where applicable, Title, Issue type, Priority, Description, Pull requests in visual and keyboard order. Assignee SHALL be absent and form saves SHALL NOT write assignment fields. Existing assignments and other assignment surfaces SHALL remain intact.

#### Scenario: Create and reopen
- **GIVEN** a new task form or an existing task opened for editing
- **WHEN** the page renders
- **THEN** its applicable fields follow the required order and no Assignee control appears
- **AND** the description editor and PR list retain their existing functionality.

#### Scenario: Save an assigned task
- **GIVEN** an existing assigned task
- **WHEN** the user changes title, description, priority, type or PR links and saves
- **THEN** its assignment is unchanged and no assignee, account ID or avatar write is sent.

### Requirement: Tracker-specific issue type choices
Desktop SHALL load issue types from the selected creation tracker (including the project's default when no tracker selector is shown) or the edited task's own tracker. It SHALL NOT use a fixed fallback list or configured import filters as type choices. Default SHALL represent the empty type and preserve existing server/tracker defaults. A loaded nonempty type SHALL remain selectable even if discovery omits it. Discovery SHALL NOT dirty the form by itself.

#### Scenario: Successful discovery
- **GIVEN** a tracker that exposes issue types
- **WHEN** its discovery completes
- **THEN** the selector offers Default and its discovered types without duplicate options
- **AND** creation starts with Default and editing starts with the loaded type.

#### Scenario: Custom loaded type
- **GIVEN** an edited task whose loaded type is absent from discovery
- **WHEN** the options refresh or an unrelated field is saved
- **THEN** that type stays selected and selectable and is not overwritten.

#### Scenario: Tracker or project switch
- **GIVEN** creation with a selected type and a pending discovery request
- **WHEN** the project or tracker changes
- **THEN** choices refresh for the new effective tracker and previous requests cannot replace them
- **AND** a selection absent from the new tracker's successful choices returns to Default
- **AND** an edited task always uses its own tracker, not a project's different default.

### Requirement: Discovery states and access
Discovery SHALL operate with the paired user's existing project authorization, without requiring administrative tracker configuration access. A tracker unrelated to the requested project SHALL be refused. Empty results and unsupported provider capabilities SHALL be distinct from request failures. Loading or unavailable discovery SHALL leave Default and the loaded type usable; unrelated edits SHALL remain saveable. Failures SHALL be visible with a retry action and SHALL NOT silently reset form values.

#### Scenario: Authorized member
- **GIVEN** a paired non-administrator authorized for the project and its selected tracker
- **WHEN** Desktop requests types
- **THEN** discovery succeeds subject to the provider's supported capabilities without exposing credentials or administrative configuration.

#### Scenario: Invalid scope
- **GIVEN** an unauthenticated request, inaccessible project, or tracker not selected by the requested project
- **WHEN** discovery is requested
- **THEN** it is refused before provider discovery occurs.

#### Scenario: Empty or unsupported discovery
- **GIVEN** a tracker that returns no types or does not support discovery
- **WHEN** discovery completes
- **THEN** Desktop explains that state without a fabricated type list
- **AND** Default and any loaded type remain usable and unrelated edits remain saveable.

#### Scenario: Failure and retry
- **GIVEN** a network, authorization, or provider discovery failure
- **WHEN** the form displays the failure and the user retries
- **THEN** retry requests the current effective tracker and stale responses are ignored
- **AND** title, description, priority, selected type and PR edits remain intact.

### Requirement: Priority and save contracts
Desktop SHALL offer Urgent, High, Medium and Low using the existing priority values. Creation SHALL default to Medium; editing SHALL load the task's stored priority. Initial creation SHALL send selected issue type and priority. Editing SHALL send only changed fields, including an explicit empty issue type when changing to Default. Type and priority changes SHALL participate in existing dirty-state and discard protection. Existing server/tracker validation and priority mappings SHALL remain authoritative.

#### Scenario: Initial creation
- **GIVEN** selected type and priority on a valid new task
- **WHEN** the user creates it
- **THEN** those values reach the initial server/tracker creation request through the local agent
- **AND** the persisted result becomes the editing baseline.

#### Scenario: Changed and unchanged fields
- **GIVEN** an edited task
- **WHEN** only priority or type changes, including choosing Default
- **THEN** only the changed editable fields are forwarded and an empty type is not lost as omission
- **AND** unchanged fields, assignment and workflow/project fields are not written.

#### Scenario: Failed save and partial creation
- **GIVEN** a save failure or successful creation followed by a failed PR-link save
- **WHEN** Desktop reports the failure
- **THEN** pending values remain available for retry and dirty-state protection remains active
- **AND** retry after successful creation edits that task rather than creating a duplicate.

#### Scenario: Outdated local agent
- **GIVEN** an agent lacking support for the new field writes
- **WHEN** a save would send type or priority
- **THEN** Desktop visibly requests an agent update/restart rather than reporting success after dropping those values
- **AND** pending edits remain intact.

### Requirement: Existing interactions and release documentation
Desktop SHALL retain title/description validation, Markdown editing, PR validation, ordering and current-link semantics, and accessible labels/focus behavior in the surface's existing English language. Implementation SHALL add an English user-facing entry under CHANGELOG.md Unreleased describing the Desktop task form change.

#### Scenario: Regression acceptance
- **GIVEN** task forms in both modes, including a narrow supported window
- **WHEN** the user operates selects, description and PR controls by keyboard and saves or leaves
- **THEN** existing validation, PR ordering, dirty-state prompts and visible focus remain functional without overlap.

#### Scenario: Changelog acceptance
- **GIVEN** the implemented user-visible change
- **WHEN** the implementation is submitted
- **THEN** Unreleased includes a concise English entry naming the Desktop form improvement.
