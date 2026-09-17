## 1. Persistence
- [x] 1.1 Add `ai_model TEXT NOT NULL DEFAULT ''` and `ai_skill_models TEXT NOT NULL DEFAULT ''` to the `settings` and `projects` tables, with idempotent `ALTER TABLE` migrations alongside the existing `ai_provider` ones in `internal/db/db.go`.
- [x] 1.2 Read and write both columns in `GetSettings`, `SaveSettings`, the project SELECT/INSERT/UPDATE statements and the project row scanner.
- [x] 1.3 Add `AIModel string` and `AISkillModels map[string]string` to `models.Settings`, `models.Project`, `models.CreateProjectRequest`, `models.UpdateProjectRequest` and the settings update payload.

## 2. Contract and resolution
- [x] 2.1 Add `AIModel` and `AISkillModels` to `agentconfig.Config` and to `agentconfig.Overrides`, keeping `Version` at 1.
- [x] 2.2 Write `agentconfig.ResolveModel(Config, skillID string) string` implementing the precedence chain; unit-test every level and the inherit-on-empty rule.
- [x] 2.3 Fold global settings into the project values in `db.AgentConfig`, mirroring the existing `AIProvider` fallback.
- [x] 2.4 Apply the workstation model and skill map in `ApplyOverrides`, and persist them through `ReadSettings`/`WriteSettings` (add the two keys to the preserved-field list).
- [x] 2.5 Add `agentconfig.ValidModel` and call it from `Config.Validate`; reject shell metacharacters, whitespace and a leading dash.
- [x] 2.6 Add `agentconfig.ModelFlag(provider string) (string, bool)` covering claude, codex, gemini, cursor, and returning false for agy and vibe.

## 3. Command builders
- [x] 3.1 Carry the resolved model on `runner.AIInvocation` and inject `--model` in `execAgentCommand` for the flag-accepting providers, leaving the template branch untouched.
- [x] 3.2 Substitute `{model}` in the template path of `execAgentCommand` and in `expandAgentTemplate` / `agentCommandContext.values`.
- [x] 3.3 Inject the flag in `cmd/agent.agentCommandLine` and in `runner.InteractiveAgentLaunch`, keeping `cursor agent` word order.
- [x] 3.4 Thread the model through `dispatchCommand` so a skill launch resolves the per-skill model and a discussion resolves the project-level one.

## 4. Observability and context
- [x] 4.1 Report the resolved model on the `🤖 Moteur IA` step line in `runner.buildAIInvocation`, omitting it when empty.
- [x] 4.2 Add `aiModel` to the `get_project_context` payload in `internal/taskmcp/server.go`.

## 5. Interfaces
- [x] 5.1 Add the model field, the per-provider suggestion list and the template-supersedes notice to `web/src/components/ProfileModal.tsx` (global) and `ProjectModal.tsx` (project), with the `AIModel` types. The two AI sections carry hard-coded French strings today, so the new labels follow them rather than introducing a lone translated field.
- [x] 5.2 Add the per-skill model map editor to the project AI section.
- [x] 5.3 Resolve the model when the desktop free console launches. Done in the agent (`cmd/agent/agent_console.go`) rather than in `desktop/src/main.js`: the console command is built after the workstation overrides are applied, which is the only place the effective model is known.

## 6. Validation
- [x] 6.1 Unit tests: resolution precedence, per-skill resolution, flag injection per provider, template `{model}` substitution including the empty case, and `ValidModel` rejections.
- [x] 6.2 Regression test: with every model field empty, the built command lines are byte-identical to the current ones for each provider.
- [x] 6.3 Run the repository gates (`make` targets for build, vet, test and the web build).
- [x] 6.4 `openspec validate 132-choose-model-to-run-against --strict`.

## 7. Second clarification pass corrections
- [x] 7.1 Make `MergeModels` follow "the most specific statement wins": a bare model on a level no longer silences a per-skill entry set below it, it governs only the skills no level singles out.
- [x] 7.2 Invert the tests that locked the previous precedence (`TestMergeModelsLevelByLevel`, `TestApplyOverridesModel`, `TestAgentConfigResolvesModelAcrossLevels`) and add a case where two levels name the same skill.
- [x] 7.3 Make `ExpandModel` remove an unresolved `{model}` slot together with the flag that introduces it, covering `--model {model}`, `--model={model}`, a quoted slot and a flagless slot, and never treating a dash inside a plain word such as `my-cli` as that flag.
- [x] 7.4 Invert the `TestExpandModel` case that locked `--model  -p` as the expected output.
- [x] 7.5 Update the stale precedence comment in `internal/db/agentconfig.go`.
