# Specification #862 - Every action a user performs uses their own tracker credential

- Ticket: https://github.com/sebastienferry/sectile/issues/862
- Branch: `feat/862`
- Clarification: `docs/clarifications/862.md` (round 1, no open question)
- Supersedes the routing of #855 (ADR 0058); keeps its OAuth connection
- Framework: Spec Kit

## Summary

#855 sent stage transitions, macro and sprint writes, skill-run writes, web
stage moves and every read to the server credential. The rule is simpler:
whatever a user does, through the web UI, the desktop app or MCP, agents
working for them included, goes out under that user's own tracker credential.
Only the asynchronous synchronisation and jobs nobody launched use the server
token.

## Scope

In scope: the credential routing of every tracker read and write; the GitHub
consent scopes; ADRs, README, user guide, API spec, `.env.sample`, CHANGELOG
and interface strings that describe the routing.

Out of scope: the OAuth connection (kept as #855 shipped it), the credential
checks (kept), registering the provider apps (#860).

## Credential routing (normative)

| Operation | Credential |
| --- | --- |
| Any write a user causes: create, edit, comment, board move, stage transition and its report, macro lifecycle and mirrors, sprints, from the web, the desktop or MCP, a skill run included | the user's own; refused without one |
| A managed run's post-back | its launcher's own |
| A read made for a user | the user's own; the server token when they have none; refused when theirs is sealed and locked |
| Synchronisation, polling, jobs nobody launched | server token |
| Administration credential check | server token |
| Personal Verify | the user's own |

## User stories

### US1 - Everything I do is signed by me (P1)

- **Given** a user with a personal credential (token or OAuth grant) **when**
  they create, edit, comment, move a card on any board view, change a stage
  from the task detail, or edit a macro or a sprint, **then** the tracker
  receives the write under their account.
- **Given** the same user running a skill (launched console or `start_run`
  session) **when** the agent creates, edits, comments or records a stage with a
  report, **then** every write is under the user's account.
- **Given** a managed run that posts back after its session ended, **then**
  its writes are under its launcher's account.
- **Given** a user without a personal credential for the provider, **when**
  they cause any of these writes, **then** nothing is written and they get the
  missing-credential refusal naming the provider and *Profile → Tracker
  credentials*.

### US2 - Reads use my access first (P1)

- **Given** a user with a personal credential, **when** a screen or an MCP read
  tool reads the tracker for them (comments, board statuses, PR states,
  assignee search), **then** the request carries their credential.
- **Given** a user without one, **then** the read carries the server token.
- **Given** a user whose credential is sealed and locked, **then** the read is
  refused, never answered under the server token.

### US3 - The synchronisation keeps the server token (P1)

- **Given** a synchronisation pass or a job nobody launched, **then** it reads
  and writes with the server token, and without one it fails with
  `tracker_server_credential_missing`.

### US4 - What stays from #855 (P1)

- Connect GitHub / Connect GitLab, OAuth apps, grant refresh and revocation
  work as shipped; the GitHub consent asks for `repo` and `read:project`.
- Administration checks the server credential only; the personal Verify
  checks the caller's own only.

## Functional requirements

- **FR-1** A write that names a person uses that person's credential, or is
  refused; it never uses the server token (ADR 0029).
- **FR-2** A read that names a person uses their credential when they have
  one, the server token otherwise; a locked credential refuses the read.
- **FR-3** Only unattended work uses the server token for writes.
- **FR-4** No marker or table routes a person's action to the server token.
- **FR-5** The GitHub consent covers every call a grant can make, ProjectsV2
  reads included.

## Documentation acceptance

- ADR 0059 records the rule and supersedes ADR 0058's routing; ADR 0058,
  ADR 0029 and ADR 0044 get status lines pointing at it.
- README, user guide, API spec, `.env.sample`, interface strings: no longer say
  that stage reports, macros, sprints, skill runs or reads use the service
  account.
- CHANGELOG `[Unreleased]`: the #855 "Changed" entry is replaced by the rule
  (#862).

## Open requirements

None.
