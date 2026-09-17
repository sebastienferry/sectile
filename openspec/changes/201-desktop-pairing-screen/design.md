# Design

## Context
This change records behaviour that shipped in `94b992a`, `42c4684` and `1ec6b42`. The decisions
below were taken in those commits; they are written here with the alternatives that were rejected,
so a later reader does not have to re-derive them from the diff.

## Decisions

### The code is spent after the URL is validated, not before
`start` (`desktop/electron/main.cjs`) validates the protocol and the absence of credentials in the
URL, then calls `resolveConnectCredential`, then `checkServer`. A pairing code is single use and
expires within ten minutes, so spending one on a URL the application was going to reject anyway
costs the user a round trip to the web interface.

**Rejected:** resolving the credential first, which reads more naturally but burns a code on a typo.

### A pure function arbitrates the credential
`resolveConnectCredential(settings, exchange, label)` in `desktop/electron/pairing.cjs` takes the
form values and returns `{token, deviceId?, paired}`. It follows the shape of `server-check.cjs`:
no Electron import, so it is unit-testable without a window.

**Rejected:** branching inside the `start` IPC handler, which would have made the arbitration
reachable only through a full Electron run.

### The code wins over the API key when both are filled
A user who types a code while an old key sits in the `Advanced` block is re-pairing on purpose. The
opposite rule would silently keep a stale key and leave them convinced they had re-paired.

### The credential is persisted with or without a keyring
`storeKey` writes `secret` (base64 of `safeStorage.encryptString`) when `safeStorage.isEncryptionAvailable()`,
and `apiKey` in clear otherwise, both under `0600` in a `0700` directory. This was the point the
ticket left open.

**Rejected:** refusing to save when no keyring exists. It is the safer-looking option and the worse
one: the code is already spent by then, so the credential would be lost and the next launch would
demand a fresh code, indefinitely, on any Linux desktop without a secret service.

**Accepted cost:** on such a host the key sits in clear in `~/.config/sectile/settings.json`. The
file permissions are the whole protection, and the comment in `main.cjs` says so.

### The API key stays reachable
Folding it away entirely would strand a user who holds a key from their web profile and no way to
reach a browser on that machine. A `<details>` disclosure keeps it one click away without making it
look like the normal path.
