# Proposal: Ask for Tracker Credentials After a Project Save Only When the Chosen Tracker Needs Them, on That Tracker (#795)

## Why

After saving a project in `ProjectModal`, the web client evaluates `selectedChoices.some(option => needsCredentialsFor(option.provider, settings, userCredentials))` and opens `TrackerSetup` without passing the tracker that triggered the prompt. This has three flaws since `#741` and `#654`:

1. `needsCredentialsFor` only inspects the provider name (`c.tracker === kind`). For Jira, it ignores the tracker's site (or the deployment's `settings.jiraUrl` when the tracker's `site` is empty), so it stays silent when a user's stored Jira API token (`siteUrl`) or OAuth grant (`grantedSites`) covers another site than the tracker's, or when their OAuth grant is `disconnected`.
2. `TrackerSetup` always preselects `initialTracker(settings.issueTracker)` and shows the provider picker over all providers rather than opening directly on the chosen tracker's provider and site.
3. `trackerCredentials.setup.description` still states that without these credentials "synchronization fetches nothing", whereas since `#741` synchronization runs with the server credential and personal credentials are used to act on the tracker under the user's own account.

## What Changes

- Carry `site` on `TrackerOption` in `web/src/lib/projectTrackers.ts`.
- Add `jiraSite`, `trackerSiteFor`, and `trackerSetupTargetFor`, and update `needsCredentialsFor` in `web/src/lib/trackers.ts` to check whether a user's credential covers the tracker's site (`siteUrl` for API tokens, `grantedSites` and `!disconnected` for Jira OAuth grants).
- Add `trackerSetupTarget` and `openTrackerSetup` in `web/src/context/AppContext.tsx`, and open `TrackerSetup` from `ProjectModal.tsx` directly on the first selected tracker needing credentials, with its site prefilled in `TrackerCredentialForm.tsx` and no provider picker when targeting a chosen tracker.
- Update `trackerCredentials.setup.description` in `web/src/locales/translations.ts` (French and English) to explain that credentials are needed to act on the tracker under the user's own account.

## Impact

- Affected specs: `tracker-credential-setup`
- Affected code:
  - `web/src/lib/projectTrackers.ts`
  - `web/src/lib/trackers.ts`
  - `web/src/components/TrackerCredentialForm.tsx`
  - `web/src/components/TrackerSetup.tsx`
  - `web/src/components/ProjectModal.tsx`
  - `web/src/context/AppContext.tsx`
  - `web/src/locales/translations.ts`
  - `web/tests/trackerSetup.test.mjs`
  - `web/tests/projectTrackers.test.mjs`
