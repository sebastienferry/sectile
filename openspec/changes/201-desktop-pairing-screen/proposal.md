# The desktop connect screen accepts a pairing code

## Why
The web interface tells the user to paste a pairing code into Sectile Desktop. For a long time the
desktop connect form offered no such field: it asked for a server address and a `Server token`, and
a code pasted there left as `Authorization: Bearer <code>` towards `/api/v1/agent/projects`, which
the server answers `401 {"error":"Valid agent bearer token required"}`. The client half of the
exchange existed (`desktop/electron/pairing.cjs`, the `pair` IPC handler, the `window.localAgent.pair`
bridge) but no file under `desktop/src` ever called it, so the instruction printed by the web
interface was impossible to follow.

That defect is **already fixed on `main`**. `94b992a` (PR #204) added the field, then `42c4684` and
`1ec6b42` (PR #208) unified the credential and put pairing first in the journey. This change writes
down the behaviour those commits delivered, so the contract is stated once and a later refactor of
the connect screen cannot silently drop it again. It carries **no production code change**.

## What Changes
- The connect screen's **pairing code is the primary way in**; an API key stays reachable behind an
  `Advanced` disclosure, for a user who already holds one.
- The credential arbitration is stated: **the code wins when both fields are filled**, because
  typing a code is a deliberate re-pairing.
- The **order of operations** is stated: the server URL is validated *before* the code is spent, so
  a single-use code is never burned on a malformed address.
- The **credential is always persisted**, encrypted with the OS keyring where one is available and
  in clear under the same `0600` permissions otherwise — a code that was spent and not saved would
  be lost, and the next launch would demand a new one.
- The **failure messages** the user can now actually reach are stated: unreachable server, expired
  or replayed code, neither field filled.

## Impact
Specification only. The behaviour lives in `desktop/src/main.js`, `desktop/electron/pairing.cjs` and
`desktop/electron/main.cjs`, and is covered by `desktop/tests/pairing.test.cjs` and
`desktop/tests/pairing.ui.cjs`. New capability `desktop-pairing` under `openspec/specs/`.

## Out of scope
- Listing the paired workstations (#198).
- Signing in with an account: the connect screen still offers no account path.
- Revoking or rotating a device credential from the desktop application.
- Generating the pairing code: that stays in the web interface.
