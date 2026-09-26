# Tasks: Auth0 browser sign-in

- [x] T1: Add a synthetic TLS provider fixture and test discovery, issuer validation, token exchange and UserInfo success/failure through the existing OIDC client.
- [x] T2: Test browser login through callback and authenticated session, returning identity, invalid/expired/replayed state, provider denial and logout revocation against a test database.
- [x] T3: Apply minimal fixes only where T1/T2 demonstrate an integration defect; preserve roles, local mode and workstation credentials.
- [x] T4: Add generic Auth0 application registration, configuration and verification instructions to the README; document logout and account-migration limits. Add a changelog entry only for a user-visible behavior change.
- [x] T5: Run `go test ./internal/auth ./internal/handlers ./internal/db` and formatting checks for modified Go files. Review public changes and commit history for internal references or secrets.
- [ ] T6: Through private deployment records, select the target deployment, register/configure the application, inject credentials and validate successful sign-in, access role and logout. Requires private environment inputs and actual live evidence.
- [x] T7: Report repository checks and live rollout results separately. Create or reuse the implementation PR under the project policy, then record the verified workflow outcome through Sectile.

T1-T5 are repository implementation work. T6 is a deployment dependency and cannot be marked done from fixture tests. Public PRs and tracker reports must contain only generic results, never private infrastructure references.

## Repository implementation results

The OIDC integration tests live together in `internal/handlers/auth_oidc_integration_test.go`, exercising the real discovery/client and browser handlers against a synthetic TLS provider. Expired state is covered at the database consumption boundary in `internal/db/sessions_test.go`. No production changes or new dependencies were needed. README setup guidance is complete; no changelog entry is required for tests and documentation alone.

Validation: `go test ./internal/auth ./internal/handlers ./internal/db`, `go vet ./internal/auth ./internal/handlers ./internal/db`, `go build -o /tmp/sectile-427-server ./cmd/server`, and `make fmt-check` passed. The targeted race check `go test -race ./internal/handlers -run TestOIDC -count=1` also passed. Live deployment (T6) remains pending and was not attempted without private deployment inputs.

Repository implementation is published as draft PR [#521](https://github.com/sebastienferry/sectile/pull/521), and the implemented stage was recorded through Sectile. T6 remains pending as private rollout work.
