# ADR 0018: The background synchronisation runs as the project's owner

Status: Superseded by
[ADR 0028](0028-tracker-sync-uses-a-server-credential-per-provider.md): the
synchronisation no longer borrows the owner's token.

## Context

ADR 0014 made a tracker credential personal and left the background queue on
the server credential, "because it has no acting user to resolve". That was
correct for the queue, whose jobs are somebody's request delayed: since #237
each of them carries the person who asked (`SkillJob.ActingUser`,
`TrackerOp.UserID`), and the worker puts them back into the context.

The auto-synchronisation loop is the one piece of work that nobody asks for. It
queues a re-read of every unfinished work item of every project that opted in,
one job per card, every few minutes. Those jobs named nobody, so each of them
resolved the server credential.

On a deployment that holds only personal credentials, which is the shape ADR
0014 pushes Jira towards and the shape the desktop install actually has, there
is no server credential to resolve. Every one of those jobs therefore failed
with

    ❌ Échec : jira did not say which fields it has: configure the Jira account e-mail

one failed activity per unfinished work item, per pass, for good. The loop could
not work at all, and the failures buried every real one in the activity feed.

The interface has the same hole from the other side: the unit synchronisation a
person triggers on one card went through `r.Context()` rather than the acting
context every other tracker call in that file uses, so it too read as the
server.

## Decision

**A project has an owner, and the background synchronisation reads as them.**

- `projects.owner_user_id` is the account. It is set to the project's creator,
  and adopted by whoever first saves a project that predates the column. It is
  never read from a payload: a client naming its own owner would borrow anybody's
  token. An owner already recorded is never replaced, or saving somebody else's
  project would hand its synchronisation to the last person who touched it.
- **The loop borrows that account, it does not act as it.** The pass queues each
  `sync_task` job with the owner as its acting user; the worker restores it into
  the context and the read resolves the owner's credential. These are reads: no
  write is attributed to anyone by this path, so the misattribution ADR 0014
  exists to prevent is not in question.
- **The owner is on the activity** (`task_activities.user_id`), so a refusal
  says whose credential was refused rather than leaving it to be guessed.
- **An ownerless project keeps the historical behaviour**, the server
  credential, which is what `SECTILE_JIRA_TOKEN` is for. Nothing fails
  differently than it did; it simply has an owner to borrow from as soon as
  somebody saves the project.
- **A locked credential still fails, loudly.** Borrowing resolves through the
  same path as everything else, so a sealed token nobody unlocked refuses the
  read instead of falling back to the server (ADR 0014).

Service accounts, which is what a shared deployment eventually wants here, are
not closed off by any of this: they would be an owner like another.

## Rejected alternatives

- **Leaving the loop on the server credential and only stopping the flood** (one
  clear refusal per project instead of one per card). Honest, and it fixes the
  symptom, but the auto-synchronisation stays dead on the deployments it was
  written for.
- **Borrowing any credential holder's token.** No rule anyone could predict:
  whose token a project reads with would depend on who signed up first.
- **Making the owner a field of the project payload.** The owner decides whose
  token is borrowed, so a client that can name it can borrow it.
- **Requiring a server credential for auto-synchronisation.** It sends a person
  to configure a deployment they usually cannot reach, to fix something they
  can, which is the reasoning `missingCredential` already follows.

## Consequences

- A deployment holding only personal credentials gets a working
  auto-synchronisation as soon as each project has been saved once.
- `owner_user_id` is the second column added after PostgreSQL support shipped,
  so it takes the route ADR 0017 opened: declared in `CREATE TABLE`, added by
  the legacy migrations for SQLite, and reconciled in `initSchema` with an
  idempotent `ADD COLUMN IF NOT EXISTS` for a PostgreSQL database created by an
  earlier version.
- The owner's Jira permissions now bound what the background pass can read. A
  card the owner cannot see is a card the loop cannot refresh, which is visible
  on the activity rather than silent.
- Deleting the owner's account, or their credential, returns the project to the
  server credential. The next person to save the project adopts it.
- GitHub refuses a locked credential as Jira does (#312). Its adapter used to
  turn a sealed token nobody unlocked into the project or server token, so a
  pass read as the service account while its activity named the owner. That
  refusal covers every call made through the GitHub tracker adapter with an
  actor, a person's write as much as a background read. The calls resolved
  through `trackerAs` (the branch pull request lookup and the GitHub GraphQL
  reads) still fall back on the project or server token; they are out of scope
  of #312.
- Two fallbacks on the project or server GitHub token remain, and both are
  decided rather than left over: an ownerless project, as above, and an actor
  who stored no personal GitHub token at all. Unlike Jira, GitHub keeps the
  second one, because a shared GitHub token (`SECTILE_GITHUB_TOKEN` or the
  project's own) is how GitHub deployments run, and refusing it would stop work
  that has nothing to do with attribution.
