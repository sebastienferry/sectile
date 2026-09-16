## ADDED Requirements

### Requirement: The retired per-checkout skill configuration leaves no supporting code
Skill installation targets the agents' user-level configuration. The source tree SHALL be free
of code whose only purpose was to build the retired per-checkout `.taskflow/config.json`, so
that no reader is pointed at an artefact the product no longer produces.

#### Scenario: Helper for the retired configuration file
- **GIVEN** a helper whose only purpose was to populate `.taskflow/config.json`
- **WHEN** that file is no longer generated
- **THEN** the helper is absent from the source tree
- **AND** the packages that used to host it still build and pass `go vet`

#### Scenario: Data still used elsewhere is preserved
- **GIVEN** a field or type that the removed helper read
- **WHEN** that field still has callers in the skill-installation path
- **THEN** it is retained unchanged
- **AND** the skill installation behaviour is unaffected
