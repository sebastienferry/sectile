# ADR 0059: Every action a user performs uses their own tracker credential

Status: Accepted (#862)

Supersedes: [ADR 0058](0058-a-personal-tracker-credential-signs-only-direct-ticket-actions.md),
its credential routing. Its GitHub and GitLab OAuth sections stay in force.
Restores: [ADR 0029](0029-server-tracker-credential-signs-unattended-work-only.md),
for every write a person causes.

## Context

ADR 0058 (#855) split a person's actions in two. Their direct ticket actions
used their own credential. What it called Sectile's own writes used the server
credential: stage transitions and reports, the macro lifecycle and its
mirrors, sprints, everything a skill run wrote, a stage move made from the web,
and every read. The owner rejected the split. A stage move, a macro edit or an
agent's comment is something a person did, through the web, the desktop app or
MCP, and is theirs to sign.

## Decision

**Every action a user performs uses that user's own tracker credential.** That
covers the web UI, the desktop app and MCP, including agents working for them
during a skill run. It holds for every write: creating, editing, commenting,
moving a card on any board view, a stage transition and its report, a macro and
its mirrors, a sprint. It holds for every kind of personal credential: a pasted
token, the Jira grant, a GitHub or GitLab grant. Without one, the write is
refused (`MissingPersonalCredentialError`), never sent under the server token.
A managed run's post-back writes as its launcher (ADR 0029).

**A read made for a user uses their credential first.** The server token
answers when they have none, on GitHub, GitLab and Jira alike, since a read
attributes nothing. A credential they sealed and did not unlock refuses the
read, rather than answer under another account.

**The server token serves the asynchronous synchronisation and the jobs nobody
launched, and nothing else.** Without it, such work fails up front with
`MissingServerCredentialError` (`tracker_server_credential_missing`, HTTP 503
over REST).

**Kept from ADR 0058:**
- the GitHub and GitLab OAuth connection;
- the checks: Administration checks the server credential only, and the
  personal Verify checks the caller's own credential only.

**The GitHub consent asks for `repo read:project`**, since a person's grant
reads the ProjectsV2 board statuses.

## Consequences

- Everything a person does appears on the tracker under their account,
  whatever screen, tool or agent they used.
- A person needs a personal credential to write anything, including the stage
  reports an agent posts for them, as before #855.
- The service accounts need only the access the synchronisation needs.

## Alternatives rejected

- **ADR 0058's split between direct ticket actions and Sectile's own writes.**
  It signed a person's own stage moves, macro edits and agent comments with the
  service account.
