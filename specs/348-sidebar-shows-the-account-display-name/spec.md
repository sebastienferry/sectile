# #348 — The chrome shows the name the account actually carries

## Context

Two identities describe the same person in Sectile, and the interface chrome reads the
wrong one.

The **account** identity lives in `users`. Settings → Account renames it through
`PATCH /api/me` → `SetDisplayName` → `users.chosen_name`, and every reader of a user
resolves it through `userColumns`
(`COALESCE(NULLIF(chosen_name, ''), display_name)`, `internal/db/identity.go:436`). That is
the name execution owners show in `RemoteRunBadge`, and the name comments prefer.

The **settings** identity is `settings.userName`, a free-text column in `user_settings`.
The sidebar account button (`web/src/components/Sidebar.tsx:955,960`) and the status bar
(`web/src/components/StatusBar.tsx:43`) render it. Nothing in the interface writes it any
more — `ProfileModal.tsx:153` only posts back the value it just read — so it stays at
whatever the database seeded, the literal `'Developer'` (`internal/db/db.go:319,883,900,932,3348,3591`,
mirrored by `web/src/context/AppContext.tsx:368`). Renaming the account therefore changes
the name everywhere except the two places the person is looking at.

ADR 0015 already fixed the symmetric defect for the address: *"`userEmail` is a projection.
The read answers the account's address; a write ignores it."* `userName` was left out of
that retirement. This ticket finishes it.

This file describes behaviour only. The implementation choices are in [`plan.md`](./plan.md),
the ordered work in [`tasks.md`](./tasks.md). The decisions it applies are recorded in
[`docs/clarifications/348.md`](../../docs/clarifications/348.md), where the owner accepted
all six Round 1 recommendations.

## Decision being specified

`settings.userName` becomes a read-only projection of the signed-in account's identity,
exactly as `settings.userEmail` already is. `GET /api/settings` answers the account's
resolved name; a `POST`/`PUT` that carries `userName` is accepted and the field ignored.
Settings → Account stays the one and only place a name is changed, and `'Developer'` stops
being a value anything displays.

## User stories

### US1 — Renaming the account renames it in the chrome (P1)

As anyone signed in, I want the name I set in Settings → Account to be the name the sidebar
and the status bar show, so that the interface agrees with itself about who I am.

- **Given** I am signed in and my account's display name is `Alice Martin`
- **When** the interface loads `GET /api/settings`
- **Then** `userName` answers `Alice Martin`, and the sidebar account button, its initials
  and the status bar button all show that name.
- **Given** the sidebar shows `Developer`
- **When** I rename myself to `Alice Martin` in Settings → Account and the settings are
  re-read
- **Then** the sidebar and the status bar show `Alice Martin`, the same name
  `RemoteRunBadge` already shows for my executions.
- **Given** two people are signed in on the same deployment
- **When** each reads `GET /api/settings`
- **Then** each is answered their own account's name, never the other's and never a
  deployment-wide value.

### US2 — A name that was never set still reads as somebody (P1)

As the holder of an account with no display name — the `default` account, a local agent's
implicit user — I want the chrome to name me by something real, so that no row reads as
anonymous and `Developer` never appears.

- **Given** my account has an empty `chosen_name` and an empty `display_name` but the
  address `bob@example.com`
- **When** the interface reads `GET /api/settings`
- **Then** `userName` answers `bob@example.com`.
- **Given** my account has no name and no address either
- **When** the interface reads `GET /api/settings`
- **Then** `userName` answers my account id.
- **Given** any signed-in caller
- **When** `GET /api/settings` answers
- **Then** `userName` is never empty and never the literal `Developer`.

### US3 — Writing the name has no effect and no error (P1)

As an agent or a component posting the whole settings object, I want a `userName` in my
payload to be ignored rather than refused, so that an existing caller keeps working and the
stored value cannot drift from the account again.

- **Given** I am a member (not an admin)
- **When** I `POST /api/settings` with `{"userName":"someone else"}`
- **Then** the call answers `200`, not `403` — the key stays in `personalSettingsKeys`, so a
  whole-row post from the interface is not read as an admin-only violation.
- **Given** that same post has been made
- **When** I read `GET /api/settings` back
- **Then** `userName` still answers my account's resolved name, not `someone else`.
- **Given** I post a payload carrying `userName` alongside preferences I do own
- **When** the call returns
- **Then** those preferences are saved as usual: ignoring `userName` never discards the rest
  of the payload.

### US4 — No maintainer's name ships as a default (P2)

As anyone reading the interface before the settings have loaded, I want the placeholders to
be neutral, so that I never see somebody else's name presented as mine.

- **Given** the interface renders the sidebar account button
- **When** `settings.userName` or `settings.userEmail` is momentarily empty
- **Then** nothing shows `Sylvain Ferry`, `SF`, `Paramètres & Profil` or `Profile` — those
  literals are gone from the source.
- **Given** the pre-fetch default settings in `AppContext`
- **When** they are used before the first `GET /api/settings` answers
- **Then** `userName` is an empty string, not `Developer`.

### US5 — The recorded decision matches the code (P2)

As whoever reads the architecture records next, I want ADR 0015 to say what the code does,
so that the next reader does not re-derive this ticket.

- **Given** `docs/adrs/0015-sign-in-is-mandatory-and-settings-are-personal.md`
- **When** I read its projection paragraph and its personal-half list
- **Then** `userName` is named there as a projection alongside `userEmail`, and is not
  presented as a field anyone types.

## Scope

| Concern | Where |
|---|---|
| Projection on read | `internal/handlers/handlers.go` — `composedSettings` |
| Write ignored | `internal/handlers/handlers.go` — `HandleSettings` |
| Fallback chain | `internal/db/identity.go` — the existing `User.Name()` |
| Sidebar placeholders | `web/src/components/Sidebar.tsx:955,960,963` |
| Status bar placeholder | `web/src/components/StatusBar.tsx:43` |
| Pre-fetch default | `web/src/context/AppContext.tsx:368` |
| Payload no longer carries the field | `web/src/components/ProfileModal.tsx:153` |
| Record | `docs/adrs/0015-sign-in-is-mandatory-and-settings-are-personal.md` |
| Tests | `internal/handlers/authz_test.go`, `web/tests/` |

## Out of scope

- **The "My Tasks" filter.** `Sidebar.tsx:260,631` keeps comparing `task.assignee` to
  `settings.userName`. That comparison pits a tracker login against a human display name and
  is a separate defect; it is raised as its own issue, not folded in here.
- The avatar image, the sign-in flow, roles and the admin users view.
- `internal/db/comments.go:155-160`, whose author fallback already prefers the identity.
- `personalSettingsKeys` (`internal/handlers/authz.go:147`), which is unchanged: removing
  `userName` from it would turn a member's whole-row post into a `403`.
- Any schema change or migration. The `user_name` columns and their `'Developer'` defaults
  stay as they are; they simply stop being read for display.
- The desktop shell (`desktop/`) and the MCP tool surface, neither of which exposes the
  field for editing.

## Open requirements

None. The clarification closed in two rounds with zero open product questions.
