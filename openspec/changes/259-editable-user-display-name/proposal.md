# A person chooses the name the board shows for them

## Why
Since sign-in became mandatory (ADR 0015) every request is made by an account, and that
account carries a name — `users.display_name` — that its owner cannot choose. An account
created by the local e-mail sign-in gets its address as its name, so the board calls people
`firstname.lastname@company.com` in the account section, in the admin users panel and next
to every execution in the activity log. An account created through the identity provider
gets the provider's claim, which may be a corporate spelling its owner does not recognise.
The one free-text name field that used to exist is hidden as soon as an account exists
(`ProfileModal.tsx`, the block guarded by `hasAccount`), so today there is no field at all.

Worse, the name is rewritten at every sign-in: `UpsertUser` runs
`UPDATE users SET email = ?, display_name = ?` on each visit, so even a name written
straight into the database would be lost at the next one.

## What Changes
A signed-in person can change their own display name from the account section of their
profile. The name they chose is theirs: it is shown wherever the account is named, and no
later sign-in overwrites it. Clearing it hands the account back to the provider's spelling,
and from there to the existing `displayName || email || id` fallback. An account that never
chose a name keeps behaving exactly as it does today.

Out of scope: the avatar, the e-mail address (it identifies the account and stays
read-only), and the legacy `settings.userName` used by the sidebar's "My tasks" filter,
which names the person *as the tracker spells them* and is a different thing.

## Capabilities
### Added Capabilities
- `user-display-name`: the account name its owner chooses, and where it is shown.

## Impact
`internal/db/identity.go` (a `chosen_name` column and the resolution of `User.DisplayName`),
`internal/db/roles.go` (validation and the store write), `internal/handlers/auth.go` (a
`PATCH /api/me`), `web/src/hooks/useCurrentUser.ts`, `web/src/components/SignInStatus.tsx`,
`web/src/locales/translations.ts`. One additive column, no data migration: an account with
no chosen name reads exactly as it did before.

## Open questions
The clarification report of #259 could not be read back — the issue body is empty, it
carries no comment, and the tracker credential was unavailable to the server during the
run. Every decision it may already have settled is recorded below rather than silently
taken. Each answer is a recommendation.

- **OQ1 — Does `settings.userName` converge with the account name?** It is used by the
  sidebar to filter the board on the assignee *as the tracker spells it*
  (`Sidebar.tsx`, `isMyTasksActive`). Merging the two would break that filter for anyone
  whose tracker name differs from their account name. *Recommendation: leave it alone here.*
- **OQ2 — May an admin rename another account?** *Recommendation: no, not in this change.*
- **OQ3 — Under an identity provider, is the claim or the person the authority?** This
  proposal says the person. *Recommendation: keep it, and say so in the interface.*
- **OQ4 — Is 80 characters the right ceiling?** Chosen for lack of a stated one.

None of them blocks the task list.
