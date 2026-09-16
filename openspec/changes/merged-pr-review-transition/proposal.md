# Accept an already merged PR for the reviewed transition

## Why
Adjustment required the task-branch PR to be open at the moment the reviewed stage
was recorded. When the human merges the PR before that record lands — a routine
race, since merging is the human's own call and adjustment deliberately stops
just short of it — the forge lookup finds no open PR and the transition is
rejected with `expected one matching open pull request, got 0`.

The task then stays at `implemented` for ever: the branch is merged, so no new
open PR can legitimately be produced, and the configured earlier creation stage
has nothing left to recover. Observed on #131, whose PR #140 was merged before
its reviewed transition was recorded.

## What Changes
- Treat a merged task-branch PR as valid adjustment evidence, alongside an open
  one, when recording the reviewed stage.
- Keep an open PR authoritative when both exist for the branch, and keep a
  closed-unmerged PR rejected: that is abandoned work, not evidence.
- Keep every other adjustment gate intact — branch identity, recorded-PR
  identity, the agent checkout commit matching the PR head, and a clean checkout.
- Tell the agent, in the adjustment contract, that a merged PR is reviewed in
  place and never pushed onto.

## Impact
`internal/trackerapi` PR lookup, `internal/db` adjustment evidence,
`internal/runner` agent-side lookup and the adjustment contract text.
No change to the human merge boundary: Sectile still never merges, approves or
closes a task.
