# Phase 0: preserved prototypes and runtime baseline

Date: 2026-10-29
Status: phase 0 complete; runtime extraction has not started.

## Preserved source and branches

| Repository | Preserved prototype | New branch |
| --- | --- | --- |
| Sectile Go | `4576ca30ea4969bcec9be1a424e96ad677107cf7` on `feat/desktop-local-runtime` | `refactor/embedded-local-backend` |
| Tauri | `afa466c801f146a00d7ef745907b5fc95c59be4c` on `feat/tauri-bootstrap` | `feat/local-fleet-modes` |

The Go checkout was already an isolated worktree and was reused. Tauri had no
commits; its 122 source/documentation files were captured in an initial commit
before branching. The Go checkpoint captured 75 changed/new files. Binary build
outputs, local state and credentials were excluded by repository ignores; the
candidate files contained no matched high-confidence provider/private-key
patterns. This is a scoped check, not a universal secret-scanner guarantee.
No commits were pushed and no running installation was restarted or migrated.

## Confirmed decisions

- A separate combined Local executable preserves the lightweight Fleet agent.
- Internal HTTP/WebSocket transport is retained initially.
- The current OAuth installation continues to work; no token-only conversion.
- Runtime topology and tracker authentication are separate choices.
- Tauri is the future primary agentic board. The headless backend retains API,
  MCP, auth and required domain logic; roadmap separation is an independent lot.

The canonical engineering contract is in the Go repository at
`docs/desktop-runtime-contracts.md`, recorded by ADR 0066. It specifies immutable
configuration, durable connection identity, exclusive lifetime ownership,
stop/join sequencing, version boundaries and transaction-like migration.
The desktop architecture is recorded in ADR 0013 here.

## Baseline checks

Commands were run against the preserved runtime source, with no test skips added:

```sh
go test -json ./cmd/server ./cmd/agent ./internal/agent ./internal/agentexec ./internal/db ./internal/handlers ./internal/taskmcp ./internal/agentmcp ./internal/localruntime
npm run check
npm test
npm run test:ui
npm run build
cargo fmt --manifest-path src-tauri/Cargo.toml -- --check
npm run test:rust
```

- Go: eight tested packages passed; `cmd/agent` has no test files. There were
  2,150 passing test events including subtests and no failures.
- Tauri: typecheck, 8 domain tests, 83 UI tests, production build, Rust formatting
  and 22 Rust tests passed.
- A previously observed `TestTryPullLocalAgentTasks` temporary-directory cleanup
  failure did not reproduce in this full baseline run. It was not skipped.
- Real MCP `list_projects` calls succeeded through the private companion bridge,
  the shared installed Codex bridge and Electron's shared HTTP connection.
- The current personal OAuth grant remains `oauth`; a real Jira account check
  through the backend succeeded. No credential values or account data are saved
  in this report, and no OAuth app configuration was changed.

PostgreSQL integration, distributed cluster deployment, Windows/Linux execution
and a new full provider workflow run are not claimed by this phase 0 baseline.
They remain required gates for later changes affecting those paths.

## Measurements

Environment: macOS 27.0.1, arm64, current development/debug artifacts.
Raw sanitized results, artifact hashes and toolchain versions are in
[phase0-runtime-baseline.json](phase0-runtime-baseline.json).

| Measurement | Baseline |
| --- | --- |
| Long-lived Local Go services | 2: server + execution agent |
| Busy executions during memory observation | 0 |
| Server median RSS, 5 samples | 34.55 MiB |
| Agent median RSS, 5 samples | 21.12 MiB |
| Warm authenticated discovery median, 10 requests | 0.34 ms |
| Fresh SQLite backend readiness median, 3 disposable runs | 75.0 ms |
| Paired agent connection/readiness median, 3 disposable runs | 109.5 ms |
| Fresh backend + pairing + agent readiness median | 187.4 ms |
| Existing debug app allocated disk size | 92.93 MiB |
| Production JS main chunk | 300.14 kB; gzip 90.74 kB |

RSS is per-process resident memory, not unique/proportional memory: summing it
can double-count shared pages. Desktop/WebKit/provider children are excluded.
Warm discovery is not cold desktop startup. Fresh-store measurements used
isolated temporary homes/databases and ephemeral-port ordinary server mode;
filesystem caches were warm, no trackers/providers ran, and native UI launch was
excluded. The fixed-port production installation remained running. Temporary
services were stopped and their data removed after each sample.

Deployed artifact hashes are recorded separately from source commit identities;
these debug artifacts were built from the prototype before its checkpoint commit.
This report is a reproducible comparison reference, not a performance promise.

## Next phase gate

Phase 1 extracts backend assembly with the standalone server as its first
consumer. It must preserve the current API/MCP/auth/OAuth behavior and dependency
boundaries before a combined Local runtime or data migration is introduced.
The [V1 migration plan](sectile-v1-migration-plan.md) is the product-level roadmap;
[the reviewed extraction plan](local-fleet-runtime-plan.md) defines the technical
sequence. Phase 0 documents contracts; it does not implement their guarantees.
