# Tasks: Auth0 browser sign-in

- [ ] T1: Add a synthetic TLS provider fixture and test discovery, issuer validation, token exchange and UserInfo success/failure through the existing OIDC client.
- [ ] T2: Test browser login through callback and authenticated session, returning identity, invalid/expired/replayed state, provider denial and logout revocation against a test database.
- [ ] T3: Apply minimal fixes only where T1/T2 demonstrate an integration defect; preserve roles, local mode and workstation credentials.
- [ ] T4: Add generic Auth0 application registration, configuration and verification instructions to the README; document logout and account-migration limits. Add a changelog entry only for a user-visible behavior change.
- [ ] T5: Run `go test ./internal/auth ./internal/handlers ./internal/db` and formatting checks for modified Go files. Review public changes and commit history for internal references or secrets.
- [ ] T6: Through private deployment records, select the target deployment, register/configure the application, inject credentials and validate successful sign-in, access role and logout. Requires private environment inputs and actual live evidence.
- [ ] T7: Report repository checks and live rollout results separately. Create or reuse the implementation PR under the project policy, then record the verified workflow outcome through Sectile.

T1-T5 are repository implementation work. T6 is a deployment dependency and cannot be marked done from fixture tests. Public PRs and tracker reports must contain only generic results, never private infrastructure references.
