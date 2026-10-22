# ADR 0058: A personal tracker credential signs only the ticket actions its owner performs

Status: Accepted (#855)

Amends: [ADR 0029](0029-server-tracker-credential-signs-unattended-work-only.md),
whose rule "a tracker write a person causes uses that person's own credential"
becomes "a ticket action a person performs uses their own credential", and
whose reads kept the person's credential first.
Amends: [ADR 0044](0044-personal-jira-access-is-authorised-through-oauth.md):
a Jira grant now serves the same, narrower set of operations.

Replaces: the OAuth connection of #804 (PR #807, closed unmerged), whose ADR
never landed.

## Context

ADR 0029 made every tracker write a person causes go out under that person's
own credential, or not at all: their direct edits, but also what Sectile writes
later on their behalf (stage labels and reports, a managed run's post-back,
macro milestones and epic mirrors). A read made in their name used their
credential first and fell back to the server's.

PR #807 added *Connect GitHub* and *Connect GitLab* on that path, so a grant
signed everything a pasted token did: stage reports, mirrors, board reads,
pull-request states. The owner rejected it: a person's connection is for the
ticket and comment operations that person performs, and Sectile's own work runs
on a service account. One symptom was visible on the setup check, where an
empty GitLab token verified the caller's grant.

#855 settles the model. Its clarification is in `docs/clarifications/855.md`,
and its specification in `specs/855-personal-oauth-for-direct-ticket-actions/`.

## Decision

**A personal credential signs only the ticket actions its owner performs
themself, outside a skill run.** That is creating a ticket, editing it (fields,
labels, a board column drop, assignee, parent or macro attachment, team,
sprint, transfer, delete) and posting a comment, from the web, the desktop
app, the REST API or an interactive MCP session. It holds for every kind of
personal credential: a pasted token, the Jira grant, a GitHub or GitLab grant.

**Everything else uses the server credential of the provider, and still names
the person on its activity:**

| Operation | Credential |
| --- | --- |
| A ticket action outside a skill run | personal |
| A ticket action inside a skill run | server |
| A stage transition: label, status, report comment | server |
| The macro lifecycle and every macro or epic mirror | server |
| Sprint management | server |
| Every read | server |
| Synchronisation, polling, Administration | server |
| Checking one's own credential on the profile | personal |

**A skill run is recognised by the call, not guessed.** An MCP call belongs to
a run when its request carries the run header (a console Sectile launched), or
when its session started a run with `start_run` and has not finished it. A
managed run's post-back and every stage transition are Sectile's own writes
whoever triggers them.

**The routing is declared in one place.** `tracker.WithServiceCredential`
marks a write Sectile makes by itself; `trackerapi.Client.ForWrite` gives it
the server client, and `ForRead` always does. Each queued operation kind
declares its route in `trackerOpRoutes`; a kind without one is refused. Tests
pin the table against every kind, and every tracker write the store makes
outside the queue against a declared route. `Actor.ServiceCredential` and the
queued jobs carry a skill run's marker past the request that set it.

**A write still needs an author.** A write marked for the server credential
that names neither a person nor unattended work is refused
(`ErrNoActingUser`), as ADR 0029 requires.

**Without a personal credential, a ticket action is refused.** The pasted
token remains a personal credential; with neither a grant nor a token the
action fails with `MissingPersonalCredentialError`, never under the server
credential.

**Without a server credential, Sectile's own writes are refused up front.**
`MissingServerCredentialError` (`tracker_server_credential_missing`, HTTP 503
over REST) is raised before any request leaves. A stage transition keeps the
local stage and fails its activity with a step saying an administrator sets
the credential in Administration; a macro edit is kept locally and reports the
refusal. A read without a server credential fails with the same error, as the
synchronisation always did.

**The checks name the account they used.** Administration checks the server
credential only. The check of the personal form checks the token typed, else
the caller's own stored token or grant, and refuses rather than answer with the
service account; GitLab, which checked the server token there, now checks the
caller's own. The setup of a tracker's site never probes a grant.

### GitHub and GitLab OAuth

**A person connects GitHub or GitLab with a consent screen.** *Profile →
Tracker credentials* offers *Connect GitHub* and *Connect GitLab* when the
forge's app is configured. The flow is ADR 0044's: a random, single-use
`state` bound to the web session and stored in `jira_oauth_flows`, which
migration 55 gives a `tracker` column; a confidential client exchanging the
code on the server; `GET /user` read through the new token to record the
account (`login` on GitHub, `username` on GitLab).

**GitHub uses an OAuth App, GitLab is gitlab.com only.** A GitHub OAuth App
token never expires and has no refresh token. A GitLab access token lasts two
hours and its refresh token rotates, so its refresh is claimed by the same
compare-and-set on the row's version as a Jira grant.

**The scopes are the least the personal calls need.** GitHub asks for `repo`
(issues, comments, labels, milestones on an issue, `transferIssue`); the
ProjectsV2 board status needed `read:project` but is a read, now made with the
server credential. GitLab asks for `api`, the least scope that performs the
REST writes. `github_scopes_test.go` and `gitlab_scopes_test.go` fail when the
consent asks for more or less than the calls need.

**A grant is a personal credential like the others, one row per provider**, in
`user_tracker_credentials` with `kind = 'oauth'`, sealed under the server key,
never behind a passphrase. Connecting replaces a pasted token of that forge,
and "Use a token instead" replaces the grant. Existing tokens are not migrated.

**A grant serves the public instance only**, `api.github.com` or `gitlab.com`
over HTTPS on the default port. A GitHub Enterprise or self-hosted GitLab
tracker refuses it with the missing-credential error, telling the person to use
a personal token for that site.

**A dead grant is reported, not repaired.** A GitLab refresh answered
`invalid_grant`, or a GitHub `401` on a call made through a grant, marks the
grant disconnected; every later ticket action fails until the person
reconnects. Disconnecting, or replacing a grant, revokes it at the provider,
best effort.

**The apps are configured by an admin, never committed**, from Administration
(`/api/admin/github-oauth`, `/api/admin/gitlab-oauth`) or the
`SECTILE_GITHUB_OAUTH_*` and `SECTILE_GITLAB_OAUTH_*` variables; a saved app
wins over the environment as a whole.

## Consequences

- What a screen shows no longer depends on who opened it: every read uses the
  server credential. A private repository or project the service account cannot
  see is not shown to a person who could see it with their own token.
- The GitHub, GitLab and Jira service accounts now sign stage labels and
  reports, milestones, epic mirrors and sprints. They need write access to
  every project Sectile serves; a deployment missing a server credential now
  fails those writes, not only the synchronisation.
- Stage reports and everything a skill run writes are attributed to the service
  account on the tracker; the person who caused them stays on Sectile's
  activity.
- A person needs a personal credential only to create, edit and comment on
  tickets themself.
- Deploying the OAuth connection needs an OAuth App registered under the GitHub
  organisation (approved by organisations that restrict OAuth apps) and a
  confidential application on gitlab.com, with the callback
  `https://<server>/auth/<tracker>/callback`.

## Alternatives rejected

- **Keep ADR 0029 and only fix the setup check.** It would leave a person's
  grant signing stage reports and mirrors, which the owner rejected.
- **Restrict only the GitHub and GitLab grants.** The same action would go out
  under a different account depending on how the person authenticated.
- **Fall back to the service account when a person has no credential.** Their
  own edits would then carry a name nobody chose.
- **A GitHub App, or one GitLab application per instance.** Rejected on #804
  for the reasons it gave: installations per organisation, permissions per
  installation, an app and a secret per instance.
