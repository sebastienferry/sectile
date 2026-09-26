# ADR 0014: A tracker credential is personal, encrypted, and optionally sealed by its owner

Status: Accepted. Superseded in part by
[ADR 0028](0028-tracker-sync-uses-a-server-credential-per-provider.md) for
unattended work. Amended by
[ADR 0031](0031-unlocked-sealed-credentials-live-with-their-owners-presence.md)
for the lifetime of an unlock.

## Context

Until now every tracker credential was a single server-wide value: one row in
`settings`, constrained to `id = 1`. That is workable for GitHub, where a write
carries the repository's history rather than an identity anyone reads. It is
not workable for Jira, where a comment, an assignment and a transition are all
attributed to the account whose token made the call. With one shared token the
whole team signs as one integration account.

So a credential has to be personal. Which raises the question this record
answers: how to store it so that someone who obtains the database cannot use
somebody else's token.

The first instinct, hashing, does not apply. Sectile *verifies* a device token,
which is why `device_credentials` stores only a SHA-256 and can afford to. It
*presents* a tracker token, so the plaintext has to come back to build the
Authorization header. Encryption is the only option, and the entire value of it
is where the key lives.

## Decision

**A personal credential is encrypted with AES-256-GCM, bound to its owner.**
The additional authenticated data carries the user id and the tracker name. A
row moved from one user to another in the database stops opening, which is the
cheapest attack and the one the question was about. The nonce is random per
record, so two people storing the same token produce different rows.

**The server key lives outside the database**: `SECTILE_SECRET_KEY`, or a 0600
file beside the database file, generated on first use. A copy of the database
alone is useless. An operator who backs up the database without the key file
ends up with a backup that opens nothing, which is the intended shape.

**A sealing passphrase is offered, not imposed.** Its key is derived with
Argon2id from a passphrase that is never stored, in any form, not even hashed:
there is nothing to gain from being able to confirm a guess offline. The
derived key is held in memory for as long as the server runs and is forgotten
on restart.

**A locked credential fails the operation, loudly.** It never falls back to the
server token. Writing under the service account while somebody believes they
are acting as themselves is a misattribution, and misattribution is the problem
this whole record exists to fix.

**The acting user travels in the context**, not in every signature: every
`TicketingSystem` method already takes one. A context naming nobody resolves to
the server credential, which is what the background queue does.

## Consequences

- What this protects: a stolen or leaked copy of the database, and one user
  reusing another's row.
- What it does not protect: someone who is root on the server. They read the
  key file, and the server has to be able to decrypt to work at all. Claiming
  otherwise would be theatre, so it is written down here instead.
- A sealed credential cannot serve background work. Sectile writes to trackers
  from a queue in eleven places, so that is not a corner case: a person who
  seals their token accepts that their queued writes fail until they unlock.
  The interface says so at the moment of the choice rather than afterwards.
- Losing a passphrase costs nothing but re-entering the token, which Jira lets
  anyone recreate. No recovery path is offered, because a recovery path the
  server can walk is a second way in.
- The background queue still uses the server credential, because it has no
  acting user to resolve. Carrying one onto the job is ticket #237's work
  (`task_activities.user_id`), and this change deliberately does not duplicate
  it.
  ADR 0028 settles it the other way: every synchronisation uses the server
  credential of its provider, and none borrows a personal token.

## Alternatives rejected

- **Hash it, like `device_credentials`.** Impossible: the token has to be
  replayed, not compared.
- **Keep the key in the database.** The key would travel with the file it
  protects. It is worse than nothing, because it looks like protection.
- **Make the sealing passphrase mandatory.** Strongest on paper. Rejected: it
  would break every queued write for every user, and the queue is how Sectile
  writes to trackers.
- **Fall back to the server token when a personal one is locked.** Nothing ever
  fails, and every attribution becomes a lie. Rejected on those terms.
- **OAuth 2.0 three-legged authorisation with Atlassian**, storing a revocable
  scoped refresh token instead of a permanent one. The better long-term answer,
  and a much larger piece of work than the one at hand. Not closed off by
  anything here.
