# Specification #744 - Workstation Sandbox allow rules lost after the #730 migration

- Ticket: https://github.com/sebastienferry/sectile/issues/744
- Branch: `feat/744`
- Clarification: `docs/clarifications/744.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Extends: `specs/730-global-sandbox-settings/`
- Framework: Spec Kit

## Summary

The Sandbox rules the #730 upgrade moved to the workstation level were lost
on a workstation, from both levels and without a log line. No Desktop save
path drops them, but an agent that predates #730 does: it does not know the
workstation Sandbox values, replaces `defaults` whole and stamps layout 3.
This change makes the loss impossible for agents built from now on, and
traceable when an older binary still causes it: the one-time fold runs only
at agent start, a file rewritten at an older layout is backed up and logged,
and an agent refuses to save over a settings file newer than itself.

## Scope

In scope: the regression test over every Desktop save path, the fold limited
to the start-up migration, the downgrade trace, the refusal to save over a
newer file, the contract and ADR text, and the changelog.

Out of scope (decided in the clarification):

- The Electron read-modify-write of `settings.json` without a shared lock:
  tracked in #746.
- Restoring rules automatically from a backup.
- Validating the syntax of Sandbox rules.
- Binaries already released: they keep their behaviour.

## User stories

### US1 - A save never removes a rule the owner did not remove (P1)

As the owner, after the upgrade folded my project rules into the workstation
Sandbox settings, every save I make from Desktop keeps them.

1. **Given** a layout 3 settings file whose projects hold allow rules, **when**
   the agent starts, **then** the rules are in the workstation values and no
   project keeps them.
2. **Given** that folded file, **when** Desktop saves the workstation execution
   defaults, the workstation Sandbox category (with the base it read, with an
   empty base, or with a stale base), a project's settings with its Sandbox
   values, a "Move to global", or an "Always allow" answer, **then** every
   folded rule is still in the workstation values afterwards.

### US2 - The fold moves values once, at start (P1)

1. **Given** a running agent, **when** it reads or saves a settings file at an
   older layout, **then** it leaves the project Sandbox values where they are.
2. **Given** a settings file that an agent of layout 4 once wrote and an older
   agent then rewrote at layout 3, **when** the agent starts, **then** it does
   not fold the project values again: they were added per project on purpose.
3. **Given** a layout 3 settings file that no agent of layout 4 or later ever
   wrote, **when** the agent starts, **then** the fold runs and logs as today,
   with its `.bak-layout3` backup.

### US3 - A rewrite by an older agent leaves a trace (P2)

1. **Given** a settings file written at layout 4 then rewritten at layout 3,
   **when** the agent starts, or a running agent next saves, **then** the file
   as the older agent left it is copied beside it as
   `settings.json.bak-layout3` (with a `-<timestamp>` suffix when that name is
   taken) and the agent log says an older Sectile agent rewrote the
   workstation settings.
2. The trace is written once per downgrade: the following saves find the file
   at the current layout again.

### US4 - An older agent does not save over newer settings (P2)

1. **Given** a settings file whose layout, or the highest layout ever written
   to it, is above the agent's own, **when** any save of the agent runs,
   **then** the file is left unchanged and the save fails with an error saying
   the settings were written by a newer Sectile agent and this one must be
   updated.
2. In Desktop, that error reaches the owner through the save's existing error
   display.
3. The start-up migration leaves such a file alone and logs why.

## Functional requirements

- FR1 `ReadSettings` and `UpdateSettings` never move project Sandbox values to
  the workstation level.
- FR2 `MigrateSettingsReport` folds them only when the file's layout and the
  highest layout ever written to it are both below 4.
- FR3 Every write records the highest layout ever written to the file in a
  top-level key that agents of every layout keep when they save.
- FR4 A file whose layout is below the highest layout ever written to it is a
  downgrade: it is backed up beside the settings file and logged, at start
  and on the next save of a running agent.
- FR5 `WriteSettings` refuses a file whose layout, or highest layout ever
  written, is above `SettingsLayout`, with an exported error.
- FR6 The behaviour of the Desktop save paths is unchanged.
- FR7 Log lines and error messages keep the language of their neighbours
  (English in the agent).

## Acceptance

- The regression test of US1 runs every save path listed there against a
  folded file and passes.
- Tests cover US2 to US4, including a file rewritten by a simulated older
  agent (layout 3, no `defaults.claudeSandbox`, the high-water key kept).
- `CHANGELOG.md` gains a `Fixed` line under `[Unreleased]`.
- `docs/contracts/server-agent-v1.md` and ADR 0050 describe the fold at start
  only, the high-water key, the downgrade trace and the refusal.
