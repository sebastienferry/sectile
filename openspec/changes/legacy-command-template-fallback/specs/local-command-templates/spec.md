## ADDED Requirements

### Requirement: Legacy template fallback
A configured command template SHALL drive a launch only when it is nonempty and either the provider is `custom` or the template contains `{prompt}`. For a named provider, a template without `{prompt}` SHALL be treated as absent: the server SHALL serve an empty template and the launch SHALL use the provider default. Server execution and served configuration SHALL apply the same rule.

#### Scenario: Bare CLI name stored for a named provider
- **GIVEN** a project or global setting whose template is a bare CLI name such as `claude`
- **WHEN** the agent downloads its configuration or a task is executed on the server
- **THEN** configuration download succeeds with an empty template
- **AND** the launch runs the named provider's default command.

#### Scenario: Template with the prompt token
- **GIVEN** a named provider whose template contains `{prompt}`
- **WHEN** configuration is served
- **THEN** the template is served unchanged.

#### Scenario: Custom provider without the prompt token
- **GIVEN** a `custom` provider whose template lacks `{prompt}`
- **WHEN** configuration is served
- **THEN** the request is rejected as an invalid template.
