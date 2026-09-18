# Validation

## Automated
- `make test` (fails on a gofmt-dirty tree), `npm test --prefix web`, `npm test --prefix desktop`.
- `internal/db`: `users.role` migration leaves existing rows `member`; owner round-trips on
  every `task_activities` read path; legacy rows read an empty owner; last admin cannot be
  demoted.
- `internal/auth`: role claim mapping for string, array, absent and unknown values; UserInfo
  preferred over the ID token; no configured claim leaves the stored role.
- `internal/handlers`: the admin-only table test (member `403`, admin `200`); anonymous `401`
  distinct from `403`; first local account is admin and closes the implicit mode; `/auth/local`
  answers `404` with a provider; `publicPath` test extended with `/auth/local`; a member's stop on
  a foreign run is `403` and the run stays running; an admin's stop reaches the owner's agent; a
  member's dispatch ignores a foreign `userId`.
- `internal/mcptest`: `start_run` over `/mcp` records the key's user; a key bound to a member
  cannot perform an admin-only tool action.
- `web/tests/session.test.mjs`: safe redirect helper, mode and role labels.

## Manual
1. Fresh database, no provider: the board opens; create `alice@example.com` from the e-mail
   screen, she is admin; sign out; create `bob@example.com`, he is member; an anonymous tab is
   sent to the sign-in screen.
2. As Bob: settings and tracker credentials answer `403`; task edits and a skill launch on his
   own agent work.
3. Alice starts a run on her agent; Bob sees it with her name and gets `403` on Stop while it keeps
   running; Alice or an admin stops it and the owner's agent ends it.
4. Bob sends a dispatch naming Alice: it goes to his own agent.
5. Set `SECTILE_OIDC_ISSUER`, `SECTILE_OIDC_ROLE_CLAIM` and `SECTILE_OIDC_ADMIN_GROUP` against an
   Auth0 tenant with a post-login Action: the e-mail screen is gone; a user in the group is admin,
   another is member; a manual promotion is reverted at the next sign-in.
