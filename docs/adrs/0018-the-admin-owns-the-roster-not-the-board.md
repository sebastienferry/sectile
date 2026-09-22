# ADR 0018: An admin owns the roster, not the board

Status: Accepted

Amends [ADR 0013](0013-roles-owned-executions-and-local-sign-in.md) and
[ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md).

## Context

ADR 0013 gave an admin "users and their roles, projects, global settings,
tracker credentials, anyone's workstations and anyone's execution". Projects and
tracker credentials were on that list because they looked like deployment
configuration.

In use they are not. Opening a project is the first thing anyone does on this
board, and pointing that project at the tracker it reads from is the same act
continued: a project with no tracker shows an empty column. Reserving both to
admins means every new piece of work waits on one person, on a board whose whole
premise (ADR 0007, ADR 0013) is that it is shared and that everyone sees
everything on it. The rule was protecting a boundary that was not there.

The roster is a different matter. Who holds an account, what role it carries and
whether it still opens are decisions about access itself, and they are the ones
that cannot be delegated to whoever happens to be signed in.

There was also nothing between "this person works here" and "this person never
existed". An admin could demote someone, which changes nothing about their
access, or leave the account alone. Someone leaving the team left a working
session, a working workstation key and no way to end either.

## Decision

**The admin-only table is the accounts, and nothing else.** `adminOnlyRoute`
names `/api/users` and `/api/users/{id}`. Creating, renaming, reconfiguring and
deleting a project, and configuring the tracker it reads from, are a member's.

**The tracker keys of the settings row follow the same rule.** They stay on the
shared row, because there is one tracker per deployment, but a member may write
them. `trackerSettingsKeys` is that list, read alongside `personalSettingsKeys`
by the one authorization check. The rest of the shared row, the AI
configuration, the prompts, the repository path and the sync loop, stays an
admin's: it configures how the deployment runs rather than what it looks at.

**An account can be blocked, and blocking is not deletion.** `users.blocked_at`
holds the instant it was closed. A blocked account keeps its rows, its history
and its ownership of past executions; what stops is access, and it stops
everywhere at once: the open browser sessions are revoked in the same operation,
the workstation keys stop authenticating, and the next sign-in is refused with a
message that says why rather than a generic failure. Unblocking restores all
three. A blocked admin does not count towards the admin count, because an
account that does not open cannot administer anything.

**An account can be deleted, and deletion takes only the credentials.** The
sessions, the workstation keys and any pairing code in flight go with the row.
The work does not: tasks, comments and executions record a user id without a
foreign key, so they outlive their author and read as having no owner, which is
a state the board already displays (ADR 0013 named it for records written before
ownership existed).

**Three things the board protects from itself.** The last admin cannot be
demoted, blocked or deleted. Nobody can block or delete their own account, with
or without a second admin: the account that would undo it is the one being
closed. The implicit account is not a person and is neither blockable nor
deletable; it owns everything that runs without a session.

## Consequences

- A member can change the deployment's tracker credential, including replacing a
  working one. That is the accepted cost of not gating project work behind an
  admin. A credential that must not be shared is a *personal* one (ADR 0014),
  which is a different mechanism and is unaffected.
- Blocking is the reversible measure and deletion the final one, so the panel
  offers both rather than making people choose between over- and under-reacting.
- Deleting the last account holding a workstation key leaves the deployment with
  no device credential at all, which re-enables the legacy open mode in
  `resolveAgentCredential`, where any syntactically valid token names the
  implicit user. That fallback predates this record and is not narrowed here; it
  is written down because account deletion is a new way to reach it.
