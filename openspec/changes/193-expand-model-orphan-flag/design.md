# Design

## Where the rule lives
`agentconfig.ExpandModel` keeps its signature and becomes the single place that knows what an
unresolved `{model}` means. Both expansion paths call it:

- `internal/runner/runner.go:525` and `:1108` already call it; nothing changes at the call site.
- `internal/agent/agent_config.go:454` (`modeCommandLine`) today puts the model in the value map
  and lets `expandAgentTemplate` substitute and quote it. It instead runs the template through
  `agentconfig.ExpandModel(template, model)` **before** `expandAgentTemplate`, so a blank model
  leaves no `{model}` marker for the quoting pass to turn into `''`. A non-blank model must still
  be quoted by `expandAgentTemplate`, so the value map keeps carrying it and `ExpandModel` is
  applied only on the blank-model branch.

Putting the rule inside `expandAgentTemplate` was rejected: that function is a generic shell-aware
interpolator shared by every placeholder, and teaching it that one marker owns a neighbouring flag
would leak a model-specific rule into it.

## Trigger
`strings.TrimSpace(model) == ""` is the only trigger. A template with no `{model}` marker returns
untouched, as today.

## What "neighbouring option" means
Scanned on the raw template text, left of each `{model}` occurrence:

| Template form | Result |
| --- | --- |
| `--model {model}` | both tokens removed |
| `-m {model}` | both tokens removed |
| `--model={model}` | the whole token removed |
| `--model "{model}"` / `--model '{model}'` | both tokens removed, empty quotes included |
| `my-cli {model} run` | only the marker removed, `run` untouched |

A preceding token counts as an option only when it starts with `-`. Anything else is a word the
author wrote on purpose and is left alone — guessing there would silently break templates that use
the slot positionally.

Whitespace left behind by a removal is collapsed to a single space and trimmed at the ends. The
collapsing is done on the template text only; `{prompt}` is still an unsubstituted marker at that
point, so no prompt content can be reflowed by it.

## Rejected alternatives
- **An explicit optional segment, e.g. `{model?--model {model}}`.** The repository already has a
  parameterised marker (`{mode:AUTONOMOUS|INTERACTIVE}`), so the precedent exists. Rejected: it
  invalidates every template already written and demands a user migration, while the ticket asks
  for the marker to carry its flag away on its own. The heuristic fixes existing configurations
  with no user action.
- **Shell-quoting the empty model as `''` on both paths.** It replaces a truncated prompt with a
  clean CLI error, which is better than today, but the launch still fails and the user still has
  to edit the template by hand.
- **Fixing only the runner path.** The agent path has the same defect with a different symptom;
  leaving it would keep the two paths disagreeing on the same template.

## Tests
- `internal/agentconfig/model_test.go` — the assertion at line 97 is inverted, and one case per
  row of the table above is added, plus the unchanged-resolved-model cases.
- `internal/agent/agent_model_test.go:76-80` and `internal/runner/model_test.go:86-98` — updated
  to the new expected command lines; both must show a blank model producing a command with no
  orphan flag.
