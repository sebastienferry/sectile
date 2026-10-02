# Implementation plan: Auth0 browser sign-in

## Existing architecture

Reuse Go `internal/auth/oidc.go` and `golang.org/x/oauth2`, the login/callback/logout handlers in `internal/handlers/auth.go`, their routes in `cmd/server/main.go`, and existing database session and role handling. No migration, new dependency or endpoint is planned.

The current test file `internal/auth/oidc_test.go` covers configuration rejection, PKCE and authorization URL construction. It does not exercise a complete discovery/token/UserInfo exchange. Add meaningful coverage before deciding whether production changes are necessary.

## Configuration contract

Use a confidential server-side Auth0 application with authorization code flow and the exact deployment callback ending in `/auth/callback`. Configure `SECTILE_OIDC_ISSUER`, `SECTILE_OIDC_CLIENT_ID`, `SECTILE_OIDC_CLIENT_SECRET` and `SECTILE_OIDC_REDIRECT_URL` through deployment configuration. Keep credentials out of version control. Continue discovering provider endpoints rather than adding endpoint override settings.

Preserve `openid profile email` scope behavior and existing optional role-claim settings. No new API audience, API resource registration or machine credential change is needed for the browser-only baseline. Do not copy an unrelated application's grant list.

Only synthetic domains such as `https://sectile.example.com` may appear in public examples. Application registration, actual callback values, credentials and rollout evidence belong in private deployment records.

## Implementation and validation approach

1. Add TLS-backed provider fixtures for metadata discovery, token exchange and UserInfo. Use a narrowly injected HTTP client or test-scoped transport if needed; never weaken production HTTPS validation or globally disable certificate verification.
2. Exercise the actual browser handlers with a test database and fixture provider. Verify redirects, authorization code/PKCE exchange, session creation, stable identity, replay rejection and logout invalidation. Exercise token and UserInfo failures as well as metadata/issuer failures. Keep all fixture credentials synthetic.
3. Retain existing role, local-sign-in and machine-authentication regression coverage. Fix only integration defects demonstrated by these checks; a configuration-only result is valid.
4. Expand the README sign-in section with a generic Auth0 setup and verification guide. Public documentation must distinguish local session logout from provider logout and flag that automatic migration of existing local accounts is outside scope.
5. Prepare application/deployment changes through the operator's private workflow. Confirm the deployed account can access the application and retains the intended role. Never report this step as passed without live evidence.

## Target files

- `internal/auth/oidc_test.go`: discovery and exchange fixtures/coverage.
- `internal/handlers/auth_test.go` or a dedicated integration test: complete sign-in and logout flow.
- `internal/auth/oidc.go`, `internal/handlers/auth.go`: only minimal verified fixes or test seams if necessary.
- `README.md`: generic operator instructions.
- `CHANGELOG.md`: an entry only if implementation changes user-visible behavior; documentation or tests alone do not require one.

## Checks and limits

Run `go test ./internal/auth ./internal/handlers ./internal/db` for the relevant integration/regression checks. Run formatting checks on modified Go files. No frontend build is required unless frontend code changes.

A live provider may expose configuration differences that fixtures do not reproduce; keep private rollout acceptance separate from repository validation. Existing local accounts are not automatically merged with provider accounts. The existing first-admin/role-claim policy remains authoritative and must be considered when provisioning access.

No architecture decision record is needed for reusing the established design. A change to identity, role authority or session architecture exceeds this plan and requires revisiting scope.

## Implementation outcome

Discovery and exchange coverage is colocated with the handler integration fixture in `internal/handlers/auth_oidc_integration_test.go` to exercise the real client without duplicating a provider fixture or introducing a production test seam. The serial tests temporarily install the TLS fixture's trusted transport and restore it on cleanup. Expired-state coverage is at the database boundary. All tested flows work with the current production code; only tests and setup documentation change.
