# Design

Add `agentconfig.UsesCommandTemplate(provider, template)` and `EffectiveCommandTemplate` beside the contract validation so the server, the agent and the runner share one rule. `db.AgentConfig` applies it to the effective template after project-over-global inheritance and before `Validate`; the runner's two inline checks call the predicate instead of restating it.

`Validate` stays strict: a nonempty template served to the agent must contain `{prompt}`, and `custom` always requires one. Normalizing before validation means only templates a launch would actually run reach the agent, whose `agentCommandLine` then falls back to its provider defaults exactly as server execution does. Stored rows are not rewritten: the web UI already writes `{prompt}` templates, and a migration would not protect other writers or exports.
