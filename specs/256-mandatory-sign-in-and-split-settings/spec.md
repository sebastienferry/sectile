# #256 — Make sign-in mandatory and split settings into personal and deployment

## Context

Sign-in already exists: the e-mail sign-in screen, the `HttpOnly` session cookie, the
server-side session and the sign-out button shipped with ADR 0013; personal tracker
credentials shipped with ADR 0014. Two things keep that identity from being real.

1. **The implicit mode hides sign-in.** While no local account exists, `signInMode()`
   returns `implicit`, `webSessionUser()` resolves an anonymous request to the `default`
   user and `RequireSession` lets everything through. On a fresh deployment there is no
   sign-in screen and every visitor is an admin.
2. **`settings` is a single global row.** Theme, language, density, identity and
   workstation commands are shared by the whole team. ADR 0013 already recorded this as
   debt.

This specification describes behaviour only. The technical choices are in `plan.md`,
the ordered work in `tasks.md`.

## Decision being specified

Sign-in becomes mandatory: an unauthenticated visitor sees the sign-in screen and nothing
else. One e-mail is one account, one session, and its own presentation, identity and
workstation settings. There is **no login password**; the only secret ever asked for is the
ADR 0014 sealing passphrase, and only when the person chose to seal their tracker tokens.

## User stories

### US1 — A fresh deployment asks who you are (P1)

As the first person opening a newly installed Sectile, I want to be asked for my e-mail
before I see anything, so that the board is never open to an anonymous visitor.

- **Given** a deployment with no account at all
- **When** I open the interface
- **Then** the sign-in screen is shown, the board does not mount, and no business call is
  fired before `/api/me` has answered.
- **Given** the same deployment
- **When** I sign in with my e-mail
- **Then** an account is created, it holds the admin role (unchanged ADR 0013 rule), and a
  session cookie carries me to the board.

### US2 — An anonymous API call is refused (P1)

As the operator of a deployment, I want every interface API call without a session to be
refused, so that the cookie is the only way in.

- **Given** no session cookie and no workstation API key
- **When** a call reaches an interface API route
- **Then** the server answers `401` with the message `Sign in to use this interface`.
- **Given** the same absence of credential
- **When** the call is `/api/me`, `/auth/…`, `/api/v1/agent/…`, `/mcp`, `/ws/agent-connect`,
  `/health` or a static file of the interface
- **Then** it is served as before.
- **Given** a valid workstation API key on the `Authorization` header
- **When** the agent gateway forwards a console call
- **Then** it is served as the user that key names, exactly as today.

### US3 — Signing in and out (P1)

As a signed-in person, I want my session to behave as ADR 0013 described it, so that
nothing about the cookie changes with this ticket.

- **Given** I sign in with my e-mail
- **When** the server answers
- **Then** the session cookie is `HttpOnly`, `SameSite=Lax`, `Secure` behind TLS, lives 12
  hours, and only its hash is stored.
- **Given** I am signed in in two tabs or on two devices
- **When** I open a second session
- **Then** the first one stays valid.
- **Given** I press sign out
- **When** the call returns
- **Then** the session is revoked server-side and the next API call answers `401`.

### US4 — Two people, two presentations (P1)

As one of two people using the same server, I want my theme, language, density, default
view, scale, detail mode, identity and workstation commands to be mine, so that a colleague
changing theirs changes nothing for me.

- **Given** two accounts signed in on the same deployment
- **When** each of them sets a different theme, language, density, default view, UI scale,
  detail mode, avatar, editor command and external terminal command
- **Then** each sees their own values, and neither sees the other's.
- **Given** a brand-new account
- **When** its settings are read for the first time
- **Then** the values are those of the deployment's current global row, which is the
  default for every new account.
- **Given** an execution owned by a person
- **When** an external terminal or editor is launched for it
- **Then** the editor and external terminal commands used are that owner's, not the global
  row's.

### US5 — The deployment configuration stays shared and admin-only (P1)

As an admin, I want the tracker, repository, auto-sync, AI and prompt settings to stay one
configuration for the whole deployment, so that a member cannot repoint the board.

- **Given** I am a member
- **When** I save a change to `issueTracker`, `github*`, `jira*`, `gitlab*`, `repoPath`,
  `autoSyncEnabled`, `autoSyncIntervalSec`, `aiProvider`, `aiModel`,
  `aiCommandTemplate*`, `aiSkillModels`, `aiProviderModels`, `prompt*` or `specFramework`
- **Then** the save is refused with `403` naming the offending keys, and nothing is written.
- **Given** I am a member
- **When** I save only personal preferences
- **Then** the save succeeds and no deployment value is altered, including when my payload
  omits keys.
- **Given** an execution or a skill launch
- **When** the AI configuration is read (`applySkillCommandOverride`, `applyProjectSettings`)
- **Then** it keeps coming from the deployment row.

### US6 — My e-mail is my account, not a free-text field (P2)

As a signed-in person, I want the identity shown in the interface to be the account I
signed in with, so that it cannot drift from it.

- **Given** I am signed in as `me@example.com`
- **When** I read my settings
- **Then** `userEmail` is that address.
- **When** I try to change `userEmail`
- **Then** the stored address is unchanged; the field is a projection of the account, which
  is the retirement ADR 0013 asked for.

### US7 — Unlocking sealed tracker tokens at sign-in (P2)

As a person who sealed their tracker tokens, I want to give my sealing passphrase on the
sign-in screen, so that my tokens are usable straight away.

- **Given** I have sealed tracker tokens
- **When** I sign in with my e-mail and the right passphrase
- **Then** the session opens and every sealed token of mine is unlocked.
- **When** I sign in with a wrong passphrase
- **Then** the session still opens, a message reports the failure, and the tokens stay
  locked until I unlock them from the profile.
- **When** I sign in without a passphrase
- **Then** I am signed in normally with my sealed tokens locked; a sealed token that is then
  needed fails loudly and never falls back to the server token (ADR 0014).

### US8 — An existing deployment is not disrupted (P1)

As the operator of a deployment already in use, I want the upgrade to change nothing I
depend on, so that nobody is locked out and nothing has to be re-paired.

- **Given** an existing deployment where `default` is an admin
- **When** the new version starts
- **Then** the `default` row stays with its stored role, it stays the admin, and it is an
  ordinary account: no automatic adoption and no merge.
- **Then** its executions keep their owner and its paired workstations keep naming a valid
  user and keep working without re-pairing.
- **Given** an admin who wants that data
- **When** they act from the users view
- **Then** they can claim the `default` account or reassign its data.

## Functional requirements

- **FR1** `signInMode()` never returns `implicit`; the mode is `oidc` with a provider and
  `local` otherwise.
- **FR2** `webSessionUser()` returns the empty string when there is neither a valid session
  cookie nor a valid workstation API key. It never falls back to `ImplicitUser`.
- **FR3** `RequireSession` lets through only `publicPath()`; every other route requires a
  principal, and `adminOnlyRoute` keeps refusing members with `403`.
- **FR4** `ImplicitUser` disappears from every HTTP path. It survives only, if at all, for
  the entry points that run without an HTTP request, and those are named explicitly.
- **FR5** Personal settings are stored per account: `theme`, `accentColor`, `language`,
  `density`, `defaultView`, `uiScale`, `detailMode`, `userName`, `userEmail`, `userAvatar`,
  `editorCommand`, `externalTerminalCommand`.
- **FR6** Deployment settings stay in the single existing row: trackers, `repoPath`,
  auto-sync, AI configuration, prompts and `specFramework`.
- **FR7** A new account's personal settings are seeded from the deployment row's current
  values.
- **FR8** `userEmail` is read from the account and is not writable through the settings API.
- **FR9** The web interface renders the sign-in screen and mounts nothing else while
  `/api/me` reports an unsigned caller.
- **FR10** The sign-in form takes a required e-mail and an optional sealing passphrase, and
  never a login password.
- **FR11** A supplied passphrase unlocks every sealed tracker credential of the account
  through the existing `POST /api/me/tracker-credentials/unlock`, which takes one tracker
  at a time: either loop client-side or add an "all trackers" mode.
- **FR12** The session and cookie contract of ADR 0013 is unchanged, concurrent sessions
  included.
- **FR13** The sign-out button of `SignInStatus.tsx` is kept as it is.

## Out of scope

- The shared board: ADR 0013 stands, everyone sees every project, task and running
  execution; only execution ownership matters.
- The desktop application: unchanged, and existing pairings keep working.
- Passwords, magic links and any new OIDC wiring.

## Acceptance criteria

- On a deployment with no account, opening the interface shows the sign-in screen and no
  board.
- An anonymous call to an interface API answers `401`, except on the public paths.
- Signing in with an e-mail opens a session carried by the `HttpOnly` cookie; signing out
  revokes it server-side.
- Two accounts signed in on the same server each see their own theme, language, density,
  view, scale, detail mode, identity and workstation commands.
- The tracker, auto-sync, AI and prompt settings stay shared and are refused to a member.
- A sealing passphrase given at sign-in unlocks that person's sealed tracker tokens; a
  wrong one signs them in with the tokens still locked and says so.
- An existing deployment keeps its `default` account, its executions keep their owner, and
  its paired workstations keep working without re-pairing.

## Open questions

None blocking. The clarification settled the two that mattered: a wrong passphrase does not
refuse the sign-in, and `default` is neither adopted nor merged. One point is left to the
implementer as a presentation detail, not a behaviour: whether the "all trackers" unlock is
a client-side loop or a server mode (FR11). Either satisfies US7.
