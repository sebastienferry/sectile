# Tasks

## 1. The rule
- [x] 1.1 In `internal/agentconfig/template.go`, extend `ExpandModel` so a blank model removes the
      `{model}` marker together with its neighbouring option: preceding token starting with `-`,
      `--flag={model}` as one token, and the `"{model}"` / `'{model}'` quoted forms.
- [x] 1.2 Collapse and trim the whitespace left by a removal, without touching `{prompt}`.
- [x] 1.3 Leave a `{model}` not preceded by an option token as a marker removal only.
- [x] 1.4 Keep the resolved-model and no-slot paths byte-identical to today.

## 2. Call sites
- [x] 2.1 `internal/agent/agent_config.go` (`modeCommandLine`): on a blank model, run the template
      through `agentconfig.ExpandModel` before `expandAgentTemplate` and stop feeding an empty
      `model` value to the quoting pass.
- [x] 2.2 Confirm `internal/runner/runner.go:525` and `:1108` need no change beyond the new rule,
      including that `UsesCommandTemplate` is still evaluated on a template that kept `{prompt}`.

## 3. Tests
- [x] 3.1 Invert `internal/agentconfig/model_test.go:97` and cover each form of the design table.
- [x] 3.2 Update `internal/agent/agent_model_test.go:76-80` for the blank-model command line.
- [x] 3.3 Update `internal/runner/model_test.go:86-98` for the blank-model command line.
- [x] 3.4 Run `go test ./internal/agentconfig/... ./internal/agent/... ./internal/runner/...`.

## 4. Documentation
- [x] 4.1 Amend the "Placeholder with no model configured" scenario in
      `openspec/changes/132-choose-model-to-run-against/specs/agent-model-selection/spec.md`
      so it no longer mandates the faulty output.
- [x] 4.2 Add one sentence on the rule to the `{model}` help text in
      `web/src/components/AIModelField.tsx` (and `web/src/components/ProfileModal.tsx` if it
      duplicates the wording). Wording to be settled in review.
- [x] 4.3 `openspec validate 193-expand-model-orphan-flag --strict`.
