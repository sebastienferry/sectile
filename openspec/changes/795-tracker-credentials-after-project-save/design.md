# Design: Site-Aware Tracker Credential Prompt After Project Save (#795)

## Context

- `GET /api/trackers` returns `TrackerSummary[]` where each entry carries `provider`, `site` (empty when using the deployment's site), `scope`, and `identity` (`provider|address|scope`).
- `GET /api/me/tracker-credentials` returns `StoredUserCredential[]` with `tracker`, `siteUrl` (for API tokens), `kind` (`'api_token' | 'oauth'`), `disconnected`, and `grantedSites` (for Jira OAuth grants).
- On the server (`ForActingUser` in `internal/trackerapi/client.go` and `ResolvePersonalCredential` in `internal/db/jiragrants.go`), a Jira call resolves against the tracker's `site` (falling back to the deployment's `JiraURL`).

## Decisions

1. **Carry `site` on `TrackerOption` (`web/src/lib/projectTrackers.ts`)**:
   - `trackerOptions(summaries, selected)` copies `site: summary.site || ''` for every summary, and derives `site` from the middle segment of `ref.identity` (`provider|site|scope`) for fallback options not in `summaries`.

2. **Normalize Jira Sites & Check Site Coverage (`web/src/lib/trackers.ts`)**:
   - `jiraSite(raw)` normalizes a Jira site URL to `<http|https>://<lowercased-host>`, matching `trackerapi.JiraSite` on the server.
   - `trackerSiteFor(tracker, settings)` returns the normalized site for a tracker (`jiraSite(tracker.site) || jiraSite(identitySite) || jiraSite(settings.jiraUrl)` for Jira; the tracker's `site` or `storedFor(settings, provider).siteUrl` for GitHub/GitLab).
   - `needsCredentialsFor(tracker, settings, mine)` accepts either a provider string or `{ provider, site?, identity? }`:
     - For `jira`, returns `false` only when `mine` has a Jira credential covering the resolved site:
       - `kind === 'oauth'`: `!c.disconnected` and (`!site ? (c.grantedSites?.length ?? 0) > 0 : (c.grantedSites ?? []).some(s => jiraSite(s) === site)`).
       - `kind !== 'oauth'`: `!site || jiraSite(c.siteUrl) === site`.
     - For `github` and `gitlab`, returns `false` when `mine` has a credential for that provider whose `siteUrl` is empty or matches the tracker's site, or when the tracker uses the deployment's site and `storedFor(settings, provider)` has `tokenIsSet` or `tokenFromEnv`.
   - `trackerSetupTargetFor(trackers, settings, mine)` finds the first non-local tracker in `trackers` where `needsCredentialsFor(tracker, settings, mine)` is `true` and returns `{ provider: initialTracker(tracker.provider), site: trackerSiteFor(tracker, settings) }`, or `null` when none needs credentials.

3. **Open `TrackerSetup` on the Chosen Tracker (`TrackerSetup.tsx`, `TrackerCredentialForm.tsx`, `ProjectModal.tsx`, `AppContext.tsx`)**:
   - `AppContext` exposes `trackerSetupTarget: TrackerSetupTarget | null` and `openTrackerSetup(target)`. Closing `TrackerSetup` clears `trackerSetupTarget`.
   - When `TrackerSetup` has a `target` (from prop or `trackerSetupTarget` in context), it locks `tracker` to `target.provider`, omits the provider picker, and passes `initialSiteUrl={target.site}` to `TrackerCredentialForm`.
   - `TrackerCredentialForm` initializes `siteUrl` from `initialSiteUrl?.trim() || mine?.siteUrl || serverStored.siteUrl`.
   - `trackerCredentials.setup.description` in `web/src/locales/translations.ts` is updated in French and English to state that credentials are needed to act on the tracker under the user's own account.
