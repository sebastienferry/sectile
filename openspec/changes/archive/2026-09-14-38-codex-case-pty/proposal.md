## Why
PTY skill actions always prepend `/`, so Codex receives the slash form instead of the plain skill name requested in #38. Codex is also missing from the interactive launcher allowlist.

## What Changes
- Recognize Codex in the interactive agent launcher.
- Format manual and automatic skill calls in an existing PTY according to the effective provider: plain name for Codex, existing slash syntax otherwise.
- Match terminal button text and tooltips to the effective provider and project skill override.
- Preserve task context, existing launch/injection guards, and stored configuration.

## Capabilities
### New Capabilities
- `pty-skill-invocation`: provider-aware skill actions in an interactive terminal.

## Impact
Runner formatting and launcher, database PTY entry points, terminal action labels, regression tests. Validation exposed a missing existing `settings.jira_api_token` column that prevents settings reads and thus PTY actions on fresh databases. Restore its additive migration as a prerequisite. Headless prompt generation and switching providers inside a live session are outside scope. Closes #38 when implemented and merged.
