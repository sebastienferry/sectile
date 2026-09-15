# ADR 0008: Web sign-in through OpenID Connect

Status: Accepted

## Context

[ADR 0007](0007-user-identity-and-agent-binding.md) made every workstation
resolve to a user, but left the question of who the person driving the web
interface is. That was answered by a single implicit user, with a development
switch allowing a caller to name itself through a header.

The deployment target uses Okta. Binding the server to one vendor would make a
later change expensive, and the organization may not be the only one running
this server.

## Decision

Sign people in through OpenID Connect, configured by environment: issuer,
client id, client secret and redirect URL. The provider's endpoints are read
once at startup from its metadata document, and the declared issuer must match
the configured one. Okta is then a configuration value, not a dependency.

The server is a confidential client using the authorization code flow with
PKCE. It reads the subject from the UserInfo endpoint with the access token it
has just obtained, rather than verifying the ID token signature itself. OpenID
Connect Core permits this when the token comes directly from the token endpoint
over TLS (section 3.1.3.7), and it avoids carrying a JWT verification stack
whose failure modes are subtle and silent.

Sessions are opaque and server side. The cookie is a random value; only its
hash is stored, alongside the user and an expiry. Signing out revokes the row,
so a session ends on the server and not merely in the browser. The cookie is
HttpOnly, SameSite=Lax so it survives the provider's redirect back, and Secure
whenever the request arrived over TLS, directly or through a proxy.

A configured provider is the only authority: the development switch from
ADR 0007 is then ignored rather than left as a way around sign-in. Without a
provider the interface keeps its single implicit user, which is how a personal
deployment runs, and a provider that cannot be reached at startup stops the
server rather than silently serving the interface to everyone.

## Consequences

The pairing flow of ADR 0007 now issues codes for the person actually signed
in, which is what makes several users on one server real rather than nominal.

Interface APIs require a session once a provider is configured. Agent APIs do
not: they carry a device credential and must keep working while nobody has a
browser open. That list of exemptions is the authentication boundary, so it is
covered by a test that names both what bypasses the guard and what must not.

Identity is authentication, not authorization. Everyone who signs in sees the
same board, as before. Roles would be a separate decision.

The sign-in flow is verifiable only against a real provider. The parts that can
be tested without one, single-use state, session lifetime, redirect safety and
cookie flags, are; the exchange itself is not.
