# ADR 0032: An unlocked sealed credential lives with its owner's presence

Status: Accepted

Amends: [ADR 0014](0014-personal-tracker-credentials-are-sealed.md), its
sentence "The derived key is held in memory for as long as the server runs and
is forgotten on restart."; and
[ADR 0030](0030-several-server-replicas-share-one-postgresql.md), its row on
keys derived from sealing passphrases (#409) and the alternative it rejected.

## Context

A person may seal their personal tracker token behind a passphrase (ADR 0014).
Unlocking it kept the key the passphrase derives in the memory of the process
that received it. Two things broke that model (#501):

- **Restarts.** Every merge into `main` redeploys the server, 32 times on
  2026-09-25. The browser session survives a restart, since it lives in the
  database, but the derived key did not. So the person stayed signed in while
  their sealed token locked itself, and their next write, or a write Sectile
  made later on their behalf (ADR 0029), failed with "unlock it".
- **Replicas.** With several instances sharing one database, an unlock reached
  only the instance that served it. The others answered "locked". #409 (ADR
  0030) answered that by relaying the wrapped key between the live instances,
  in memory only, with a lock generation in the database. It still loses every
  key when all instances restart together, which a redeploy does, and always on
  a single instance: the restart complaint stays.

Nothing expired an unlock either: it lasted as long as the process, including
after its owner had signed out.

The clarification is in `docs/clarifications/501.md` and the specification in
`specs/501-unlocked-credentials-survive-restarts/`.

## Decision

**The derived key is stored in the database, sealed under the server key.**
Table `user_credential_unlocks`, one row per person and tracker, as the
credential it opens. The record is bound (AES-GCM additional data) to its owner
and its tracker under a prefix of its own, so it opens neither for somebody
else nor as a credential. It survives restarts and every instance reads it.

**It lasts while its owner is connected, then 30 minutes.** Connected means an
open browser session seen within the last 30 minutes, or a local agent held by
a live instance. The owner's last presence is the latest of: their open
sessions' last requests, their agents' last signs of life (now while held, the
disconnection, or the last heartbeat of the instance that died holding it) and
the unlock itself. The window is fixed; there is no setting.

**Each instance sweeps once a minute, and once before it serves.** The sweep is
one `DELETE` computed from `web_sessions` and `agent_presence`, so every
instance reaches the same answer, and two sweeping at once forget each unlock
once. No instance keeps a clock of anyone's activity. An unlock past its window
can therefore be used for at most one more minute.

**The agent presence outlives its row.** When an instance is reclaimed, or a
single-process server restarts, the `agent_presence` rows go. Their last sign
of life is first copied into the unlock (`agent_seen_at`), so an agent that
reconnects after a redeploy finds its owner's token still unlocked.

**Leaving locks at once.** Signing out forgets the person's unlocks when, after
it, they have no other session seen within the window and no connected agent;
the time of the unlock does not count then. Lock, Lock all, re-storing or
deleting the credential, blocking or deleting the account forget them whatever
the presence.

**The unlock belongs to the person, not to one session row.** The clarification
said "attached to the person's server-side web session". Read literally, a
foreign key to one `web_sessions` row, it contradicts two of its own decisions:
an agent keeps the unlock alive with no tab open, and signing out of one of
several sessions keeps it. The row is keyed like the credential, and its life is
bounded by the presence the sessions and agents provide.

## Consequences

- During the window an unlock lasts, a copy of the database **together with**
  the server key opens the sealed token. Before, the same attacker needed the
  memory of the running process. Someone who is root on the server already had
  both, so the practical change is for a stolen backup that includes the key
  file, which ADR 0014 already tells operators not to make.
- A copy of the database alone still opens nothing: the unlock is sealed under
  the server key, and the credential under the passphrase.
- Unlocking now needs the server key. Without it the unlock is refused with an
  error naming `SECTILE_SECRET_KEY`, and a sealed credential saved without it
  is stored locked, rather than claiming an unlock that would not outlive the
  process.
- The relay of #409 is gone: the in-memory key map, the internal route
  `/internal/credentials/keys`, and the pull of held keys at start. Lock is a
  `DELETE`, seen by every instance at its next read. The column
  `user_tracker_credentials.unlock_generation` (migration 28) is left unused:
  dropping it would take a migration of its own for no behaviour.
- An unlock is not copied by `sectile-migrate`: people unlock again on the
  destination.
- A redeploy during working hours no longer locks the sealed tokens of people
  who stay connected, and no sealed token is usable more than 31 minutes after
  its owner left.

## Alternatives rejected

- **Keep the key in memory.** It is what broke.
- **Keep #409's relay and only add the idle expiry and the lock on sign-out.**
  Nothing is written down, but a redeploy that restarts every instance still
  locks everyone, which is the complaint of #501. The owner chose the database
  (answer A on #501).
- **Hold the key in the browser**, sent with each request. The writes Sectile
  makes later on a person's behalf, and the ones an agent causes, have no
  browser to ask.
- **A foreign key to one session row.** See the decision above.
- **An `expires_at` pushed forward on every presence.** It writes on every
  session touch and every agent heartbeat, from several places; computing
  expiry from the presence rows that already exist needs no new write path.
- **Check the expiry when the credential is resolved.** Exact to the second, but
  it adds the presence subqueries to every tracker call, including from paths
  that hold the store's lock. The one-minute sweep meets the requirement.
- **A setting for the delay.** The owner chose a fixed 30 minutes.
