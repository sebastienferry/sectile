# Desktop command template placeholders

## Why
Local CLI templates only expand `{prompt}`, so templates using server task and repository parameters fail on the desktop.

## What Changes
Support all eight existing tokens in local task launches, using current task metadata and actual local execution context. Preserve inserted values as shell data, template inheritance, provider defaults and raw terminal behavior. Show the supported tokens in desktop settings and documentation.

## Capabilities
### New Capabilities
- `local-command-templates`: task-aware local CLI template expansion.

## Impact
Agent dispatch, secret-free agent configuration, desktop settings guidance and regression tests. No migrations or new dependencies. Server command execution and skill prompt composition are out of scope.
