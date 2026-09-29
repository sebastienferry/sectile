# Plan #645 - Offer to register a missing tracker token

## Stack

Go server (`internal/trackerapi`, `internal/handlers`, `internal/db`,
`internal/models`), SQLite and PostgreSQL through the numbered migrations,
React 19 + TypeScript web app (`web/`), `node --test` unit tests and Go
tests. No agent, desktop, MCP or tracker adapter change.

## Design

### 1. The refusal names its provider (unchanged)

`trackerapi.MissingPersonalCredentialError{Tracker}` (`client.go:155`) already
carries the provider as the registry names it (`github`, `gitlab`, `jira`),
and its `Error()` text stays as it is (#482 FR-8, AC3). Jira's own check
(`jira.go:66`) returns the same type. Everything below reads the provider
with `errors.As`, never from the message.

A small helper in `internal/trackerapi/client.go`:

```go
// MissingCredentialTracker returns the provider a write was refused for, when
// err is a refusal for want of the acting person's own credential, "" otherwise.
func MissingCredentialTracker(err error) string
```

### 2. Immediate writes: a code in the 403 body (FR1, FR2)

`writeTrackerError` (`internal/handlers/handlers.go:285`) answers the
missing-credential case with the same status and message plus two fields,
following the `writeTaskBusy` precedent of a richer body:

```json
{"error": "<unchanged message>", "code": "tracker_credential_missing", "tracker": "github"}
```

A constant `TrackerCredentialMissingCode = "tracker_credential_missing"` sits
next to `writeTrackerError`. The `ErrNoActingUser` branch and the default
branch are unchanged (FR9). Every handler already routed through
`writeTrackerError` (create epic, macro writes, migrate, create task single
and batch, convert to remote, post comment, clone, sprints) gets the fields
with no change of its own.

Check while implementing: every handler listed in the spec vocabulary as an
immediate write goes through `writeTrackerError` on a tracker error; one that
calls `writeError` directly on that path is switched to it.

### 3. Queued writes: the activity records the refused provider (FR1, FR4)

- **Migration 35** `task_activities.credential_missing` in
  `internal/db/migrations.go`, nowhere else (not in the frozen baseline):

  ```sql
  ALTER TABLE task_activities ADD COLUMN credential_missing TEXT NOT NULL DEFAULT '';
  ```

  Empty means the activity was not refused for want of a token, which is what
  every existing activity reads as (AC4). If main has taken 35 by the time
  this lands, take the next free number.
- `models.TaskActivity` gains:

  ```go
  // CredentialMissing names the provider ("github", "gitlab", "jira") a
  // failed tracker write was refused for, because the person who asked for it
  // has no token of their own there (#645). Empty otherwise.
  CredentialMissing string `json:"credentialMissing,omitempty"`
  ```

- `finishTrackerOp` (`internal/db/trackerops.go:889`) sets the column to
  `trackerapi.MissingCredentialTracker(opErr)` in the same `UPDATE`, so only
  a failed operation carries it.
- The activity reads that fill a `TaskActivity` for the API select the new
  column: at least `db.go:3578`, `db.go:5371`, `db.go:5483` and
  `postback.go:309`. The copy in `activityattachment.go:58` carries it. Grep
  `completed_at, error` / `a.error` in `internal/db` for any other.
- **Runners must keep the refusal in their error chain.** `runStageOp`
  already returns the refusal it collected. The multi-ticket runners
  (move to epic, set sprint, set team, and every runner that aggregates its
  failures into `fmt.Errorf("aucun ticket ... : %s", ...)`) keep the first
  missing-credential refusal they meet and, when **no** ticket was written,
  return it wrapped with `%w` (`fmt.Errorf("aucun ticket modifié : %s: %w",
  joined, refused)`), so the message stays and `errors.As` finds the type.
  When at least one ticket was written they keep returning no error (US3.4).
  Single-ticket runners (assign, set parent, transition, epic horizon,
  priority, quarter) must return the tracker error itself or wrap it with
  `%w`; any `%v` wrapping on that path is changed to `%w`.

### 4. Web: one recogniser, one notification (FR3, FR5, FR7, FR8)

New module `web/src/lib/trackerRefusal.ts` (pure, unit-tested):

```ts
import type { TrackerKind } from './trackers' // existing type, not redefined
export const TRACKER_CREDENTIAL_MISSING = 'tracker_credential_missing'

/** The provider a failed response body was refused for, or null. */
export function missingCredentialFromBody(status: number, body: unknown): TrackerKind | null
/** The provider a failed activity was refused for, or null. */
export function missingCredentialFromActivity(act: { status: string; credentialMissing?: string }): TrackerKind | null

/** An Error that remembers the refusal, so a catch far from the fetch can recognise it. */
export class TrackerCredentialMissingError extends Error {
  constructor(message: string, readonly tracker: TrackerKind)
}
```

`missingCredentialFromBody` requires status 403, `code ===
TRACKER_CREDENTIAL_MISSING` and a known `tracker`; anything else is null.

In `web/src/context/AppContext.tsx`:

- A helper `trackerError(res, data, fallback)` returns a
  `TrackerCredentialMissingError` when `missingCredentialFromBody` matches,
  and `new Error(data.error || fallback)` otherwise. The call sites of the
  immediate writes that today do `throw new Error(data.error || ...)` use it.
- Task creation (`AppContext.tsx:2259`) reads the body before throwing, and
  uses the same helper with `t.operations.notifications.createFailed` as the
  fallback, so a missing-token refusal is recognised there too (FR3).
- A helper `tokenOfferToast(tracker)` builds the notification:
  `type: 'error'`, the title `t.trackerRefusal.title` and the description
  `t.trackerRefusal.description` with the provider name (from
  `getTrackers(t)`), and `link: { label: t.trackerRefusal.offer
  (provider), onOpen: () => openTrackerCredentials(tracker) }`. It keeps no
  `duration`: a toast with a link already stays on screen
  (`ToastContainer.tsx:10`), which is FR7.
- The catch blocks of those call sites test `err instanceof
  TrackerCredentialMissingError` and call `addToast(tokenOfferToast(...))`
  instead of their generic error toast.
- The activity poller (`AppContext.tsx:1866`): on a transition to `failed`,
  when `missingCredentialFromActivity(act)` matches **and** `act.userId` is
  the current user's id, it shows `tokenOfferToast` (with the task key in
  the description) instead of the generic `skillFailed` toast. Otherwise it
  keeps the generic toast (US3.5, US3.6).
- The web `TaskActivity` type (`web/src/types/index.ts`) gains
  `credentialMissing?: string`.

### 5. Web: open the profile on a provider (FR6)

- The app context gains a profile target, beside `isProfileOpen`:

  ```ts
  profileTarget: { tab: 'trackers'; tracker: TrackerKind } | null
  openTrackerCredentials: (tracker: TrackerKind) => void // sets the target and opens the modal
  ```

  `setIsProfileOpen(false)` clears the target. The existing openers
  (`Sidebar.tsx:775`, `StatusBar.tsx:34`, `CommandPalette.tsx:375`) keep
  calling `setIsProfileOpen(true)` with no target, so they keep opening on
  the tab last shown, as before (US2.3; the modal stays mounted and keeps its
  tab state between openings).
- `ProfileModal` (`web/src/components/ProfileModal.tsx:57`) switches
  `activeTab` to `'trackers'` whenever `profileTarget` changes to a value,
  including while it is already open (US2.2).
- `TrackerCredentialsTab` takes an optional `initialOpen?: TrackerKind` prop
  and starts its `open` state from it, and follows it when it changes, so the
  provider's entry is expanded (US2.1). `ProfileModal` passes
  `profileTarget?.tracker`.
- Using the offer also dismisses the toast; `ToastContainer` already removes
  a toast whose link is opened, to be checked, and made so if not.

### 6. Strings (FR8)

`web/src/locales/translations.ts`, French and English, a new
`trackerRefusal` group:

| key | fr | en |
| --- | --- | --- |
| `title` | Écriture refusée par le tracker | Tracker write refused |
| `description` | Vous n'avez pas de jeton {provider} personnel : cette action n'a pas été écrite sur {provider}. | You have no personal {provider} token: this action was not written to {provider}. |
| `queuedDescription` | {task} : l'écriture sur {provider} a été refusée, faute de jeton {provider} personnel. | {task}: the write to {provider} was refused, for want of a personal {provider} token. |
| `offer` | Ajouter mon jeton {provider} | Add my {provider} token |

### 7. Changelog (FR10)

One line under `## [Unreleased]` → `### Added`:
"When a tracker write is refused because you have no personal token for the
tracker, the error notification now offers to add it, and opens your profile
on that tracker's credentials. (#645)"

## Target files

- `internal/trackerapi/client.go` (helper)
- `internal/handlers/handlers.go` (`writeTrackerError`)
- `internal/db/migrations.go` (migration 35)
- `internal/db/migrations_test.go` (rewind helpers drop the new column)
- `internal/models/models.go` (`TaskActivity.CredentialMissing`)
- `internal/db/trackerops.go` (`finishTrackerOp`, runner wrapping)
- `internal/db/db.go`, `internal/db/postback.go`,
  `internal/db/activityattachment.go` (activity columns)
- `web/src/lib/trackerRefusal.ts` (new)
- `web/src/context/AppContext.tsx`, `web/src/types/index.ts`
- `web/src/components/ProfileModal.tsx`,
  `web/src/components/TrackerCredentialsTab.tsx`,
  `web/src/components/ToastContainer.tsx` (only if US2.1 needs it)
- `web/src/locales/translations.ts`
- `CHANGELOG.md`

## Rejected alternatives

- **Detect the refusal from the message text** on the client or on the
  stored activity: it couples the UI to an English sentence that #482 may
  reword, and the multi-ticket runners wrap it inside French text.
- **A generic `error_code` column** on activities: no second code exists
  today; a column named after what it holds says more, and a generic one can
  come when a second code does.
- **A dialog embedding the credential form with an automatic retry**: the
  owner chose the notification action (clarification round 2, decision 2).
- **Reusing `TrackerSetup`** instead of the profile: the ticket asks for the
  user profile, and `TrackerSetup` is the first-run wrapper that also picks
  the tracker.

## Risks

- **Rewind tests** (memory: new ADD COLUMN breaks ~10 db tests): the helpers
  in `migrations_test.go` that rewind below 35 must drop
  `task_activities.credential_missing`.
- **PostgreSQL**: run the db suite with `SECTILE_TEST_POSTGRES_DSN` on a
  throwaway database, never on dev.
- **Poller identity**: the current user's id must be the one activities
  carry in `userId` (`usr_...`); check the source used by `useCurrentUser`.

## Test plan

- Go, `internal/trackerapi`: `MissingCredentialTracker` on a bare refusal, a
  `%w`-wrapped one, `ErrNoActingUser`, `ErrSealed` and a plain error.
- Go, `internal/handlers`: a create-task and a post-comment request by a user
  without a token answer 403 with the unchanged `error`, `code` and
  `tracker`; an anonymous key still answers 403 without `code`.
- Go, `internal/db`: a stage op and a multi-ticket op refused for every
  ticket record `credential_missing = 'github'`; a multi-ticket op with one
  success records nothing and completes; an op failing for another reason
  records nothing; the column reads back through the activity list. SQLite
  and PostgreSQL.
- Go, migrations: 35 applies on an existing database; old activities read
  as empty.
- Web, `web/tests/trackerRefusal.test.mjs`: `missingCredentialFromBody` for a
  matching 403, a 403 without code, a 500 with the code, an unknown tracker;
  `missingCredentialFromActivity` for failed/completed and empty/known
  values.
- Web, a browser test (`web/tests/*.browser.mjs`, Playwright from
  `desktop/node_modules`): a stubbed 403 on task creation shows the toast with
  the offer; using it opens the profile on the Trackers tab with the GitHub
  entry open; a failed activity of the current user shows the offer, one of
  another user shows the generic toast.
- `cd web && npm test`, `npx tsc --noEmit`, `go test ./...`.
