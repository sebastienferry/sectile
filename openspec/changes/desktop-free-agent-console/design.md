# Design

Add an authenticated POST /desktop/consoles endpoint accepting projectId and provider. Resolve the existing local project mapping and register a taskless controlledRun with kind=console and provider metadata. Use an allowlist of executable names rather than editing a shell command template. Queue the run as a shared-checkout execution; do not create or change a branch or scaffold workflow artifacts.

The daemon launches the existing agent-exec supervisor inside the existing PTY host. Taskless completion does not call the server. Renderer grouping uses the run ID for free consoles, skips task metadata and workflow controls, and reuses terminal transport, archival, and relaunch UI.
