# Tasks

## 1. Provider location table
- [ ] 1.1 Add a provider→locations resolver in `internal/agentconfig` returning skill root, command
      root (optional) and MCP target for a provider, plus a "no skill convention" outcome.
- [ ] 1.2 Confirm each provider's real user-level paths against its CLI documentation; treat any
      unconfirmed skill path as "no skill convention" and record the finding in the change.
- [ ] 1.3 Unit-test the resolver for every supported provider, for `custom` (first word of the
      template), and for unknown/empty providers.

## 2. Scope skill installation to the selected provider
- [ ] 2.1 Rework `skillFiles` to take the resolved locations and emit destinations for the selected
      provider only, returning an empty set when the provider has no skill convention.
- [ ] 2.2 Re-express `managedSkillPath` and identifier validation against the new roots; add a test
      asserting no crafted provider or skill identifier escapes the configuration root.
- [ ] 2.3 Update `Config.Validate` so an empty destination set is valid.

## 3. Move the scaffold root out of the checkout
- [ ] 3.1 Root `Scaffold` at the provider configuration home, keeping atomic write, backup and
      manifest retirement behaviour.
- [ ] 3.2 Introduce the global manifest in the agent state directory, keyed by provider.
- [ ] 3.3 Stop calling `projectContextFiles`; delete its invocation and retire `AGENTS.md` block and
      `.taskflow/config.json` through the manifest when their content is unchanged.
- [ ] 3.4 Test: provider switch retires the previous provider's unedited files and keeps edited ones.

## 4. Move MCP registration to user level
- [ ] 4.1 Resolve the MCP target through the new table for every provider, not only `agy`.
- [ ] 4.2 Keep the failure fatal in `bootstrapLocalMCP`/`prepareDispatch`, with an error naming the
      provider and the target path; assert the original file is untouched on failure.
- [ ] 4.3 Extend `internal/agentconfig/mcp_test.go` and `mcp_migration_test.go` to the user-level
      roots, including the legacy `taskflow`→`sectile` migration at the new location.

## 5. Migrate existing checkouts
- [ ] 5.1 On dispatch, read `.taskflow/agent-manifest.json`, retire the unedited managed copies
      (skills, commands, `.mcp.json`, context files), and empty the manifest.
- [ ] 5.2 Report preserved edited copies through the existing "Saved previous skill content" log.
- [ ] 5.3 Test: a checkout seeded with the old layout ends clean; a checkout with one edited skill
      keeps exactly that file.

## 6. Desktop read-back
- [ ] 6.1 Point `localSkillFiles` (`cmd/agent/agent_skill_files.go`) at the resolved skill root.
- [ ] 6.2 Return an explicit empty result for providers with no skill convention and render it as an
      informational state, not an error.
- [ ] 6.3 Update `cmd/agent/agent_config_test.go` fixtures that assert the five repository prefixes.

## 7. Documentation and gates
- [ ] 7.1 Update `README.md` and the desktop documentation to state where Sectile writes skills and
      MCP configuration, and that checkouts stay clean.
- [ ] 7.2 Add a changelog entry flagging the breaking removal of the managed `AGENTS.md` block and
      `.taskflow/config.json`.
- [ ] 7.3 Run the project's build, vet/lint and full test suite; run `openspec validate
      113-provider-scoped-agent-setup --strict`.
