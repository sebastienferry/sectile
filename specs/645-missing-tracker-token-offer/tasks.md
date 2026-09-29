# Tasks #645 - Offer to register a missing tracker token

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. Server: recognise the refusal (FR1, FR2)

- [ ] T1.1 `trackerapi.MissingCredentialTracker(err) string` in
  `internal/trackerapi/client.go`.
- [ ] T1.2 Tests: bare refusal, `%w`-wrapped refusal, `ErrNoActingUser`,
  `secrets.ErrSealed`, plain error.
- [ ] T1.3 `writeTrackerError` adds `code: "tracker_credential_missing"` and
  `tracker` to the missing-credential 403, message unchanged; constant
  `TrackerCredentialMissingCode`.
- [ ] T1.4 Check that every immediate write of the spec vocabulary answers a
  tracker error through `writeTrackerError`; switch any that does not.
- [ ] T1.5 Handler tests: create task and post comment by a user without a
  token answer 403 with `error`, `code`, `tracker`; an anonymous key answers
  403 without `code` (FR9).

## 2. Server: record it on the failed activity (FR1, FR4)

- [ ] T2.1 Migration 35 `task_activities.credential_missing` in
  `internal/db/migrations.go` only (next free number if 35 is taken).
- [ ] T2.2 `internal/db/migrations_test.go`: the rewind helpers drop the new
  column wherever they rewind below 35.
- [ ] T2.3 `models.TaskActivity.CredentialMissing` (`credentialMissing`).
- [ ] T2.4 `finishTrackerOp` writes the column from
  `MissingCredentialTracker(opErr)`.
- [ ] T2.5 Every activity read that fills a `TaskActivity` for the API
  selects the column (`db.go`, `postback.go`); the activity copy in
  `activityattachment.go` carries it.
- [ ] T2.6 Runners keep the refusal in their error chain: single-ticket
  runners return or `%w`-wrap the tracker error; multi-ticket runners return
  the first refusal `%w`-wrapped when no ticket was written, and no error
  when at least one was (US3.3, US3.4).
- [ ] T2.7 Tests, SQLite and PostgreSQL: stage op refused → `github`;
  multi-ticket op refused for every ticket → `github`; one success →
  completed, empty; other failure → empty; the list endpoint returns
  `credentialMissing`; an old activity reads as empty (AC4).

## 3. Web: recognise and notify (FR3, FR5, FR7, FR8)

- [ ] T3.1 `web/src/lib/trackerRefusal.ts`: `TRACKER_CREDENTIAL_MISSING`,
  `missingCredentialFromBody`, `missingCredentialFromActivity`,
  `TrackerCredentialMissingError`.
- [ ] T3.2 `web/tests/trackerRefusal.test.mjs`: matching 403, 403 without
  code, 500 with code, unknown tracker; failed/completed activity with and
  without a provider.
- [ ] T3.3 `TaskActivity.credentialMissing?` in `web/src/types/index.ts`.
- [ ] T3.4 `translations.ts`: the `trackerRefusal` group in French and
  English (`title`, `description`, `queuedDescription`, `offer`).
- [ ] T3.5 `AppContext`: `trackerError(res, data, fallback)` and
  `tokenOfferToast(tracker, taskKey?)`.
- [ ] T3.6 The immediate-write call sites use `trackerError` and show
  `tokenOfferToast` on a `TrackerCredentialMissingError`; task creation reads
  the body before throwing (FR3).
- [ ] T3.7 The activity poller shows `tokenOfferToast` for a failed activity
  of the current user with `credentialMissing`, and the generic toast
  otherwise (US3.1, US3.5, US3.6).

## 4. Web: open the profile on the provider (FR6)

- [ ] T4.1 App context: `profileTarget` and `openTrackerCredentials(tracker)`;
  closing the modal clears the target; the existing openers are unchanged.
- [ ] T4.2 `ProfileModal` switches to the `trackers` tab when a target is set,
  also while open.
- [ ] T4.3 `TrackerCredentialsTab` takes `initialOpen` and expands that
  provider's entry, following changes.
- [ ] T4.4 Using the offer dismisses its toast (check `ToastContainer`).

## 5. Browser test and documentation

- [ ] T5.1 Browser test: stubbed 403 on task creation shows the offer; the
  offer opens Profile → Trackers with GitHub expanded; a failed activity of
  the current user shows the offer, another user's shows the generic toast;
  the profile opened from the sidebar starts on its first tab.
- [ ] T5.2 `CHANGELOG.md`: the `Added` line under `[Unreleased]` (FR10).
- [ ] T5.3 `docs/USER_GUIDE.md`: one sentence where it explains the refusal
  (l.27-33), saying the notification now offers to add the token.

## 6. Gates

- [ ] T6.1 `go test ./...` (SQLite), then the db suite against a throwaway
  PostgreSQL (`SECTILE_TEST_POSTGRES_DSN`, never the dev database).
- [ ] T6.2 `cd web && npm test` and `npx tsc --noEmit`.
- [ ] T6.3 The browser test of T5.1.
