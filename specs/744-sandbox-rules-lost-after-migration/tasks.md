# Tasks #744 - Workstation Sandbox allow rules lost after the #730 migration

Order matters: each step builds on the previous one. Tests go with the step
they cover.

## 1. High-water key and refusal

- [x] T1.1 `WriteSettings` writes `maxLayout` (outside `ownedKeys`).
- [x] T1.2 `ErrSettingsNewer`: `WriteSettings` refuses a file whose `layout` or
      `maxLayout` is above `SettingsLayout`, before writing.
- Tests: `maxLayout` written on a fresh file and kept at its highest value; a
  key `maxLayout` survives a simulated layout 3 writer; a newer file is
  refused through `WriteSettings` and `UpdateSettings` and left byte for byte
  unchanged.

## 2. Fold at start only

- [x] T2.1 `readConverted` no longer folds; `MigrateSettingsReport` folds when
      `layout` and `maxLayout` are both below 4.
- [x] T2.2 Fold tests read through the migration path.
- Tests: `ReadSettings` and `UpdateSettings` on a layout 3 file leave the
  project values on their projects; `MigrateSettingsReport` still folds a
  never-upgraded file once, with its backup; it does not fold a downgraded
  file.

## 3. Downgrade trace

- [x] T3.1 `backupSettingsFile` shared by the migration and the write.
- [x] T3.2 `MigrateSettingsReport` reports `Downgraded` and `NewerLayout`.
- [x] T3.3 `WriteSettings` backs up and logs a downgrade found on disk; the
      migration's own write skips it.
- [x] T3.4 `internal/agent/agent.go` logs both reports at start.
- Tests: a downgraded file is backed up once by the next save, with the
  timestamp suffix when `.bak-layout3` exists; the following save writes no
  new backup; the migration reports the downgrade and leaves one backup.

## 4. Regression over the Desktop save paths

- [x] T4.1 Agent test: a layout 3 file with project rules, then
      `MigrateSettingsReport`, then each save with the dialogs' payloads:
      `PUT /desktop/workstation`, `PUT /desktop/workstation/sandbox` with the
      base read, an empty base, a stale base and no base sending the folded
      values, `POST /desktop/projects` with `claudeSandbox` and
      `claudeSandboxBase`, `POST /desktop/project/sandbox/promote`, and
      `addProjectAllowRules`. Every folded rule stays in the workstation
      values after each one.
- [x] T4.2 Agent test: a simulated older writer (layout 3, `defaults` without
      `claudeSandbox`, `maxLayout` kept) over a folded file, then an "Always
      allow" and a save: the new rule stays on its project and no fold moves
      it.

## 5. Documentation

- [x] T5.1 `docs/contracts/server-agent-v1.md`: `maxLayout`, fold at start
      only, downgrade backup, refusal of a newer file.
- [x] T5.2 ADR 0050: amend "The project values move up once".
- [x] T5.3 `CHANGELOG.md`: a `Fixed` line under `[Unreleased]`.

## 6. Checks

- [x] `go build ./...`, `go vet ./...`, `go test ./internal/agentconfig/...
      ./internal/agent/...`, then the full `go test ./...`.
