# Plan #746 - Desktop rewrites settings.json without a lock shared with the agent

## Stack

Electron main process (`desktop/electron`, CommonJS) and the Go agent
(`internal/agentconfig`, `internal/agent`). No server, database or web change.
No new dependency.

## Files

| File | Change |
| --- | --- |
| `desktop/electron/settings-file.cjs` (new) | Desktop's own file: path, read with one-time migration, write helper, effective credential |
| `desktop/electron/main.cjs` | every `readSettings()` and every write of `settingsPath()` goes through the new module; `startAgent` passes the pairing date and device |
| `desktop/electron/credential-store.cjs` | unchanged API; called on Desktop's own record |
| `desktop/electron/connection-settings.cjs` | `connectionView` reads the effective record |
| `desktop/electron/datadir.cjs` | `desktop.json` joins `CARRIED_FILES` |
| `internal/agentconfig/settings.go` | `desktopOnlyKeys` dropped by `storeSettings` |
| `internal/agentconfig/engines_migration.go` | `MigrateSettingsReport` rewrites a file that still holds a Desktop-only key |
| `internal/agentconfig/connection.go` | `PairedAt`; `WriteConnection` stamps it, drops the Desktop-only keys and holds `settingsMu`; new `RecordDesktopConnection` |
| `internal/agent/agent.go` | reads `SECTILE_PAIRED_AT` and `SECTILE_PAIRED_DEVICE_ID`, records the connection at start |
| `internal/agent/agent_mcp_settings.go` | `newerStoredKey` applies the pairing-date rule |
| `docs/adrs/0053-desktop-keeps-its-own-settings-file.md` (new), `docs/adrs/0049-...md`, `docs/contracts/server-agent-v1.md`, `CHANGELOG.md` | documentation |

## Design

### The two files

- `settings.json` (`~/.config/sectile/settings.json`, or `userData/settings.json`
  under `SECTILE_DESKTOP_DATA_DIR`, as `settingsPath()` already resolves it):
  written by the agent and `sectile-agent pair` only. Desktop reads it.
- `desktop.json` in `app.getPath('userData')`: written by Desktop only. The
  name differs from `settings.json` because the tests already place
  `settings.json` in `userData`.

Desktop's keys, `DESKTOP_KEYS`: `appearance`, `consoleView`, `repo`, `binary`,
`server`, `deviceId`, `secret`, `apiKey`, `pairedAt`. `pairedAt` is an RFC 3339
UTC timestamp.

### `settings-file.cjs`

- `desktopSettingsPath()`: `path.join(app.getPath('userData'), 'desktop.json')`.
  The module takes `app` (and `fs`) as parameters so `node --test` can drive it
  without Electron, as `datadir.cjs` does.
- `readSharedSettings()`: today's `readSettings()` (shared file, then the
  legacy `agent-settings.json`), read-only; `{}` when neither exists.
- `readDesktopSettings()`: parses `desktop.json`. When it does not exist, picks
  `DESKTOP_KEYS` from `readSharedSettings()`; when that yields any key, writes
  them to `desktop.json` (the migration, once) and returns them. A parse error
  of an existing `desktop.json` throws, as `readSettings()` does today, and the
  callers keep their `try` / fallback.
- `updateDesktopSettings(change)`: reads, applies `change` to a copy, writes
  `desktop.json.tmp` with mode 0600 and renames it, creating the directory with
  0700. Synchronous: every caller runs in the Electron main process, so cycles
  never interleave and no lock is needed.
- `effectiveCredential(desktop, shared, store)`: returns
  `{server, deviceId, token, state, pairedAt, source}`. Desktop's own key, unless
  `shared.apiKey` is set, `sameServer(shared.server, desktop.server)` (or
  Desktop has no server) and `shared.pairedAt` is later than
  `desktop.pairedAt`; a missing `pairedAt` compares as the oldest. `state` is
  `keyState` of the chosen record. `sameServer` moves here from `main.cjs`.

### `main.cjs`

- `saveCredential(server, credential)`: `updateDesktopSettings` with `server`,
  `deviceId`, `storeKey(...)`, `pairedAt: new Date().toISOString()`, and
  `delete binary`.
- `storedDeviceId(server)`: Desktop's `deviceId` for that server, else the
  shared file's for that server.
- `settings`, `credential-state`: from `effectiveCredential`.
- `save-settings`: `updateDesktopSettings` with `connectionUpdates(updates)`.
- `set-appearance`, `set-console-view` and their readers: `desktop.json`.
- `startAgent`: the stored key is the effective credential; `repo` comes from
  Desktop's record (default `path.dirname(settingsPath())`, as today); the
  write before the spawn becomes an `updateDesktopSettings` of `server`,
  `repo`, `deviceId`. The merge of the legacy `.taskflow/agent.json` into the
  file is dropped: the agent reads that legacy file itself
  (`ReadSettings(legacyRoot)`). The spawn adds `SECTILE_PAIRED_AT` (the chosen
  key's `pairedAt`, empty when none) and `SECTILE_PAIRED_DEVICE_ID`.
- `lifecycle`, the identity check, `openWindow`: read `server` and
  `appearance` from Desktop's record.
- `readSettings` and every `fs.writeFileSync(settingsPath()+'.tmp', ...)` are
  gone from `main.cjs`; a grep for `settingsPath()+'.tmp'` returns nothing.

### Agent

- `desktopOnlyKeys = []string{"appearance", "consoleView", "repo", "binary", "secret"}`.
  `storeSettings` deletes them with the legacy keys. `MigrateSettingsReport`
  does not return early while the raw file holds any of them; it then writes
  without a layout backup when nothing else changed (the removal is not a
  layout conversion).
- `Connection.PairedAt time.Time` (`json:"pairedAt,omitempty"`), read by
  `ReadConnection`. `WriteConnection` takes `settingsMu`, stamps `pairedAt` with
  the current UTC time when it writes a key, keeps deleting `secret`, and drops
  `desktopOnlyKeys`.
- `RecordDesktopConnection(server, deviceID string) error`: under
  `settingsMu`, sets `server` and `deviceId` only when the file holds no
  `apiKey` or holds one for the same server; leaves the file untouched when
  nothing changes. It never writes a key.
- `agent.go`: after resolving the server and the token, when
  `SECTILE_PAIRED_DEVICE_ID` is set, calls `RecordDesktopConnection` and logs a
  failure without stopping. `SECTILE_PAIRED_AT` is parsed into
  `d.link.pairedAt` (zero when empty or invalid).
- `newerStoredKey`: when `d.link.pairedAt` is set, a stored key is newer only
  when its `PairedAt` is after it (an undated stored key is never newer);
  when it is not set (an agent started by hand), the rule is today's.
- `resolveCredential` is unchanged: a Desktop-started agent always has `TOKEN`.

### Rejected alternatives

From the clarification: a cross-process lock file, routing Desktop's writes
through the agent, a compare-and-swap with retry. Removing Desktop-only keys
at start only, rather than on every save, was rejected: a file already at the
current layout would keep them until an unrelated save.

## Risks

- A Desktop older than this change, run after the agent removed `secret`,
  asks to pair again (accepted in the clarification).
- A workstation that relied on the standalone agent reading Desktop's key in
  clear needs `sectile-agent pair` once (accepted, `Changed` line).
- The pairing-date comparison needs clocks that agree on one machine only:
  both stamps come from the same host.
