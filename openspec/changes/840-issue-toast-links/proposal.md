# Issue links in WebUI success notifications (#840)

## Why
After creating or editing an issue, users need to reopen the affected issue directly from its notification. Ordinary creation already supports this, but clone, update and some macro story paths omit links. Accepted baseline: docs/clarifications/840.md, round 3; no open product questions.

## What Changes
- Offer issue navigation on success notifications for ordinary creation, cloning, macro story creation and ordinary edits, including bulk callers.
- Open imported tasks in Sectile with an optional tracker icon; open local tasks internally; open tracker-only stories directly in their tracker.
- Include every created story in an all-success macro batch's navigation actions.
- Preserve warnings, errors, credential offers, clone auto-opening and existing toast presentation, dismissal and timing behavior.

## Capabilities
### New Capabilities
- `issue-toast-links`: navigation from issue creation and edit success notifications.

### Modified Capabilities
None.

## Impact
Web toast types, producer helpers, renderer and timer eligibility, focused tests and an implementation-time Unreleased changelog entry. Existing API responses already supply story URLs. No new dependencies, schema migrations or tracker writes. Move/export, macro editing, deletion and unrelated notifications are excluded.
