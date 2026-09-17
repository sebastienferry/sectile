# An unresolved {model} slot carries away its own flag

## Why
`agentconfig.ExpandModel` substitutes the empty string for `{model}`. A template written as
`my-cli --model {model} -p "{prompt}"` with no configured model expands to
`my-cli --model  -p "…"`, which is handed to `sh -c`: the CLI reads `-p` as the value of
`--model`, and the prompt falls through as a positional argument. The run either fails or
executes on a truncated prompt, and nothing in the output says why.

The same template on the agent path fails differently. `expandAgentTemplate` quotes every
substituted value, so an unresolved model produces `--model ''` — an explicit CLI error rather
than a silently truncated prompt, but a launch that still does not run.

`internal/agentconfig/model_test.go:97` asserts the faulty output `--model  -p`, so the defect
is pinned by the test suite instead of caught by it. The spec of change
`132-choose-model-to-run-against` states the same behaviour literally ("`{model}` is replaced by
the empty string and the surrounding command still runs"), so it has to be amended in the same
breath or the specification contradicts the code.

## What Changes
- An unresolved `{model}` slot removes the option it belongs to, not just itself: the preceding
  token when it starts with `-`, the whole `--flag={model}` token, and the surrounding quotes of
  `"{model}"` or `'{model}'`. Residual whitespace is collapsed.
- A `{model}` slot that is not preceded by an option token loses only the marker; no neighbouring
  word is guessed.
- The rule lives in `agentconfig` and is applied on both expansion paths — the runner
  (`internal/runner/runner.go`) and the agent (`internal/agent/agent_config.go`) — so the two
  no longer disagree on what an unresolved model produces.
- A resolved model keeps today's behaviour exactly, including a template with no `{model}` slot.
- `internal/agentconfig/model_test.go` is inverted: it asserts the flag is gone.
- The `agent-model-selection` scenario in change 132 is amended to describe the new behaviour.

## Impact
`internal/agentconfig/template.go` (the rule), `internal/runner/runner.go`,
`internal/agent/agent_config.go` (call sites), and the tests
`internal/agentconfig/model_test.go`, `internal/agent/agent_model_test.go`,
`internal/runner/model_test.go`. Spec text amended in
`openspec/changes/132-choose-model-to-run-against/specs/agent-model-selection/spec.md`.

No schema change, no configuration migration, no new template syntax: templates already written
are fixed without the user touching them.

## Out of scope
- Model resolution itself (`ResolveModel`, overrides, per-skill models).
- `ModelArgs`, the flag-injection path used when no template is configured.
- Model shape validation (`ValidModel`).
- Any new or parameterised template marker.
