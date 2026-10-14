# Tasks: Ask for Tracker Credentials After a Project Save Only When the Chosen Tracker Needs Them, on That Tracker (#795)

- [x] 1. Carry `site` on `TrackerOption` in `web/src/lib/projectTrackers.ts` and cover it in `web/tests/projectTrackers.test.mjs`.
- [x] 2. Add `jiraSite`, `trackerSiteFor`, and `trackerSetupTargetFor`, and update `needsCredentialsFor` in `web/src/lib/trackers.ts` to check site coverage for API tokens and Jira OAuth grants.
- [x] 3. Update `web/src/context/AppContext.tsx`, `web/src/components/ProjectModal.tsx`, `web/src/components/TrackerSetup.tsx`, `web/src/components/TrackerCredentialForm.tsx`, and `web/src/locales/translations.ts` to open `TrackerSetup` directly on the chosen tracker's provider and site without a provider picker and with updated copy.
- [x] 4. Add unit tests in `web/tests/trackerSetup.test.mjs` and update `CHANGELOG.md`.
