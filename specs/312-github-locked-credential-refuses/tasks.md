# #312 — Implementation checklist

## 1. Adapter (`internal/trackerapi/adapters.go`)

- [ ] **T1** Change `GithubAdapter.forProject` to return `(*Client, error)` and return the `ForActingUser` error
  instead of `g.client.For(projectID)`. Rewrite its doc comment: a locked credential refuses; no personal token keeps
  the project or server token, on purpose.
- [ ] **T2** Propagate the error in every caller, before any request is built.

## 2. Tests (`internal/trackerapi/adapters_test.go`)

- [ ] **T3** Locked credential with an actor: every GitHub adapter operation returns the resolution error, and the
  test server receives no request.
- [ ] **T4** Actor without a personal token: the project token is used when stored, the server token otherwise.
- [ ] **T5** No actor: the project or server token is used (the existing
  `TestGithubWritesUseTheActingPersonsToken` keeps covering the personal and unattended cases).

## 3. Documentation

- [ ] **T6** Amend ADR 0018's consequences (plan D4).

## 4. Verification

- [ ] **T7** `go build ./...`, `go vet ./...`, `go test ./internal/trackerapi/... ./internal/db/...` green.
