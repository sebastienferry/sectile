## ADDED Requirements

### Requirement: Provider setup installs no Claude Code hook
Setting up a provider SHALL install no hook script and SHALL add no hook
registration to the user's Claude Code settings. The user's Claude settings file
SHALL be created by no part of the setup.

#### Scenario: Claude provider on a clean workstation
- **GIVEN** a workstation with no `~/.claude/hooks` directory and no
  `~/.claude/settings.json`
- **WHEN** a project using the Claude provider is set up
- **THEN** the managed skills are installed
- **AND** no `~/.claude/hooks` directory exists
- **AND** no `~/.claude/settings.json` exists

#### Scenario: Settings Sectile never wrote to
- **GIVEN** a `~/.claude/settings.json` holding only the user's own hooks and keys
- **WHEN** a project is set up, once or several times
- **THEN** the file is unchanged byte for byte

### Requirement: Hooks installed by earlier releases are retired
Setting up a project SHALL remove the hook scripts an earlier release of Sectile
installed and SHALL drop the registrations that pointed at them, for whichever
provider the project uses, leaving everything else in the settings file as it
was.

#### Scenario: A workstation carrying the script
- **GIVEN** `~/.claude/settings.json` registers a Sectile hook script on several
  events, beside hooks the user wrote, and the script is recorded in the managed
  manifest
- **WHEN** a project is set up
- **THEN** the script is removed and the emptied hook directory with it
- **AND** every registration pointing at a Sectile script is removed
- **AND** every hook the user wrote is kept, with its matcher
- **AND** every key of the file that is not a Sectile registration is kept
- **AND** an event left with no registration is removed rather than kept empty

#### Scenario: A registration written under a former home or by a pre-release build
- **GIVEN** a registration naming a Sectile hook script under a home directory
  that no longer exists, or naming an agent binary followed by the `sectile-hook`
  argument
- **WHEN** a project is set up
- **THEN** that registration is removed

#### Scenario: A script the user edited
- **GIVEN** an installed Sectile hook script whose content the user changed
- **WHEN** a project is set up
- **THEN** the file is left as the user wrote it
- **AND** its registration is still removed

#### Scenario: The project uses another provider
- **GIVEN** a workstation whose Claude settings still register a Sectile hook
- **WHEN** a project using a provider other than Claude is set up
- **THEN** the Sectile registration is removed
- **AND** no Claude settings file is created where none existed

#### Scenario: Settings that cannot be parsed
- **GIVEN** a `~/.claude/settings.json` that is not valid JSON
- **WHEN** a project is set up
- **THEN** the file is left exactly as it is
- **AND** the setup reports that the Sectile hooks could not be removed
- **AND** the rest of the setup completes

#### Scenario: Nothing left to retire
- **GIVEN** a workstation already cleaned by a previous setup
- **WHEN** a project is set up again
- **THEN** the settings file is not written
