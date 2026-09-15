# Implementation validation — #64

Branch: `feat/64`. Existing draft PR: https://github.com/sebastienferry/sectile/pull/92.

## Scope delivered

- HTTP/stdio server identity `sectile`, eight generic tool names, Sectile bridge
  and desktop identities, and no legacy aliases or fallback calls.
- Canonical launch/completion callers, the web copy-command prompt, built-in
  templates, generated project policies and 54 checked-in managed instruction files.
- Six-provider registration migration, preserved unrelated entries and explicit
  restrictions, exact tool-list reference mapping, idempotent refresh, and
  non-destructive rejection of malformed or unsafe policies/collisions.
- README, architecture and agent contract updates, plus ADR 0005 and the upgrade
  sequence. Stored custom instructions and historical records are retained.

## Checks

- `openspec validate 64-sectile-mcp-tool-names --strict`: passed.
- `git diff --check` and staged whitespace checks: passed.
- `go test ./...`: passed across all packages.
- `go vet ./...`: passed with no output.
- `make test`: passed; frontend output reports 22 tests, 22 passed, 0 failed;
  TypeScript succeeded. Oxlint emitted warnings but exited successfully.
- `make binary-build`: passed; frontend bundled and `bin/sectile` built, with
  the existing `bin/sectile` compatibility copy. Vite reported its large-chunk
  warning. The final Go changes were rechecked and rebuilt.

Initial checks could not write the sandboxed default Go cache and could not find
frontend dependencies in this worktree. Tests were rerun with cache/listener
access after `npm ci --prefix web` installed the lockfile dependencies unchanged.
These setup failures are resolved.

## Regression coverage

- Shared HTTP/stdio checks assert server identity, the exact eight-tool catalog,
  successful calls, original run reuse/completion, and unknown-tool rejection for
  all eight legacy names with unchanged workflow snapshots.
- Existing input, authentication, workflow validation and run isolation coverage
  remains active. Desktop completion verifies the same run ID and unchanged stage.
- Provider tests cover fresh, legacy-only, canonical-only and both-name states,
  repeated refresh, explicit restrictions, Vibe preservation, unrelated entries,
  credential removal, unsafe collisions, invalid entries, unsupported patterns,
  external policy files and byte-preserving failures.
- Built-in/project policy tests verify canonical references while stored custom
  overrides remain verbatim. Existing managed-file backup tests pass.
- Remaining old-name occurrences in production Go are migration recognition;
  other occurrences are negative/custom-content tests or preserved machine keys.

## Review and recovery notes

The branch incorporated the remote default baseline before implementation, without
rewriting its published specification history. Adjustment subsequently integrated
`origin/main` at `adc4a71` and retained both documentation contracts when resolving
the append-only merge conflict. Final adjustment evidence is in `adjustment.md`.

Earlier local workflow/configuration edits are retained in the named stash
`preserve local #64 configuration during adjustment`, so the reviewed checkout
can remain clean. The earlier preservation stash also retains the original
pre-implementation state. These local customizations are not part of the PR.

The MCP SDK can close an HTTP session after an unknown-tool error. Negative tests
therefore establish a fresh client for each legacy call and check the database
snapshot; a later connection-closed error is not accepted as evidence of rejection.
Provider bootstrap tests isolate HOME to avoid reading real workstation policies.

Custom or enterprise policy sources outside known configuration locations remain
an operator migration step. No live client registrations were rewritten and no
server/agent process was restarted by this implementation run.
