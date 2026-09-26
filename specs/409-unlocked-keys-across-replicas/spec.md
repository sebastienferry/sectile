# #409: Passphrase-unlocked credentials usable on every replica

Parent macro: #397 (child ticket 7, S9, FEAT-4 / US-5). Clarification:
[`docs/clarifications/409.md`](../../docs/clarifications/409.md) (Rounds 1-2). Depends
on #406 (merged), whose internal channel it reuses.

## Problem

A person may seal their tracker credential behind a passphrase. The key derived from
it lives only in the memory of the server that received the passphrase. When several
server instances share one PostgreSQL database behind a load balancer, the credential
is unlocked on one replica and locked on the others, so a tracker call served by
another replica fails with "locked" although the person just unlocked it.

## User stories

### US1 (P1): unlock once, use it through any replica

- **Given** a sealed credential and two live replicas A and B,
  **when** its owner unlocks it through A,
  **then** a tracker call acting as that person and served by B opens the credential,
  and B's credentials list reports it unlocked.
- **Given** the credential was unlocked through A,
  **when** a replica C starts afterwards,
  **then** C can open it too, without the owner unlocking again.
- **Given** a sealed credential stored through A with its passphrase,
  **then** it is unlocked on B as well, exactly as a fresh unlock would be.

### US2 (P1): lock, replace and delete hold everywhere

- **Given** a credential unlocked on A and B,
  **when** its owner locks it through either,
  **then** neither replica can open it any more, including a replica that was not
  reachable at that moment.
- **Given** a credential unlocked on A and B,
  **when** its owner stores it again (new token or new passphrase) or deletes it,
  **then** no replica opens the new record with a key derived for the previous one.

### US3 (P1): the key is never written down

- **Given** any unlock, lock, store or start of a replica,
  **then** no derived key is written to the database, to a log line or to a file, and a
  derived key crossing the internal channel is encrypted.

### US4 (P2): degraded but safe

- **Given** a peer is unreachable when a key changes,
  **then** the person's request still succeeds, the failure is logged without key
  material, and the lock guarantee of US2 still holds.
- **Given** a server without a server key, or on SQLite,
  **then** nothing is shared and the single-instance behaviour is unchanged.

## Functional requirements

- **FR1**: Unlocking a sealed credential, and storing a sealed one, makes its key
  available to every live replica.
- **FR2**: A replica that starts on a shared store obtains the keys its live peers hold.
- **FR3**: Locking a credential makes it unopenable on every replica, whether or not the
  replica received the lock message.
- **FR4**: Storing a credential again or deleting it makes every key derived for its
  previous record unusable on every replica.
- **FR5**: A key received from a peer is kept only if it opens the credential's current
  record at its current lock generation.
- **FR6**: A derived key crosses the internal channel only encrypted under the server key
  and bound to its owner and tracker; the channel requires the internal credential.
- **FR7**: No derived key, wrapped or not, is written to the database, the logs or disk.
- **FR8**: A failure to reach a peer never fails the owner's request.
- **FR9**: On a store shared with nobody (SQLite, or no internal credential), behaviour is
  unchanged apart from the lock generation check, which is invisible there.

## Acceptance criteria

- A credential unlocked through replica A works for tracker calls served by replica B.
- No derived key is written to the database, logs or disk.
- A lock through A makes the credential locked on B even when B missed the broadcast.
- A replica started after the unlock can use the credential.
- CHANGELOG: one line under `[Unreleased]`, shared with #410's multi-replica entry (the
  change is visible to operators and to users of sealed credentials).

## Out of scope

SQLite multi-instance, unsealed credentials (the shared server key opens them
everywhere), the sealing format, and the passphrase itself (never sent, never stored).
