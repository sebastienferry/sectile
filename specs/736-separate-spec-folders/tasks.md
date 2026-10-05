# Tasks #736 - Separate Macro and Issue specifications folders on the workstation

Order matters: each step leaves the tree building and green. Tests go with the
step they cover.

## 1. Workstation settings

- [ ] T1.1 Rename `ProjectSettings.SpecPath` to `MacroSpecPath` (JSON
      `specPath` kept) and add `IssueSpecPath` (`issueSpecPath`); `isZero`,
      `overlay`, the edit helper, `Settings.MacroSpecPath(id)` /
      `IssueSpecPath(id)` (`internal/agentconfig`).
- Tests: round trip of both keys; a section holding only `issueSpecPath` is
  kept; emptying it removes the key; an existing file with `specPath` reads as
  the Macro folder.

## 2. Resolution

- [ ] T2.1 `resolveSpecFolder` with setting-named errors;
      `localMacroSpecRepo` / `localIssueSpecRepo`; macro callers on the
      Macro resolver.
- Tests: empty, relative, absolute, missing (message names the setting) for
  both; macro operations ignore the Issue folder.

## 3. Specifications worktree

- [ ] T3.1 Extract `ensureSpecWorktree` from `ensureMacroWorktree`, no
      behaviour change (existing macro worktree tests stay green).
- [ ] T3.2 `ensureTaskSpecWorktree`: not distinct returns the code worktree;
      distinct prepares `.tasks/worktrees/<key>` on the task branch from the
      default branch or `origin/<branch>`; reuse; worktrees off; plain folder;
      fetch failure warning; #487 exclusion applied in the Issue folder.
- Tests: one per case above, on temporary Git repositories.

## 4. Launches

- [ ] T4.1 `prepareTaskLaunch` returns the specifications workspace; both
      launch paths set `SECTILE_SPEC_*` and the notice line.
- [ ] T4.2 `taskFolderMap` lists the Issue folder as `spec` with its worktree.
- [ ] T4.3 `attachFolder` refuses either folder by name.
- Tests: launch environment with and without a distinct Issue folder; the code
  worktree unchanged; folder map entries; attach refusals.

## 5. Agent operation, server and MCP

- [ ] T5.1 `models.TaskSpecWorkspace`; `task_spec_worktree` in
      `agentprotocol.Operations`; agent answer in `agent_operations.go`.
- [ ] T5.2 `DB.PrepareTaskSpecWorktree`.
- [ ] T5.3 MCP tool `prepare_task_spec_worktree`; `agentmcp` whitelist and
      count; `mcptest` contract.
- Tests: operation answer (distinct and not); DB method relays the task and
  branch; MCP contract and count tests.

## 6. Desktop

- [ ] T6.1 Settings API: `issueSpecPath` in, `issueSpecPath` /
      `issueSpecDefault` / `issueSpecKind` out.
- [ ] T6.2 `main.js`: two folder rows from one builder; save sends both.
- [ ] T6.3 UI tests: `spec-folder.ui.cjs` covers both rows (set, clear, kind,
      persistence); other suites' labels updated.
- Tests: Go handler test of the API; UI tests after `npx vite build`, outside
  the sandbox.

## 7. Skills

- [ ] T7.1 Read-first paragraph on where issue artefacts live (clarify,
      specify, implement, adjust, handoff).
- [ ] T7.2 Commit and push of the specifications branch in clarify and
      specify; PR description mention in implement, adjust and create-pr
      when distinct.
- [ ] T7.3 Handoff cleanup rule.
- [ ] T7.4 Regenerate plugin goldens (`UPDATE_GOLDEN=1`, unsandboxed);
      catalog tests green.

## 8. Docs and changelog

- [ ] T8.1 ADR 0051; pointer in ADR 0027.
- [ ] T8.2 `docs/USER_GUIDE.md`, `desktop/README.md`,
      `docs/contracts/server-agent-v1.md`.
- [ ] T8.3 `CHANGELOG.md` `[Unreleased]`: `Changed` and `Added` lines.

## 9. Verification

- [ ] T9.1 `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./...`
      (outside the sandbox for httptest), desktop UI tests, web unaffected.
- [ ] T9.2 Re-read the diff against this specification.
