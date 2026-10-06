# Specification #691 - Roadmap: republish every pending framing copy

- Ticket: https://github.com/sebastienferry/sectile/issues/691 (macro M-11)
- Branch: `feat/691`
- Clarification: `docs/clarifications/691.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Extends: #636 (framing copy of a Jira epic), ADR 0046
- Framework: Spec Kit

## Summary

The roadmap of a Jira project gains one action, beside "labels à pousser",
that republishes in one gesture the framing copy of every epic whose copy is
pending. The action queues one tracker activity, signed by the person who
starts it, that writes the epics one after another. Each failure is reported
on the activity and on the epic's own copy status, as the single "Republier"
does.

## Scope

In scope: listing the pending framing copies of a project, queuing their
republication as one batch activity, the roadmap button and its toast, the
API documentation and `CHANGELOG.md`.

Out of scope:

- The todos copy (`epic_todos`): it keeps its per-epic "Republier" (Q1).
- The single framing save and the single "Republier" button, unchanged.
- GitHub, GitLab and local projects, `M-<n>` keys and declared roadmap
  projects' epics: never listed, never written.
- Replaying copies automatically at startup (rejected by ADR 0046).
- MCP tools, the desktop app. No migration.

## Vocabulary

- **Copied epic**: a macro whose framing `eligibilityOf` sends to a Jira
  comment (`kind == jira_comment`).
- **Pending copy**: a copied epic whose framing copy status is not up to date:
  its framing is non-empty and was never copied, or the hash of its current
  rendering differs from the one last written.
- **Skipped epic**: a copied epic whose framing is empty and that has no
  framing comment yet. Nothing is written for it. Only the epics Sectile
  holds a `macros` row for are counted: an epic nobody ever classified,
  framed or copied has no row, and reading the tracker's epics to count it
  would put a Jira read on a gesture that needs none.

## User stories

### US1 (P1) - The roadmap shows how many framings wait for their copy

1. Given a Jira project with three copied epics, two framed before #636 and
   never copied, one copied and unchanged since, when the roadmap opens, then a
   button "2 cadrages à publier" shows beside "labels à pousser".
2. Given a Jira project where every copy is up to date, then the button does
   not show.
3. Given a GitHub, GitLab or local project, or a Jira project whose only
   framed macros are `M-<n>` keys or epics of a declared roadmap project, then
   the button does not show.
4. Given an epic whose framing was edited after its last copy, then it counts
   as pending.
5. Given an activity finishing, or the project changing, then the count is
   read again, as the labels count is.

### US2 (P1) - One gesture queues the writes

1. Given two pending copies and one skipped epic, when the owner clicks the
   button, then the server answers 202 with `queued: 2`, `skipped: 1` and one
   activity, and the toast says "2 cadrages en file, 1 épic ignoré (cadrage
   vide)", and the activities list is refreshed.
2. The request returns before any Jira call: the writes run in the tracker
   activity queue.
3. The activity is signed by the person who clicked, and every write goes out
   with that person's own Jira credential.
4. Given nothing pending, when POST is called, then no activity is queued and
   the answer is 200 with `queued: 0`, the skipped count and no activity; the
   toast says there was nothing to publish.
5. Given the GET and POST on `/macros/framing-mirror` and on
   `/epics/framing-mirror`, then both spellings answer the same.

### US3 (P1) - Writes and failures

1. Given the batch activity runs, then it writes the listed epics one after
   another, never in parallel, each with `force=false`.
2. Each written epic adds the step `✅ cadrage recopié en commentaire sur KEY`;
   an epic saved and copied between the click and the write adds `✅ cadrage
   de KEY déjà à jour` and makes no call.
3. Each failed epic adds `❌ KEY : <error>` and stores the error on the macro,
   so its framing line shows "échec" with its own "Republier".
4. Given at least one epic written or already up to date, the activity ends
   `completed` with "N cadrage(s) recopié(s), M échec(s) : …".
5. Given every epic failed, the activity ends `failed`; when every failure is
   a missing personal Jira token, the activity names the missing token (#645).
6. Given an epic that stopped being a copied epic before the write (its
   project changed tracker, its key became foreign), then it fails with the
   reason it stays in Sectile, and the others are still written.
7. Given a skipped epic, then no comment is ever created for it.

## Functional requirements

- FR1: `GET /api/projects/{id}/macros/framing-mirror` (and `epics/`) answers
  200 with the list of pending copies, as macros (`key`, `title`), computed
  from local state only, with no tracker read.
- FR2: `POST` on the same path answers 202 with
  `{queued, skipped, activity}` when at least one copy is pending, 200 with
  `{queued: 0, skipped, activity: null}` otherwise. Any other method answers
  405.
- FR3: The batch is a tracker op of its own kind, `epic_framing_bulk`, holding
  the keys listed at POST time. The single `epic_framing` op is unchanged.
- FR4: Eligibility is `eligibilityOf(framingCopy, key)`, checked at listing
  time and again for each epic when it is written.
- FR5: The button text and title, and the toasts, are in both web catalogs
  (French and English).

## Acceptance criteria

- [ ] US1.1 to US1.4 and FR1 covered by Go tests of the listing.
- [ ] US2.1, US2.3 to US2.5 and FR2 covered by handler tests.
- [ ] US3.1 to US3.7 covered by Go tests of the batch op.
- [ ] Web type check and lint pass; the button and toasts read from the
      catalogs.
- [ ] `docs/API_AND_DATA_SPEC.md` documents the route.
- [ ] `CHANGELOG.md` has one `Added` line under `## [Unreleased]`.

## Open points

None. The clarification settled every product question.
