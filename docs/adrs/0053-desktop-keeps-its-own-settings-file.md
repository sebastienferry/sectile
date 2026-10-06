# ADR 0053: Sectile Desktop keeps its own settings file

- Status: Proposed
- Date: 2026-10-06
- Issue: [#746](https://github.com/sebastienferry/sectile/issues/746)
- Amends: [ADR 0049](0049-workstations-sign-in-through-the-browser.md), its key
  storage

## Context

Sectile Desktop and the local agent both rewrote
`~/.config/sectile/settings.json`. Each read the whole file, changed its own
keys and renamed a new file over it, and nothing serialized the two processes:
the agent's `settingsMu` only orders the agent's own cycles. A save landing
between the other process's read and its rename was lost without a trace, and
any agent-owned value could be reverted that way.

Launched by Desktop, the agent gets what it needs from its arguments and
environment (`--url`, `--repo`, `TOKEN`). Of what Desktop wrote, only the
connection was read by anyone else: the standalone agent and
`sectile-agent pair`. The encrypted key (`secret`) was readable by Desktop
alone, so on macOS and Windows the standalone agent never reused a Desktop
pairing anyway.

A lock shared by the two processes, a compare-and-swap with retry and routing
Desktop's writes through the agent were weighed and rejected: the first needs
a portable lock in Node and stale-lock rules, the second narrows the race
without closing it, the third leaves the writes made before any agent runs.

## Decision

**One writer per file.** Desktop keeps what it stores (appearance, console
view, repository folder, server, device, its key and the moment it was
paired) in `desktop.json` in its data directory, and never writes
`settings.json`. It reads `settings.json` for a key `sectile-agent pair`
stored and for the device to replace on a new pairing. `settings.json` is
written by the agent and by `sectile-agent pair` only.

**A one-time migration.** When `desktop.json` does not exist, Desktop copies
its keys from `settings.json` (or the legacy `agent-settings.json`) into it,
leaving `settings.json` as it is. The agent removes from `settings.json` the
keys only Desktop used (`appearance`, `consoleView`, `repo`, `binary`,
`secret`) at start and on every save.

**The pairing date decides the newest key.** Every pairing records `pairedAt`
beside its key, Desktop in `desktop.json`, `sectile-agent pair` in
`settings.json`. Desktop uses the `settings.json` key instead of its own only
when it is for the same server and paired later, or when Desktop holds no key.
A key without a date, stored before this decision, is older than any dated
one. This replaces "`pair` deletes `secret`" of ADR 0049, which only worked
while both lived in one file.

**The agent learns its key's date and device.** Desktop starts the agent with
`SECTILE_PAIRED_AT` and `SECTILE_PAIRED_DEVICE_ID`. The agent takes a key
stored in `settings.json` as newer than its own only when it was paired later,
and, when `settings.json` holds no key, records there the server and device of
the pairing it was started on, never the key, so `sectile-agent pair` replaces
the same device. A stored key keeps the device it was paired as. An agent
started by hand keeps the rule of ADR 0049.

**The standalone agent pairs on its own.** It no longer reuses a key Desktop
stored, on any platform: `sectile-agent pair` or `TOKEN`.

## Consequences

- Desktop and the agent can no longer revert each other's settings.
- A workstation that relied on the standalone agent reading the key Desktop
  stored in clear (a host with no OS key store) runs `sectile-agent pair`
  once.
- A Desktop older than this change, run after the agent removed `secret` from
  `settings.json`, asks to pair again.
- `sectile-agent pair` run while the agent saves remains two processes writing
  one file; it is rare, interactive, and outside this decision.
- A new Desktop setting goes to `desktop.json`; a new agent setting goes to
  `settings.json`. Neither process writes the other's file.
