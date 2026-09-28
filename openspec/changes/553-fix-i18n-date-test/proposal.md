# Fix the i18n date test in hash-named worktrees

## Why

The date and time-zone test spawns a Node ESM script that imports the i18n module using a filesystem path. In a task worktree such as `#553`, Node treats `#` as a URL fragment, so the script fails before testing the formatters.

## What Changes

- Embed an encoded file URL for the i18n module in the spawned script.
- Keep the date-only and instant assertions for both time zones.
- Verify the targeted test and web suite from the assigned hash-named worktree.

## Scope

This changes test code only. Application formatting, worktree naming, the browser harness, and infrastructure are outside scope. No new dependency or migration is needed.
