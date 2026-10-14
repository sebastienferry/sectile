## ADDED Requirements

### Requirement: Skills Describe Task Folders by Role, Not by Project Kind
The generated skills SHALL describe the task's folders by their `$SECTILE_REPOSITORIES` role and SHALL NOT mention a mono-repo or multi-repo project.

#### Scenario: No fragment names a project kind
- **GIVEN** the skill fragments under `internal/skills/fragments/`
- **WHEN** they are searched for `mono-repo`, `monorepo`, `multi-repo` or `multirepo`, ignoring case
- **THEN** no fragment matches

#### Scenario: The implement and pickup skills state the folder roles
- **GIVEN** the rendered implement, pickup and pickup-issues skills
- **WHEN** an agent reads the implementation steps
- **THEN** they say that the folders are listed whenever a task has more than one, that a `context` folder is read-only until `prepare_repository_worktree` returns a worktree whose repository then needs its own pull request in `prUrls`, and that a `local` folder is changed in place with no worktree and no pull request
