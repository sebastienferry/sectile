# Tasks

## 1. Documentation updates

- [ ] 1.1 Replace the retired-file source-of-truth statement in `AGENTS.md`
  with the live Sectile MCP project-context source.
- [ ] 1.2 Remove the claim in `docs/ARCHITECTURE.md` that the agent writes
  `.taskflow/config.json`, while retaining the diagnostic snapshot statement.
- [ ] 1.3 Replace the two stale contract references in
  `docs/contracts/server-agent-v1.md` while preserving the active workstation
  fallback and skill-installation behavior.

## 2. Verification

- [ ] 2.1 Confirm no current-document reference claims the retired file is an
  active source of truth or an agent-written artifact.
- [ ] 2.2 Confirm historical OpenSpec and clarification references remain
  unchanged.
- [ ] 2.3 Run `openspec validate 550-retire-taskflow-config-documentation --strict`.
