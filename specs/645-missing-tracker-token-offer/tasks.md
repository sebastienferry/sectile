# Tasks #645 - Offer to register a missing tracker token

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. Server: recognise the refusal (FR1, FR2)

- [x] T1.1 `trackerapi.MissingCredentialTracker(err) string` in
  `internal/trackerapi/client.go`.
- [x] T1.2 Tests: bare refusal, `%w`-wrapped refusal, `ErrNoActingUser`,
  `secrets.ErrSealed`, plain error.
- [x] T1.3 `writeTrackerError` adds `code: "tracker_credential_missing"` and
  `tracker` to the missing-credential 403, message unchanged; constant
  `TrackerCredentialMissingCode`.
- [x] T1.4 Check that every immediate write of the spec vocabulary answers a
  tracker error through `writeTrackerError`; switch any that does not.
- [x] T1.5 Handler tests: create task and post comment by a user without a
  token answer 403 with `error`, `code`, `tracker`; an anonymous key answers
  403 without `code` (FR9).

## 2. Server: record it on the failed activity (FR1, FR4)

- [x] T2.1 Migration 35 `task_activities.credential_missing` in
  `internal/db/migrations.go` only (next free number if 35 is taken).
- [x] T2.2 `internal/db/migrations_test.go`: the rewind helpers drop the new
  column wherever they rewind below 35.
- [x] T2.3 `models.TaskActivity.CredentialMissing` (`credentialMissing`).
- [x] T2.4 `finishTrackerOp` writes the column from
  `MissingCredentialTracker(opErr)`.
- [x] T2.5 Every activity read that fills a `TaskActivity` for the API
  selects the column (`db.go`, `postback.go`); the activity copy in
  `activityattachment.go` carries it.
- [x] T2.6 Runners keep the refusal in their error chain: single-ticket
  runners return or `%w`-wrap the tracker error; multi-ticket runners return
  the first refusal `%w`-wrapped when no ticket was written, and no error
  when at least one was (US3.3, US3.4).
- [x] T2.7 Tests, SQLite and PostgreSQL: stage op refused → `github`;
  multi-ticket op refused for every ticket → `github`; one success →
  completed, empty; other failure → empty; the list endpoint returns
  `credentialMissing`; an old activity reads as empty (AC4).

## 3. Web: recognise and notify (FR3, FR5, FR7, FR8)

- [x] T3.1 `web/src/lib/trackerRefusal.ts`: `TRACKER_CREDENTIAL_MISSING`,
  `missingCredentialFromBody`, `missingCredentialFromActivity`,
  `TrackerCredentialMissingError`.
- [x] T3.2 `web/tests/trackerRefusal.test.mjs`: matching 403, 403 without
  code, 500 with code, unknown tracker; failed/completed activity with and
  without a provider.
- [x] T3.3 `TaskActivity.credentialMissing?` in `web/src/types/index.ts`.
- [x] T3.4 `translations.ts`: the `trackerRefusal` group in French and
  English (`title`, `description`, `queuedDescription`, `offer`).
- [x] T3.5 `AppContext`: `trackerError(res, data, fallback)` and
  `tokenOfferToast(tracker, taskKey?)`.
- [x] T3.6 The immediate-write call sites use `trackerError` and show
  `tokenOfferToast` on a `TrackerCredentialMissingError`; task creation reads
  the body before throwing (FR3).
- [x] T3.7 The activity poller shows `tokenOfferToast` for a failed activity
  of the current user with `credentialMissing`, and the generic toast
  otherwise (US3.1, US3.5, US3.6).

## 4. Web: open the profile on the provider (FR6)

- [x] T4.1 App context: `profileTarget` and `openTrackerCredentials(tracker)`;
  closing the modal clears the target; the existing openers are unchanged.
- [x] T4.2 `ProfileModal` switches to the `trackers` tab when a target is set,
  also while open.
- [x] T4.3 `TrackerCredentialsTab` takes `initialOpen` and expands that
  provider's entry, following changes.
- [x] T4.4 Using the offer dismisses its toast (check `ToastContainer`).

## 5. Browser test and documentation

- [x] T5.1 Browser test: stubbed 403 on task creation shows the offer; the
  offer opens Profile → Trackers with GitHub expanded; a failed activity of
  the current user shows the offer, another user's shows the generic toast;
  the profile opened from the sidebar starts on its first tab.
- [x] T5.2 `CHANGELOG.md`: the `Added` line under `[Unreleased]` (FR10).
- [x] T5.3 `docs/USER_GUIDE.md`: one sentence where it explains the refusal
  (l.27-33), saying the notification now offers to add the token.

## 6. Gates

- [x] T6.1 `go test ./...` (SQLite), then the db suite against a throwaway
  PostgreSQL (`SECTILE_TEST_POSTGRES_DSN`, never the dev database).
- [x] T6.2 `cd web && npm test` and `npx tsc --noEmit`.
- [x] T6.3 The browser test of T5.1.

## Notes from the implementation

- T2.6: a batch runner keeps the refusal only when every failed ticket was
  refused for want of the token (`refusalOrFailures`), not merely the first
  one met, so FR4 holds for mixed failures.
- T2.6: `runTransitionOp` on a tracker without `CapTransition` (GitHub) used
  to swallow the `UpdateIssue` error and complete; a missing-token refusal
  now fails the activity there, like the stage operation.
- T2.7: `stagecommits_test.go` also rewinds below 35 and drops the column.
- T3.6: task creation recognises the refusal but keeps its generic
  "La création a échoué" for every other failure, as before.
- T4.2 / US2.3: the profile modal stays mounted and keeps the tab last shown
  between openings; the spec now says so. The offer's target is forgotten on
  close, so it never steers a later ordinary opening.
- T4.4: `ToastContainer` already removes a toast whose link is used.
- T1.4 / T1.5: the 15 handlers of `handlers.go` and the one of `sprints.go`
  that answer a tracker write already go through `writeTrackerError`; the
  handler test covers task creation, and the comment post shares the same
  branch without a test of its own.
- T6.1: `go test ./...` is green on SQLite. On PostgreSQL the db suite is
  green except `TestPostgresConcurrentOutputAppendsTruncateOnce`, which fails
  identically on `origin/main` (e227c762), unrelated to this change.
- T6.3: every `*.browser.mjs` passes except `board-filters`, which times out
  on the sprint filter placeholder identically on `origin/main`.
