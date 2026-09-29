# Plan #580 - Open the draft pull request at clarification

Implements `spec.md`. The feature reuses the `specified` value of #61
(ADR 0004) and the dropped-artefacts deferral of #487: every place that knows
two creation stages learns a third, and every place that names `specify` as
the early owner learns `clarify`.

## Stack and constraints

- Go server and agent (`internal/db`, `internal/agentconfig`,
  `internal/agent`, `internal/runner`), React web (`web/src`), Electron
  desktop (`desktop/src`).
- No migration: `projects.pr_creation_stage` is a free `TEXT` column; only its
  validation changes. The frozen baseline is not touched.
- Runtime strings (UI labels, server error messages) keep the language their
  surface already speaks; code, comments and docs are in English.

## Data contract

`prCreationStage`: `"clarified" | "specified" | "implemented"`, default
`"implemented"`. Carried unchanged by `models.Project`,
`CreateProjectRequest`, `UpdateProjectRequest` (`*string`),
`agentconfig.Config` and `get_project_context`. No new field.

A helper in `internal/models` names the rule once:

```go
// PRCreationStages lists the accepted values of Project.PRCreationStage, in
// workflow order.
var PRCreationStages = []string{"clarified", "specified", "implemented"}

func ValidPRCreationStage(s string) bool
// PRCreationOwner maps a creation stage to the skill that opens the draft:
// clarify, specify, or implement (default and unknown values).
func PRCreationOwner(stage string) string
```

## Server (`internal/db`)

1. **Validation** (`db.go`, `CreateProject` ~6405 and `UpdateProject`
   ~6553): replace the two literal comparisons with
   `models.ValidPRCreationStage`; the error message lists the three values
   (`prCreationStage must be clarified, specified or implemented`).
2. **Owner** (`adjustment.go`): `prCreationOwner` returns
   `models.PRCreationOwner(p.PRCreationStage)`.
3. **Evidence requirement** (`adjustment.go`, `stagePRRequired`): add
   `clarify` when the owner is `clarify`; `specify` is required when the
   owner is `clarify` or `specify` (FR6). Equivalently: the skill's stage rank
   is at or after the owner's stage rank, restricted to the stage skills.
4. **Stage-to-skill maps** (`stage.go` ~82, `postback.go` ~41): add
   `"clarified": "clarify"`. `validateStagePRs` then runs for a `clarified`
   transition and returns immediately (`stagePRRequired` false) unless the
   owner is `clarify` (FR9). The `implemented`/`specified` fold below it is
   unchanged.
5. **Deferral** (`stageprs.go`, `prDeferredBySpecArtifacts`): accept `clarify`
   and `specify` when the owner is `clarify`, and `specify` when it is
   `specify`; same `spec_artifacts` agent question. `prDeferredNotice` is
   already stage-neutral ("Pull request deferred to the implemented stage:
   the specification artefacts are dropped on this workstation."): keep it.
6. **Skill policy text** (`projectskills.go`, `EffectiveProjectSkills`):
   - `timing` takes the stored value when valid, else `implemented`.
   - The policy paragraph is appended to `clarify` only when the timing is
     `clarified` (FR9: the clarification skill of other projects is
     unchanged), and to `specify`, `implement`, `adjust`, `pickup`,
     `pickup_issues` as today.
   - New `clarified` wording (all those skills get the same paragraph):
     "In the final clarification round only (the owner confirmed, or no
     product question remains open unattended), after committing the report,
     push the task branch (`git push -u origin <branch>`, never force),
     discover and reuse its PR/MR or create a draft on confirmed absence, and
     include its URL as prUrl in the clarified transition. Intermediate rounds
     open no PR. Later stages push to the same branch and update the same
     PR/MR: include it as prUrl in the specified and implemented transitions;
     when a later stage finds no PR (a task clarified before the setting
     changed), create the draft on confirmed absence. Keep it draft until
     adjustment; preserve an existing ready PR. Lookup failure is not absence.
     When the clarification report or the specification files are ignored by
     Git (dropped artefacts), open no PR at those stages, say so in the
     report, and create the draft after implementation."
   - The `specified` and `implemented` wordings are unchanged.
7. **PR discovery** (`prdiscovery.go`): no change; `stageRank("clarified")`
   already orders it. Covered by a test.
8. **MCP** (`internal/taskmcp/server.go`): no change; it forwards the value.

## Agent and runner

1. **Validation** (`internal/agentconfig/validation.go`): drop the
   two-value rejection. `Config.Validate` no longer fails on the creation
   stage (FR11); consumers read it through `models.PRCreationOwner`, where an
   unknown value falls back to `implement`. Rejected alternative: keep
   rejecting unknown values, which makes the next addition break every
   lagging agent again.
2. **Managed prompt lines**: `internal/runner/runner.go` ~434 and
   `internal/agent/agent.go` ~1127 add `clarify` to the skills that receive
   the PR policy reminder, with "clarification owns creation only for
   clarified timing, specification only for specified timing, otherwise
   implementation", and the retry/recovery line.
3. `specArtifactsNotice` already covers `clarify`: no change.

## Web

- `web/src/types/index.ts`: `prCreationStage?: 'clarified' | 'specified' |
  'implemented'` (export a `PRCreationStage` alias).
- `web/src/components/ProjectModal.tsx`: state typed with the alias; a third
  `<option value="clarified">` first, then `specified`, then `implemented`.
- `web/src/locales/projectSettings.ts`: `draftAfterClarification`:
  "Brouillon après la clarification" / "Draft after clarification". `prHelp`
  is already stage-neutral.
- `web/src/lib/workflow.ts`, `prRecoverySkill`: `specify` only for
  `specified`, `implement` otherwise (unchanged code, new test for
  `clarified`).

## Desktop

- `desktop/src/workflow.mjs`, `implementedStep`: unchanged logic (only
  `specified` maps to `specify`); add the `clarified` case to
  `desktop/tests/workflow.test.mjs` and `workflow.ui.cjs`.

## Documentation

- `docs/contracts/server-agent-v1.md`, "PR/MR creation timing" and
  "Adjustment and PR ownership": three values, `clarify` as owner, the
  final-round rule, the deferral for `clarify`, and the tolerant agent
  validation. Merge `origin/main` first: #561 edited this file.
- `docs/adrs/0004-adjustment-and-earlier-pr-ownership.md`: an "Amendment
  (#580)" section recording the earlier owner and the final-round decision
  (Q1 of the clarification), and the tolerant validation.
- `CHANGELOG.md`, `[Unreleased]` / `Added`: "Projects can open the draft pull
  request at clarification: choose "Draft after clarification" in the project
  options. Update Sectile Desktop first; older versions refuse the setting.
  (#580)".

## Target files

| File | Change |
| --- | --- |
| `internal/models/models.go` (or a new `prstage.go`) | `PRCreationStages`, `ValidPRCreationStage`, `PRCreationOwner` |
| `internal/db/db.go` | validation on create and update |
| `internal/db/adjustment.go` | `prCreationOwner`, `stagePRRequired` |
| `internal/db/stage.go`, `internal/db/postback.go` | `clarified -> clarify` |
| `internal/db/stageprs.go` | `prDeferredBySpecArtifacts` |
| `internal/db/projectskills.go` | policy paragraph and `clarify` |
| `internal/agentconfig/validation.go` | tolerant creation stage |
| `internal/runner/runner.go`, `internal/agent/agent.go` | prompt lines |
| `web/src/types/index.ts`, `ProjectModal.tsx`, `locales/projectSettings.ts` | option |
| tests next to each | see `tasks.md` |
| `docs/contracts/server-agent-v1.md`, ADR 0004, `CHANGELOG.md` | docs |

## Risks

- **Lagging desktop agent**: an agent built before this change rejects the
  whole configuration of a project set to `clarified`, so no stage runs on it
  until the desktop app is updated. Mitigated by the changelog line and by the
  tolerant validation for the future; not preventable for already-shipped
  builds.
- **Forge access at clarification**: projects set to `clarified` need the
  forge credential one stage earlier. Same failure mode as `specified` today.
- **Skill text tests**: `internal/skills/catalog_test.go` and
  `internal/db/pr_policy_test.go` assert on exact phrases; extend them rather
  than loosen them.
