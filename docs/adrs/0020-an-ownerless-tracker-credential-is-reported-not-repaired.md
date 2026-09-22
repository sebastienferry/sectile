# ADR 0020: An ownerless tracker credential is reported, never repaired

Status: Accepted

Completes [ADR 0014](0014-personal-tracker-credentials-are-sealed.md), whose
binding is what makes the repair impossible, and
[ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md), whose
retirement of the implicit visitor is what produced the rows.

## Context

`user_tracker_credentials` is keyed on `(user_id, tracker)` and nothing
constrains `user_id` to a row in `users`. Before sign-in became mandatory, an
anonymous visitor resolved to `default`, so a token entered then was stored in
its name. ADR 0015 kept `default` as an ordinary account and migrated nothing,
which is right for a deployment where the row exists. On a deployment where it
never did — nothing ever paired a workstation, nothing ever called `EnsureUser`
— the credential is left under an identity the users table does not know.

The observed failure is a deployment whose only Jira access is stored under
`default`, while `users` holds two real accounts. Every signed-in person misses
it: `UserTrackerCredentialsFor` finds nothing under their own id, the resolution
falls back to the empty server credential, and the call fails on a missing
account e-mail. The token itself is fine; nobody can reach it.

The row is not inert, which is what makes this worth deciding rather than
sweeping up. `CallOperation` still defaults an operation carrying no user id to
`ImplicitUser`, unconditionally, and `resolveAgentCredential` still ends on it
for a deployment that pins `SECTILE_SERVER_TOKEN`. So `default`'s credential is
still resolvable — by an identity nobody can sign in as, and therefore nobody
can revoke from the interface.
[ADR 0019](0019-the-machine-surfaces-have-no-open-mode.md) narrowed that reach
by removing the legacy open mode, so fewer paths get there than before; none of
them disappeared.

## Decision

**Sectile reports these rows and changes none of them.** A startup scan names
each one in the log — the identity, the tracker, the account the token was
entered for — and `GET /api/me/tracker-credentials` carries the same finding, so
Profil > Trackers can say why a personal access looks absent while the tracker
behaves as if one were configured.

**No automatic rebinding.** The server can technically open an unsealed record
under the old binding and re-seal it under a new one, and that is exactly the
recovery path ADR 0014 refused: one the server can walk is a second way in.
Nothing in the database establishes whose token it is either — the stored
e-mail names a tracker account, not a Sectile one — and the "only one real
account" heuristic does not even hold on the deployment that raised this. A
sealed credential settles the question rather than merely declining it: the
server cannot open it at all.

**No automatic deletion.** The agent paths may be resolving the row, and an
upgrade that silently breaks a working setup is worse than one that says what it
found. This is the one argument ADR 0019 weakened rather than removed; the two
above stand on their own.

**Discarding it is an explicit act, and an admin's.** `DELETE
/api/me/tracker-credentials/orphaned` removes one row, and the storage layer
checks ownerlessness in the same statement as the delete, so the route cannot be
turned into a way to delete a colleague's token by naming their id.

**The way out is the one ADR 0014 already accepts for a lost passphrase**:
re-enter the token under your own account, which Jira lets anyone recreate.

## Consequences

- The failure stops being silent. It was previously indistinguishable from a
  tracker misconfiguration, and the message it produced — "configure the Jira
  account e-mail" — pointed at a field that was in fact filled in.
- A deployment can leave the row in place indefinitely with no further harm,
  which is the correct outcome where the agent paths still use it.
- The report is visible to every signed-in person as a count and the trackers it
  occupies, since that is what explains their own missing access. The identity
  behind it and the discard action are an admin's.
- For `default` specifically there is a second, non-destructive exit ADR 0015
  already names: an admin claims the account. Writing the `users` row ends the
  orphan status without touching the record, so the token keeps opening.
- Nothing here adds a foreign key on `user_tracker_credentials.user_id`. It
  would have refused the row at insert time, years ago, and today it would only
  turn a readable leftover into a failed migration.

## Alternatives rejected

- **Rebind to the sole real account when the deployment has exactly one.**
  Silently hands one person's Jira identity to another account, cannot work for
  a sealed credential, and would not have applied to the deployment that raised
  the question. The count of accounts is not evidence of ownership.
- **Delete them at startup.** Destroys, without asking, a credential the agent
  paths may be using, and teaches the operator nothing about why their tracker
  stopped working.
- **Offer an admin a "claim this token" button.** The same rebinding, behind a
  click. An admin adopting a token attributed to somebody else's tracker account
  is the misattribution ADR 0014 exists to prevent, seen from the other side.
- **Leave it entirely undetected.** The status quo, and the reason the failure
  took an inspection of the database to explain.
