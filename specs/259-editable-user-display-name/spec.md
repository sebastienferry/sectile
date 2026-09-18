# #259 — Allow a person to edit their display name

## Context

Since sign-in became mandatory (ADR 0015) every request is made by an account, and that
account carries a name: `users.display_name`. Nothing lets its owner choose it.

- An account created by the local e-mail sign-in gets its address as its name
  (`SignInLocal` passes `email` as `displayName`), so the board shows
  `firstname.lastname@company.com` everywhere a person is named.
- An account created through the identity provider gets the provider's claim, which may be
  a corporate spelling its owner does not recognise.
- The one free-text name field that used to exist in the profile — the legacy
  `settings.userName` — is now hidden as soon as an account exists (`ProfileModal.tsx`,
  the block guarded by `hasAccount`), so today there is no field at all.
- Worse, the name is rewritten at every sign-in: `UpsertUser` runs
  `UPDATE users SET email = ?, display_name = ?` on each sign-in, so even a name written
  straight into the database would be lost at the next visit.

The account name is shown by `/api/me` (the account section of the profile), by
`/api/users` (the admin users panel) and next to every execution in the activity log
(`ownerDisplayName` in `internal/db/db.go`).

This file describes behaviour only. The technical choices are in `plan.md`, the ordered
work in `tasks.md`.

## Decision being specified

A signed-in person can change their own display name from the account section of their
profile. The name they chose is theirs: it is shown wherever the account is named, and no
later sign-in overwrites it. An account that never chose a name keeps behaving exactly as
it does today.

Out of scope: the avatar, the e-mail address (it identifies the account and stays
read-only), and the legacy `settings.userName` used by the "My tasks" filter, which names
the person *as the tracker spells them* and is a different thing — see the open question
OQ1.

## User stories

### US1 — I choose the name the board shows for me (P1)

As a signed-in person, I want to type the name I am known by, so that the board stops
calling me by my e-mail address.

- **Given** I am signed in and my profile is open on the account section
- **When** I read the section
- **Then** a **Display name** field is shown, filled with the name currently on my account.
- **Given** I type `Sébastien Ferry` and save
- **When** the call returns
- **Then** the account section says I am signed in as `Sébastien Ferry`, and the change is
  visible without reloading the interface.
- **Given** I reopen the interface later
- **When** `/api/me` answers
- **Then** it carries `Sébastien Ferry`.

### US2 — My name survives the next sign-in (P1)

As a person who chose a name, I want it to still be mine tomorrow, so that the change is
not undone by the mechanism that created my account.

- **Given** I chose a display name and signed out
- **When** I sign in again with the local e-mail sign-in
- **Then** my chosen name is still shown, not my e-mail address.
- **Given** the same chosen name on a deployment behind an identity provider
- **When** I sign in and the provider sends its own `name` claim
- **Then** my chosen name stands and the claim does not replace it, while my e-mail keeps
  following the provider.

### US3 — An empty name falls back, it does not blank the board (P2)

As a person who cleared the field, I want the board to name me by something, so that no
row shows an empty owner.

- **Given** my profile is open
- **When** I clear the display name and save
- **Then** the call succeeds, my account carries no chosen name any more, and everywhere I
  am named the board shows my e-mail address — the fallback `displayName || email || id`
  that `/api/me`, the users panel and the activity log already apply.

### US4 — A name is a name, not a payload (P2)

As the operator of a deployment, I want the field to refuse what is not a name, so that a
crafted value cannot reach the board's rows.

- **Given** a display name of more than 80 characters
- **When** it is saved
- **Then** the call is refused with `400` and the message
  `A display name is at most 80 characters`, and the stored name is unchanged.
- **Given** a value with leading or trailing whitespace, or with a line break or a control
  character
- **When** it is saved
- **Then** the surrounding whitespace is trimmed and the control characters are refused
  with `400` and the message `A display name cannot contain line breaks`.
- **Given** a name another account already uses
- **When** it is saved
- **Then** it is accepted: the account is identified by its e-mail, not by its name.

### US5 — I can only rename myself (P1)

As a person using a shared deployment, I want nobody else's name to be changeable through
my session, so that the account list stays truthful.

- **Given** I am signed in as a member or as an admin
- **When** the rename call names an account
- **Then** it only ever writes the caller's own account; there is no route that renames
  another one (OQ2 is the open question of whether an admin should get one).
- **Given** no session cookie and no workstation API key
- **When** the rename call is made
- **Then** it answers `401`, like every other interface route since ADR 0015.

### US6 — The admin users panel shows the chosen names (P2)

As an admin, I want the users panel to show people as they chose to be shown, so that I
grant roles to names I recognise.

- **Given** a colleague chose a display name
- **When** I open the users panel
- **Then** their row shows that name, with their e-mail underneath, and the role control
  keeps working exactly as before.

## Non-functional requirements

- **NFR1** — The rename writes one row and answers in the same order of magnitude as the
  existing settings save; no reload of the board is required.
- **NFR2** — The display name is shown, never used as an identifier: no join, no filter and
  no permission check may key on it.
- **NFR3** — The interface labels are added to both locales of
  `web/src/locales/translations.ts`, like every other profile label.

## Open questions

These are recorded because the clarification report for #259 could not be read back: the
tracker answers `HTTP 401` on this deployment, so the clarification comment is stored
neither on the GitHub issue nor in a readable Sectile comment. The body of the issue is
empty. Each answer below is a recommendation, not a decision.

- **OQ1 — Does `settings.userName` converge with the account name?** Today
  `settings.userName` is a separate personal setting used by the sidebar to filter the
  board on the assignee *as the tracker spells it* (`Sidebar.tsx`, `isMyTasksActive`).
  Merging the two would break that filter for anyone whose tracker name differs from their
  account name. *Recommendation: leave it alone in this ticket and treat the convergence as
  its own.* Blocks: nothing in this spec; T9 only prefills the field.
- **OQ2 — May an admin rename another account?** *Recommendation: no, not in this ticket.*
  Blocks: an extra branch in `PUT /api/users/{id}` and a field in the users panel.
- **OQ3 — Under an identity provider, is the claim or the person the authority?** This spec
  says the person (US2), by symmetry with the role, where the claim *is* the authority.
  *Recommendation: keep the person, and say so in the interface.* Blocks: the wording of
  the hint under the field and the `UpsertUser` behaviour (D2 of `plan.md`).
- **OQ4 — Is 80 characters the right ceiling?** Chosen for lack of a stated one. Blocks:
  one constant.
