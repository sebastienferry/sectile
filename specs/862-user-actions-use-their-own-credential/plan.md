# Plan #862 - Every action a user performs uses their own tracker credential

Behaviour: `spec.md`. Base: `origin/main` at `e9e618bb` (the squash of #855).
The #855 branch is still on `origin/feat/855`; its routing commits are
`c52a8576` (tracker), `062c9459` (db routes), `554bb0c6` (reads), `32de3fd2`
(taskmcp), the update_task test of `3ec7b91e` and `a37912f5` (web stage move).

## Approach

Revert those routing changes on top of `main` (`git revert --no-commit` of the
listed commits, newest first), then restore what later #855 commits built on
them:

- `MissingServerCredentialError`, `ServerCredentialMissingCode`,
  `MissingServerCredentialTracker` (from `c52a8576`), used by the 503 answer of
  `ce095db7` and the macro batch code;
- `serverWriter` (from `062c9459`), kept for unattended writes only;
- the personal-check split of `427a9305` and the OAuth port of `de60da27`,
  which the revert must not undo.

## Changes

- `internal/tracker/tracker.go`: drop `WithServiceCredential`.
- `internal/trackerapi/client.go`: `ForWrite` without the service branch (an
  unattended write keeps `serverWriter`); drop `ForRead`; `ForActingUser`
  comment says it serves writes and reads.
- `internal/trackerapi/adapters.go`, `gitlab.go`: read resolvers back on
  `ForActingUser` (person first, server fallback, locked refuses).
- `internal/trackerapi/jira.go`: same, without the pre-#855 refusal of a read
  for a person who has no credential (FR-2).
- `internal/db`: drop `trackerOpRoutes`, the `ServiceCredential` fields and
  `Actor.trackerContext`'s marker, the service marks in macro, mirror, axis,
  label and sprint writes, the on-the-fly epic mark, `stageMoveOnly`; reads back
  on `trackerAs` / `trackerClientAs` / `ForActingUser` in `board.go`,
  `prstates.go`, `adjustment.go`.
- `internal/taskmcp/server.go`: drop `inSkillRun`, `writeContext`,
  `writeActor`; write tools go back to `WithActingUser` / `db.Actor{...}`.
- `internal/forgeoauth/oauth.go` + scope tests + `web/src/lib/oauthApp.ts`:
  GitHub scopes `repo read:project`.
- Tests: revert the #855 routing tests to their pre-#855 assertions; delete
  `credentialroutes_test.go` and `skillrun_test.go`; keep the OAuth, check and
  503 tests; add a test that a Jira read falls back to the server token for a
  person without a credential, and that a skill-run comment carries the
  person's token.
- Docs: ADR 0059; status lines on ADR 0058, 0029, 0044; README, user guide,
  API spec, `.env.sample`, CHANGELOG, translations (connection strings and the
  server-credential panel intro).

## Test strategy

`gofmt`, `go vet ./...`, `go test ./...`; web `tsc`, `oxlint`, `npm test`.
Replay the #855 end-to-end run against the fake GitHub: every user action must
log the user's account, reads the user's account, the sync the service account.
