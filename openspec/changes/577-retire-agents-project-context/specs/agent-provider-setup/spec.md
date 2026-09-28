## MODIFIED Requirements

### Requirement: Checkouts receive no managed agent configuration
Preparing a dispatch SHALL NOT create managed skill files, managed command files, MCP registration files or a managed project-context block inside the repository or any worktree. Normal scaffold cleanup SHALL remove complete managed project-context blocks delimited by either `sectile:project-context` or legacy `taskflow:project-context` markers. It SHALL preserve all instructions outside those blocks and report incomplete managed markers without deleting unrelated instructions. Local state the agent owns under `.taskflow/` remains permitted.

#### Scenario: Clean checkout after dispatch
- **GIVEN** a repository with no Sectile-managed files under version control
- **WHEN** a task is dispatched and the agent runs
- **THEN** the working tree shows no added or modified skill, command, MCP or project-context file

#### Scenario: Retire previously scaffolded copies
- **GIVEN** a checkout carrying managed files from an earlier release, recorded in the local manifest
- **WHEN** a task is dispatched
- **THEN** the copies whose content still matches the recorded managed content are backed up and removed
- **AND** copies the user has edited are left in place and reported as preserved

#### Scenario: Retire both project-context marker spellings
- **GIVEN** an `AGENTS.md` containing one or more complete managed blocks using `sectile` or `taskflow` markers
- **WHEN** normal scaffold cleanup runs
- **THEN** all complete managed blocks are removed
- **AND** instructions outside the blocks remain unchanged
- **AND** an empty `AGENTS.md` is removed

#### Scenario: Incomplete project-context markers
- **GIVEN** an `AGENTS.md` with an unmatched managed marker
- **WHEN** normal scaffold cleanup runs
- **THEN** it reports an incomplete project-context block
- **AND** the file remains unchanged

#### Scenario: Personal files are never touched
- **GIVEN** skill, command or MCP files the user authored at paths Sectile has never managed
- **WHEN** a task is dispatched
- **THEN** those files are unchanged
