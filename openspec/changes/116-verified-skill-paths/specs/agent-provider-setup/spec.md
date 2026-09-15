## MODIFIED Requirements

### Requirement: Skills install for the agents the project sets up
The agent SHALL install the managed skill set into the user-level configuration folder each
agent actually reads, and into no other location. An agent with no supported skill convention
SHALL receive no managed skill files, and the dispatch SHALL proceed. Each skill SHALL produce
exactly one installed file, so that no agent resolves one command from two sources.

#### Scenario: Provider with a skill convention
- **GIVEN** a project whose selected AI provider supports managed skills
- **WHEN** a task is dispatched
- **THEN** the managed skills exist under that provider's documented user-level skill folder
- **AND** no managed skill file is written under any other provider's location

#### Scenario: One file per skill
- **GIVEN** any agent that receives managed skills
- **WHEN** a task is dispatched
- **THEN** each skill is installed as a single `SKILL.md` under its own directory
- **AND** no second file declares the same command for that agent

#### Scenario: Provider without a skill convention
- **GIVEN** a project whose selected AI provider has no supported skill convention
- **WHEN** a task is dispatched
- **THEN** no managed skill file is written
- **AND** the dispatch proceeds and the agent is launched

#### Scenario: Selected provider changes
- **GIVEN** managed skills previously installed for one provider
- **WHEN** the project's selected provider changes and a task is dispatched
- **THEN** the new provider's location holds the current managed skills
- **AND** the previous provider's unedited managed skills are removed

#### Scenario: Corrected destination replaces an earlier one
- **GIVEN** managed skills installed at a location an earlier release used and the agent does not read
- **WHEN** a task is dispatched
- **THEN** the unedited files at the earlier location are backed up and removed
- **AND** the agent's actual skill folder holds the current managed skills

### Requirement: A user-invoked skill keeps its argument
A skill the workflow invokes with a ticket reference SHALL receive that reference. Where an
agent substitutes arguments into the skill body, the installed body SHALL contain the
substitution the agent documents.

#### Scenario: Dispatch names a ticket
- **GIVEN** an agent whose skills accept arguments through the installed body
- **WHEN** a skill is invoked with a ticket reference
- **THEN** the ticket reference reaches the running skill

#### Scenario: Agent without argument substitution
- **GIVEN** an agent that does not substitute arguments into the skill body
- **WHEN** skills are installed for it
- **THEN** the installed body contains no unsubstituted placeholder text
