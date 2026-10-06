# Specification #746 - Desktop rewrites settings.json without a lock shared with the agent

- Ticket: https://github.com/sebastienferry/sectile/issues/746
- Branch: `feat/746`
- Clarification: `docs/clarifications/746.md` (rounds 1 to 3, confirmed by the
  owner on 2026-10-06)
- Framework: Spec Kit

## Summary

Sectile Desktop and the local agent both rewrite
`~/.config/sectile/settings.json`, with nothing serializing them, so a save by
one can silently revert what the other just saved. The fix gives that file a
single writer on the Desktop side: Desktop keeps what it stores in a file of
its own and only reads the shared file. The two never write the same file
again, so neither can revert the other.

## Scope

In scope: Desktop's own settings file and its one-time migration, Desktop no
longer writing the shared file, the agent removing what only Desktop used
from the shared file, the choice of the newest key between a Desktop pairing
and a `sectile-agent pair`, the agent recording the public part of the
connection Desktop starts it with, the documentation and the changelog.

Out of scope (decided in the clarification):

- A cross-process lock file, a compare-and-swap, or routing Desktop's writes
  through the agent (rejected for option D).
- The race between `sectile-agent pair` and a running agent saving at the same
  moment.
- Two agents alive at once during a restart.
- Binaries already released: they keep their behaviour.

## User stories

### US1 - A Desktop save never reverts an agent setting (P1)

As the owner, when I change the appearance, the console view, the connection
or pair the workstation in Desktop, nothing the agent saved is lost.

1. **Given** a running agent and Desktop open, **when** Desktop saves the
   appearance, the console view, the connection settings, a pairing, or
   starts the agent, **then** `settings.json` is byte for byte what it was
   before that save.
2. **Given** an agent save (workstation defaults, project settings, Claude
   settings, engines, repositories) made at any moment relative to a Desktop
   save, **when** both are done, **then** both changes are in effect.

### US2 - Upgrading keeps every Desktop setting (P1)

As the owner of a workstation set up before this change, after the upgrade
Desktop opens as it was.

1. **Given** a `settings.json` holding the appearance, the console view, the
   repository folder, the server, the device and the key Desktop stored (in
   clear or encrypted), and no Desktop file yet, **when** Desktop starts,
   **then** it shows the same appearance and console view, starts the agent
   on the same server with the same key and folder, and does not ask to pair.
2. The legacy `agent-settings.json` fallback is migrated the same way when no
   `settings.json` exists.
3. The migration runs once: a Desktop file that exists is never overwritten
   from `settings.json`.
4. **Given** the upgraded agent starts, **then** `settings.json` no longer holds
   the appearance, the console view, the repository folder or the encrypted
   key, and still holds the server, the device and any key in clear.

### US3 - The newest pairing wins (P1)

1. **Given** Desktop paired the workstation, **when** the owner then runs
   `sectile-agent pair` for the same server, **then** Desktop starts the agent
   on the key `pair` stored.
2. **Given** `sectile-agent pair` stored a key, **when** the owner then pairs
   again from Desktop, **then** Desktop uses its own new key, and the running
   agent never takes the older `pair` key for a newer one.
3. **Given** a key in `settings.json` written before this change (no pairing
   date), **then** it is older than any dated key: it neither overrides a
   Desktop pairing nor is taken by the agent as newer than the key Desktop
   started it with.
4. A key stored for another server is never used.

### US4 - A Desktop pairing and a CLI pairing name the same device (P2)

1. **Given** Desktop paired the workstation, **when** the owner runs
   `sectile-agent pair` for the same server, **then** the server replaces that
   device's key instead of registering a second device.
2. **Given** `sectile-agent pair` paired the workstation, **when** the owner
   pairs again from Desktop, **then** the same holds.

### US5 - The standalone agent pairs on its own (P2)

1. **Given** a workstation paired only through Desktop, **when** the owner runs
   `sectile-agent` by hand without `--token` or `TOKEN`, **then** it reports
   that no key is stored for the server and names `sectile-agent pair`, as it
   already does on macOS and Windows today.
2. The agent never writes an API key to `settings.json` on Desktop's behalf.

## Functional requirements

- FR1 Desktop stores its keys (appearance, console view, repository folder,
  server, device, key in clear or encrypted, pairing date) in its own file in
  its data directory, and never writes `settings.json`.
- FR2 When its own file does not exist, Desktop creates it once from
  `settings.json`, or the legacy `agent-settings.json`, without writing either.
- FR3 The agent removes from `settings.json` the keys only Desktop used
  (appearance, console view, repository folder, encrypted key), at start and
  on every save.
- FR4 Every pairing records the moment it was made beside its key: Desktop in
  its own file, `sectile-agent pair` in `settings.json`.
- FR5 Desktop uses the key in `settings.json` instead of its own only when it
  is for the same server and its pairing date is later than Desktop's. A key
  with no date is older than any dated one.
- FR6 The running agent takes a key stored in `settings.json` as newer than the
  one it started with only under the rule of FR5, compared with the date of
  the key Desktop handed it.
- FR7 A Desktop pairing replaces the device `settings.json` names when Desktop
  has none of its own for that server; when `settings.json` holds no key, the
  agent Desktop starts records there the server and the device of the pairing
  it was started on, never the key.
- FR8 The Desktop data directory carry-over (app rename) carries Desktop's own
  file too.
- FR9 Log lines and error messages keep the language of their neighbours.

## Acceptance

- Tests cover US1 to US5, including a UI test asserting `settings.json` is
  unchanged after each Desktop save path, the migration from a file with an
  encrypted key and from one with a key in clear, and the pairing-date rule on
  both sides.
- `CHANGELOG.md` gains, under `[Unreleased]`, a `Fixed` line (a Desktop save no
  longer silently reverts a workstation setting the agent saved at the same
  moment) and a `Changed` line (the standalone agent no longer picks up the
  key Desktop paired with; pair it with `sectile-agent pair`).
- A new ADR records the single writer and Desktop's own file; ADR 0049 and
  `docs/contracts/server-agent-v1.md` describe the pairing date instead of
  "`pair` deletes `secret`".
