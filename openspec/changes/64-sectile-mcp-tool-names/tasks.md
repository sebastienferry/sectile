# Implementation checklist

All items remain unchecked because this change is specified, not implemented. Preserve progress and accepted code when retrying.

## 1. Confirm baseline and inventory
- [ ] 1.1 Read the accepted clarification and specification; inspect current branch/base and preserve existing edits and PR state.
- [ ] 1.2 Inventory active MCP names, callers, policy fields, template refresh behavior and current documentation; distinguish historical records and retained environment/storage contracts.

## 2. Canonical protocol names
- [ ] 2.1 Rename the shared eight-tool catalog and cross-tool descriptions, HTTP/stdio server identities and bridge/desktop client identities without changing schemas or handlers.
- [ ] 2.2 Update native launch prompts and desktop completion to canonical calls, preserving run IDs and ownership with no legacy fallback.
- [ ] 2.3 Extend HTTP and stdio tests for exact catalogs, initialization identities, canonical operation parity and all eight legacy unknown-tool failures without side effects; retain invalid-input/authentication/workflow rejection coverage.

## 3. Provider migration
- [ ] 3.1 Implement single-entry migration for all six provider formats, retaining provider locations, refreshed transport settings, unrelated configuration and credentials policy.
- [ ] 3.2 Preserve effective restrictions by normalizing recognized tool/service policy references; preserve Vibe policy fields; reject unsupported or conflicting policy migrations before writes.
- [ ] 3.3 Cover fresh, legacy-only, canonical-only and both-name cases, idempotence, gateway changes, unrelated entries, restrictive policies, duplicate entries and byte-preserving malformed/unsafe failures across providers.

## 4. Generated instructions and upgrade guidance
- [ ] 4.1 Update built-in skill templates, project policy text and other managed MCP instruction producers; verify canonical output and preservation of custom overrides/local-edit backups.
- [ ] 4.2 Update README, architecture and server-agent contract documentation with all eight tools, supported registration locations, coordinated upgrades/reconnection and manual custom-reference migration.
- [ ] 4.3 Add an ADR for the breaking MCP naming decision and conservative migration; maintain a changelog if present. Preserve historical records and out-of-scope contracts.

## 5. Verify implementation and update existing PR
- [ ] 5.1 Run focused MCP, provider bootstrap, launch/desktop and generated-instruction tests; resolve failures in scope.
- [ ] 5.2 Run `go test ./...`, `go vet ./...`, `make test` and `make binary-build`; record actual results and any environment-dependent skips.
- [ ] 5.3 Run `openspec validate 64-sectile-mcp-tool-names --strict` and `git diff --check`; review remaining legacy references against explicit exclusions.
- [ ] 5.4 Update the same specification PR with implementation and verification evidence under the implementation workflow; preserve attained stage on retry and leave review/readiness to adjustment.
