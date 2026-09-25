# ADR 0029: The server tracker credential signs unattended work only

Status: Accepted

Amends: [ADR 0028](0028-tracker-sync-uses-a-server-credential-per-provider.md),
its sentence "GitHub and GitLab fall back to the server credential" for writes
somebody asks for.

## Context

Since #464 each provider has one server credential, meant for the
synchronisation. It also signed writes a person had caused, in two ways.

- **By design.** On GitHub and GitLab, a person who stored no personal token
  wrote under the server account (ADR 0028). Jira already refused.
- **By accident.** A write that lost its author on the way looked exactly like
  the synchronisation: an empty acting user meant "unattended". That is how a
  stage report posted as a comment, a ticket created by an agent through MCP, a
  local ticket converted to a remote one, a story created under a macro and
  every GitHub milestone write backing a macro went out under the server
  account, on Jira too. They were called with `context.Background()`, or on the
  project's server client directly.

#482 settles the model. Its clarification is in `docs/clarifications/482.md`,
and its specification is in `specs/482-token-isolation/`.

## Decision

**A tracker write a person causes uses that person's own credential, on every
provider, or does not happen.** It is refused when they stored none, and when
theirs is sealed and not unlocked, with a message naming the provider and
*Profile → Tracker credentials*. The message is the same on Jira, GitHub and
GitLab. This includes the writes Sectile makes later on their behalf: a queued
operation, a stage report, a managed run's transition.

**Unattended work says so.** The synchronisation pass, timer-driven or asked
for, marks its context with `tracker.WithUnattended`, and a queued operation
records `TrackerOp.Unattended` from the context that queued it. Only such work
writes with the server credential.

**A write that names neither a person nor the marker is refused**
(`trackerapi.ErrNoActingUser`). Losing the author is a failure, never a
fallback. Every write resolves its client through one function,
`trackerapi.Client.ForWrite`:

| Context | Personal credential | Result |
| --- | --- | --- |
| person named | found | their client |
| person named | none | `MissingPersonalCredentialError` |
| person named | sealed, locked | the resolution error |
| nobody, unattended | - | the server client |
| nobody, not unattended | - | `ErrNoActingUser` |

The Jira and GitHub adapters resolve reads and writes separately. Writes made on
the client itself (GitHub milestones, issue transfers) go through
`DB.trackerForWrite`. GitLab has no write adapter yet; `ForWrite` already
handles it, so its first write gets the rule.

**A caller Sectile cannot tie to a person writes nothing.** An MCP write tool
(`create_task`, `update_task`, `add_comment`, `transition_stage`) called with
the shared server key, or with no resolved caller, is refused before anything
is written. Over REST the same write fails at the tracker client and is answered
with a 403. Reads and run reporting are unchanged.

**Reads are unchanged.** A person without a GitHub token still reads with the
server credential; Jira keeps refusing, as before.

## Consequences

- Breaking for deployments that relied on a shared GitHub or GitLab token for
  people's writes: every person who changes something on the tracker needs a
  personal credential. The synchronisation keeps working without one.
- Breaking for agents using the shared server key: they can still read and
  report runs, but no longer write to a tracker. Pairing the desktop app, or a
  personal API key, restores it.
- A stage recorded by a person without a credential is still recorded on the
  board; its activity ends in failure with the refusal as a step, and no report
  or label reaches the tracker.
- A macro edit, creation or deletion refused on GitHub keeps its local effect,
  as any failed milestone write did, and the refusal is now returned instead of
  dropped. A migration is refused before anything moves.
- The next write path that forgets its context fails loudly in its activity
  instead of leaking.

## Alternatives rejected

- **Keep "empty actor means unattended" and fix the known paths.** It fixes
  today's leaks, and the next lost context leaks again, silently.
- **Refuse reads too for people without a token.** A read attributes nothing,
  and refusing it would blank screens for no gain in attribution.
- **Refuse unidentified callers at authentication.** It would also cut their
  reads and run reporting, which nobody asked for.
- **Fall back to the server credential for GitHub and GitLab, as before.** It is
  exactly the misattribution this record removes.
