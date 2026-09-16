# Tasks

## 1. Let a headless run use its tools
- [x] 1.1 Carry the provider's attested non-interactive approval mode in `headlessCommandLine`:
      `--permission-mode bypassPermissions` for claude, `--auto-approve` for vibe.
- [x] 1.2 Leave `codex exec` and every interactive form untouched, and say in the comment why
      codex carries nothing yet.

## 2. Keep a live session out of the headless path
- [x] 2.1 Pin a discussion and a bare terminal to the interactive mode before the command line
      is built, in `liveSessionMode`.
- [x] 2.2 Leave every other launch on the mode the server resolved.

## 3. Record how a run was launched
- [x] 3.1 Add `run_mode`, `launch_stage` and `chain_stop_stage` to `task_activities`, all
      defaulting to empty so an older run reads as unknown.
- [x] 3.2 Fill them from the job worker and from the run-skill launch path, reading the stage at
      the instant the run starts.
- [x] 3.3 Replace `SkillJob.AutoChain`, which nothing read, with the chain's stop stage.

## 4. Check the hand-back and continue the chain
- [x] 4.1 Compare, when a run closes, the task's stage with the stage it started from.
- [x] 4.2 Record on the run that it ended without moving the task, keeping the reason the
      process gave, for an autonomous run of a skill that owns a stage.
- [x] 4.3 Enqueue the next step of a full chain, autonomous, when a step completed having
      advanced the stage and the stop stage is not reached.
- [x] 4.4 Record why the chain stopped in every other case.
- [x] 4.5 Guard the continuation on the row count of the running-to-finished update, so a
      repeated `finish_run` cannot chain twice.

## 5. Tests
- [x] 5.1 The headless command line carries the approval mode, the interactive one does not.
- [x] 5.2 A discussion and a bare terminal never resolve to autonomous.
- [x] 5.3 A completed step that advanced the stage enqueues the step that follows it.
- [x] 5.4 A step that moved nothing stops the chain and says so, keeping the process reason.
- [x] 5.5 The chain stops at its stop stage and says which one.
- [x] 5.6 An interactive run, a skill owning no stage, and a run recorded before these columns
      are judged on nothing.
- [x] 5.7 A repeated `finish_run` does not enqueue a second step.

## 6. Gates
- [x] 6.1 `go build ./...`, `go vet ./...`, `go test ./...`.
- [x] 6.2 `openspec validate autonomous-run-hand-back --strict`.
- [ ] 6.3 Rerun a full chain on a real task and check the board advances past its first step.

## 7. Follow-ups
- [ ] 7.1 Reconcile the wording of `123-autonomous-or-interactive-skill-runs`, whose
      "the stage is posted by the worker" is intentionally not implemented — see `design.md`.
- [ ] 7.2 Attest codex's non-interactive approval flag and add it to `headlessCommandLine`.
