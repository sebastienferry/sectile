# Tasks

## 1. Provider location table
- [x] 1.1 Add a provider→locations resolver in `internal/agentconfig` returning skill root, command
      root (optional) and MCP target for a provider, plus a "no skill convention" outcome.
- [ ] 1.2 Confirm each provider's real user-level paths against its CLI documentation. Carried over
      from the prefixes this repository already wrote (`.claude`, `.agy`) plus `~/.codex/skills`;
      none of them verified against an external source yet.
- [x] 1.3 Unit-test the resolver for every supported provider, for `custom` (first word of the
      template), and for unknown/empty providers.

## 2. Scope skill installation to the selected provider
- [x] 2.1 Rework `skillFiles` to take the resolved locations and emit destinations for the selected
      provider only, returning an empty set when the provider has no skill convention.
- [x] 2.2 Re-express `managedSkillPath` and identifier validation against the new roots; add a test
      asserting no crafted provider or skill identifier escapes the configuration root.
- [x] 2.3 Update `Config.Validate` so an empty destination set is valid.

## 3. Move the scaffold root out of the checkout
- [x] 3.1 Root `Scaffold` at the provider configuration home, keeping atomic write, backup and
      manifest retirement behaviour.
- [x] 3.2 Introduce the global manifest in the agent state directory, keyed by provider.
- [x] 3.3 Stop calling `projectContextFiles`; delete its invocation and retire `AGENTS.md` block and
      `.taskflow/config.json` through the manifest when their content is unchanged.
- [x] 3.4 Test: provider switch retires the previous provider's unedited files and keeps edited ones.

## 4. Move MCP registration to user level
- [x] 4.1 Resolve the MCP target through the new table for every provider, not only `agy`.
- [x] 4.2 Keep the failure fatal in `bootstrapLocalMCP`/`prepareDispatch`, with an error naming the
      provider and the target path; assert the original file is untouched on failure.
- [x] 4.3 Extend `internal/agentconfig/mcp_test.go` and `mcp_migration_test.go` to the user-level
      roots, including the legacy `taskflow`→`sectile` migration at the new location.

## 5. Migrate existing checkouts
- [x] 5.1 On dispatch, read `.taskflow/agent-manifest.json`, retire the unedited managed copies
      (skills, commands, `.mcp.json`, context files), and empty the manifest.
- [x] 5.2 Report preserved edited copies through the existing "Saved previous skill content" log.
- [x] 5.3 Test: a checkout seeded with the old layout ends clean; a checkout with one edited skill
      keeps exactly that file.

## 6. Desktop read-back
- [x] 6.1 Point `localSkillFiles` (`cmd/agent/agent_skill_files.go`) at the resolved skill root.
- [x] 6.2 Return an explicit empty result for providers with no skill convention and render it as an
      informational state, not an error.
- [x] 6.3 Update `cmd/agent/agent_config_test.go` fixtures that assert the five repository prefixes.

## 7. Choose the agents to set up
- [x] 7.1 Add `setupProviders` to the project model, the SQLite projects table and the agent
      configuration contract, normalising to the agents Sectile supports installing for.
- [x] 7.2 Resolve the set at dispatch: the running agent plus the declared ones, deduplicated,
      rejecting an unsupported entry.
- [x] 7.3 Add the checkboxes to the project AI tab and carry the value through create and update.
- [x] 7.4 Test: installing for several agents, and retirement when one is unchecked.

## 8. Documentation and gates
- [x] 8.1 Update `README.md` and the desktop documentation to state where Sectile writes skills and
      MCP configuration, and that checkouts stay clean.
- [x] 8.2 Changelog entry flagging the breaking removal of the managed `AGENTS.md` block and
      `.taskflow/config.json` — not applicable, this repository keeps no `CHANGELOG.md`; the
      breaking change is stated in `README.md` and in the pull request instead.
- [x] 8.3 Run the project's build, vet/lint and full test suite; run `openspec validate
      113-provider-scoped-agent-setup --strict`.
