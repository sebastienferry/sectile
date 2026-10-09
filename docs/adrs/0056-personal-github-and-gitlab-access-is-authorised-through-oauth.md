# ADR 0056: A person's GitHub and GitLab access is authorised through OAuth, not only a pasted token

Status: Accepted (#804)

Amends: [ADR 0044](0044-personal-jira-access-is-authorised-through-oauth.md),
which gave Jira a consent screen and left GitHub and GitLab on pasted personal
tokens.

## Context

Since ADR 0044, a person writes to Jira under their own account after one
click. For GitHub and GitLab they still create a personal token on the forge,
copy it and paste it under *Profile → Tracker credentials*, then possibly seal
it behind a passphrase (ADR 0014, ADR 0032). The writes they cause must go out
under their own credential, or not at all (ADR 0029), so the step cannot be
skipped, only made easier.

Both forges offer the OAuth 2.0 authorisation code grant. The consent screen
yields the person's own credential, scoped and revocable from their account,
with nothing to copy.

#804 tracks the implementation.

## Decision

**A person connects GitHub or GitLab with a consent screen.** *Profile →
Tracker credentials* offers *Connect GitHub* and *Connect GitLab* when the
server has the app of that forge configured and at least one tracker of the
deployment is on github.com or gitlab.com. The flow is ADR 0044's: a random,
single-use `state` bound to the web session and stored in the database, a
confidential client exchanging the code on the server, then `GET /user` read
through the new token to record the account (`login` on GitHub, `username` on
GitLab). The pending consents share Jira's `jira_oauth_flows` table, which
migration 51 gives a `tracker` column (default `jira`); a state is consumed only
by the callback of the tracker that started it. Signing in to Sectile does not
change.

**GitHub uses an OAuth App, not a GitHub App.** An OAuth App acts as the person
with classic scopes on every repository they can reach, which is what a pasted
token does today. Its token normally never expires and has no refresh token.

**GitLab is gitlab.com only.** One application, registered on gitlab.com,
serves every person. Its access token lasts two hours and its refresh token
rotates, so the refresh is claimed by the same compare-and-set on the row's
version as a Jira grant (ADR 0044).

**The token answer is handled generically.** An answer with a refresh token and
an expiry takes the refresh path; one without them is used as stored until the
provider refuses it. A GitHub OAuth App that opted into expiring tokens is
therefore covered too.

**The scopes are the least the current calls need.** GitHub asks for
`repo read:project`, GitLab for `api`. The clarification expected `repo` alone
on GitHub, plus `read:user` if needed. `repo` covers the issue, milestone and
comment writes, private repositories and the GraphQL `transferIssue`, but the
ProjectsV2 board status read needs `read:project`. `GET /user` reads `login`
with no scope, so `read:user` is not requested. On GitLab, `read_api` and
`write_repository` cannot perform the REST writes or the iteration mutations,
so `api` is the least that works. `internal/trackerapi/github_scopes_test.go`
and `gitlab_scopes_test.go` map every path and GraphQL operation Sectile calls
to its scope, and fail when the consent asks for more or less than that union.

**A grant is a personal credential like the others, one row per provider.** It
lives in `user_tracker_credentials` with `kind = 'oauth'`, keyed by
`(user_id, tracker)`, sealed under the server key with the owner, the tracker
and the kind in its additional authenticated data, never behind a passphrase.
Connecting replaces a pasted token of that forge, and saving a pasted token
("Use a token instead") replaces the grant. Existing pasted tokens are not
migrated, and the server credential stays a token (ADR 0028, ADR 0029).

**A grant serves the public instance only.** A personal call through a forge
grant is made only when the tracker's API host is `api.github.com` or
`gitlab.com`. A GitHub Enterprise or self-hosted GitLab tracker refuses it with
the missing-credential error, telling the person to use a personal token for
that site, and nothing falls back to the server credential. Since there is one
row per provider, a person who connects GitLab loses their pasted token for a
self-hosted GitLab tracker until they switch back to a token. This is not new
in kind: a pasted token already had to serve every tracker of its provider.

**A dead grant is reported, not repaired.** A GitLab refresh answered
`invalid_grant` marks the grant disconnected, as for Jira; a GitLab `401` is
only reworded to say to reconnect. GitHub has no refresh to fail, so a REST
`401` on a call made through a GitHub grant marks it disconnected, by a
compare-and-set on the version the call read, so a late `401` never
disconnects a newer grant. A GraphQL answer with status 200 carrying errors
does not. Every later write fails with the missing-credential error until the
person reconnects.

**Disconnecting revokes the grant at the provider, best effort.** Deleting the
row, or replacing the grant with a pasted token, then calls the provider's
revocation (`DELETE /applications/{client_id}/grant` on GitHub, which revokes
every token of the app for that person; `POST /oauth/revoke` on GitLab). A
failed revocation is logged and changes nothing: the row is already gone.

**The apps are configured by an admin, never committed.** Each forge has its
row in `tracker_oauth_apps`, edited from Administration
(`/api/admin/github-oauth`, `/api/admin/gitlab-oauth`), or comes from
`SECTILE_GITHUB_OAUTH_CLIENT_ID`, `_CLIENT_SECRET` and `_REDIRECT_URL`, and
`SECTILE_GITLAB_OAUTH_*`. A saved app wins over the environment as a whole, as
for Jira.

**Desktop and the MCP tools do not change.**

## Consequences

- Nobody has to create, copy or paste a GitHub or gitlab.com token any more.
  The pasted token stays available behind "Use a token instead", and is the
  only way for GitHub Enterprise and self-hosted GitLab.
- A stolen database together with the server key yields grants limited to
  their scopes and revocable per person from the forge, instead of tokens of
  arbitrary scope.
- Deploying it needs work outside Sectile: an OAuth App registered under the
  GitHub organisation, a confidential application on gitlab.com, both with the
  callback `https://<server>/auth/<tracker>/callback`. A GitHub organisation
  that enforces OAuth app access restrictions must approve the app, or its
  private repositories stay invisible to every grant.
- The grant of a GitHub OAuth App reaches every repository its owner can reach
  with `repo`, as a classic token does. A narrower grant would need a GitHub
  App.
- Migration 51 adds `jira_oauth_flows.tracker`. The table keeps its name: a
  rename would touch both dialects for no change in behaviour.

## Alternatives rejected

- **A GitHub App.** Its user-to-server tokens are narrower and expire, but it
  must be installed on every organisation or repository it touches, its
  permissions are granted per installation rather than per person, and its
  writes carry the app's badge. The OAuth App matches what a pasted token does
  today with one consent.
- **One GitLab application per instance.** Each self-hosted instance would
  need its own registered app, client secret and callback, and the profile one
  row per instance. gitlab.com covers the need; self-hosted instances keep the
  pasted token.
- **A new flows table for the forges.** `jira_oauth_flows` already holds what a
  consent needs; a `tracker` column is enough to keep the flows of each
  provider apart.
