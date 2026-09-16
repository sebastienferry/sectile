# Remove the dead skillDirNames helper

## Why
`skillDirNames` in `internal/db/db.go` has no callers. Its comment ties it to
`.taskflow/config.json`, a per-checkout file that change 113 stopped generating and untracked.
The helper was already unreachable before 113 — it has no callers at `86ec4b1` either — so 113
only made a pre-existing piece of dead code obvious, and left its removal out to keep that diff
on topic.

Leaving it in place is misleading: a reader of `db.go` finds a helper that advertises an
artefact the product no longer writes, and the compiler gives no signal because Go does not
report unused package-level functions.

## What Changes
- Delete `skillDirNames` and its comment from `internal/db/db.go`.
- Nothing else: `ProjectSkillTemplate.DirName`, the only field the helper read, stays in use by
  `internal/db/agentconfig.go`, `internal/db/projectskills.go` and the skill templates.

## Impact
`internal/db/db.go` only. No behaviour change, no exported symbol touched — the helper is
unexported and unreferenced — and no test asserts it. Verified by `go build ./...`,
`go vet ./...` and the existing Go suite.
