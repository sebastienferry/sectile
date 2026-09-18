# Tasks

## 1. Storage
- [ ] 1.1 Add `role` to `users` (default `member`) with the idempotent `ALTER TABLE`; update
      every explicit `users` column list; add `Role` to `db.User`, `ListUsers`, `SetUserRole`
      (refusing to demote the last admin) and `AdminCount`.
- [ ] 1.2 Add `user_id` to `task_activities` and `task_comments`; update the insert in
      `remoterun.go`, `insertTaskActivity` and the four reads; add `UserID` and `UserName` to
      `models.TaskActivity` and to comments, resolved from `users` at read time.
- [ ] 1.3 Test: a run inserted with an owner reads back with it on every read path; a legacy
      row reads back with an empty owner; the last admin cannot be demoted.

## 2. Principal and authorization
- [ ] 2.1 Introduce `principal{UserID, Role, Mode}` and `principalFor`; make `webSessionUser`
      and `resolveAgentCredential` return it (keep the string helpers as thin wrappers).
- [ ] 2.2 Add `requireSession`, `requireAdmin`, `requireOwnerOrAdmin` writing `401` / `403`
      with stable messages.
- [ ] 2.3 Guard the admin-only mutations of the design table; add the test that names each
      route with the member and admin outcomes.

## 3. Sign-in modes
- [ ] 3.1 Expose the mode in `internal/auth` (`oidc`, `local`, `implicit`); `implicit` only
      while no local account exists.
- [ ] 3.2 Add `POST /auth/local` (normalised e-mail, `local|<email>` subject, first-admin rule,
      web session); make `GET /auth/login` redirect to `/signin` without a provider; add
      `/auth/local` to `publicPath` and to its test.
- [ ] 3.3 Remove `SECTILE_DEV_IDENTITY` and the `X-Sectile-User` header; move
      `implicit_user_test.go` to the local mode; keep `dev|` users as ordinary rows.
- [ ] 3.4 Extend `/api/me` and `/api/v1/agent/identity` with `mode` and `role`.
- [ ] 3.5 Test: first local account is admin and closes the implicit mode; second is member;
      an anonymous call after that gets `401`; with a provider configured `/auth/local`
      answers `404`.

## 4. Roles from the provider
- [ ] 4.1 Read `SECTILE_OIDC_ROLE_CLAIM` and `SECTILE_OIDC_ADMIN_GROUP`; in `Exchange`, read the
      claim from UserInfo, then from the ID token payload; accept string or string array;
      return the resolved role on `Identity`.
- [ ] 4.2 In `HandleAuthCallback`, store the role when a claim is configured; otherwise apply
      the first-admin rule and leave the stored role alone.
- [ ] 4.3 Test: claim mapping (string, array, absent, unknown value), UserInfo over ID token,
      no claim configured leaves the stored role.

## 5. Execution ownership
- [ ] 5.1 Propagate the `/mcp` bearer principal into the request context; add
      `taskmcp.CallerFrom(ctx)`; record the owner in `start_run` and in the agent-owned run
      path from the registering agent's user.
- [ ] 5.2 Record the owner on `run-skill`, `tty-skill`, transitions, tracker operations and
      comments from the web session; make `AddComment` use the principal's name.
- [ ] 5.3 `handleCancelRemoteRun`: owner-or-admin, routed to the owner's agent; legacy
      ownerless runs admin-only via the caller's agent.
- [ ] 5.4 `HandleAgentDispatch`: honour a body `userId` only for admins.
- [ ] 5.5 Test: a member stopping another user's run gets `403` and the run stays running; an
      admin's stop reaches the owner's agent; a member's dispatch with a foreign `userId`
      goes to their own agent; `start_run` over `/mcp` records the key's user
      (`internal/mcptest` contract).

## 6. Interface
- [ ] 6.1 Install the same-origin `fetch` wrapper in `main.tsx` redirecting to
      `/signin?redirect=` on a `401` from `/api/`; add `lib/session.ts` with the safe-redirect
      helper and the mode and role labels, covered by `web/tests/session.test.mjs`.
- [ ] 6.2 Add `SignInScreen.tsx`: e-mail form in `local` mode with the trust notice, provider
      link in `oidc` mode; route `/signin`; render it when `/api/me` says not signed in.
- [ ] 6.3 Profile: show mode, identity, role and the projects with a registered agent; hide the
      legacy name and e-mail fields once a principal exists.
- [ ] 6.4 Admin **Users** section: list, role selector, last-admin refusal message, claim
      override note.
- [ ] 6.5 Show the owner on running executions; surface the server's `403` text on Stop.
- [ ] 6.6 Translations `fr` / `en` for every new string.

## 7. Documentation
- [ ] 7.1 ADR 0013: roles from the provider, temporary local sign-in and its limits,
      ownership rule; note the decided line of ADR 0008.
- [ ] 7.2 `docs/contracts/server-agent-v1.md`: `mode` and `role`, owner on runs, `403` cases.
- [ ] 7.3 `.env.sample`, `README.md` (Okta and Auth0 claim setup, local mode warning, removal
      of `SECTILE_DEV_IDENTITY`), `docs/CAPABILITIES.md`.
- [ ] 7.4 `.agents/MEMORY.md`: sixth `task_activities` column-list site, principal rule,
      local mode boundary.

## 8. Verification
- [ ] 8.1 `make test` (gofmt-clean tree), `npm test --prefix web`, `npm test --prefix desktop`.
- [ ] 8.2 Manual: fresh database, two e-mail accounts, cross-user stop and dispatch, settings
      as member; then `SECTILE_OIDC_ISSUER` against a tenant with the role claim.
