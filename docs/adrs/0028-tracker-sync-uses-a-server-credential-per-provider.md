# ADR 0028: Tracker sync uses one server credential per provider

Status: Accepted; amended by [ADR 0029](0029-server-tracker-credential-signs-unattended-work-only.md)

Supersedes: [ADR 0018](0018-the-background-synchronisation-runs-as-the-project-owner.md).
Supersedes in part: [ADR 0014](0014-personal-tracker-credentials-are-sealed.md),
its consequence "The background queue still uses the server credential";
completes
[ADR 0020](0020-an-ownerless-tracker-credential-is-reported-not-repaired.md)
for unattended work.

## Context

A tracker credential was resolved in two layers: a personal token per user
(ADR 0014), and a chain of environment variables per provider. For GitHub the
chain was `SECTILE_GITHUB_TOKEN`, `SECTILE_TRACKER_TOKEN`, `GH_TOKEN`,
`GITHUB_TOKEN`; for Jira `SECTILE_JIRA_TOKEN`, `SECTILE_TRACKER_TOKEN`,
`JIRA_API_TOKEN`. The settings row and each project could also hold a token, in
clear text, and a member could write either.

The background synchronisation has no acting user, so it borrowed the token of
the project's owner (`projects.owner_user_id`). That setup was hard to explain
and to operate:

- when the only variable set was the generic `SECTILE_TRACKER_TOKEN`, a server
  driving several trackers sent one provider's credential to another;
- the sync failed whenever the owner's token was sealed and locked, missing, or
  its owner gone;
- whoever clicked *Sync* read under their own token, so the same pass
  succeeded or failed depending on who asked.

## Decision

**Every synchronisation, timer-driven or started by hand, authenticates with
the server credential of its project's provider, and with nothing else.** The
activity still records who asked for a manual pass; that user is never put on
the tracker call. Writes somebody asks for (stage transitions, comments,
assignments) are unchanged: they use the acting person's personal credential,
Jira refuses without one, GitHub and GitLab fall back to the server credential.
*Amended by ADR 0029: GitHub and GitLab now refuse too; the server credential
signs unattended work only.*

**There is one server credential per provider**, and it is resolved in this
order, the first source that has one winning as a whole:

1. the credential an admin stored from the Administration page;
2. the provider's own environment variable: `SECTILE_GITHUB_TOKEN`,
   `SECTILE_JIRA_EMAIL` with `SECTILE_JIRA_TOKEN`, or `SECTILE_GITLAB_TOKEN`.

`SECTILE_TRACKER_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, `JIRA_API_TOKEN` and
`GITLAB_TOKEN` are no longer read. A server that still has one set starts, and
logs a warning naming its replacement. Per-project tokens are removed: one
credential serves every project of its provider.

**A stored server credential is sealed under the server key, without a
passphrase**, in `server_tracker_credentials`. Its additional authenticated
data carries the provider and a server marker, so it never opens as another
provider's or as somebody's personal credential. A credential the key does not
open fails the call with an error saying so; it never falls back to the
environment, which would put another account's name on the writes.

**Only an admin can set, check or clear it.** Members can see whether each
provider is configured, and where from, nothing more. A credential is checked
against its instance before it is stored.

**The upgrade seals the clear-text tokens once.** At startup, a token still in
the settings row is sealed into the new table and its column blanked. The step
is idempotent. With a token to seal and no key to seal it with, the server
refuses to start and names `SECTILE_SECRET_KEY`, rather than lose the only
working credential.

## Consequences

- A sync no longer depends on who owns the project or who clicked. It works
  while its provider has a server credential, and fails with an error naming
  the variable and the Administration page when it has none.
- Unattended writes are attributed to the account that owns the server
  credential. Sectile cannot change that: a tracker attributes a write to the
  account behind the token.
- Breaking for deployments relying on a removed variable, or on the owner's
  personal token for Jira: they have to set the provider's variable, or store
  the credential from the Administration page.
- One instance per provider per deployment. A project pointing at another
  GitHub or GitLab instance gets the deployment's credential, and that
  instance's own refusal.
- Changing or losing the server key strands the stored server credentials, as
  it strands the unsealed personal ones: an admin saves them again.
- The settings columns that held the clear-text tokens stay in the schema for
  one release, blank, as the input of the upgrade step. A later migration
  drops them.

## Alternatives rejected

- **Keep borrowing the owner's token.** The failure modes above are the reason
  for this record.
- **Seal the tokens in place in the settings row.** The row is written by the
  general settings path, which members reach; a table of its own gives the
  admin-only rule one place to live and a natural key per provider.
- **Fall back to the environment when a stored credential cannot be opened.**
  The admin believes the stored one is in use; a silent switch misattributes.
- **Discard a clear-text token the upgrade cannot seal.** Silent loss of the
  only working credential, where a refused start with a named fix is
  recoverable.
- **Run the upgrade as a numbered migration.** It needs the server key, which
  SQL cannot reach; the runner stays SQL-only (ADR 0021), and an idempotent
  step needs no version.
- **Cache the decrypted credential.** A change on one replica would not reach
  the others; one row read and one AES-GCM open per call is what the settings
  row already cost.
