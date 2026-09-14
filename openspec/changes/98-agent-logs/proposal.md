# Agent log workspace

## Why
The desktop diagnostics modal limits readable log space and exposes terminal control sequences as garbage.

## What Changes
- Show Agent logs in the available main workspace, retaining sidebar navigation and its width/collapse preference.
- Display readable plain text without ANSI/terminal control sequences, preserving Unicode, line breaks and literal text safety.
- Retain local, bounded snapshot reads, refresh, status messages and offline access.

## Impact
Desktop renderer, log presentation helper, desktop UI tests and usage documentation. No log-file mutation, backend protocol changes or terminal emulation.
