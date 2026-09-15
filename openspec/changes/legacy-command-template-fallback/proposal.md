# Legacy command templates fall back to provider defaults

## Why
Server rows written before command templates carried `{prompt}` hold a bare CLI name (`agy`, `claude`) in `aiCommandTemplate`. Server execution always ignored such a template for a named provider and ran the provider default, but the shared configuration contract now rejects it, so `/api/v1/agent/config` answers 400 for every affected project: task dispatch fails and the desktop logs a handler error at startup.

## What Changes
Define one predicate for whether a launch uses the configured template: a template is used when it is nonempty and either the provider is `custom` or it contains `{prompt}`. The server serves the template only when it would be used, so a named provider with a legacy bare name receives an empty template and its built-in command. Server execution reuses the same predicate. `custom` templates and local desktop overrides keep the `{prompt}` requirement.

## Capabilities
### Modified Capabilities
- `local-command-templates`: legacy template fallback for named providers.

## Impact
Secret-free agent configuration, server command execution, the server–agent contract and desktop documentation, regression tests. No migration or schema change; stored values are untouched.
