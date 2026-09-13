## ADDED Requirements

### Requirement: Current local task template values
Configured local task command templates SHALL support `{prompt}`, `{issueKey}`, `{issueTitle}`, `{issueDesc}`, `{branchName}`, `{repoPath}`, `{tracker}` and `{repo}`.

#### Scenario: Launch or relaunch a task
- **GIVEN** a task with current title, description and display key
- **WHEN** a skill, terminal-routed skill or custom-instructions command launches
- **THEN** task tokens contain current task values and prompt contains the assembled instructions including run metadata
- **AND** branch and path identify the resolved local execution branch and absolute directory, including a worktree when selected.

#### Scenario: Repository metadata fallback
- **GIVEN** effective project settings override global settings when nonempty
- **WHEN** a local command expands metadata
- **THEN** tracker is lowercase task source, otherwise effective tracker, otherwise `github`
- **AND** repo is effective GitHub repository, otherwise the local execution directory basename.

### Requirement: Literal argument preservation
Inserted values SHALL remain shell argument data in unquoted, single-quoted and double-quoted arguments, including repeated and embedded tokens; substitution SHALL NOT recursively expand values.

#### Scenario: Special or empty task text
- **GIVEN** values containing quotes, spaces, newlines, dollar signs, backticks, shell metacharacters, placeholder-looking text or an empty description
- **WHEN** a template launches
- **THEN** the CLI receives those exact values without executing their contents.

### Requirement: Existing configuration and launch behavior
Template inheritance, local overrides, reset, provider defaults and the required `{prompt}` token SHALL remain supported. Unknown tokens SHALL remain literal.

#### Scenario: Save a template
- **GIVEN** an inherited or locally overridden template
- **WHEN** settings are saved or refreshed and another task launches
- **THEN** the template remains unexpanded in settings and values reflect the new task.

#### Scenario: Explicit terminal command or interactive launch
- **GIVEN** an explicit raw terminal command or a terminal request without a skill
- **WHEN** the request launches
- **THEN** existing raw-command or interactive-provider behavior is preserved.

### Requirement: Discoverable placeholders
Desktop command settings and documentation SHALL describe all supported tokens and local path semantics.

#### Scenario: Configure a local command
- **GIVEN** the desktop project settings are open
- **WHEN** the user edits the CLI template
- **THEN** help identifies the required prompt token and the available task and repository tokens.
