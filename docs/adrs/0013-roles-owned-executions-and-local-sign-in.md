# ADR 0013: Two roles, owned executions, and a local sign-in until a provider exists

Status: Accepted

Decides the question [ADR 0008](0008-web-sign-in-through-openid-connect.md) left
open: "Identity is authentication, not authorization. Everyone who signs in sees
the same board. Roles would be a separate decision."

## Context

ADR 0007 bound each workstation to a user, ADR 0011 made that binding one API
key, and ADR 0008 signed people in through OpenID Connect. Sectile could
therefore tell two people apart and could not treat them differently. Any
session rewrote the global settings and the tracker credentials of everyone.
`POST /api/agent/dispatch` took the target user from its request body, so
naming a colleague drove their agent.

Stopping an execution was worse than permissive, it was wrong. The cancel was
routed to the *caller's* agent. A colleague's agent answers that it does not
have that run, which is exactly the signal the server reads as "orphan", so it
closed a run that was still executing on another machine while the process kept
going. The bug and the missing rule are the same fix.

The deployment target is Okta or Auth0, neither of which is wired up yet. A
team that wants to share one server before then has only
`SECTILE_DEV_IDENTITY=1` and an `X-Sectile-User` header, an impersonation
switch that was never meant to be a sign-in.

## Decision

**Two roles, `admin` and `member`, stored on the user.** An admin manages
users and their roles, projects, global settings, tracker credentials, anyone's
workstations and anyone's execution. A member does everything else: the board,
its tasks, its transitions, its comments, and executions on their own agent. A
third role is a decision for the day someone needs one.

**The board stays shared.** The user-to-project binding stays what it already
was, the agent registration per `(user, project)` that routes a run to the right
machine. Everyone sees every project, task and running execution. No
`project_members` table, no visibility filter. The shared-board line of ADR 0007
stands.

**Executions have an owner, and the owner is who the stop is routed to.** The
run records the user who started it: the signed-in person, the key holder for a
run reported over MCP, the agent's user for a run the agent owns. Only that
person or an admin may stop it, and the stop is dispatched to the *owner's*
agent, so a run is closed as orphaned only when the agent that should have it
says it does not. A member's dispatch reaches their own agent whatever user the
request body names. Records written before this decision carry no owner; they
belong to no one and are an admin's to close.

**Roles come from the identity provider when it supplies them.**
`SECTILE_OIDC_ROLE_CLAIM` names the claim and `SECTILE_OIDC_ADMIN_GROUP` the
value that grants admin; the two are set together or not at all. The claim is
read from UserInfo, and from the ID token payload when UserInfo does not carry
it, on the same reasoning ADR 0008 gives for trusting that channel. It is the
authority: it overwrites a manual change at the next sign-in, and the users view
says so. Okta and Auth0 are then configuration, as ADR 0008 intended.

**Without a claim, the first account is the admin.** Someone has to be able to
assign the first role, and every account arrives through sign-in as an equal.
The first user created while no admin exists takes the role; admins change roles
from there. The last admin cannot be demoted.

**A local sign-in until a provider is connected.** Without
`SECTILE_OIDC_ISSUER`, `POST /auth/local` creates or finds an account from an
e-mail address alone. It identifies people; it does not authenticate them, and
the sign-in screen, the profile and the documentation all say so. It replaces
`SECTILE_DEV_IDENTITY` and the `X-Sectile-User` header, which are removed.

**The implicit user lasts until the first account.** *(Superseded by
[ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md): signing in
is mandatory and there is no implicit mode.)* A deployment with no
provider and no account keeps the single implicit user of ADR 0008, who holds
the admin role. The first local account ends that mode: from then on an
anonymous request is a stranger and the interface leads to the sign-in screen.

**A workstation key answers for a session on the interface API.** The agent
gateway forwards its consoles' `/api/` calls with the key rather than a cookie,
and that key names a user exactly as a session does, with that user's role.
Only a real key counts: the deprecated shared token and the legacy open mode,
where any value named the implicit user, would otherwise hand anyone a way past
sign-in by setting one header.

## Consequences

Authorization has one shape and one place: a principal carrying a user, a role
and the deployment's sign-in mode, resolved from the cookie or the key, and
three checks, signed in, admin, owner-or-admin. No handler compares user ids by
hand. The admin-only routes are a table with a test that names each of them,
next to the test that names which paths bypass the session guard, so both
boundaries are read in one place.

The local mode is the knowing compromise. Anyone who types a colleague's
address becomes that colleague, which is why it is bounded to a trusted network,
announced wherever it appears, and disabled the moment a provider is
configured. A mailed link is the first hardening if the transition lasts.

The shared settings row mixes personal preferences with the deployment's
configuration. Members keep their theme, language, density and view; the rest is
refused by naming the offending keys rather than by silently dropping them. That
row is due to be split, and the legacy free-text name and e-mail retired, now
that an account carries the identity. *(Done by
[ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md).)*

A team that upgrades finds every existing user a member, because no role had
been stored, and the next person to sign in becomes the first admin, exactly as
on a fresh installation. Runs already recorded have no owner and are stoppable
by admins only.
