# ADR 0015: Sign-in is mandatory, and the settings row is split in two

Status: Accepted

Supersedes two decisions of
[ADR 0013](0013-roles-owned-executions-and-local-sign-in.md): *"the implicit
user lasts until the first account"*, and the shared settings row it left
standing.

## Context

ADR 0013 shipped a real identity — an e-mail sign-in, an `HttpOnly` session
cookie, a server-side session, two roles and owned executions. Two things kept
that identity from being real in practice.

**The implicit mode hid sign-in entirely.** While no local account existed,
`signInMode()` answered `implicit`, `webSessionUser()` resolved every anonymous
request to the `default` user, and `RequireSession` let everything through. A
fresh deployment therefore opened its board with no sign-in screen and no
session, and every visitor was an admin. The mode was meant as a bridge for a
personal deployment; what it actually produced was a server that looks
authenticated and is not until somebody happens to sign in.

**One settings row served everyone.** `GetSettings()` reads a single row
constrained to `id = 1`. Theme, language, density, default view, scale, detail
mode, identity and workstation commands were shared by the whole team, next to
the tracker, AI and prompt configuration. ADR 0013 recorded this as debt.

## Decision

**Signing in is mandatory.** There is no mode that opens the board to an
anonymous visitor. `signInMode()` answers `oidc` or `local` and nothing else,
`webSessionUser()` returns the empty string when neither a session cookie nor a
workstation API key names anyone, and `RequireSession` lets through exactly
`publicPath()`: `/api/me`, `/auth/`, the agent APIs, `/mcp`, `/health` and the
interface's own static files. The interface mounts nothing before `/api/me` has
answered.

**`default` is an ordinary account.** It is no longer an admin by construction:
`principalFor` resolves it through the users table like any other id and it
keeps its stored role. No adoption and no merge run: its row stays, its
executions keep their owner, and the workstations paired to it keep naming a
valid user. An admin claims it or reassigns its data from the users view. An
existing deployment where it is an admin stays administrable; one where it is
not is not locked out either, because the first sign-in while no admin exists
still takes the role, the unchanged ADR 0013 rule.

**The settings row is split in two.** The personal half — `theme`,
`accentColor`, `language`, `density`, `defaultView`, `uiScale`, `detailMode`,
`userName` and `userEmail` (projections of the account, see below),
`userAvatar`, `editorCommand`,
`externalTerminalCommand` — moves to a `user_settings` table, one row per
account. The deployment half — trackers, `repoPath`, auto-sync, the whole AI
configuration, the prompts and `specFramework` — stays in the single row and
stays an admin's. `memberSettingsKeys` is that routing table, and the same list
is what a member is allowed to write.

**The HTTP shape does not change.** `GET /api/settings` composes its answer from
both stores; a `POST`/`PUT` routes each key to the store it belongs to. The
interface posts and reads the whole object in a dozen components, and changing
that shape would spread this ticket across the front end for no behavioural
gain.

**Nothing is migrated.** `user_settings` is created empty and an account without
a row reads the deployment row's personal columns, writing its own on the first
save. An existing deployment therefore shows exactly what it showed before, per
account, from the first render.

**`userName` and `userEmail` are projections.** The read answers the account's
own name and address; a write ignores both. This is the retirement of the
free-text identity ADR 0013 asked for. The name follows the same chain every
other reader of a user follows — the chosen name, then the one the sign-in
supplied, then the address, then the id — so the chrome names a person exactly
as an execution owner or a comment author is named, and Settings → Account stays
the one place a name is changed. Both keys stay in `memberSettingsKeys`: a member
posting the whole row is answered `200` and the two fields are dropped from the
payload, rather than the post being refused as an admin-only change.

> Superseded by [ADR 0030](0030-the-workstation-owns-the-invocation.md): the
> terminal, the editor and the AI configuration are workstation settings, read
> by the local agent from its own file, and no longer server settings.

**The workstation commands follow the execution's owner.** The external terminal
and the editor read the owner's `externalTerminalCommand` / `editorCommand`,
with the project's own override still first and the deployment value last. The
AI configuration is untouched and keeps reading the deployment row, since a
launch is configured by the deployment and not by whoever started it.

**The sealing passphrase is optional and never refuses a sign-in.** The sign-in
form takes a required e-mail and an optional ADR 0014 sealing passphrase, and no
login password. A supplied passphrase unlocks that person's sealed tracker
tokens, one call per tracker. A wrong one opens the session anyway, reports the
failure and leaves the tokens locked: refusing the sign-in would turn an
optional passphrase into a de facto mandatory password. No passphrase signs the
person in with their sealed tokens locked, and a sealed token that is needed
fails loudly rather than falling back to the server token, as ADR 0014 requires.

**The session and the cookie are unchanged.** `HttpOnly`, `Secure` behind TLS,
`SameSite=Lax`, a twelve hour TTL, only the hash stored, revoked server-side on
sign-out, and concurrent sessions still allowed.

## Consequences

A deployment with no account opens on the sign-in screen and on nothing else,
and an anonymous call to an interface API answers 401 outside the public paths.
Two accounts on the same server each see their own presentation, identity and
workstation commands, while the tracker, auto-sync, AI and prompt settings stay
shared and are refused to a member by name.

`ImplicitUser` survives only where there is no HTTP request to resolve: the
launches started outside a browser session and the local agent's own operations.
It is an ordinary account there, not a privilege.

The board stays shared, as ADR 0013 decided: everyone still sees every project,
task and running execution, and only execution ownership matters. The desktop
application is untouched and existing pairings keep working.

## Alternatives rejected

- **A `user_id` column on `settings`, one row per account.** It would duplicate
  the twenty deployment columns per account and leave two writable copies of the
  tracker configuration, which is the bug this record exists to remove.
- **Migrating the existing row into a personal row per account.** Nothing to
  gain over seeding on read, and a migration to get wrong on every upgrade.
- **Keeping the implicit mode behind a flag.** A flag that hands an anonymous
  visitor the admin role is the same hole with a switch in front of it.
- **Refusing a sign-in on a wrong sealing passphrase.** It would make an
  optional secret mandatory and lock out whoever lost it.
