## ADDED Requirements

### Requirement: Successful issue operations provide navigation
The WebUI SHALL offer navigation to the affected issue in success notifications from ordinary issue creation, cloning, macro story creation (from a todo or a title, including legacy epic entry points), and ordinary edits including bulk edits. Each action SHALL identify the issue by its key and use the successful operation's returned issue data.

#### Scenario: Creation or edit of an imported issue
- **GIVEN** an imported issue is successfully created, cloned or edited
- **WHEN** its success notification appears
- **THEN** it offers an internal Open action for that issue and a separate tracker icon when its tracker URL exists
- **AND** internal activation opens that issue with the returned data and dismisses the notification
- **AND** the tracker action opens the supplied URL in a separate tab without navigating the application away.

#### Scenario: Local issue and imported issue without a tracker URL
- **GIVEN** a successful operation returns a local issue or an imported issue without a tracker URL
- **WHEN** its success notification appears
- **THEN** the internal issue action works and no empty tracker action is shown.

#### Scenario: Bulk edits and clone auto-opening
- **GIVEN** multiple issues are edited through the ordinary edit flow, or an issue is cloned
- **WHEN** the operations succeed
- **THEN** each existing per-issue success notification targets its own returned issue
- **AND** cloning preserves its existing automatic-opening choice while also offering the notification action.

### Requirement: Tracker-only story navigation
A story that exists only in the tracker SHALL offer a direct tracker link on its success notification. It SHALL NOT offer internal navigation or acquire a fabricated local identity. The link SHALL use the actual story's tracker URL, including when parent assignment produces a separate warning.

#### Scenario: Story created in a declared roadmap project
- **GIVEN** a macro todo targets a tracker-only roadmap project
- **WHEN** story creation succeeds and returns its key and tracker URL without a local identity
- **THEN** the success notification names the story and provides a labeled tracker link opening in a separate tab
- **AND** the story remains absent from the local board and no internal detail action is offered.

#### Scenario: Successful creation with a parent warning
- **GIVEN** a story exists but its parent could not be written
- **WHEN** the WebUI reports the success and its existing warning
- **THEN** the success notification still links the created story and the warning remains unchanged.

### Requirement: Successful macro batches expose created stories
An all-success macro batch's success notification SHALL expose a distinct action for every created story, using the same local/imported/tracker-only rules. Skipped entries SHALL NOT be presented as newly created issues. Existing batch counts, summary text and per-line results SHALL remain intact. Warning and error batch notifications remain outside this change.

#### Scenario: Multiple created stories
- **GIVEN** a macro batch creates several stories with no failures, possibly with skipped entries
- **WHEN** the success summary appears
- **THEN** each created story has its own identifiable navigation action and skipped entries add none
- **AND** internal and external navigation retain their respective dismissal behavior.

#### Scenario: Partial failure or no created stories
- **GIVEN** a batch has failures or creates no stories
- **WHEN** its notification appears
- **THEN** existing warning, credential and error behavior is preserved
- **AND** no fabricated issue link is added for a failed or skipped result.

### Requirement: Preserve notification interactions and scope
Linked notifications SHALL retain the existing eight-second default lifetime and hover/focus pause behavior; explicit durations SHALL remain authoritative. Internal actions SHALL dismiss the toast, external links SHALL retain current tracker-link dismissal behavior, and manual dismissal SHALL remain available. Labels SHALL use existing localization conventions and keyboard-accessible controls. Move/export, macro edits, deletion, warnings, failures and unrelated notifications SHALL remain unchanged. Implementation SHALL add an English user-facing entry under CHANGELOG.md Unreleased.

#### Scenario: Pause and keyboard use
- **GIVEN** a notification offers one or several issue actions, including an external-only action
- **WHEN** the user hovers it or moves keyboard focus among its controls
- **THEN** expiry pauses until all holds are released, resumes with the remaining time and never resets while focus moves inside it
- **AND** its actions have meaningful accessible labels and external links use the existing safe new-tab behavior.

#### Scenario: Unrelated and failed operations
- **GIVEN** an excluded operation or a failed write emits a notification
- **WHEN** the notification renders
- **THEN** its existing content, credential actions, presentation and lifetime remain unchanged.
