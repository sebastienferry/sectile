## ADDED Requirements

### Requirement: An unresolved model slot removes the option it belongs to
When no model is resolved, Sectile SHALL remove the `{model}` marker from a command template
together with the option it belongs to, so the expanded command line carries no option left
without a value. Sectile SHALL NOT leave an empty argument, an empty quoted string, or a dangling
option in the command it runs.

#### Scenario: A flag and its slot are separate tokens
- **GIVEN** the template `my-cli --model {model} -p "{prompt}"` and no model resolved
- **WHEN** the command is built
- **THEN** the command is `my-cli -p "{prompt}"`
- **AND** `-p` is still the flag introducing the prompt

#### Scenario: A flag and its slot are one token
- **GIVEN** the template `my-cli --model={model} -p "{prompt}"` and no model resolved
- **WHEN** the command is built
- **THEN** the whole `--model={model}` token is gone from the command

#### Scenario: The slot is quoted
- **GIVEN** the template `my-cli --model "{model}" -p "{prompt}"` and no model resolved
- **WHEN** the command is built
- **THEN** neither the option nor an empty pair of quotes remains

#### Scenario: A short option
- **GIVEN** the template `my-cli -m {model} -p "{prompt}"` and no model resolved
- **WHEN** the command is built
- **THEN** `-m` is removed together with the marker

#### Scenario: The slot has no option beside it
- **GIVEN** the template `my-cli {model} run -p "{prompt}"` and no model resolved
- **WHEN** the command is built
- **THEN** only the marker is removed
- **AND** `run` is still in the command

#### Scenario: Whitespace left by the removal
- **GIVEN** any template whose `{model}` slot has been removed with its option
- **WHEN** the command is built
- **THEN** the command carries no doubled or trailing whitespace from the removal
- **AND** the `{prompt}` marker and its surrounding quotes are unchanged

### Requirement: A resolved model keeps the current behaviour
Sectile SHALL substitute a resolved model into the `{model}` marker and SHALL leave the rest of
the template untouched. A template carrying no `{model}` marker SHALL be returned unchanged,
whether or not a model is resolved. A model made only of whitespace SHALL be treated as no model.

#### Scenario: Resolved model
- **GIVEN** the template `my-cli --model {model} -p "{prompt}"` and the resolved model `M`
- **WHEN** the command is built
- **THEN** the command is `my-cli --model M -p "{prompt}"`

#### Scenario: Template without the marker
- **GIVEN** the template `my-cli -p "{prompt}"`
- **WHEN** the command is built, with or without a resolved model
- **THEN** the template is unchanged and no model flag is added

#### Scenario: Whitespace-only model
- **GIVEN** the template `my-cli --model {model} -p "{prompt}"` and a model made only of spaces
- **WHEN** the command is built
- **THEN** the result is the same as with no model resolved

### Requirement: Both launch paths expand the slot the same way
The runner path and the local-agent path SHALL produce the same removal for an unresolved
`{model}` marker in the same template. The local-agent path SHALL NOT emit a quoted empty value
for an unresolved model.

#### Scenario: The same template on both paths
- **GIVEN** the template `my-cli --model {model} -p "{prompt}"` and no model resolved
- **WHEN** the command is built by the runner and by the local agent
- **THEN** both command lines carry neither `--model` nor an empty value for it

#### Scenario: A resolved model on the agent path is still quoted
- **GIVEN** the template `my-cli --model {model} -p "{prompt}"` and a resolved model
- **WHEN** the local agent builds the command
- **THEN** the model value is shell-quoted as it is today
