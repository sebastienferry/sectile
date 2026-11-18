# Plan #921 - Shared Git worktree permissions

## Stack and current architecture

The Go agent builds Codex app-server RPC policy in `internal/agent/agent_codex_conversation.go`. `worktreeGitWritableRoots` resolves and canonicalizes worktree/common Git directories, currently allowing the per-worktree directory plus shared objects, refs and logs. `startCodexTurn` adds these to workspace-write roots; initialization selects its sandbox separately. Hooks and common configuration are intentionally excluded.

Built-in autonomous CLI construction is separate in `internal/agent/agent_config.go`: `codex exec --approve-for-me --json` and attached-folder flags. The Git-root helper is not currently shared with that path. `internal/agent/repositories.go` resolves attached folders. `internal/agent/specexclude.go` also resolves common Git metadata but is not permission enforcement.

## Investigation before design selection

1. Build a disposable primary repository with a committed seed and real linked task worktree. Record canonical paths and Git configuration affecting hooks/maintenance without copying user credentials or modifying the real repository.
2. Launch through each real built-in path on macOS. Record actual provider/Git versions, launch settings, serialized policy and effective protected-path behavior. Trace the Git process tree and auxiliary operation (for example with sanitized Git tracing); do not assume maintenance or pack-refs is responsible.
3. Correlate stderr, exit status and Git object/index/ref state. Establish which layer requests and denies the lock, and how overlapping permissions are enforced. If the trigger is not reproduced, preserve that limitation and investigate rather than claiming success.
4. Evaluate only provider-supported scoped corrections against the installed version's contract. Record supporting primary documentation/version evidence at implementation time. No CLI flag or policy field is prescribed without that check.

A real denial ends that intention under AGENTS.md. Preserve the refusal and stop for owner direction before rerunning or changing routes for it. Negative boundary checks use disposable sentinel locations; they do not authorize subsequent denied-intention retries. Do not change the current session's permissions to perform qualification.

## Conditional implementation

If supported, adjust narrow Git metadata scope and propagate it through both built-in launch paths without broadening the common Git directory, hooks, configuration or unrelated paths. Reuse canonical path resolution where appropriate; retain read-only mode semantics, approvals and existing network policy. Test ordinary repositories, linked worktrees, missing metadata and symlink/canonical paths where scope resolution changes.

If no supported local correction exists, retain enforcement and provide an evidence-backed limitation diagnostic at the existing trace/error surface. It must name the responsible layer, affected versions/path and supported remedy; distinguish primary success from auxiliary denial without marking an actually failed primary operation successful. Final surface and message are selected after the responsible layer is known, with French runtime text.

No schema, API, dependency or persisted settings change is currently justified. Do not introduce one merely to work around enforcement. Any new internal subprocess must use existing hidden wrappers or `agentexec.Hidden` and satisfy Windows subprocess conventions.

## Contracts and target files

- `internal/agent/agent_codex_conversation.go` and `_test.go`: root discovery, thread/turn policy propagation and conversation reporting as evidence requires.
- `internal/agent/agent_config.go` and its tests: built-in CLI flags and scope propagation; preserve custom templates.
- `internal/agent/repositories.go`, `internal/agent/specexclude.go`: inspect interactions; modify only if causally required.
- Existing agent trace/error handling: change only the surface identified by investigation if diagnostics are needed.
- `docs/TESTING.md`: reproducible live qualification recipe and evidence expectations.
- `docs/CAPABILITIES.md`, ADR 0069 and `README.md`: update relevant permission/launch claims if affected. Add an ADR only for a significant new trade-off.
- `CHANGELOG.md`: user-visible correction or diagnostic only.

Evidence contract: one result per launch path with fixture identity, platform, Git/provider versions, requested/effective policy, lock requester/trigger, sanitized trace, stderr/status, resulting commit/index/ref state, protected-boundary results, selected disposition and remaining limitations. Store durable sanitized evidence with implementation documentation; never include credentials.

## Validation

Extend real-worktree fixtures in agent tests and launch serialization tests. Add provider-backed macOS qualification for the actual reproducing operation and boundary checks. For an upstream-only outcome retain the reproducer and diagnostic regression, documenting the upstream denial as a limitation rather than a passing corrected-operation assertion. Execute focused agent tests, then repository-required build/lint/tests appropriate to changed files. Mocked RPC assertions are complementary to live enforcement evidence.

## Rejected approaches

- Granting the whole shared Git directory or disabling sandbox protections.
- Disabling Git maintenance, suppressing stderr or silently retrying as an assumed correction.
- Treating repository-root write access or generated roots as proof of effective access.
- Applying app-server conclusions to CLI launches or macOS conclusions to other platforms without qualification.
