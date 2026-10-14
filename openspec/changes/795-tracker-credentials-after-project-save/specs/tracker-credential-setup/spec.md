## ADDED Requirements

### Requirement: Post-Save Tracker Credential Check Matches the Chosen Tracker's Site
After a project is created or updated, the web client SHALL ask for tracker credentials only when a selected remote tracker is not covered by the user's credentials for that tracker's site, and SHALL open the credential setup dialog directly on that tracker's provider and site without a provider picker.

#### Scenario: User's Jira credential or OAuth grant covers the chosen tracker's site
- **WHEN** a user saves a project selecting a Jira tracker whose site (or deployment fallback site) is covered by the user's stored Jira API token `siteUrl` or connected OAuth grant `grantedSites`
- **THEN** `needsCredentialsFor` returns `false` and `trackerSetupTargetFor` returns `null`, so the tracker setup dialog does not open

#### Scenario: User's Jira credential covers another site or OAuth grant is disconnected
- **WHEN** a user saves a project selecting a Jira tracker whose site is not covered by the user's stored Jira API token `siteUrl` or OAuth grant `grantedSites`, or the user's Jira OAuth grant is `disconnected`
- **THEN** `needsCredentialsFor` returns `true` and `trackerSetupTargetFor` returns `{ provider: 'jira', site }` for that tracker, opening `TrackerSetup` directly on `jira` with `site` prefilled and no provider picker

#### Scenario: Setup dialog explains personal write attribution
- **WHEN** `TrackerSetup` is displayed
- **THEN** its description states that the credentials are needed to act on the tracker under the user's own account and are verified with the instance before being saved
