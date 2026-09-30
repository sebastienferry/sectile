# Specification #645 - Offer to register a missing tracker token

- Ticket: https://github.com/sebastienferry/sectile/issues/645 (milestone M-11,
  Roadmap)
- Branch: `claude/clarify-issue-gh-11a4f59c-7004b5`
- Clarification: `docs/clarifications/645.md` (rounds 1 and 2, every
  recommendation accepted by the owner)
- Framework: Spec Kit

## Summary

When a person does something in the web app that writes to the tracker, and
Sectile refuses the write because that person has no token of their own for
the provider, the error notification offers a button that opens the profile
on the tracker credentials, on the provider concerned. The offer is made
both for the writes refused at once and for the queued writes that fail
later, in their activity.

## Scope

In scope: the web app (error notifications, the profile modal), the refusal
the server sends to the web app and records on a failed activity, the French
and English strings, and the changelog.

Out of scope:

- The refusal rule itself (ADR 0029): a person-caused write is never signed
  with the server credential.
- A credential that exists but is sealed and not unlocked: its error stays
  as it is, with no action.
- Sectile Desktop, and the message MCP tools return to agents.
- Disabling or annotating write actions in advance when the token is missing.
- Retrying the refused action automatically once the token is saved.
- The credential form itself, and the admin server credential.

## Vocabulary

- **Missing-token refusal**: a tracker write refused because the acting
  person has no personal credential for the provider (GitHub, GitLab or
  Jira). It is distinct from a refusal for a key tied to no user, and from a
  locked credential.
- **Immediate write**: an action whose request answers the refusal at once
  (create a task, post a comment, clone, convert to remote, migrate, sprints,
  macro and epic writes).
- **Queued write**: an action recorded on the board at once and written to
  the tracker later by a queued activity (move a card between columns,
  change the stage, assign, change the parent or epic, team, sprint, epic
  horizon, priority and quarter).
- **Token offer**: the button an error notification carries, labelled after
  the provider (for example "Ajouter mon jeton GitHub"), which opens the
  profile on the tracker credentials of that provider.

## User stories

### US1 (P1) - An immediate write refused for want of a token offers to add it

As a person without a GitHub token, when I create a ticket, I learn what is
missing and reach the place to fix it in one click.

1. Given a GitHub project and a person with no personal GitHub token, when
   they create a task, then an error notification says the write was refused
   because they have no GitHub token, and carries the token offer for GitHub.
2. Given the same person, when they post a comment on a task, then the same
   notification and offer appear.
3. Given a Jira project, then the notification and the offer name Jira; on a
   GitLab project they name GitLab.
4. Given the notification with its offer, then it stays on screen until the
   person closes it or uses the offer.
5. Given an immediate write refused for another reason (a tracker error, a
   locked credential, a key tied to no user), then the notification carries
   no token offer and shows the error as today.

### US2 (P1) - The offer opens the right place

1. Given a token offer for GitHub, when the person uses it, then the profile
   modal opens on the tracker credentials tab, with the GitHub entry open and
   ready to be filled, and the notification closes.
2. Given the profile modal already open on another tab, when the person uses
   an offer, then the modal switches to the tracker credentials tab and opens
   the provider's entry.
3. Given the profile modal opened any other way (sidebar, status bar, command
   palette), then it opens as it does today, on the tab last shown, and an
   earlier offer never forces the tracker credentials tab again.
4. Given the person saved their token, then nothing is replayed: the refused
   action is performed again by the person.

### US3 (P1) - A queued write refused for want of a token offers to add it

As a person without a token, when I move a card, I learn that the move never
reached the tracker and how to fix it.

1. Given a GitHub project and a person with no personal GitHub token, when
   they move a card to another column, then the card moves on the board, and
   when the queued activity fails, the error notification says the tracker
   write was refused for want of a GitHub token, and carries the token offer
   for GitHub, instead of the generic "skill failed" notification.
2. Given the same person, when they change a task's stage, assignee, parent,
   team or sprint, or an epic's horizon, priority or quarter, then the same
   notification and offer appear when the activity fails.
3. Given a queued write on several tickets where every ticket was refused
   for want of the token, then the offer appears once for the activity.
4. Given a queued write on several tickets where some were written and
   others refused, then the activity completes with its failures listed, as
   today, and no offer is made.
5. Given a queued activity failed for another reason, then the notification
   stays the generic one, as today.
6. Given a queued activity started by another person that fails for want of
   their token, then the current person sees the generic notification, never
   an offer to add a token of their own.
7. Given the failed activity, then its detail still shows the refusal
   message, as today.

## Functional requirements

- **FR1** The server tells a missing-token refusal apart from every other
  error, with the provider concerned, in the answer to an immediate write and
  on the failed activity of a queued write. The existing error message is
  kept unchanged.
- **FR2** A missing-token refusal of an immediate write is answered with the
  status it has today (403).
- **FR3** Every immediate write listed in the vocabulary shows, on a
  missing-token refusal, an error notification with the token offer. A call
  that today replaces the server message with a generic text (task creation)
  recognises the refusal too.
- **FR4** A queued activity that fails because every one of its tracker
  writes was a missing-token refusal is recorded as such. An activity with at
  least one successful write is not.
- **FR5** When a queued activity started by the current person fails with a
  missing-token refusal, the notification says so and carries the token
  offer, replacing the generic failure notification. Activities of other
  people keep the generic notification.
- **FR6** The token offer opens the profile modal on the tracker credentials
  tab with the provider's entry open, whether the modal was closed or open
  on another tab. Every other way of opening the modal keeps its current
  behaviour (the tab last shown); the target of an offer is forgotten once
  the modal closes.
- **FR7** A notification carrying the token offer stays on screen until
  closed or used, like the other notifications that carry an action.
- **FR8** The offer label and the notification text name the provider, and
  exist in French and English. They are interface strings, not the server's
  message.
- **FR9** Nothing else changes: a locked credential, a key tied to no user,
  Desktop and the MCP tool results behave as today.
- **FR10** `CHANGELOG.md` gets one `Added` line under `[Unreleased]` for the
  token offer.

## Acceptance criteria

- **AC1** US1 to US3 pass on a GitHub project; US1.3 is checked for Jira and
  GitLab through the provider carried by the refusal.
- **AC2** A person with a token sees no change in any scenario.
- **AC3** An existing client reading only the error message of a 403 still
  reads the same message.
- **AC4** Activities recorded before the upgrade read as not refused for want
  of a token; no activity is lost.
- **AC5** The web unit tests, the Go tests (SQLite and PostgreSQL) and the
  typecheck pass.

## Edge cases

- A person without a token on two providers gets the offer naming the
  provider of the project they acted on.
- A person whose token was deleted while a queued write waited: the activity
  fails with a missing-token refusal and carries the offer (US3.1).
- Several refused queued activities finishing together produce one
  notification each; each carries the offer.
- The person saves the token, then moves the card again: the move reaches
  the tracker, and no stale offer reappears.

## Open points

None. Every product question of the clarification is settled.
