# #355 — Plan

## Stack

Go skill catalogue. Skills are assembled from Markdown fragments under `internal/skills/fragments/<skill>/` and checked against goldens in `internal/skills/testdata/golden/`, regenerated with `UPDATE_GOLDEN=1`.

## Decisions

- **D1 — One shared wording.** The same push rule sentence set goes into `create_pr/steps.md` step 3 and `adjust/steps.md` step 6, so the two skills cannot drift in meaning.
- **D2 — Rule lives in the push step.** `create_pr` step 2 keeps the reconciliation policy but drops its force-with-lease clause; the push decision moves to step 3, where the push happens. This removes the reading "force after reconciling".
- **D3 — Detection by ancestry.** `git merge-base --is-ancestor origin/<branch> HEAD` is the test; a missing `origin/<branch>` is checked first (`git rev-parse --verify --quiet origin/<branch>` or the fetch output).
- **D4 — Resync by rebase onto the remote branch**, merge acceptable when the rebase conflicts cannot be resolved safely; one retry.

Rejected: keeping the rule in step 2 (it is where the ambiguity came from); a Go-side helper that pushes (the agents own git, and the ticket is about the skill text).

## Target files

- `internal/skills/fragments/create_pr/steps.md`
- `internal/skills/fragments/adjust/steps.md`
- `internal/skills/catalog_test.go` — assertions for `create_pr`, `adjust`, `pickup`, `pickup_issues`
- `internal/skills/testdata/golden/*` — regenerated
