# #267: Implementation checklist

Ordered so the branch is clean before anything is added, the plugin exists before the
dispatch can look for it, and the writes are removed only once the dispatch can resolve a
skill without them. `make test` (`go test ./...`, then `npm test`, `tsc --noEmit`, `oxlint`)
is the gate at the end of each section. The round-3 checklist is in the history of this file
(`a9406ffa`).

## 0. The branch

- [ ] **T0** Check that PR #350 is closed; if not, stop and ask the owner.
- [ ] **T1** Revert the round-3 commits (`6a79d266`..`a9406ffa`, code only, not the
      clarification or this specification) in one `revert(267): drop the third-party skill
      packs` commit, then merge `origin/main` (plan, "Starting point"). `go build ./...`
      and `make test` pass on the result before T2.

## 1. The plugin

- [ ] **T2** `RenderGenericSkillContent` in `internal/skills/catalog.go` (D2): framework
      subsections derived from the embedded fragment names, generic pull-request policy
      section, contracts appended.
- [ ] **T3** `internal/skills/plugin.go`: `RenderPlugin(version)` and
      `RenderMarketplace(version)` (D1), version validation.
- [ ] **T4** `cmd/sectile-plugin/main.go` and the `make plugin` target.
- [ ] **T5** Tests, `internal/skills/plugin_test.go`:
      - golden output under `testdata/plugin/` for version `0.0.0-test`, with an
        `-update` flag; running twice gives byte-identical files (FR1);
      - one `SKILL.md` per catalogue entry, frontmatter `name` equal to the directory;
      - `.mcp.json` URL and header reference `user_config.server_url` / `user_config.api_key`,
        `api_key` is `sensitive` and `required` (FR3);
      - no generated file contains `http://`, `https://` other than the `example.com`
        help text, a key, or a host name (FR3);
      - D2 assertions: each framework-specific fragment appears only inside its subsection,
        and every fragment `RenderSkillContent` uses for `openspec` and for `speckit` is in
        the generic output (FR2);
      - empty or `v`-prefixed version refused.

## 2. Knowing what is custom, and the settings

- [ ] **T6** `agentconfig.Skill.Custom`; set in `db.AgentConfig` from non-empty
      `project_skills.content` and in `Resolve` from `Settings.Skills` (D4). Record
      `CommandOverridden` in `Resolve`.
- [ ] **T7** `Defaults.CustomSkillsWin` and `Defaults.InstalledSkillSource`, validation,
      `GET|PUT /desktop/workstation` (D5).
- [ ] **T8** Namespaced command validation in `workstation.go`, `validation.go` and
      `desktop/src/execution-fields.mjs` (D10).
- [ ] **T9** Tests: `db.AgentConfig` marks an edited skill custom and leaves a mode-only
      row and the PR-policy suffix non-custom; `Resolve` marks a `Settings.Skills`
      override custom; settings round-trip with both fields absent, set and invalid;
      `sectile:clarify-issue` accepted as a command and refused as an ID or directory;
      the desktop field test for `SKILL_COMMAND`.

## 3. Resolving the skill at dispatch

- [ ] **T10** `internal/agent/skillsource.go`: `chooseSkill`, `claudePluginSkill`, direct
      copy probe (D6).
- [ ] **T11** Run-private file: write under `~/.config/sectile/runs/<runID>/`, remove at run
      end, sweep directories older than 7 days at start, validate the run ID (D7).
- [ ] **T12** `dispatchCommand` takes the choice for task and macro dispatches; `custom`
      prompt and `--add-dir` for Claude (D6).
- [ ] **T13** `errSkillNotInstalled` through the existing launch-failure path, message of D8.
- [ ] **T14** Tests, `internal/agent/skillsource_test.go`, with a fake home through
      `testhome`:
      - resolution table: custom × setting on/off × direct present/absent × plugin
        present/absent/disabled/project-scoped elsewhere × preference direct/plugin ×
        provider claude/codex/gemini × explicit `SkillCommands` (US4, US5, US6);
      - malformed `installed_plugins.json` reads as not installed;
      - two projects with different custom content get two files and two prompts (FR6);
      - the run file is gone after success, failure and cancel; the sweep removes an old
        directory and keeps a recent one;
      - a run ID with a path separator is refused;
      - the custom prompt names the file and carries the payload prompt; for `adjust` the
        adjustment contract is still appended;
      - nothing found fails before any PTY is opened, with the D8 message per provider.

## 4. Removing the writes

- [ ] **T15** Remove the `Scaffold` / `bootstrapLocalMCP` calls from `prepareDispatchLocked`
      and `prepareMacroSkills`; delete `syncLocalProject` and its call in `connect` (D3).
- [ ] **T16** Remove the start-up `refreshMCPConnections` call, after the check D3 describes;
      record its outcome in the pull request.
- [ ] **T17** Remove `WriteProjectSkillToRepo` from save and reset; delete the functions left
      without callers; keep `sync_config` and `install-skills` (D3).
- [ ] **T18** Tests (FR4, US2): with a fake home, a task dispatch, a macro dispatch, an agent
      connect on a single project and a skills-editor save leave the fake home's
      `.claude`, `.claude.json`, `.agents`, `.codex`, `.gemini`, `.config/sectile` (except
      `runs/`) byte-identical, compared by a tree hash before and after; a dispatch whose
      MCP file is read-only still launches; `initializeProvider` still writes skills and MCP
      (regression guard for US3). Update or delete the existing tests that asserted the
      writes at dispatch and at connect, saying which in the commit message.

## 5. Passive signal and wording

- [ ] **T19** `customSkillUse` in the dispatcher, `customSkillsUsed` on
      `GET /desktop/workstation`, the activity step (D9).
- [ ] **T20** Desktop: the two settings in "Execution defaults", the warning badge on the
      settings button, the "Custom skills used" notice (D5, D9). `npx vite build` before the
      desktop UI tests, then restore `webui/.gitkeep` if the build removed it.
- [ ] **T21** Optional-setup wording in `init` and in the desktop Deployment tab (D11); the
      skills editor badge says "direct copy" (D3).
- [ ] **T22** Tests: `customSkillsUsed` filled by a custom dispatch only, not by a direct or
      plugin one, and not after the setting is turned off; desktop UI test for the badge and
      the notice appearing from a stubbed `/desktop/workstation` response and absent when
      the list is empty; desktop UI test for the two settings saving through
      `saveWorkstationSettings`.

## 6. Documentation

- [ ] **T23** ADR 0034, `docs/contracts/server-agent-v1.md`, `README.md`, `CHANGELOG.md`
      `[Unreleased]` (plan, "Documentation"). No internal host, project or secret name in any
      of them: the repository is public.

## Test plan by level

| Level | What | Where |
| --- | --- | --- |
| Unit, pure | generic rendering, plugin files, validation patterns | `internal/skills`, `internal/agentconfig` |
| Unit, fake home | skill resolution, plugin detection, run file lifecycle, no-write guarantee | `internal/agent` with `testhome` |
| Integration, DB | `custom` flag in the agent config; save no longer calls the agent | `internal/db` (SQLite, and PostgreSQL when `SECTILE_TEST_POSTGRES_DSN` names a throwaway database) |
| Desktop UI | settings fields, badge, notice | `desktop` UI tests after `npx vite build` |
| Manual, once | `go run ./cmd/sectile-plugin -version 0.0.0 -out $TMPDIR/p -marketplace`, `claude plugin marketplace add $TMPDIR/p`, `claude plugin install sectile@sectile`, answer the two prompts, check `/sectile:clarify-issue` is listed and the `sectile` MCP server connects; set "installed skills source" to plugin, remove the direct copies of a throwaway home, dispatch a task step and read the prompt in the run log | a throwaway `HOME`, never the dev database |
