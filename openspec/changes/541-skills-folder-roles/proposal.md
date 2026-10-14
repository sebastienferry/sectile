# Proposal: Skills No Longer Describe Multi-Repo Projects (#541)

## Why
#484 (ADR 0036) removed the mono-repo/multi-repo project setting and added folders attached to a project on each workstation. The implement skill fragment still opens its step 2 with "On a multi-repo project", which is no longer a thing, and calls every other repository "read-only context", which is wrong for `local` folders. The text is rendered into the implement, pickup and pickup-issues skills that agents follow.

## What Changes
- Reword step 2 of `internal/skills/fragments/implement/steps.md` around the folder map: when it is listed, and the rule for the `context`, `changed` and `local` roles.
- In `internal/skills/fragments/handoff/steps.md`, name the repositories to clean by their `changed` role, as `adjust` already does, and put step 7 back on its own line (it is currently glued to the end of step 6).
- Add a test that fails when any skill fragment mentions a mono-repo or multi-repo project.
- Regenerate the golden files under `internal/skills/testdata/`.

## Impact
- `internal/skills/fragments/implement/steps.md`
- `internal/skills/fragments/handoff/steps.md`
- `internal/skills/catalog_test.go`
- `internal/skills/testdata/golden/*`, `internal/skills/testdata/plugin/*`
