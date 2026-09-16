# Design

## Context
Model selection has to travel the same road the provider already travels:
`settings`/`projects` rows → `db.AgentConfig` → `agentconfig.Config` (the versioned, secret-free
contract) → `ApplyOverrides` on the workstation → the two command builders
(`runner.execAgentCommand` for headless runs, `cmd/agent.agentCommandLine` and
`runner.InteractiveAgentLaunch` for terminal and interactive launches). Anything that skips one of
those hops produces a run whose reported engine is not the engine that ran.

## Decisions

### Resolution lives in one function, not in each builder
A single `agentconfig.ResolveModel(c Config, skillID string) string` owns the precedence chain.
`db.AgentConfig` folds global settings into the project fields exactly as it already does for
`AIProvider` and `AICommandTemplate`; `ApplyOverrides` folds the workstation file on top; the
builders only ever read the resolved string.

Rejected: resolving inside each command builder. There are three of them and they already disagree
subtly (`agentCommandLine` has no `codex` special-casing, `execAgentCommand` has a `default` branch
that falls back to `agy`); duplicating precedence there would guarantee drift.

### Flag capability is a provider property
A small table `modelFlag(provider string) (flag string, ok bool)` returns `--model` for `claude`,
`codex`, `gemini`, `cursor` and `false` for `agy`, `vibe`. `custom` never reaches it, because a
custom provider always uses a template.

Rejected: a per-provider free-text "model flag" setting. It exposes a second command-line knob to
users who already have `aiCommandTemplate` for exactly that purpose.

### The template wins, and gains `{model}`
`UsesCommandTemplate(provider, template)` already decides whether the template governs the command.
When it does, no flag is injected; `{model}` joins the existing placeholder set
(`{prompt}`, `{issueKey}`, …) and is substituted with the resolved model or the empty string.

Rejected: injecting the flag before the template's first word. It would produce
`claude --model M --dangerously-skip-permissions -p "..."` for some templates and a broken line for
templates whose first word is `sh` or a wrapper script.

### Validation is shape-only, at both ends
A shared `agentconfig.ValidModel(string) error` accepts `^[A-Za-z0-9][A-Za-z0-9._:@/-]*$` and is
called by the HTTP settings/project handlers on write and by `Config.Validate` on the agent side
before any command is built. This is a security boundary, not a convenience check: the value is
interpolated into a line executed through `sh -c` for the template path, the same reason
`escapeForDoubleQuotes` exists.

Rejected: a closed enum of known models. It ages out with every provider release and would force a
Sectile release to use a new model.

### Per-skill map shape
`map[string]string` keyed by the skill ID already used by `SkillOverrides` (`clarify`, `specify`,
`implement`, …), stored as JSON in one column per level. Keys matching no configured skill are
ignored rather than rejected, so removing a skill from a project cannot make its settings
unsavable.

Rejected: a per-skill row table. The map is small, always read whole, and the codebase already
stores `stage_mapping` and `skill_overrides` as JSON columns.

### Contract stays version 1
`aiModel` and `aiSkillModels` are additive optional fields on `agentconfig.Config`. An older agent
ignores them and behaves exactly as today; a newer agent reading an older server's config resolves
an empty model, which is the documented "no flag" case. No `agentconfig.Version` bump.

## Open questions
None blocking. The suggested-model lists per provider are presentation data in the web UI and can be
filled from each provider's current public model names at implementation time.
