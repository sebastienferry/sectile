# ADR 0042: A person's Jira access is authorised through Atlassian OAuth, not a pasted token

Status: Proposed. Becomes Accepted with the pull request that implements it.

Amends: [ADR 0014](0014-personal-tracker-credentials-are-sealed.md), its
rejected alternative "OAuth 2.0 three-legged authorisation with Atlassian",
which it called the better long-term answer and left open.

## Context

Every write a person causes on Jira goes out under their own credential, or not
at all (ADR 0014, ADR 0029). Today that credential is a Jira Cloud API token,
sent with the account e-mail as HTTP Basic authentication
(`trackerapi.jiraBasicAuth`). To get one, each person:

1. opens id.atlassian.com, creates a token and copies it;
2. pastes it with their e-mail under *Profile → Tracker credentials*;
3. optionally seals it behind a passphrase, and unlocks it again after every
   absence longer than the presence window (ADR 0032).

That is the step people stumble on. It is also a weak credential to hold: an
API token carries every permission of its account, has no scope, and lives
until its owner revokes it or it reaches its expiry date.

Sectile cannot remove the person from the loop. Jira attributes a write to the
account behind the token, and Jira Cloud offers a plain integration no way to
act as somebody else. What it can remove is the copy and paste: Atlassian's
OAuth 2.0 authorisation code grant ("3LO") lets a person grant Sectile scoped,
revocable access with one consent screen.

#654 tracks the implementation.

## Decision

**A person connects Jira with a consent screen instead of pasting a token.**
*Profile → Tracker credentials* offers *Connect Jira* when the server has an
Atlassian OAuth app configured. The flow is the authorisation code grant:

1. The server redirects to `https://auth.atlassian.com/authorize` with
   `audience=api.atlassian.com`, the scopes below, `prompt=consent` and a
   `state` value.
2. The `state` is random, single use, short lived, and bound to the web session
   that started the flow. It is stored in the database, so the callback may
   land on any replica (ADR 0030).
3. The callback exchanges the code at `https://auth.atlassian.com/oauth/token`.
   The server is a confidential client: the client secret never reaches a
   browser or a workstation.
4. The server reads `https://api.atlassian.com/oauth/token/accessible-resources`
   and keeps the `cloudId` of the site the deployment is configured for. A
   grant that does not include that site is refused and nothing is stored.
5. It reads `/rest/api/3/myself` through the grant and records the account, as
   the API token path already does.

**The grant is a personal credential like the others.** It lives in
`user_tracker_credentials`, one row per person and tracker, so connecting
replaces a pasted API token and deleting the row disconnects. A new column
records the kind of credential (`api_token` or `oauth`). The record holds the
refresh token, the current access token, its expiry and the `cloudId`, sealed
with AES-256-GCM under the server key. Its additional authenticated data carries
the owner, the tracker and the kind, so an OAuth record never opens as an API
token, as another person's credential or as a server credential.

**An OAuth grant is never sealed behind a passphrase.** The access token lasts
about an hour and has to be refreshed from background work: queued writes,
stage reports, managed runs (ADR 0029). A passphrase would lock every one of
them after each absence, which is the friction this record removes. The grant is
scoped and revocable from the person's Atlassian account, so the passphrase has
far less to protect. An API token already sealed stays sealed until its owner
connects or deletes it.

**Refreshing is serialised in the database.** Atlassian rotates refresh
tokens: each refresh returns a new one and invalidates the previous one after a
short grace period. Two replicas refreshing the same grant at the same moment
would each hold a token the other just rotated. So a refresh is a
compare-and-set on the row's version:

- the instance that wins writes the new pair;
- an instance that loses reads the row again and uses the access token the
  winner stored;
- nothing is cached in process memory beyond one call, as ADR 0028 already
  rules for server credentials.

**A dead grant is reported, not repaired.** When the refresh answers
`invalid_grant` (the person revoked the app, their account lost access to the
site, or the refresh token expired after about 90 days without use), the row is
marked disconnected, never deleted. The write that hit it fails with the error
ADR 0029 gives a missing credential, naming *Profile → Tracker credentials*, and
the profile shows *Reconnect Jira*. Nothing falls back to the server credential.

**Only the transport changes.** `ForWrite`, `ForActingUser` and the rules of
ADR 0029 are untouched. A client built from an OAuth grant sends
`Authorization: Bearer <access token>` to
`https://api.atlassian.com/ex/jira/{cloudId}` followed by the same
`/rest/api/3/...` and `/rest/agile/1.0/...` paths. Links shown to people keep
the site URL.

**The scopes are the least the current calls need.** Classic scopes
`read:jira-work`, `write:jira-work`, `read:jira-user` and `offline_access` cover
`/rest/api/3`. The `/rest/agile/1.0` calls (boards, sprints, backlog) need Jira
Software's granular scopes, at least `read:board-scope:jira-software`,
`read:sprint:jira-software` and `write:sprint:jira-software`. The specification
checks every endpoint Sectile calls against a scope and lists the result. A new
endpoint that needs a scope the app lacks is a change to this list and to the
registered app, made together.

**The app is configured by an admin, never committed.** The client id and
secret come from `SECTILE_JIRA_OAUTH_CLIENT_ID` and
`SECTILE_JIRA_OAUTH_CLIENT_SECRET`, or from the Administration page, sealed like
the server credentials (ADR 0028). The callback URL is derived from the
server's public URL. Without an app configured, the profile keeps today's API
token form.

**The server credential does not change.** Unattended work keeps using the
provider's server credential (ADR 0028, ADR 0029), an API token of a service
account. Moving it to OAuth is a separate decision.

## Consequences

- Nobody creates, copies or pastes a Jira token any more. Connecting is one
  click and one consent screen, once per person, and again only after a
  revocation or about 90 days without any Jira call made on their behalf.
- A stolen database together with the server key yields refresh tokens limited
  to the granted scopes, revocable per person from Atlassian, instead of
  unscoped API tokens. Someone who is root on the server can still use them, as
  ADR 0014 already states for every credential.
- Deploying it needs work outside Sectile. Someone registers an OAuth 2.0
  integration on developer.atlassian.com, enables its distribution so people
  other than its owner can authorise it, and registers the callback URL. The
  Atlassian organisation's admins must allow people to authorise third-party
  apps. If they block it, this record cannot apply and the API token form
  stays.
- Calls made through OAuth grants count against the app's rate limits on the
  site, which Atlassian computes for the app rather than per person. A large
  synchronisation still runs under the server credential, so the per-person
  writes are the only load on it.
- API tokens keep working. Removing the form is a later decision, once every
  active person has connected.
- The row gains a column (`kind`) and a version for the compare-and-set, both
  added by a numbered migration, never in the frozen baseline.

## Alternatives rejected

- **One service account signing every write, with "by X through Sectile" in
  the text.** No friction at all, and exactly the misattribution ADR 0014 and
  ADR 0029 removed.
- **A Forge or Connect app acting as the user.** It would ask people for
  nothing, but a site admin has to install it, Connect is being retired, and
  Sectile would become an app hosted on Atlassian's platform. Far larger than
  the problem.
- **Reusing the workstation's Atlassian login** (`acli`, the Atlassian MCP
  server). It serves only writes made from that workstation while it runs, not
  the server's queued writes, and borrows a token Sectile does not control.
- **Allowing a passphrase on the OAuth grant.** It brings back the unlock step
  and breaks background writes after each absence. The grant being scoped and
  revocable is the protection instead.
- **Keeping the access token in memory and refreshing per instance.** With
  rotating refresh tokens, two replicas refreshing independently disconnect the
  person. The compare-and-set in the database is the only shared answer.
