# Tasks #746 - Desktop rewrites settings.json without a lock shared with the agent

Order matters: each step builds on the previous one. Tests go with the step
they cover.

## 1. Agent: pairing date and Desktop-only keys

- [x] T1.1 `Connection.PairedAt`; `WriteConnection` stamps it, holds
      `settingsMu`, drops `desktopOnlyKeys`.
- [x] T1.2 `desktopOnlyKeys` dropped by `storeSettings`; `MigrateSettingsReport`
      rewrites a current-layout file that still holds one, without a layout
      backup.
- [x] T1.3 `RecordDesktopConnection`.
- Tests: `WriteConnection` writes `pairedAt` and removes the Desktop-only keys
  while keeping the owned sections; `WriteSettings` and the start-up migration
  remove them and keep `server`, `deviceId`, `apiKey`; `RecordDesktopConnection`
  writes server and device, never a key, leaves a key for another server and
  its server alone, and does not rewrite an unchanged file.

## 2. Agent: start and key rotation

- [x] T2.1 `agent.go` reads `SECTILE_PAIRED_AT` and `SECTILE_PAIRED_DEVICE_ID`
      and records the connection at start.
- [x] T2.2 `newerStoredKey` applies the pairing-date rule when the start date
      is known.
- Tests: a stored key dated after the start date is newer; dated before, or
  undated, is not; without a start date the rule is today's.

## 3. Desktop: its own file

- [x] T3.1 `settings-file.cjs`: path, `readSharedSettings`,
      `readDesktopSettings` with the one-time migration,
      `updateDesktopSettings`, `effectiveCredential`, `sameServer`.
- [x] T3.2 `datadir.cjs`: `desktop.json` in `CARRIED_FILES`.
- Tests (`node --test`): migration from a shared file with `secret`, with
  `apiKey`, from the legacy `agent-settings.json`, never twice, never writing
  the shared file; `effectiveCredential` for a later-dated shared key, an
  earlier one, an undated one, another server; carry-over of `desktop.json`.

## 4. Desktop: every writer moves

- [x] T4.1 `saveCredential`, `storedDeviceId`, `settings`, `credential-state`,
      `save-settings`, appearance and console view handlers, `syncConsoleView`,
      `lifecycle`, the identity check and `openWindow` read and write
      Desktop's record.
- [x] T4.2 `startAgent`: effective credential, Desktop's `repo`, no write of
      the shared file, no merge of `.taskflow/agent.json`, spawn env
      `SECTILE_PAIRED_AT` and `SECTILE_PAIRED_DEVICE_ID`.
- [x] T4.3 UI tests that seed or read Desktop keys in `settings.json` move to
      `desktop.json` where they test Desktop's own storage, and keep
      `settings.json` where they test a key `pair` stored:
      `appearance.ui.cjs`, `console-view-sync.ui.cjs`, `pairing.ui.cjs`,
      `autostart.ui.cjs`, `run-folders.ui.cjs`, `conversation.ui.cjs`.
- Tests: a UI test saves the appearance, the console view, the connection,
  pairs and starts the agent, and asserts `settings.json` is byte for byte
  unchanged after each; an upgrade test starts Desktop on a pre-change
  `settings.json` and asserts appearance, console view and no pairing prompt;
  a `pair`-dated key in `settings.json` wins over an older Desktop key.
- Run: `npx vite build` before the UI suites, which load `dist/index.html`.

## 5. Documentation

- [x] T5.1 ADR 0053: Desktop keeps its own settings file; `settings.json` is
      written by the agent side only; the pairing date decides the newest key.
- [x] T5.2 ADR 0049: amend the key-storage paragraph (pairing date replaces
      "`pair` deletes `secret`" as the precedence rule; the standalone agent
      no longer reads Desktop's key).
- [x] T5.3 `docs/contracts/server-agent-v1.md`: the Desktop launch settings
      paragraph, the companion paragraph and the settings example (`pairedAt`,
      no Desktop keys).
- [x] T5.4 `CHANGELOG.md` `[Unreleased]`: `Fixed` and `Changed` lines (#746).

## 6. Verification

- [x] `go test ./internal/agentconfig/... ./internal/agent/...`
- [x] `node --test desktop/tests/*.test.cjs`
- [ ] Desktop UI suites, serially, after `npx vite build`.
- [x] `grep -n "settingsPath()+'.tmp'" desktop/electron/main.cjs` prints nothing.
