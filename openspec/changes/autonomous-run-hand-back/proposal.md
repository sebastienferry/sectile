# An autonomous run hands the workflow back

## Why
An autonomous run on #132 was launched four times on 2026-09-16 and moved the board zero times.
Nothing hung: each run started, ran for four to six minutes, exited, and was recorded as
`completed`. What never happened was the transition, and nothing anywhere said why.

The recorded output of the `pickup` run of 15:17 says it plainly: `mcp__sectile__get_task` and
`mcp__sectile__get_project_context` were *refused* — "Claude requested permissions to use … but
you haven't granted it yet" — and the run ends "Sans accès aux outils `start_run` /
`transition_stage` / `finish_run`, je ne peux […] enregistrer la moindre transition." The
`adjust` run of 17:16 reports the same for file writes, the network, and the MCP tools, and
states that no `implemented → reviewed` transition was performed.

That is the whole defect. A headless run is launched as `claude -p '<prompt>'`
(`cmd/agent/agent_config.go`) with `Stdin = nil` (`cmd/agent/agent_headless.go`), so there is
nobody to answer a permission prompt and every tool the CLI asks for is denied — including the
Sectile MCP tools, which are the only way a run reports itself and moves the stage. The CLI then
does the one thing it still can: print why it is blocked, and exit zero.

Two further defects sit on the same path.

**A live session is forked headless.** A project defaulting to autonomous sends a discussion
through the headless path too. `dispatchCommand` builds a live provider session for `discuss`,
which carries no prompt; run with a closed stdin it exits immediately. The run of 17:16:16 on
#132 is exactly that: `Error: Input must be provided either through stdin or as a prompt
argument when using --print`.

**A full chain runs one step.** `EnqueueFullChainRun` documents itself as "each step enqueues
the next until the work reaches the project's stop stage" and sets `SkillJob.AutoChain`, which
is read nowhere: `grep -rn "\.AutoChain"` returns nothing. One step is enqueued and the chain
ends there, silently.

## What Changes
- A headless launch carries the provider's non-interactive approval mode, so the CLI can use
  the tools it needs without a human: `claude -p --permission-mode bypassPermissions` and
  `vibe -p --auto-approve`. Only attested flags are passed, as for the provider list itself;
  `codex exec` keeps its current form until its bypass flag is verified here.
- A launch that opens a live provider session — a discussion, a bare terminal — is interactive
  whatever the project default says, instead of becoming a CLI with no input.
- A run records how it was launched: its resolved mode, the stage the task sat on, and, for a
  step of a full chain, the stage that chain stops at. When it finishes, that is compared with
  where the task now is, and a run that ended without moving a stage it owned says so on its own
  activity. The server never invents the transition: it cannot tell correct work from a CLI that
  printed a refusal and exited zero.
- A full chain actually chains: a completed step that advanced the stage enqueues the next one,
  headless, until the stop stage. Every way it stops — the stop stage reached, the run failed,
  the stage did not move — is recorded on the run that ended.

## Impact
`cmd/agent/agent_config.go` and `cmd/agent/agent.go` (launch), `internal/db` (three additive
columns on `task_activities`, `SkillJob.AutoChain` becomes `ChainStopStage`, run start and
finish), `internal/handlers` (the run-skill launch path), plus their tests. Additive schema
change, no new endpoint, no protocol message added or removed.

## Out of scope
Whether a provider CLI should be sandboxed further inside an autonomous run, the content of the
skills themselves, and the wording of change `123-autonomous-or-interactive-skill-runs`, whose
"the stage is posted by the worker" is deliberately not implemented here — see `design.md`.
