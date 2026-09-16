# Lowercase default label when adding a task

## Why
GitHub #48 reports that newly added tasks show `#New` instead of `#new`. Consistent creation defaults avoid a visible casing mismatch with the existing workflow labels.

## What Changes
- Default the add form to lowercase `new`, displayed as `#new`.
- Return and persist lowercase `new` for newly created tasks, including clones.
- Preserve existing custom-label, status, and hash-prefix conventions.

## Capabilities
### New Capabilities
- `task-creation-label`: consistent lowercase default workflow labels across the add form, creation, and cloning.

### Modified Capabilities
None.

## Impact
The Go task creation and cloning paths and React quick-add form need small default-value corrections. No schema change, new dependency, or migration is needed.

## Out of Scope
Historical label migration, global label normalization, other workflow stages, provider taxonomy changes, and changes to status selection or clone options.

## Decision Source
`docs/clarifications/48.md`, current standalone verification dated 2026-09-12. Live Sectile identifies the intended task as clarified with description “Currently the default label is #New, it should be #new (lowercap)”. Creation-only scope, inclusion of cloning, and inclusion of the form are the clarification's explicit reversible assumptions. No open requirements remain.
