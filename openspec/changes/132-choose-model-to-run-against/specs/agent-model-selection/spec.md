## ADDED Requirements

### Requirement: A model can be configured at three levels
Sectile SHALL accept an optional model identifier on the global settings, on a project and on the
workstation override file, and SHALL resolve the effective model most-specific-first: workstation
override, then project, then global. An empty value at a level SHALL mean "inherit", never "no
model".

#### Scenario: Global model only
- **GIVEN** a global model is configured and the project defines none
- **WHEN** a task of that project is dispatched
- **THEN** the execution contract carries the global model

#### Scenario: Project overrides the global model
- **GIVEN** a global model and a different project model
- **WHEN** a task of that project is dispatched
- **THEN** the execution contract carries the project model

#### Scenario: Workstation override wins
- **GIVEN** a project model and a model in the workstation override file
- **WHEN** the local agent applies its overrides
- **THEN** the effective configuration carries the workstation model

#### Scenario: Nothing configured
- **GIVEN** no model configured at any level
- **WHEN** a task is dispatched
- **THEN** the execution contract carries an empty model
- **AND** the command line is byte-identical to the one built before this change

### Requirement: A skill can run against its own model
Sectile SHALL accept a per-skill model map at each of the three levels, keyed by skill identifier.
The most specific statement SHALL win: naming a skill outranks a bare model, whatever level that
bare model sits on. Resolution for a given skill SHALL therefore take the first non-empty value
among: workstation skill map, project skill map, global skill map, workstation model, project
model, global model. A bare model SHALL govern only the skills that no level singles out.

#### Scenario: One skill differs from the rest
- **GIVEN** a project model and a project skill map binding `implement` to another model
- **WHEN** the `implement` skill is dispatched
- **THEN** the effective model is the one bound to `implement`

#### Scenario: A skill with no entry inherits
- **GIVEN** a project model and a skill map that does not mention `clarify`
- **WHEN** the `clarify` skill is dispatched
- **THEN** the effective model is the project model

#### Scenario: A per-skill entry survives a bare model set above it
- **GIVEN** a global skill map binding `implement` to one model and a bare project model
- **WHEN** the `implement` skill is dispatched
- **THEN** the effective model is the one the global skill map binds to `implement`
- **AND** every other skill of that project resolves the bare project model

#### Scenario: A per-skill entry survives a bare workstation model
- **GIVEN** a project skill map binding `implement` to one model and a bare model in the
  workstation override file
- **WHEN** the local agent applies its overrides and the `implement` skill is dispatched
- **THEN** the effective model is the one the project skill map binds to `implement`

#### Scenario: Two levels name the same skill
- **GIVEN** a global skill map and a project skill map both binding `implement`
- **WHEN** the `implement` skill is dispatched
- **THEN** the effective model is the one bound by the more specific level, the project

#### Scenario: Unknown skill key
- **GIVEN** a skill map containing a key matching no configured skill
- **WHEN** the configuration is validated
- **THEN** the entry is ignored and the configuration is accepted

### Requirement: The model reaches the command line only where the provider accepts it
A resolved model SHALL be passed as `--model <value>` for the providers that accept the flag —
`claude`, `codex`, `gemini` and `cursor` — in both the headless invocation and the command line
built for an interactive or terminal launch. For `agy` and `vibe` a configured model SHALL be
ignored without error.

#### Scenario: Headless run on a provider accepting the flag
- **GIVEN** the provider is `claude` and the resolved model is `M`
- **WHEN** a skill runs headlessly
- **THEN** the invocation carries `--model M` alongside the provider's own headless
  and approval flags, that is `claude -p --permission-mode bypassPermissions --model M "<prompt>"`

#### Scenario: Cursor keeps its subcommand
- **GIVEN** the provider is `cursor` and the resolved model is `M`
- **WHEN** an interactive or terminal launch is built
- **THEN** the invocation is `cursor agent --model M -p "<prompt>"`
- **AND** a headless launch on `cursor` is still refused, as it was before this change,
  because no headless invocation is attested for it

#### Scenario: Provider with no model flag
- **GIVEN** the provider is `agy` or `vibe` and a model is resolved
- **WHEN** a skill runs
- **THEN** the invocation carries no model flag and the run proceeds

#### Scenario: Interactive launch
- **GIVEN** the provider is `claude` and the resolved model is `M`
- **WHEN** an interactive session or a discussion console is opened
- **THEN** the launch command carries `--model M`

### Requirement: A command template keeps control of the command line
When the launch uses a command template, Sectile SHALL NOT inject a model flag. The template SHALL
instead expose a `{model}` placeholder substituted with the resolved model. When no model is
resolved, the placeholder SHALL be removed together with the option that introduces it, so the
template never runs with a flag whose value is missing.

#### Scenario: Template without the placeholder
- **GIVEN** a template that carries `{prompt}` but no `{model}` and a resolved model
- **WHEN** the command is built
- **THEN** the command is the substituted template and no model flag is added

#### Scenario: Template with the placeholder
- **GIVEN** a template containing `{model}` and a resolved model `M`
- **WHEN** the command is built
- **THEN** `{model}` is replaced by `M`

#### Scenario: Placeholder with no model configured
- **GIVEN** a template `my-cli --model {model} -p "{prompt}"` and no model resolved at any level
- **WHEN** the command is built
- **THEN** the command is `my-cli -p "{prompt}"`, carrying neither the placeholder nor its flag
- **AND** the prompt remains the value of `-p` rather than being read as the model

#### Scenario: Placeholder with no model, other flag spellings
- **GIVEN** no model resolved and a template writing the slot as `--model={model}`,
  as `--model "{model}"` or as a bare `{model}` with no flag
- **WHEN** the command is built
- **THEN** the slot is removed whole in each case, along with its flag when it has one
- **AND** a dash inside a plain word such as `my-cli` is never mistaken for that flag

### Requirement: A model identifier is validated on shape, not on membership
Sectile SHALL accept any model identifier whose shape is safe to place on a command line, and SHALL
reject a value carrying shell metacharacters, whitespace or a leading dash. Sectile SHALL NOT reject
a value merely because it is not in a known list.

#### Scenario: Unknown but well-formed identifier
- **GIVEN** a model identifier that Sectile does not know
- **WHEN** the settings are saved and the configuration is validated
- **THEN** the value is accepted and reaches the command line unchanged

#### Scenario: Unsafe identifier
- **GIVEN** a model identifier containing a shell metacharacter, whitespace or a leading dash
- **WHEN** the settings are saved or the configuration is validated
- **THEN** the value is rejected with an error naming the field
- **AND** no command is built from it

### Requirement: The interface suggests models and accepts free text
The project and profile settings SHALL offer a suggested model list for the selected provider while
accepting a free-text identifier, and SHALL state that a command template supersedes the model
selection.

#### Scenario: Selecting a suggested model
- **GIVEN** a provider with suggested models
- **WHEN** the user opens the AI engine settings
- **THEN** the suggestions for that provider are offered and any of them can be selected

#### Scenario: Entering an unlisted model
- **GIVEN** a provider with suggested models
- **WHEN** the user types an identifier that is not in the list
- **THEN** the value is kept and saved

#### Scenario: Template in use
- **GIVEN** a command template is configured for the project
- **WHEN** the user opens the AI engine settings
- **THEN** the interface states that the template governs the command line and that the model is
  only applied through a `{model}` placeholder

### Requirement: The resolved model is observable
A run SHALL report the model it actually resolved, and the project context served to an agent SHALL
expose it.

#### Scenario: Launch step line
- **GIVEN** a run whose resolved model is `M`
- **WHEN** the run reports its steps
- **THEN** the engine step line names both the provider and `M`

#### Scenario: No model resolved
- **GIVEN** a run with no model resolved
- **WHEN** the run reports its steps
- **THEN** the engine step line names the provider alone

#### Scenario: Project context over MCP
- **GIVEN** a project with a resolved model
- **WHEN** an agent calls `get_project_context`
- **THEN** the payload carries the model alongside the provider

### Requirement: The free console runs against the resolved model
The desktop free console SHALL launch its provider with the model resolved for that project, and
SHALL launch without a model flag when none is resolved.

#### Scenario: Opening the console on a configured project
- **GIVEN** a project whose resolved model is `M` and whose console provider accepts the flag
- **WHEN** the user opens the agent console
- **THEN** the console process is launched with `--model M`

#### Scenario: Console provider differs from the project provider
- **GIVEN** the user picks a console provider other than the project's
- **WHEN** the console is opened
- **THEN** the model resolved for the picked provider is applied, and no flag is passed when that
  provider accepts none
