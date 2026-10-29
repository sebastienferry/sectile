# Desktop OAuth companion contract

The existing OAuth app configuration and encrypted grant implementations remain
authoritative. A companion does not pass its device bearer credential in a
browser URL or copy a normal web-session cookie into the browser.

## Authenticated native API

- `POST /api/desktop-oauth` with `{provider}` (`github`, `gitlab`, `jira`)
  creates an attempt for the authenticated caller. Returns `{id, startPath,
  expiresAt}`. The caller opens `startPath` on this same server in the system
  browser. The path contains a one-use capability; do not log it.
- `GET /api/desktop-oauth?id=...` returns `{id, provider, status}` for the owner.
  Unknown, expired and other users' attempts all return `{status: "expired"}`.
- `DELETE /api/desktop-oauth?id=...` cancels a waiting attempt. An exchange already
  completing returns 409; a terminal outcome is retained rather than relabelled.

A provider must have an OAuth app configured first. Attempts expire after ten
minutes, with at most one waiting/completing attempt per user and provider and
256 attempts per handler. Pending handoffs are in memory and lost on restart.
The DB independently enforces OAuth state expiry and single-use consumption.

## Browser handoff

`GET /auth/desktop-oauth/start?ticket=...` consumes the ticket and creates a
short-lived browser cookie named `sectile_desktop_<provider>`, HttpOnly and
SameSite=Lax, scoped to `/auth/`. It does not replace the website session cookie.
The existing callback for that provider recognizes the matching state and cookie,
then uses the ordinary DB completion methods. Wrong state does not consume the
desktop flow, and another web consent can coexist with a desktop cookie.

Completion revokes the temporary session, clears the cookie and renders a fixed
result page. Native polling reports success only on `connected`. Other terminal
outcomes include `cancelled`, `invalid`, `unreachable` and provider-specific
failures. No status response or page includes an OAuth code, grant or app secret.

The local desktop uses per-installation confidential OAuth app configuration;
this is not a public native OAuth client implementation or a license to embed a
shared client secret in a distributed binary. GitHub/GitLab self-managed OAuth
endpoints are not supported by the current forge provider implementation.

## Go-owned local callback contract

The isolated companion binds `127.0.0.1:52963`. Its Atlassian console callback is
`http://127.0.0.1:52963/auth/jira/callback`. The server rejects an incompatible
`PORT`, non-loopback binding or missing private data directory before opening
the database. An occupied port is an error; there is no fallback port.

`sectile-server --local-runtime-info` returns public `{protocol, port, origin,
oauthCallbacks}` metadata without loading environment files or opening the DB.
The installation-key-protected `/api/local-runtime` probe returns the same fields
plus `ready`. Native onboarding uses this contract rather than allocating a port.
An older installation manifest on another port is explicitly refused rather than
silently migrating live agent connections; migrate that installation with its
services stopped. The current installation already uses 52963.

In companion mode only, OAuth configuration GET includes `expectedRedirectUrl`.
Go supplies that callback on save and on both authorization and token exchange.
A supplied conflicting callback is rejected. Existing application credentials
remain encrypted and are reused. Tauri sends only client ID and secret, displays
Go's URL, opens the one-use browser handoff and polls completion. Ordinary web
and Electron deployments continue using their saved/environment callbacks.

Jira reports `connected` only after fetching accessible resources, finding a
configured site and successfully calling the site's Jira account API with the
new access token. A valid consent without a configured matching site returns
`no_site`; add the site's tracker before connecting. No tracker data is mutated
by this check.

Site matching includes standalone `trackers.site` records as well as legacy project fields and deployment settings. A tracker need not be linked to a project to permit its account connection.
