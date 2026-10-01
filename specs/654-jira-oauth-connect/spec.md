# Specification #654 - Connect Jira through Atlassian OAuth instead of a pasted API token

- Ticket: https://github.com/sebastienferry/sectile/issues/654
- Branch: `feat/654`
- Clarification: `docs/clarifications/654.md` (rounds 1 and 2, every
  recommendation accepted by the owner)
- Decision record: `docs/adrs/0044-personal-jira-access-is-authorised-through-oauth.md`
  (Proposed; amended and Accepted by the pull request implementing this)
- Framework: Spec Kit

## Summary

A person on a Jira project connects their Jira account from *Profile →
Tracker credentials* with one Atlassian consent screen, instead of creating,
pasting and unlocking an API token. Sectile keeps the grant, refreshes it in
the background, and every Jira write that person causes keeps going out
under their own Jira account, exactly as the attribution rules of ADR 0029
require. The API token path stays available.

## Scope

In scope: the web profile (*Profile → Tracker credentials*), the Administration
page (the OAuth app configuration), the server's authorisation flow, the storage
and refresh of the grant, the Jira transport used for a person's writes and
reads, the French and English interface strings, the user guide, the API
specification, the ADRs, `.env.sample` and the changelog.

Out of scope:

- The server credential used by unattended work: it stays an API token (ADR
  0028, ADR 0029).
- Removing the API token path, or migrating existing tokens.
- GitHub and GitLab.
- Jira Data Center and Jira Server: Atlassian OAuth 2.0 (3LO) is Jira Cloud
  only.
- Sectile Desktop and the MCP tools: tracker credentials are managed in the
  web profile only. Desktop and agents benefit through the server, with no
  change of their own.
- Revoking the grant at Atlassian: Atlassian offers no revocation endpoint to
  3LO apps.

## Vocabulary

- **OAuth app**: the OAuth 2.0 integration registered on
  developer.atlassian.com for the deployment, identified by a client id and a
  client secret, with one callback URL.
- **OAuth configured**: the server knows a client id, a client secret and a
  callback URL for the OAuth app, from the environment or from the
  Administration page.
- **Grant**: what a person's consent gives Sectile: a refresh token, a
  short-lived access token, and the list of Jira sites (each with its
  `cloudId`) the person allowed.
- **Configured Jira sites**: the global Jira site of the deployment and every
  site a Jira project sets for itself.
- **Effective site of a project**: the project's own Jira site when it sets
  one, the global Jira site otherwise.
- **Connected**: the person's Jira credential is a grant that is not
  disconnected.
- **Disconnected**: a grant Atlassian refused to refresh (`invalid_grant`):
  the person revoked the app, lost access, or left the grant unused for about
  90 days. The row is kept and marked; it is never used again.
- **Missing-credential error**: the refusal ADR 0029 gives a person-caused
  write that has no credential of the person to go out under, naming *Profile
  → Tracker credentials*, which the web turns into the token offer (#645).

## User stories

### US1 (P1) - A person connects Jira with one consent screen

As a person without a Jira credential, I connect my Jira account in a few
clicks and never handle a token.

1. Given OAuth configured and a person with no Jira credential, when they open
   *Profile → Tracker credentials* on Jira, then *Connect Jira* is the primary
   action, and a secondary link "Use an API token instead" reveals today's API
   token form.
2. Given that person, when they use *Connect Jira*, then the browser goes to
   Atlassian's consent screen, listing the scopes of this specification.
3. Given they accept and pick at least one configured Jira site, when Atlassian
   sends them back, then they land on *Profile → Tracker credentials* on Jira,
   which shows them connected, under the Jira account name Atlassian confirmed,
   with the sites the grant covers, and a success notification.
4. Given they are connected, when they then transition, comment on and assign
   a ticket of a Jira project whose site the grant covers, then each write is
   made by their own Jira account.
5. Given they decline on the consent screen, when Atlassian sends them back,
   then nothing is stored and the profile says the connection was cancelled.

### US2 (P1) - The grant keeps working without the person

As a connected person, my queued writes and my agent's stage reports go out
under my account hours later, while I am away.

1. Given a connected person whose access token expired, when a queued write
   they caused runs, then Sectile refreshes the grant and the write succeeds
   under their account, with no action from them.
2. Given the same person, when a managed run reports a stage transition on
   their behalf, then it succeeds the same way.
3. Given two server replicas that both need to refresh the same grant at the
   same moment, then both calls succeed and the person stays connected.
4. Given Atlassian is briefly unreachable or answers a server error during a
   refresh, then the call fails with that error and the person stays
   connected; the next call tries again.

### US3 (P1) - A dead grant is reported, never worked around

1. Given a connected person who revoked Sectile from their Atlassian account,
   when a write they caused needs a refresh, then the write fails with the
   missing-credential error naming *Profile → Tracker credentials*, it is never
   made with the server credential, and the web shows the token offer (#645).
2. Given that person, when they open *Profile → Tracker credentials* on Jira,
   then it says the connection was lost and offers *Reconnect Jira*, which
   starts US1.2 again.
3. Given they reconnect, then the next write succeeds under their account.

### US4 (P1) - Sites covered by a grant

1. Given a grant covering sites A and B, and one Jira project on each, then a
   write on either project goes out under the person's account on that
   project's site.
2. Given a grant covering site A only, when the person causes a write on a
   project whose effective site is C, then the write fails with the
   missing-credential error, saying to reconnect Jira and pick C on the consent
   screen, and never falls back to the server credential.
3. Given the consent returns a grant covering none of the configured Jira
   sites, then nothing is stored, any existing credential of the person stays
   as it was, and the profile names the configured sites the grant should have
   included.

### US5 (P1) - Connecting replaces, deleting disconnects

1. Given a person with a pasted API token, sealed or not, when they connect,
   then the grant replaces the token; no passphrase or unlock is ever asked for
   the grant.
2. Given a connected person, when they use *Disconnect*, then the grant is
   forgotten, the profile returns to the not-connected state, and it tells them
   they can also remove Sectile from the connected apps of their Atlassian
   account.
3. Given a connected person, when they switch to "Use an API token instead"
   and save a token, then the token replaces the grant.

### US6 (P1) - An admin configures the OAuth app

1. Given an admin on the Administration page, then a Jira OAuth section shows
   the client id, whether a secret is set (never the secret), the callback URL,
   and whether the values come from the page or from the environment.
2. Given the admin saves a client id, a secret and a callback URL, then OAuth
   is configured, and *Connect Jira* appears in every person's profile.
3. Given the admin saves again without typing a secret, then the stored
   secret is kept.
4. Given the admin clears the configuration, then the environment values apply
   if set; otherwise OAuth is not configured and profiles show today's form.
   Existing grants stay stored but cannot refresh; each shows *Reconnect Jira*
   once the app is configured again and the grant fails.
5. Given a person who is not an admin, then the section and its API are
   refused.

### US7 (P2) - A forged or stale callback stores nothing

1. Given a callback with no `state`, a `state` already used, a `state` older
   than its lifetime, or a `state` started by another web session or another
   person, then nothing is stored and the profile says the connection failed
   and can be tried again.
2. Given a callback that lands on another replica than the one that started
   the flow, then it succeeds.

## Functional requirements

### Configuration

- **FR1** OAuth is configured by a client id, a client secret and a callback
  URL, read from `SECTILE_JIRA_OAUTH_CLIENT_ID`,
  `SECTILE_JIRA_OAUTH_CLIENT_SECRET` and `SECTILE_JIRA_OAUTH_REDIRECT_URL`, or
  from the Administration page. A configuration saved on the page wins as a
  whole over the environment, as ADR 0028 rules for server credentials. OAuth
  is configured only when all three values are present.
- **FR2** The client secret is write-only: no API response, log line, error
  message, activity or web bundle ever contains it. It is stored sealed under
  the server key.
- **FR3** The callback URL is taken as configured, never derived from request
  headers. It is the URL Atlassian redirects to and must match the one
  registered on the OAuth app.

### Authorisation flow

- **FR4** *Connect Jira* starts the authorisation code grant: Atlassian's
  authorisation endpoint with `audience=api.atlassian.com`, the scopes of FR16,
  `prompt=consent`, the configured callback URL and a `state`.
- **FR5** The `state` is random, single use, valid 10 minutes, stored in the
  database with the web session and the person that started the flow, and
  consumed atomically, so the callback works on any replica and a replay finds
  nothing. Only a signed-in web session can start a flow; an agent key cannot.
- **FR6** The callback accepts the flow only when its `state` is known, unused,
  unexpired, and belongs to the same web session and person. Any other callback
  stores nothing (US7).
- **FR7** The callback exchanges the code for a grant server side (confidential
  client), reads the sites the grant covers, keeps every covered site with its
  `cloudId`, and refuses the grant, storing nothing, when it covers none of the
  configured Jira sites; the refusal names those sites (US4.3).
- **FR8** The callback reads the person's Jira account through the grant on a
  covered configured site, records it as the confirmed account, and stores the
  grant as the person's Jira credential, replacing any existing one and its
  unlock (US5.1).
- **FR9** The callback always ends on *Profile → Tracker credentials* on Jira,
  with the outcome: connected, cancelled by the person, flow expired or
  invalid, no configured site covered (with the sites), or Atlassian
  unreachable.

### Storage

- **FR10** A person keeps one Jira credential, either an API token or a grant.
  The row records its kind. A grant is sealed under the server key, bound to
  the owner, the tracker and the kind, so it never opens as an API token, as
  another person's credential or as a server credential. A grant is never
  sealed behind a passphrase and never takes part in lock and unlock.
- **FR11** Every existing credential reads as an API token and behaves exactly
  as today.

### Using the grant

- **FR12** A Jira call made for a connected person goes to Atlassian's API
  gateway for the `cloudId` matching the effective site of the project, with
  the person's access token, on the same REST paths as today. Links shown to
  people keep the site URL.
- **FR13** When the effective site of the project is not covered by the grant,
  the call fails with the missing-credential error, stating that the grant does
  not cover that site and that reconnecting and picking it fixes it (US4.2).
- **FR14** An access token expired or expiring within a minute is refreshed
  before the call. Refreshing is serialised across replicas: exactly one
  refresh result is kept for a given state of the row; a replica whose refresh
  lost reads the row again and uses the kept access token. No access token is
  cached in memory beyond one call.
- **FR15** A refresh answered `invalid_grant` marks the grant disconnected;
  that call and every later one fail with the missing-credential error saying
  the Jira connection was lost, until the person reconnects (US3). Any other
  refresh failure fails the call and changes nothing (US2.4). A disconnected or
  missing grant never falls back to the server credential.

### Scopes

- **FR16** The OAuth app asks for exactly the scopes below, and every Jira
  endpoint Sectile calls maps to one of them. A new endpoint needing another
  scope changes this table and the registered app together (ADR 0044).

  Classic scopes: `read:jira-work`, `write:jira-work`, `read:jira-user`,
  `offline_access` (refresh tokens). Jira Software granular scopes for
  `/rest/agile/1.0`, which accepts no classic scope:
  `read:board-scope:jira-software`, `read:board-scope.admin:jira-software`,
  `write:board-scope:jira-software`, `read:sprint:jira-software`,
  `write:sprint:jira-software`, `delete:sprint:jira-software`,
  `read:project:jira`.

  | Method | Endpoint | Used for | Scope |
  | --- | --- | --- | --- |
  | GET | `/rest/api/3/myself` | confirm the account | `read:jira-user` |
  | GET | `/rest/api/3/search/jql` | search issues | `read:jira-work` |
  | GET | `/rest/api/3/field` | field ids | `read:jira-work` |
  | GET | `/rest/api/3/priority` | priorities | `read:jira-work` |
  | GET | `/rest/api/3/priority/search` | priorities, server credential only | `manage:jira-configuration`, not asked: a grant reads `/rest/api/3/priority` instead |
  | GET | `/rest/api/3/issue/createmeta/{project}/issuetypes` | issue types | `read:jira-work` |
  | GET | `/rest/api/3/issue/createmeta/{project}/issuetypes/{id}` | create fields, priorities | `read:jira-work` |
  | GET | `/rest/api/3/issue/{key}/editmeta` | editable priorities | `read:jira-work` |
  | GET | `/rest/api/3/issue/{key}` | read an issue, its status | `read:jira-work` |
  | POST | `/rest/api/3/issue` | create an issue | `write:jira-work` |
  | PUT | `/rest/api/3/issue/{key}` | edit, parent, labels, team, priority | `write:jira-work` |
  | GET | `/rest/api/3/issue/{key}/transitions` | list transitions | `read:jira-work` |
  | POST | `/rest/api/3/issue/{key}/transitions` | transition | `write:jira-work` |
  | GET | `/rest/api/3/issue/{key}/comment` | read comments | `read:jira-work` |
  | POST | `/rest/api/3/issue/{key}/comment` | comment | `write:jira-work` |
  | PUT | `/rest/api/3/issue/{key}/assignee` | assign | `write:jira-work` |
  | GET | `/rest/api/3/user/bulk` | resolve users | `read:jira-user` |
  | GET | `/rest/api/3/user/assignable/search` | assignable people | `read:jira-user` |
  | GET | `/rest/api/3/jql/autocompletedata/suggestions` | team search | `read:jira-work` |
  | GET | `/rest/api/3/status` | status names | `read:jira-work` |
  | GET | `/rest/api/3/project/{key}/statuses` | project statuses | `read:jira-work` |
  | GET | `/rest/agile/1.0/board` | list boards | `read:board-scope:jira-software`, `read:project:jira` |
  | GET | `/rest/agile/1.0/board/{id}/configuration` | board columns | `read:board-scope.admin:jira-software`, `read:project:jira` |
  | GET | `/rest/agile/1.0/board/{id}/sprint` | list sprints | `read:sprint:jira-software` |
  | POST | `/rest/agile/1.0/sprint` | create a sprint | `write:sprint:jira-software` |
  | POST | `/rest/agile/1.0/sprint/{id}` | update a sprint | `write:sprint:jira-software` |
  | DELETE | `/rest/agile/1.0/sprint/{id}` | delete a sprint | `delete:sprint:jira-software` |
  | POST | `/rest/agile/1.0/sprint/{id}/issue` | move issues to a sprint | `write:sprint:jira-software` |
  | POST | `/rest/agile/1.0/backlog/issue` | move issues to the backlog | `write:board-scope:jira-software` |

  Verified on 2026-10-01 against the `security` of each operation in
  Atlassian's published OpenAPI documents of the Jira platform and Jira
  Software REST APIs. Compared with the first version of this table, the board
  configuration needs `read:board-scope.admin:jira-software`, the sprint and
  backlog moves need neither `read:issue:jira-software` nor
  `write:issue:jira-software` (dropped), and the priority search needs an
  administration scope a grant never asks for. `TestEveryJiraPathHasAScope`
  (`internal/trackerapi/jira_scopes_test.go`) keeps the code and this table
  together.

### Profile

- **FR17** Without OAuth configured, *Profile → Tracker credentials* and every
  Jira call behave exactly as today.
- **FR18** With OAuth configured, the Jira entry shows one of:
  - not connected (no credential): *Connect Jira* as primary action, "Use an
    API token instead" as secondary link to today's form;
  - API token stored: today's token state and form, plus *Connect Jira*, which
    replaces the token;
  - connected: the confirmed account name, the covered sites, *Disconnect*,
    and the note about Atlassian's connected apps;
  - disconnected: a warning that the connection was lost, and *Reconnect Jira*.
- **FR19** The passphrase section (lock, unlock, sealing) counts and acts on
  API tokens only; a grant never appears locked.
- **FR20** Every new interface string exists in French and English.

### Errors

- **FR21** The missing-credential error of FR13 and FR15 keeps the refusal code
  and provider the web already recognises (#645), so the token offer appears.
  The text of today's missing-token error is unchanged for a person with no
  credential at all.

### Documentation

- **FR22** `CHANGELOG.md` gets one `Added` line under `[Unreleased]`. The user
  guide's Jira access section describes *Connect Jira*, its prerequisites and
  *Reconnect Jira*. `docs/API_AND_DATA_SPEC.md` describes the new endpoints and
  the stored kinds. `.env.sample` lists the three variables. ADR 0044 becomes
  Accepted, is titled ADR 0044, and is amended on the sites covered (every
  granted site) and the callback URL (explicit setting). ADR 0014's status line
  references ADR 0044.

## Acceptance criteria

- **AC1** A person with no Jira credential connects through the consent screen,
  and their next transition, comment and assignment are attributed to their
  Jira account (US1). Checked against a fake Atlassian authorisation, token,
  accessible-resources and API endpoint; checked end to end against Atlassian
  once the organisation allows third-party OAuth apps and the OAuth app is
  registered. The pull request states which of the two was done.
- **AC2** Connecting replaces an existing API token, sealed or not; deleting
  the credential disconnects (US5).
- **AC3** A queued write and a managed run's transition succeed for a connected
  person after the access token expired, with no action from them (US2.1,
  US2.2).
- **AC4** Two replicas refreshing the same grant concurrently leave the person
  connected, tested with a fake token endpoint that rotates refresh tokens and
  invalidates a used one (US2.3).
- **AC5** A revoked grant fails the write with the missing-credential error
  naming *Profile → Tracker credentials*, never falls back to the server
  credential, and the profile offers *Reconnect Jira* (US3).
- **AC6** A callback with a missing, reused, expired or foreign-session `state`
  stores nothing (US7).
- **AC7** A grant that covers none of the configured Jira sites stores nothing
  and names them; a write on a project whose site the grant lacks fails with
  the missing-credential error (US4).
- **AC8** Every Jira endpoint Sectile calls is mapped to a granted scope in
  FR16.
- **AC9** The client secret never appears in a response, a log line or the web
  bundle.
- **AC10** Without OAuth configured, the profile and every Jira call behave
  exactly as today; existing API tokens, sealed or not, keep working after the
  upgrade.
- **AC11** The Go tests (SQLite and PostgreSQL), the web unit tests and the
  typecheck pass.
- **AC12** FR22 is done.

## Edge cases

- A person connects while their API token is sealed and locked: the grant
  replaces it without asking for the passphrase.
- A person starts two flows in two tabs: each `state` is independent; the
  second callback replaces the first grant.
- A person leaves the consent screen open more than 10 minutes: the callback
  reports the flow expired and stores nothing.
- A person signs out between start and callback: the callback has no matching
  session and stores nothing.
- A grant covering a site no project uses yet is kept; it serves a project
  added later on that site.
- A site URL differing only by a trailing slash or letter case matches.
- An admin changes the client id: existing grants fail to refresh with
  `invalid_grant` or a client error. `invalid_grant` disconnects them; people
  reconnect.
- Reads a person causes (their board, their search) keep going through the
  path they use today; a read that uses the person's credential uses the grant
  the same way as a write.

## Open points

None of product. The granular scopes of FR16 were verified during
implementation, and the table corrected.

The end-to-end part of AC1 depends on two prerequisites outside the code: the
Atlassian organisation allowing third-party OAuth 2.0 apps, and an OAuth 2.0
integration registered on developer.atlassian.com with distribution enabled,
the deployment's callback URL and the scopes of FR16.
