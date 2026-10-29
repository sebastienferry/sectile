# ADR 0004: Reuse Go OAuth with a one-use browser handoff

- Date: 2026-10-28
- Status: Accepted

## Decision

Keep provider authorization, code exchange, grant encryption and refresh in the
Go backend. Each local installation configures its own confidential OAuth apps.
No common client secret is shipped in the Tauri binary. This reuses the existing
confidential-client implementation rather than claiming to implement a public
native client with PKCE. A future public-client or brokered distribution is a
separate design decision.

The authenticated desktop creates a ten-minute attempt with
`POST /api/desktop-oauth`. The response contains an opaque attempt ID and a
single-use browser ticket. Native code validates its exact loopback origin and
path, then opens it with the system browser. The renderer never receives the
browser ticket, OAuth code, server token or provider grant.

`GET /auth/desktop-oauth/start` consumes the ticket, creates a temporary session
and a provider-specific HttpOnly cookie scoped to `/auth/`, then redirects to the
provider. The cookie does not overwrite the normal website sign-in cookie.
Existing callback handlers recognize the desktop attempt and retain DB checks
binding state, provider, user, session, expiry and single consumption. A callback
with an unrelated state cannot consume the desktop attempt.

The callback records the outcome, revokes the temporary session and displays a
fixed completion page. Native polling checks `GET /api/desktop-oauth?id=...`;
`DELETE` cancels a waiting attempt. Only its authenticated owner can inspect or
cancel it. A callback already exchanging a code cannot be cancelled halfway and
reported as cancelled while actually saving a grant. Completed grants survive
leaving Settings. Pending attempts are cancelled when the view closes.

The broker is bounded to 256 in-memory attempts and one pending attempt per user
and provider. Expiry cleanup revokes temporary sessions; server restart loses
pending handoffs. The persisted DB still handles OAuth state expiry and replay
protection. Tickets and secrets never appear in diagnostic messages.

## Validation

Handler tests use the existing GitHub, GitLab and Atlassian simulators for complete
consents, replay, unrelated state, cancellation, ownership and expiry. Native tests
reject arbitrary browser URLs. UI tests distinguish browser opening from confirmed
connection and cancel an initiation that resolves after unmount. A disposable
service test checks bearer-authenticated handoff routing through the actual server.
