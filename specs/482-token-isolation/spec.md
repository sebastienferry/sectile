# Specification #482 - The server tracker credential never signs a person's action

- Ticket: https://github.com/sebastienferry/sectile/issues/482
- Branch: `feat/482`
- Clarification: `docs/clarifications/482.md` (rounds 1 and 2, owner
  confirmed on 2026-09-25, no open product question)
- Framework: Spec Kit

## Summary

Since #464, each provider (GitHub, Jira, GitLab) has one server tracker
credential, meant for the synchronisation. Today it also signs some writes that
a person caused: stage reports posted as comments, tickets created through an
agent, local tickets converted to remote ones, stories created under a macro and
GitHub milestones backing macros. On GitHub and GitLab, a person who stored no
personal token writes under the server account on purpose (ADR 0028).

After this change, every tracker write a person causes travels under that
person's own credential, on all three providers, or does not happen. The server
credential only signs the work nobody asked for: the synchronisation pass and
the jobs nobody launched.

## Scope

In scope:

- every tracker write a person causes, directly (web, desktop, REST) or through
  an agent session (MCP), on a ticket, a macro or an epic: creation, title,
  description, labels, status, assignment, sprint, team, parent, milestone,
  comment, stage report;
- the write a managed run makes when it reports back after its session ended;
- tracker writes requested by a caller that Sectile cannot tie to a person;
- ADR 0028 and the release note, which describe the fallback being removed.

Out of scope:

- tracker **reads** a person causes (comments shown on a ticket, pull-request
  states, board reads): they keep their current behaviour, fallback included;
- the synchronisation pass and the jobs nobody launched: they keep the server
  credential;
- how personal and server credentials are stored, sealed, checked and
  administered (ADR 0014, #464);
- which environment variables supply the server credential (#464).

## Definitions

- **Provider**: GitHub, Jira or GitLab.
- **Personal credential**: the tracker credential a person stored in
  *Profile → Tracker credentials* for a provider.
- **Server credential**: the credential of a provider set by an admin or given
  by the environment (#464).
- **Person-caused write**: a tracker change that exists because a person asked
  for it, whether they clicked in an interface, called the REST API or ran an
  agent session that called Sectile. It includes changes Sectile makes later on
  their behalf (a queued operation, a report posted after a transition).
- **Unattended write**: a tracker change Sectile makes on its own initiative:
  during a synchronisation pass, or in a job nobody launched.
- **Launcher**: the person recorded on a managed run as the one who started it.
- **Anonymous caller**: a REST or MCP caller whose credential Sectile cannot
  resolve to a person (for example a shared agent key, or an agent credential
  paired to no device).

## User stories (prioritised)

### US1 - My stage reports are posted under my name (P1)

As a person moving a ticket to a new stage, I want the stage report that
Sectile posts on the ticket to be signed by my tracker account, so that the
ticket shows who reported the work.

**Acceptance**

- **Given** a Jira project and a person with a personal Jira credential,
  **when** they (or an agent session running under their key) record a stage
  with a report note, **then** the report comment appears on the Jira issue
  under that person's Jira account, never under the server account.
- **Given** the same on a GitHub project with a personal GitHub credential,
  **then** the report comment and the stage label are written under that
  person's GitHub account.
- **Given** a person without a personal credential for the project's provider,
  **when** they record a stage with a report note, **then** the stage is
  recorded on the board, no comment is posted, no label is changed on the
  tracker, and the activity of the transition ends in failure with a step that
  says the personal credential is missing and where to add it
  (*Profile → Tracker credentials*).
- **Given** a report comment that the tracker refused for any other reason,
  **then** the activity shows the failure instead of claiming the report was
  posted.

### US2 - Every write I cause uses my own credential, on every provider (P1)

As a person working on a board, I want every change I make on a ticket, a macro
or an epic to reach the tracker under my own account, whatever the provider, so
that the tracker never attributes my work to the server account.

**Acceptance**

- **Given** a person with a personal credential for the project's provider,
  **when** they create, edit (title, description, labels, priority, status),
  assign, move to a sprint or a team, re-parent, comment or delete a ticket,
  **then** the tracker receives the change under that person's credential.
- **Given** the same person, **when** they create a ticket through an agent
  session (`create_task`), **then** the issue is created under their
  credential, exactly as when they create it from the web.
- **Given** the same person, **when** they convert a local ticket to a remote
  one, **then** the issue is created under their credential.
- **Given** the same person, **when** they create a story under a macro, or
  from a macro to-do, **then** the issue is created and attached to the macro
  under their credential.
- **Given** a GitHub project and the same person, **when** they create, rename,
  edit, close, delete or migrate a macro, or attach a ticket to one, **then**
  the GitHub milestone and issue writes that back it are made under their
  credential.
- **Given** a person **without** a personal credential for the provider, on
  GitHub, Jira or GitLab alike, **when** they cause any of the writes above,
  **then** nothing is written to the tracker under another credential, and they
  get an error that names the provider and says to add a personal credential in
  *Profile → Tracker credentials*.
- **Given** a person whose personal credential is sealed behind a passphrase
  they have not unlocked, **when** they cause a write, **then** it is refused
  the same way; it never falls back to the server credential.
- **Given** a write that is local and remote at once (for example a macro edit
  whose milestone update is refused), **then** the local part behaves as it
  does today for a failed tracker write, and the refusal is reported to the
  person instead of being dropped silently.

### US3 - A managed run reports back under its launcher (P1)

As a person who launched a managed run, I want what the run writes when it
reports back (stage, labels, report comment) to be signed by my account, even
though my session has ended by then.

**Acceptance**

- **Given** a managed run launched by a person with a personal credential,
  **when** the run posts its result back, **then** the stage label, the tracker
  status and the report comment are written under that person's credential.
- **Given** a managed run whose launcher has no personal credential for the
  provider, **when** it posts back, **then** the stage is recorded on the board,
  the tracker step fails with the missing-credential message, and nothing is
  written under the server credential.
- **Given** a managed run that has no launcher recorded (a job nobody
  launched), **when** it posts back, **then** its tracker writes are unattended
  and use the server credential.

### US4 - A caller Sectile cannot identify writes nothing (P1)

As an operator, I want a write request that names nobody to be refused rather
than signed by the server account, so that no user action is ever anonymous on
the tracker.

**Acceptance**

- **Given** an MCP or REST call whose credential resolves to no person, **when**
  it asks for a tracker write (create, update, comment, stage transition with a
  tracker effect, macro change), **then** it fails with an explicit error saying
  the key is not tied to a user and how to get one that is (pair the desktop
  app, or use a personal API key), and nothing is written to the tracker.
- **Given** the same anonymous caller, **when** it reads (get a task, list
  tasks, read project context), **then** it behaves as today.
- **Given** a web session or a paired device, **then** its writes are made as
  its person, as in US2.

### US5 - The synchronisation keeps the server credential (P1)

As an operator, I want the synchronisation to keep working without anybody's
personal credential.

**Acceptance**

- **Given** a synchronisation pass, timer-driven or asked for by a person,
  **then** it reads, and makes the writes it makes on its own, with the server
  credential, as specified by #464.
- **Given** a job nobody launched, **then** its tracker writes use the server
  credential.
- **Given** any person-caused write, **then** it is never counted as
  unattended, even when it runs later from a queue.

### US6 - Reads are unchanged (P2)

**Acceptance**

- **Given** a person without a personal GitHub or GitLab credential, **when**
  they open a ticket, a board or a pull-request state, **then** the data is
  read as today, with the server credential.
- **Given** any person-caused read on Jira, **then** it behaves as today.

## Functional requirements

- **FR-1** A person-caused tracker write carries the identity of the person who
  caused it from the request to the tracker call, including through queued
  operations and reports posted after a transition.
- **FR-2** A person-caused write uses that person's personal credential for the
  provider. Without one, or with one that cannot be opened, the write is
  refused; it never uses the server credential. This holds for GitHub, Jira and
  GitLab.
- **FR-3** A write that is neither person-caused nor explicitly unattended is
  refused. Losing the acting person on the way is a failure, never a fallback
  to the server credential.
- **FR-4** Only the synchronisation pass and jobs nobody launched make
  unattended writes, with the server credential.
- **FR-5** A managed run's post-back writes as its launcher; with no launcher
  recorded, it is unattended.
- **FR-6** An MCP or REST write whose caller resolves to no person is refused
  with an explicit error before anything is written.
- **FR-7** A refused tracker write during a stage transition keeps the local
  transition and ends the activity in failure with the refusal as a step.
- **FR-8** The refusal message names the provider and points to
  *Profile → Tracker credentials*. It is the same for all three providers.
- **FR-9** Person-caused reads are unchanged.
- **FR-10** The existing Jira behaviour for a person without a token (refusal)
  is unchanged; GitHub and GitLab now behave the same way.

## Documentation acceptance

- `CHANGELOG.md` `[Unreleased]` gets, under **Changed**, a line marked as
  breaking: on GitHub and GitLab, a change you make without a personal tracker
  credential is now refused instead of being written under the server account;
  add yours in *Profile → Tracker credentials*. Agent keys not tied to a user
  (shared key, credential with no paired device) can no longer write to a
  tracker; pair the desktop app or use a personal API key. (#482)
- `CHANGELOG.md` `[Unreleased]` gets, under **Fixed**: stage reports, tickets
  created by agents, converted tickets, stories created under a macro and macro
  milestones are no longer written under the server account. (#482)
- An ADR records that the server credential signs unattended work only, and
  amends ADR 0028 on the GitHub/GitLab fallback for personal writes.
- `README.md` sections that describe personal tracker credentials say they are
  required to write on every provider.

## Open requirements

None. Every product question was settled in the clarification.
