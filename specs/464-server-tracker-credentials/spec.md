# Specification #464 - One server tracker credential per provider

- Ticket: https://github.com/sebastienferry/sectile/issues/464
- Branch: `claude/clarify-issue-gh-11a4f59c-cdc99f`
- Clarification: `docs/clarifications/464.md` (rounds 1 to 3, owner confirmed,
  no open product question)
- Framework: Spec Kit

## Summary

The synchronisation of a project with its tracker authenticates with one
server credential for that project's provider, and with nothing else. There is
one such credential per provider (GitHub, Jira, GitLab). An admin sets it from
the Administration page. It is kept encrypted in the database, and when the
database holds none, one named environment variable per provider supplies it.
The generic `SECTILE_TRACKER_TOKEN` and the providers' own variables are no
longer read, per-project tokens disappear, and a sync no longer borrows anybody's
personal token.

## Scope

In scope:

- which credential a synchronisation uses, timer-driven or started by hand;
- where the server credentials are stored, who may write them, how they are
  shown;
- the environment variables read for them;
- the per-project token overrides, which are removed;
- the upgrade of existing deployments.

Out of scope:

- personal tracker credentials (ADR 0014): their storage, sealing and profile
  screen are unchanged, and so is their use for writes somebody asks for;
- tracker URLs, the GitLab project and the Jira project key, including their
  per-project overrides: they are not credentials;
- agent, desktop and pairing credentials;
- the orphaned-credential report (ADR 0020);
- a GitLab synchronisation: GitLab has no sync adapter today. The GitLab
  server credential is still resolved, stored and shown, for the calls that
  already use it (pull-request states, connection check).

## Definitions

- **Provider**: GitHub, Jira or GitLab.
- **Server credential**: the credential Sectile itself uses for a provider.
  For GitHub and GitLab it is a token. For Jira it is an account e-mail plus
  an API token, which only work together.
- **Stored credential**: a server credential an admin saved from the
  Administration page.
- **Environment credential**: the credential given by the provider's variable,
  used only when nothing is stored:
  - GitHub: `SECTILE_GITHUB_TOKEN`;
  - Jira: `SECTILE_JIRA_EMAIL` + `SECTILE_JIRA_TOKEN`;
  - GitLab: `SECTILE_GITLAB_TOKEN`.
- **Removed variables**: `SECTILE_TRACKER_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`,
  `JIRA_API_TOKEN`, `GITLAB_TOKEN`.
- **Synchronisation (sync)**: a pass that reads a project from its tracker
  into the board, whether the automatic timer or a person asked for it.
- **Personal write**: a tracker change a person asks for (stage transition,
  comment, assignment, sprint move and the like).

## User stories (prioritised)

### US1 - A sync always uses the server credential of its provider (P1)

As an operator, I want every sync to authenticate with the server credential
of the project's provider, so that it keeps working when a project owner
leaves, loses or locks their personal token, and so that one provider's
credential is never sent to another.

**Acceptance**

- **Given** a GitHub project with a server GitHub credential, **when** the
  automatic timer syncs it, **then** the tracker calls carry the server GitHub
  credential, whatever the project owner stored personally.
- **Given** a project whose owner has a personal credential sealed and locked,
  or no personal credential, or whose owner account was deleted, **when** a
  sync runs, **then** it succeeds with the server credential.
- **Given** a person with a personal credential for the provider, **when** they
  click *Sync* on a project, **then** the sync uses the server credential, not
  theirs, and the activity still records that they asked for it.
- **Given** a Jira project, **when** a sync runs, **then** it authenticates
  with the server Jira e-mail and token, and never with a GitHub or GitLab
  credential; the same holds for every pair of providers.
- **Given** a project whose provider has neither a stored nor an environment
  credential, **when** a sync runs, **then** it fails with an error that names
  the provider, the environment variable(s) to set and the Administration
  page, and does not tell the reader to add a personal token.
- **Given** a sync activity, **then** the tracker attributes whatever that
  sync writes to the account that owns the server credential.

### US2 - An admin manages the server credentials from the Administration page (P1)

As an admin, I want to see, set, replace, check and clear each server
credential from the Administration page, so that I can operate the
deployment's tracker access without editing its environment.

**Acceptance**

- **Given** an admin on the Administration page, **then** a *Server tracker
  credentials* section lists GitHub, Jira and GitLab, each with its state:
  *stored*, *provided by the environment* or *not configured*.
- **Given** a stored credential that was checked when it was saved, **then**
  the section shows the tracker account it authenticates as and when it was
  saved.
- **Given** an admin entering a token (and, for Jira, an e-mail), **when** they
  save, **then** Sectile first checks it against the provider's instance. A
  failed check saves nothing and shows the reason. A successful check stores
  the credential and shows the account.
- **Given** a stored credential, **when** an admin saves a new one, **then** it
  replaces the old one, and the next tracker call uses it without a server
  restart, on every server instance sharing the database.
- **Given** a stored credential, **when** an admin clears it, **then** the
  provider falls back to its environment credential if there is one, or
  becomes *not configured*.
- **Given** a credential provided by the environment, **when** an admin asks to
  check it, **then** the section shows the account it authenticates as, or the
  reason it is refused.
- **Given** Jira, **then** the e-mail and the token are saved, replaced and
  cleared together. A stored Jira credential is never combined with the
  environment's e-mail or token.
- **Given** any response of the server, **then** it never contains a server
  token, stored or from the environment. The Jira e-mail may be shown to an
  admin.

### US3 - Only an admin can read or change a server credential (P1)

As an admin, I want members unable to read or change a server credential, so
that the deployment's tracker identity is under the control of those who run
it.

**Acceptance**

- **Given** a member (non-admin) or an anonymous caller, **when** they call any
  endpoint that sets, clears or checks a server credential, **then** they get
  403 (member) or 401 (anonymous) and nothing changes.
- **Given** a member saving the general settings or the tracker setup with a
  token or a Jira e-mail in the payload, **then** those fields are not stored.
  Through the tracker setup, the request is refused with 403 and a message
  pointing to an admin.
- **Given** a member setting up a project, **then** they can still set its
  tracker, repository, project key and URL, and no screen they reach offers a
  token field for a server credential.
- **Given** a member, **then** they may see whether each provider is
  configured (stored or environment), never the value, the e-mail or the
  account.

### US4 - Per-project tokens are gone (P2)

As an operator, I want one credential per provider for the whole deployment,
with no hidden per-project exception.

**Acceptance**

- **Given** the project settings, **then** there is no GitHub or GitLab token
  field.
- **Given** a server upgraded with projects that had a token override, **then**
  those overrides are discarded, and the projects sync with the server
  credential of their provider.
- **Given** a project whose URL points at a different instance of its provider
  (for example GitHub Enterprise), **then** its calls carry the deployment's
  server credential, and the instance's own refusal is reported if it does not
  accept it.

### US5 - Only the provider's own variable is read (P2)

As an operator, I want each provider to read exactly one variable, so that
configuring one provider never leaks into another.

**Acceptance**

- **Given** a server with only `SECTILE_TRACKER_TOKEN` set, **then** no provider
  has a credential, and each sync fails with the US1 error naming the variable
  to set.
- **Given** `GH_TOKEN`, `GITHUB_TOKEN`, `JIRA_API_TOKEN` or `GITLAB_TOKEN` set,
  **then** Sectile does not use them as tracker credentials. They stay in the
  environment of the processes Sectile starts, as today.
- **Given** a removed variable set at startup, **then** the server starts and
  logs one warning per such variable, naming the variable that replaces it.
  With no removed variable set, no warning is logged.

### US6 - Upgrading keeps the credential that works today (P1)

As an operator upgrading a deployment, I want the server credentials already
saved in Sectile to keep working and be encrypted, without me re-entering
them.

**Acceptance**

- **Given** a database holding a server GitHub, GitLab or Jira token in clear
  text (saved before this change), **when** the server starts on the new
  version, **then** the token is encrypted with the server key, stored as that
  provider's server credential (with the Jira e-mail for Jira), and no copy in
  clear text remains in the database.
- **Given** that upgrade, **then** the account shown on the Administration page
  is empty until an admin checks or re-saves the credential.
- **Given** a database holding a clear-text token and a server with no usable
  encryption key, **when** it starts, **then** it refuses to start, with a
  message naming `SECTILE_SECRET_KEY`, and the clear-text token is left
  untouched. With nothing to encrypt, the absence of a key does not stop the
  server.
- **Given** the upgrade has run once, **when** the server restarts, **then**
  nothing is encrypted or moved again.

### US7 - A credential that cannot be decrypted says so (P2)

**Acceptance**

- **Given** a stored credential that the current server key cannot decrypt
  (key changed or lost), **when** a sync or a check needs it, **then** it fails
  with an error saying the stored credential cannot be read with the server
  key and must be saved again from the Administration page. It does not fall
  back to the environment credential.
- **Given** that situation, **then** the Administration page shows the
  credential as *stored, unreadable* and still lets an admin replace or clear
  it.

## Functional requirements

- **FR-1** A sync (timer or manual) authenticates with the server credential
  of its project's provider only. It never uses a personal credential.
- **FR-2** A server credential is resolved in this order: stored credential,
  then environment credential, then none. The first source that has one wins
  as a whole; a stored credential is never completed with environment values.
- **FR-3** Only the variables in *Environment credential* are read as tracker
  credentials. The *Removed variables* are not read.
- **FR-4** A stored credential is encrypted with the server key, bound to its
  provider and to a server marker: it does not decrypt as another provider's,
  or as a personal credential.
- **FR-5** A stored credential takes effect on the next tracker call on every
  server instance, without a restart.
- **FR-6** Setting, clearing and checking a server credential are admin-only.
  Members can know whether each provider is configured, nothing more.
- **FR-7** A credential is checked against its instance before it is stored.
- **FR-8** No API response or log line carries a server token.
- **FR-9** Per-project GitHub and GitLab token overrides no longer exist.
- **FR-10** Personal writes are unchanged: they use the acting person's
  personal credential; Jira refuses without one; GitHub and GitLab fall back to
  the server credential.
- **FR-11** A missing credential for a sync is reported with the provider, the
  variable(s) and the Administration page.
- **FR-12** A removed variable set at startup produces one warning naming its
  replacement; the server starts.
- **FR-13** Upgrade encrypts and adopts existing clear-text server tokens
  exactly once, refusing to start rather than losing them when no key is
  available.

## Documentation acceptance

- `README.md` and `.env.sample` describe the three provider variables, the
  Jira e-mail, the Administration page as the place to set the credentials,
  and the removal of the old variables.
- `CHANGELOG.md` `[Unreleased]` gets:
  - under **Changed**: syncs use the provider's server credential, set by an
    admin from the Administration page and encrypted at rest;
  - under **Removed**: `SECTILE_TRACKER_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`,
    `JIRA_API_TOKEN`, `GITLAB_TOKEN` as tracker credentials, and the
    per-project GitHub and GitLab tokens.
- A new ADR records the decision and supersedes the part of ADR 0014 about
  unattended work.

## Open requirements

None. Every product question was settled in the clarification.
