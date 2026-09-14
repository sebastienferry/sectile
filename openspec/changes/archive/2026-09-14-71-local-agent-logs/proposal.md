# Desktop local-agent logs

## Why

Users diagnosing agent startup or connection failures are told to locate agent.log manually. The desktop already owns that diagnostic file and can make it readable without a running agent.

## What Changes

- Add an Agent logs action available in connected and offline desktop states.
- Display a read-only snapshot of the latest 256 KiB with the source path and explicit truncation, missing, empty, and error states.
- Provide Refresh without changing execution selection or lifecycle.

## Capabilities

### New Capabilities

- `desktop-agent-logs`: Inspect the existing desktop-owned agent diagnostic log locally.

### Modified Capabilities

None.

## Impact

Electron main/preload, desktop renderer, desktop tests and documentation. No server API, database or agent protocol changes. Web viewing, streaming, log rotation, CLI capture, export and deletion are excluded.
