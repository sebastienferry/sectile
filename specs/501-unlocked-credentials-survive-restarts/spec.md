# Specification #501 - An unlocked sealed credential stays unlocked while its owner is connected

- Ticket: https://github.com/sebastienferry/sectile/issues/501
- Branch: `feat/501`
- Clarification: `docs/clarifications/501.md` (rounds 1 to 3, owner confirmed
  on 2026-09-26, no open product question)
- Framework: Spec Kit

## Summary

A person can seal their personal tracker token behind a passphrase (ADR 0014).
Today, once they unlock it, the unlock lasts only as long as the server process
that received it. Every merge into `main` redeploys the server, and with several
replicas an unlock only reaches one of them. So the person stays signed in (their
browser session survives the restart) while their sealed token silently locks
itself, and their next write, or a write Sectile makes later on their behalf,
fails with "unlock it".

After this change, an unlocked sealed credential stays unlocked on every
replica and across restarts for as long as its owner is connected. It locks
itself 30 minutes after they stop being connected, and at once when they sign
out of their last open browser session with no local agent connected.

## Scope

In scope:

- the lifetime of an unlock of a sealed personal tracker credential, on every
  provider (GitHub, Jira, GitLab);
- what "connected" means for that lifetime;
- what signing out, locking, blocking an account, re-storing and deleting a
  credential do to an unlock;
- ADR 0014's "forgotten on restart", `README.md` and the release note.

Out of scope:

- how a credential is sealed (passphrase, Argon2id, AES-256-GCM binding): unchanged;
- unsealed personal credentials: the server key always opens them, as today;
- server tracker credentials (ADR 0028, ADR 0029);
- the unlock surfaces (sign-in screen, *Profile → Tracker credentials*): no new
  screen, no new field;
- a setting for the 30-minute delay: it is fixed.

## Definitions

- **Sealed credential**: a personal tracker credential its owner protected with
  a sealing passphrase.
- **Unlock**: the owner giving the passphrase (sign-in screen or profile), which
  makes the sealed credential usable without the passphrase for a while.
- **Locked**: a sealed credential that is not unlocked. Any use fails with the
  existing "locked, unlock it" error and never falls back to the server
  credential.
- **Open browser session**: a browser session of the person that is neither
  signed out nor expired (12 hours after sign-in).
- **Connected**: the person has an open browser session that was seen within
  the last 30 minutes, **or** one of their local agents is connected to the
  server. A session is "seen" when it makes a request; an open tab keeps it seen
  through the event stream.
- **Idle window**: 30 minutes, fixed.

## User stories (prioritised)

### US1 - A restart does not lock my token (P1)

As a person who unlocked my sealed token and keeps working, I want it to stay
unlocked when the server restarts or redeploys, so that I am not asked for my
passphrase again, and my writes do not fail, while I am still signed in.

**Acceptance**

- **Given** a person who unlocked their sealed Jira credential and has a tab
  open, **when** the server restarts, **then** after the restart their profile
  still shows the credential unlocked and their next ticket edit is written
  under their Jira account without asking for the passphrase.
- **Given** the same person, **when** a write they caused earlier is queued and
  runs after the restart, **then** it is made under their credential.
- **Given** a person who unlocked their credential and then stopped being
  connected more than 30 minutes before the server restarts, **when** the
  server comes back, **then** the credential is locked.

### US2 - Every replica sees my unlock (P1)

As a person working against a server with several replicas, I want an unlock
made on one replica to hold on all of them, so that whichever instance serves my
request or runs my queued write finds my token usable.

**Acceptance**

- **Given** two replicas sharing a database and a person who unlocked their
  credential through replica A, **when** a request or a queued write of theirs
  is handled by replica B, **then** it is made under their credential.
- **Given** the same, **when** the person presses "Lock all" on replica A,
  **then** replica B refuses the next use of the credential as locked.
- **Given** the same, **when** the idle window expires, **then** both replicas
  treat the credential as locked.

### US3 - My token locks itself once I have left (P1)

As a person who unlocked their sealed token, I want it to lock again by itself
30 minutes after I stop being connected, so that an unlock does not outlive my
presence.

**Acceptance**

- **Given** a person who unlocked their credential, closed every tab and has no
  local agent connected, **when** 30 minutes have passed since their last
  browser request, **then** within the following minute the credential is
  locked, and a write made on their behalf fails with the "locked, unlock it"
  error, never under the server credential.
- **Given** a person who unlocked their credential and keeps a tab open for
  several hours, **then** the credential stays unlocked for as long as the tab
  keeps the session seen, up to the end of the browser session itself.
- **Given** a person who unlocked their credential less than 30 minutes ago,
  **then** it is never considered idle, even if their browser sessions were
  last seen longer ago than that.
- **Given** a person whose credential locked itself, **when** they come back,
  **then** they unlock it as today, from the profile or by signing in with the
  passphrase.

### US4 - A running agent keeps my token unlocked (P1)

As a person running a long agent session with no browser tab open, I want my
token to stay unlocked while my local agent is connected, so that the run can
still post its stage transition under my name (ADR 0029).

**Acceptance**

- **Given** a person who unlocked their credential and then closed every tab,
  **when** one of their local agents stays connected for two hours, **then** the
  credential stays unlocked for those two hours, and the stage the run reports
  is written under their credential.
- **Given** the same, **when** the agent disconnects, **then** the credential
  locks 30 minutes after the disconnection unless the person is connected
  otherwise by then.
- **Given** a person whose only presence is an agent connected to a replica
  that died without closing it, **then** the agent stops counting as connected
  once that replica is considered dead, and the 30-minute window starts from
  the replica's last sign of life.

### US5 - Signing out of my last session locks my token at once (P1)

As a person leaving Sectile, I want signing out to lock my sealed token
immediately when nothing else of mine is connected.

**Acceptance**

- **Given** a person with a single open browser session and no local agent
  connected, **when** they sign out, **then** their sealed credentials are
  locked at once, on every replica.
- **Given** a person with a second browser session seen within the idle window,
  **when** they sign out of one, **then** their credentials stay unlocked.
- **Given** a person with a local agent connected, **when** they sign out of
  their last browser session, **then** their credentials stay unlocked, and the
  idle window applies from the moment the agent disconnects.
- **Given** a person whose other browser sessions were last seen more than 30
  minutes ago, **when** they sign out, **then** their credentials are locked at
  once.

### US6 - Explicit locks still act at once (P2)

**Acceptance**

- **Given** an unlocked credential, **when** its owner presses "Lock" or
  "Lock all", **then** it is locked at once, on every replica.
- **Given** an unlocked credential, **when** its owner stores a new token or a
  new passphrase for it, **then** the old unlock is forgotten; saving a sealed
  credential unlocks it with its new passphrase, as today.
- **Given** an unlocked credential, **when** its owner deletes it, **then** the
  unlock is forgotten with it.
- **Given** an unlocked credential, **when** an admin blocks its owner's
  account, **then** it is locked at once, whatever agent of theirs is connected.

### US7 - The operator knows what an unlock now costs (P2)

As an operator, I want the documentation to say where an unlocked key lives
and what a leak of it opens.

**Acceptance**

- **Given** the documentation, **then** a new ADR amends ADR 0014: during the
  window an unlock lasts, a copy of the database **together with** the server
  key opens the sealed token; a copy of the database alone still opens nothing.
- **Given** `README.md` § personal credentials and § sign-in, **then** they say
  that an unlock survives restarts and lasts while the person is connected,
  then 30 minutes.
- **Given** `CHANGELOG.md`, **then** `[Unreleased]` carries a `Fixed` line
  saying sealed tracker tokens no longer lock themselves when the server
  restarts, and lock 30 minutes after their owner leaves (#501).

## Functional requirements

- **FR-1** An unlock of a sealed credential is held by the server, not by one
  server process: it survives a restart and is honoured by every replica sharing
  the database.
- **FR-2** An unlock stays valid while its owner is connected (see
  *Definitions*), and ends once the owner's last presence is older than the idle
  window. The owner's last presence is the latest of: the last time one of their
  open browser sessions was seen, now while one of their local agents is
  connected, the time their last agent disconnected (or its replica was last
  alive), and the time of the unlock itself.
- **FR-3** An expired unlock is forgotten no later than one minute after the end
  of the idle window, and before a server that was stopped serves its first
  request.
- **FR-4** Every replica computes expiry from the same shared presence data;
  none keeps its own clock of a person's activity.
- **FR-5** Signing out of a browser session forgets the person's unlocks at once
  when, after that sign-out, they are not connected (no other open browser
  session seen within the idle window, no local agent connected). The time of
  the unlock does not keep it alive in that case.
- **FR-6** "Lock", "Lock all", re-storing a credential, deleting it and blocking
  the account forget the matching unlocks at once, on every replica.
- **FR-7** A locked credential fails the operation with the existing error and
  never falls back to the server credential (ADR 0014, ADR 0029). Unsealed
  credentials are not affected by any of this.
- **FR-8** The stored unlock never exposes the passphrase or the token: a copy
  of the database alone opens neither, and an unlock row moved to another
  person or another tracker opens nothing.
- **FR-9** If the server key is unavailable, unlocking a sealed credential is
  refused with an explicit error naming the missing server key, instead of
  pretending to succeed.
- **FR-10** The profile's "unlocked" state of each sealed credential reflects the
  shared unlock, identically on every replica.
- **FR-11** The idle window is a fixed 30 minutes; no setting is added.

## Edge cases

- The person never had an unlock: nothing changes; the credential is locked.
- The person unlocked through the desktop app's credential routes with a device
  key and has neither a browser session nor an agent connected: the unlock time
  keeps it valid for 30 minutes, then it locks (FR-2).
- A browser session expires (12 hours) while the person is inactive: it stops
  counting as presence and the idle window runs from its last request.
- Two unlocks of the same credential from two replicas at once: the result is
  one unlock, the latest one.
- A replica that is down for longer than the idle window: on start it forgets
  the unlocks whose owners were not connected meanwhile (FR-3).
- A stored unlock that the current server key cannot open (the key was
  replaced): it is treated as locked and forgotten.

## Success criteria

- A redeploy during working hours no longer locks the sealed credentials of
  people who stay connected; no "locked, unlock it" failure follows a restart
  for a connected person.
- No sealed credential is usable more than 31 minutes after its owner stopped
  being connected.

## Open requirements

None. The clarification settled every product decision. Its wording "attached
to the person's web session" is read together with its decisions 3 and 4 (an
agent keeps the unlock alive with no tab open; signing out of one of several
sessions keeps it): the unlock belongs to the person and lives as long as their
presence, rather than to one session row. `plan.md` records that choice.
