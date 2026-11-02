## ADDED Requirements

### Requirement: Project session shortcut
Desktop SHALL show a keyboard-accessible plus action beside every project with a project-specific accessible name. It SHALL open the project-session launch dialog for that project. The existing Project prompt menu entry SHALL open the same dialog. The global add-project action SHALL retain its behavior.

#### Scenario: Choose a project
- **GIVEN** two mapped projects
- **WHEN** the user activates either project's plus with the pointer or keyboard
- **THEN** the dialog identifies that project and offers separate agent-profile and engine selections
- **AND** launching creates a taskless session in its checkout with its existing folder context

#### Scenario: Missing checkout
- **GIVEN** a project has no usable local checkout
- **WHEN** its actions are displayed or its checkout becomes unavailable before saving or launch
- **THEN** the unavailable action or failure is explained and no profile or session is reported as successfully created

### Requirement: Repository profile selection
Desktop SHALL discover repository agent profiles for the selected project and offer an explicit no-profile choice. Profiles SHALL represent agent instructions independently of provider/model configurations. Selection SHALL affect only the new session and SHALL NOT change project defaults.

#### Scenario: Existing and absent profiles
- **GIVEN** a project with valid profiles, or one with none
- **WHEN** the launch dialog opens
- **THEN** it lists available profiles deterministically, or explains that none exist
- **AND** no-profile launch remains available using existing engine behavior

#### Scenario: Discovery failure
- **WHEN** profile discovery fails or encounters an invalid profile
- **THEN** Desktop distinguishes failure from an empty catalogue and explains invalid entries without making them selectable
- **AND** a failed selection cannot silently launch without its requested instructions

### Requirement: Create a repository profile
Desktop SHALL allow creating a profile with a name and nonempty prompt in the selected project's repository. It SHALL validate input, preserve existing profiles, and select a newly created profile only after a successful save. Canceling SHALL create nothing. Creation SHALL NOT automatically commit or push files.

#### Scenario: Successful creation
- **GIVEN** a writable mapped repository
- **WHEN** the user saves a valid unused profile name and prompt
- **THEN** the profile is persisted in that repository and selected in the current launch dialog
- **AND** reopening the dialog discovers it
- **AND** no session starts until the user activates launch

#### Scenario: Invalid or conflicting creation
- **WHEN** the name is invalid, the prompt is blank or oversized, or the destination already exists
- **THEN** saving fails with an actionable message and existing contents remain unchanged
- **AND** the entered draft remains available for correction

#### Scenario: Save failure
- **WHEN** saving encounters a read-only checkout, missing path, or filesystem failure
- **THEN** no successful creation is shown and no partial profile becomes selectable
- **AND** the user can correct or cancel the draft without launching a session

### Requirement: Load instructions for the session
For supported Claude and Codex console and conversation launches, the selected profile SHALL be installed as supplemental agent instructions before user requests are processed. Repository instructions, existing engine settings, permissions and folder context SHALL remain applicable. Profile text SHALL be treated as data rather than shell syntax. The user SHALL enter their request after launch, without an initial-request composer or automatic user turn.

#### Scenario: Console and conversation
- **GIVEN** a selected valid profile and a supported engine
- **WHEN** a console or conversation opens
- **THEN** that session receives the profile instructions and retains normal repository instruction loading
- **AND** the session waits for the user's request
- **AND** another session with a different or absent profile receives only its own selected instructions

#### Scenario: Profile changes before launch
- **GIVEN** a selected profile was removed, became unreadable, or became invalid since discovery
- **WHEN** launch is requested
- **THEN** launch is rejected with an actionable error instead of silently dropping the selection

#### Scenario: Profile changes after launch
- **GIVEN** a session has loaded a profile
- **WHEN** the repository profile subsequently changes
- **THEN** the active session retains its launch-time instructions for later turns and reconnects
- **AND** a new launch reads the current repository contents

#### Scenario: Unsupported engine or launch failure
- **GIVEN** a profile selection and an unsupported provider, custom command template, or incompatible provider version
- **WHEN** launch is requested
- **THEN** Desktop reports why the selected profile cannot be loaded and does not claim a successful profiled session
- **AND** the user may choose a supported engine or explicitly choose no profile

### Requirement: Documentation and compatibility
The implementation SHALL document profile creation, storage, supported launch paths and limitations, and SHALL add a user-facing Added entry under Unreleased in CHANGELOG.md. Runtime copy SHALL follow the existing surface language.

#### Scenario: Existing launch clients
- **WHEN** an existing client launches without a profile selection
- **THEN** the existing taskless engine selection, view preference, and launch behavior remain available
