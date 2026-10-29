# OAuth from the desktop

Fleet uses OAuth applications configured by its backend. New Local installations
start with personal provider API tokens; the desktop keeps the existing Local
OAuth application flow available under **Connexion OAuth locale existante** so
an already configured installation can reconnect and refresh its grants. The
registration procedure below applies when an operator explicitly configures a
Local OAuth application.

OAuth login is implemented for GitHub.com, GitLab.com and Jira Cloud. The system
browser handles login and consent; the selected Go backend exchanges codes,
encrypts grants and refreshes tokens through Sectile's existing provider
integrations. The desktop reports connected only after that backend confirms a
saved grant. Fleet administrators configure their backend's applications;
members use the connection buttons in Desktop when that backend supports them.

## One-time application registration

This self-hosted build has no centrally registered application credentials.
Register your own application once for each provider. This is separate from
copying personal access tokens: subsequent account connections use browser login.
Never distribute one shared client secret inside the desktop binary.

1. Create the local profile and open Settings → Connection accounts.
2. Copy the exact callback URL shown for the provider. The local installation
   retains its port across restarts. Register that URL with the provider.
3. Enter the application client ID and secret in its native configuration panel.
   The secret is encrypted in the Local database; leaving the field empty on a
   later save retains it. No API response returns the secret.
4. Click **Se connecter**, grant access in your browser, then return to Desktop.
   Leave Settings open during the flow. Leaving it cancels the pending attempt.

### GitHub.com

Register a **GitHub OAuth App** with the displayed `/auth/github/callback` URL.
The existing provider requests `repo` and `read:project` to support private issues,
workflow writes and ProjectsV2 status reads. GitHub documents loopback callbacks
and their port handling in its [authorization guide](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps).

### GitLab.com

Register a confidential application with the displayed `/auth/gitlab/callback`
URL and `api` scope. See [GitLab OAuth application registration](https://docs.gitlab.com/integration/oauth_provider/).
The current Sectile integration uses gitlab.com endpoints. A GitLab self-managed
instance still uses the advanced personal-token path; it must not be presented
as connected by a grant for gitlab.com.

### Jira Cloud

Register an Atlassian **OAuth 2.0 (3LO)** integration and the exact displayed
`/auth/jira/callback` URL. Enable the Jira APIs and these scopes required by the
existing backend:

- `read:jira-work`, `write:jira-work`, `read:jira-user`, `offline_access`
- `read:board-scope:jira-software`, `read:board-scope.admin:jira-software`, `write:board-scope:jira-software`
- `read:sprint:jira-software`, `write:sprint:jira-software`, `delete:sprint:jira-software`
- `read:project:jira`

Add the intended Jira site as a ticket source in Desktop before connecting.
The backend checks granted sites against configured sites, and a consent that
covers no configured site is not shown as successful. See [Atlassian 3LO setup](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/).

## Recovery and boundaries

Attempts expire after ten minutes. Cancellation, wrong state, callbacks from a
different browser session and replayed callbacks cannot replace stored grants.
A provider refusal or unreachable token endpoint is shown as a failure, not a
connection. The authenticated status endpoint exposes only the owner's attempt.

An installation restart discards pending browser handoffs; start a new connection.
Previously saved grants and application credentials remain encrypted in the local
DB. Real provider consent still needs the registered applications and user action;
automated tests use provider simulators and do not authenticate real accounts.

When developing against an older running local server, stop that instance only
after its local tasks have finished and reopen Desktop with the updated Go build.
The new `/api/desktop-oauth` and `/auth/desktop-oauth/start` routes are required.
