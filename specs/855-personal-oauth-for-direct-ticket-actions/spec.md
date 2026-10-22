# Specification #855 - A personal tracker credential signs only the ticket actions its owner performs

- Ticket: https://github.com/sebastienferry/sectile/issues/855
- Branch: `feat/855`
- Clarification: `docs/clarifications/855.md` (rounds 1 and 2, owner confirmed
  on 2026-10-22), plus three routing answers given at specification time
  (2026-10-22, recorded under "Decisions taken at specification time")
- Replaces: #804 (closed, not planned) and PR #807 (closed unmerged)
- Framework: Spec Kit

## Summary

Today a person's tracker credential (a pasted token, or the Jira OAuth grant)
signs everything Sectile does in that person's name: their direct edits, but
also the stage labels and reports a skill run posts, the epic and macro mirrors,
and every read made while they look at a screen (ADR 0029). PR #807 plugged a
GitHub/GitLab OAuth grant into the same path, which is what the owner rejected.

After this change:

- a person can connect GitHub (github.com) and GitLab (gitlab.com) through
  OAuth, as they already can for Jira;
- a personal credential, whatever its kind, signs only the ticket actions that
  person performs themself, outside a skill run: creating a ticket, editing it,
  and posting a comment;
- everything else Sectile does on a tracker (stage transitions and reports,
  macro lifecycle and mirrors, writes made inside a skill run, every read, the
  synchronisation and the Administration screens) uses the provider's server
  credential.

## Scope

In scope:

- *Connect GitHub* and *Connect GitLab* in *Profile → Tracker credentials*,
  with a disconnect, for github.com and gitlab.com only;
- the OAuth app of each forge, set from Administration or from the environment;
- the routing of every tracker read and write between the personal and the
  server credential, on GitHub, GitLab and Jira;
- the Administration credential check;
- how a missing server credential is reported, now that it signs more than the
  synchronisation;
- ADR 0058 (amends ADR 0029), `docs/API_AND_DATA_SPEC.md`,
  `docs/USER_GUIDE.md`, `.env.sample`, `CHANGELOG.md`.

Out of scope:

- merge-request work done on the workstation through `glab` or `gh`;
- signing in to Sectile (OIDC, ADR 0008);
- Sectile Desktop screens (its REST calls follow the server's routing with no
  change on its side);
- a GitHub App, a self-hosted GitLab OAuth app, a GitHub Enterprise OAuth app;
- migrating existing pasted tokens;
- registering the provider OAuth apps and approving them in organisations: they
  are planned tasks E1 to E3 in `tasks.md`, done by the providers'
  administrators, not code.

## Definitions

- **Provider**: GitHub, GitLab or Jira.
- **Personal credential**: what a person stored in *Profile → Tracker
  credentials* for a provider: a pasted token, the Jira OAuth grant, or (new)
  the GitHub or GitLab OAuth grant.
- **Server credential**: the credential of a provider set by an administrator
  in Administration, or given by the environment (ADR 0028).
- **Skill run**: a remote run Sectile knows of: a console it launched (its MCP
  requests carry the run header), a run an MCP session opened with `start_run`
  and has not finished, or a managed run posting its result back.
- **Direct ticket action**: a write a person makes on a ticket themself,
  outside any skill run, from the web, the desktop app, the REST API or an
  interactive MCP session.
- **Sectile write**: any other tracker write: everything a skill run writes,
  stage transitions, the macro lifecycle and mirrors, the synchronisation.

## Credential routing (normative)

| Operation | Outside a skill run | Inside a skill run |
| --- | --- | --- |
| Create a ticket (web, REST, MCP `create_task`, local-to-remote conversion, story created under a macro) | personal | server |
| Edit a ticket: title, description, labels, priority, fields, status by a board column drop, assignee, parent or macro attachment, team, sprint, transfer to another project, delete | personal | server |
| Post a comment (web, REST, MCP `add_comment`) | personal | server |
| Stage transition: stage label, tracker status, stage report comment (web, REST, MCP `transition_stage`, managed run post-back) | server | server |
| Macro lifecycle: create, rename, describe, close, delete, migrate (GitHub milestone, Jira epic) | server | server |
| Macro and epic mirrors: horizon, priority, quarter, readiness, labels, todos, framing, push horizons | server | server |
| Every read: tickets, comments, board statuses, pull-request states, assignee and team search, issue lists | server | server |
| Synchronisation, polling, Administration | server | server |
| Checking one's own credential in *Profile → Tracker credentials*, reading its account | personal | n/a |

The activity of every operation still records the person who caused it, whatever
credential signs it.

## User stories (prioritised)

### US1 - My direct ticket actions are signed by me (P1)

As a person editing a ticket, I want my creations, edits and comments to reach
the tracker under my own account.

**Acceptance**

- **Given** a person with a personal credential for the project's provider (a
  pasted token, or an OAuth grant on github.com, gitlab.com or Jira), **when**
  they create a ticket, edit it (any field in the routing table, a board column
  drop included) or post a comment from the web, the REST API or an MCP session
  with no open skill run, **then** the tracker receives the change under that
  person's credential.
- **Given** a person with no personal credential for the provider, **when**
  they perform one of those actions, **then** nothing is written, and they get
  the existing refusal naming the provider and *Profile → Tracker credentials*
  (`tracker_credential_missing`).
- **Given** a person whose pasted token is sealed and locked, **then** the
  action is refused the same way; it never falls back to the server credential.

### US2 - Sectile's own writes go out under the service account (P1)

As the owner, I want stage reports, macro writes and everything a skill run
writes to be signed by the service account, so that a person's credential never
serves Sectile's machinery.

**Acceptance**

- **Given** any stage transition (web, REST, MCP `transition_stage`, managed run
  post-back), **then** the stage label, the tracker status and the report
  comment are written with the server credential, whether or not the person who
  caused it has a personal credential.
- **Given** a skill run (launched console, `start_run` session not yet finished,
  managed run), **when** it creates a ticket, edits one or posts a comment,
  **then** the write uses the server credential.
- **Given** an MCP session that called `finish_run` for its run, **when** it
  then posts a comment, **then** the comment is a direct ticket action again
  (personal).
- **Given** a macro created, renamed, described, closed, deleted or migrated,
  or any macro or epic mirror (horizon, priority, quarter, readiness, labels,
  todos, framing, push horizons), **then** the GitHub milestone or Jira epic
  write uses the server credential.
- **Given** a person who attaches a ticket to a macro or moves tickets to an
  epic, **then** the tickets' change is personal, and an epic created on the fly
  for that move is created with the server credential.
- **Given** any of these writes, **then** its activity names the person who
  caused it.

### US3 - Reads never use a personal credential (P1)

**Acceptance**

- **Given** a person with a personal credential, **when** a screen, an MCP read
  tool or a background job reads a tracker (a ticket, its comments, a board
  status list, pull-request states, an assignee or team search), **then** the
  request carries the server credential, never the person's.
- **Given** a deployment with no server credential for a provider, **when**
  something reads that provider, **then** the read fails with the error naming
  the missing server credential, as the synchronisation already did; no
  personal credential stands in.

### US4 - I can connect GitHub and GitLab through OAuth (P1)

As a person, I want to connect my GitHub and GitLab accounts with a click
instead of pasting a token.

**Acceptance**

- **Given** a GitHub OAuth app configured (Administration, else
  `SECTILE_GITHUB_OAUTH_*`), **when** a person opens *Profile → Tracker
  credentials* for GitHub, **then** they see *Connect GitHub*, and "Use a token
  instead" which keeps the pasted-token form.
- **Given** they click *Connect GitHub* and approve on github.com, **then** they
  land back on *Profile → Tracker credentials* connected, with their GitHub
  login recorded as their tracker account (for *My Tasks*).
- **Given** the same for GitLab on gitlab.com, **then** the same holds, and the
  GitLab access token is refreshed before or when it expires without the person
  noticing.
- **Given** a GitHub or GitLab tracker on another instance (GitHub Enterprise,
  self-hosted GitLab), **then** *Connect* is not offered for it and a pasted
  token is the only personal credential; a grant on github.com or gitlab.com is
  not used for that instance and the action is refused with a message saying
  the connection only covers github.com / gitlab.com.
- **Given** no OAuth app is configured for a forge, **then** *Connect* is not
  shown and the token form is.
- **Given** a person who disconnects, **then** the grant is deleted, its
  revocation is attempted on the provider (best effort), and their next direct
  ticket action is refused until they reconnect or paste a token.
- **Given** a GitHub grant that GitHub refuses with `401` during a direct ticket
  action, or a GitLab grant whose refresh is refused (`invalid_grant`), **then**
  the grant is marked disconnected, the action is refused with a message saying
  to reconnect, and nothing falls back to the server credential.
- **Given** a person connected through OAuth who stored a pasted token before,
  **then** the grant replaces it (one personal credential per provider and
  person).

### US5 - The Administration screen shows the service account (P1)

**Acceptance**

- **Given** an administrator who re-checks the stored server credential of
  GitHub, GitLab or Jira with an empty token field, **then** Sectile checks the
  server credential and shows the service account's name, never the
  administrator's own account, whatever the administrator stored personally.
- **Given** a typed token, **then** that token is checked, as today.
- **Given** no server credential for the provider, **then** the check says so
  instead of checking a personal one.
- **Given** a person who presses *Verify* on their personal form with no token
  typed, **then** their own stored token or grant is checked on every provider,
  GitLab included, and a person who stored none is told so rather than shown
  the service account.

### US6 - A missing server credential is visible (P2)

**Acceptance**

- **Given** a provider with no server credential, **when** a Sectile write
  (stage transition, macro write, skill run write) targets it, **then** nothing
  is written to the tracker, the local change is kept as for any refused
  tracker write, and the activity ends in failure with a step that names the
  provider and says an administrator sets its server credential in
  Administration.
- **Given** the same, **then** a direct ticket action by a person with a
  personal credential still succeeds.

### US7 - The OAuth apps are set from Administration (P2)

**Acceptance**

- **Given** an administrator, **when** they open Administration, **then** they
  can set, replace or clear the client id and secret of the GitHub and GitLab
  OAuth apps, as they can for Jira, and see whether each is set there, set by
  the environment, or missing. The secret is never shown back.

## Functional requirements

- **FR-1** A personal credential is used only for direct ticket actions, as
  listed in the routing table, on GitHub, GitLab and Jira, whatever its kind
  (pasted token or OAuth grant).
- **FR-2** Every other tracker write uses the server credential, and records
  the person who caused it on its activity.
- **FR-3** Every tracker read uses the server credential.
- **FR-4** A write is inside a skill run when its MCP request carries the run
  header, when its MCP session holds a run it started and has not finished, or
  when it is a managed run's post-back.
- **FR-5** A direct ticket action with no usable personal credential is
  refused (`MissingPersonalCredentialError`, `tracker_credential_missing`);
  it never falls back to the server credential.
- **FR-6** A Sectile write with no server credential is refused with an error
  naming the provider and Administration; the local state follows the existing
  rules for a refused tracker write.
- **FR-7** A write naming no person and not marked unattended is still refused
  (`ErrNoActingUser`, ADR 0029).
- **FR-8** The routing of every queued operation kind and every direct write
  site is declared in one place and pinned by a test that fails when a new kind
  or site is added without a declared route.
- **FR-9** GitHub and GitLab OAuth: authorization-code flow with a single-use
  state bound to the web session (10-minute TTL); the grant is sealed under the
  server key; GitLab tokens are refreshed under compare-and-set; a GitHub `401`
  or a GitLab `invalid_grant` marks the grant disconnected; revocation on
  disconnect is best effort.
- **FR-10** A GitHub grant serves `api.github.com` only and a GitLab grant
  `gitlab.com` only, over HTTPS on the default port.
- **FR-11** The scopes a consent asks for are the least the direct ticket
  actions need, pinned by a test that lists every forge call a personal client
  can make.
- **FR-12** The Administration credential check never reads a personal
  credential.

## Decisions taken at specification time

Asked to the owner on 2026-10-22 because the Round 2 table did not settle them;
the owner took the recommended option each time:

- A create, edit or comment made **inside a skill run** uses the server
  credential (only an interactive session with no open run signs personally).
- A stage change a person makes **from the web** is a stage transition like any
  other: server credential.
- The **macro lifecycle** (create, rename, describe, close, delete, migrate;
  GitHub milestone, Jira epic) uses the server credential.

## Documentation acceptance

- ADR 0058 records the routing and amends ADR 0029 (its rule "a write a person
  causes uses their own credential" becomes "a direct ticket action uses their
  own credential"); ADR 0029 and ADR 0044 get an "Amended by ADR 0058" status
  line.
- `docs/API_AND_DATA_SPEC.md`: the connect and callback endpoints, the
  Administration OAuth app endpoints, the routing table.
- `docs/USER_GUIDE.md`: *Connect GitHub / GitLab*, what the connection is used
  for, and that the service accounts need write access to every project.
- `.env.sample`: `SECTILE_GITHUB_OAUTH_CLIENT_ID`, `_CLIENT_SECRET`,
  `_REDIRECT_URL` and the GitLab twins.
- `CHANGELOG.md` `[Unreleased]`: **Added** Connect GitHub / GitLab through
  OAuth (#855). **Changed** (breaking): stage reports, macro writes, writes made
  by skill runs and all reads now use the server credential; the GitHub, GitLab
  and Jira service accounts need write access to every project Sectile serves
  (#855). **Fixed**: the Administration check showed the administrator's own
  account (#855).

## Open requirements

None.
