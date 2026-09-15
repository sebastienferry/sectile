# Direct link to the board from the desktop header

## Why
The connected server address is plain text, so users must manually open the board in a browser.

## What Changes
- Make the connected address a visible, keyboard-accessible link to the board.
- Open the current server in the default browser while preserving the desktop window.
- Keep disconnected and stopped status messages as plain text.

## Scope
Only the desktop header and its browser-opening bridge change. Connection setup, task deep links, and the web application are out of scope. No unresolved product decisions or new backend dependencies exist.

## Impact
Desktop renderer, preload bridge, main-process URL handling, UI regression coverage, and desktop documentation.
