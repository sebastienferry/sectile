# ADR 0049: Workstations sign in through the browser

- Status: Accepted
- Date: 2026-10-03
- Issue: [#717](https://github.com/sebastienferry/sectile/issues/717)

## Context

A workstation got its key only from a pairing code: the person opened the web
profile, generated a code and copied it by hand into Sectile Desktop or
`sectile-agent pair` (ADR 0011). Everything else in the day already went
through the browser, where a web session lasted 12 hours, so the same person
signed in again every morning.

Desktop also showed its pairing screen again after a reboot, for three
reasons: a key it failed to read was swallowed and turned into a request for a
new code; a key Desktop had encrypted with Electron safeStorage outranked the
clear `apiKey` that `sectile-agent pair` writes in the same settings file; and
writing that `apiKey` left the older encrypted `secret` in place. Go cannot
decrypt safeStorage, so a standalone agent never saw the key Desktop held.

Re-pairing created a new key without revoking the old one, and the `sectile`
entry of `~/.claude.json` kept the key it had been written with, so the MCP
client went on sending a key that was later revoked. Finally, a database
error while checking a key was answered as an invalid key, which sent people
hunting for a typo while the server was the one failing.

## Decision

- **The browser hands a pairing code to the workstation; no key travels in a
  URL.** Desktop and `sectile-agent pair` listen on `127.0.0.1` on a free port,
  pick a random `state` and open `GET /auth/workstation?port=&state=` in the
  browser. The route accepts a port from 1024 to 65535 and a state matching
  `^[A-Za-z0-9_-]{16,128}$`, and answers anything else
  `400 Invalid workstation sign-in request`. Without a session cookie it
  redirects to `/auth/login?redirect=` back to itself, which works with the
  identity provider (ADR 0008) and with the local e-mail sign-in alike. With
  one, it answers `403` for a blocked account, otherwise creates a single-use,
  ten-minute pairing code and redirects to
  `http://127.0.0.1:<port>/callback?code=&state=`. The host is fixed and the
  redirects are rebuilt from the two validated inputs only. Only the session
  cookie counts there: a bearer key never mints a code. Both answers carry
  `Cache-Control: no-store` and `Referrer-Policy: no-referrer`. The workstation
  checks the state and redeems the code on the existing
  `POST /api/v1/agent/pair`, so the key only ever travels in a response body.
  No identity-provider SDK, JWT or device-code flow is added, and the identity
  provider needs no change.
- **A new pairing replaces the workstation's previous key.** The pair body
  takes an optional `deviceId`, the device the workstation was paired as on
  that same server; the server revokes that credential in the transaction that
  creates the new one, provided it belongs to the same user and is still live.
  An unknown or foreign id is ignored and the pairing still succeeds.
- **Web sessions slide.** A session lasts at most 90 days from sign-in and ends
  after 7 days without use, counted from its last use and checked before the
  use is recorded, so an idle session is never revived. The cookie's `Max-Age`
  is the absolute lifetime; the idle rule is enforced on the server only. The
  30-minute idle window that keeps sealed tracker credentials unlocked
  (ADR 0032) is unchanged.
- **Desktop stores the key in clear in the owner-only settings file.** Desktop
  writes `apiKey` in `~/.config/sectile/settings.json` (mode 0600), as
  `sectile-agent pair` already did, so the agent started by hand reads the key
  Desktop stored. The safeStorage `secret` of earlier versions is read once,
  when no `apiKey` is there, and replaced by `apiKey` at the next successful
  start; writing `apiKey` from Go deletes any `secret`. This is option E1 of
  the plan, chosen following its recommendation; the owner may revisit it. At
  launch with a stored key, Desktop starts the agent once without asking, and
  when it cannot, its setup screen says whether no key is stored or the stored
  one cannot be read.
- **MCP registrations follow the key.** When the key changes, the user-level
  top-level `sectile` entry of Claude Code, Codex and Antigravity that
  addresses the same server with a different non-empty key is rewritten with
  the new key, keeping its transport. `sectile-agent pair` does it right after
  storing the key, and the agent at start for the registrations that have no
  saved Desktop choice. No entry is ever created, and a local entry, which
  carries no key, is never touched.
- **A failed check is not an invalid key.** On `/api/v1/agent/*`, `/mcp` and
  the `/ws/agent-connect` handshake, a credential check that fails for any
  reason other than an unknown, expired or blocked key answers
  `503 Authentication temporarily unavailable`, which clients retry. The `401`
  texts are unchanged, and the WebSocket keeps its `403 Invalid agent token`
  for an unknown key.

## Consequences

- Signing in a workstation is one click when the browser already holds a
  session, and an identity-provider sign-in otherwise. The pairing code stays
  the fallback, through `--code` or Desktop's form.
- A web page opened in a signed-in browser can navigate it to
  `/auth/workstation` and make the server mint a ten-minute pairing code sent
  to a loopback port. Only a process already listening on that port on the
  same machine can read it, and such a process runs as someone who can already
  act on that machine. There is no consent screen; adding one would mean web
  interface work the issue avoids.
- A running agent keeps the key it started with. After `sectile-agent pair`
  replaces the key, the old one is revoked, so the agent must be restarted; the
  command says so.
- `--no-browser` prints the URL instead of opening it, but the callback still
  targets this machine's loopback, so the browser must run on the same
  machine. A headless or remote host pairs with `--code`.
- If an agent started by hand with the old key is already running when Desktop
  signs in, Desktop connects to it and it fails at its next reconnect. This is
  rare: Desktop shows its setup screen only when no agent answers.
- The key sits in clear in a file only its owner can read, as the CLI
  configurations of ADR 0011 already hold it.
- An idle session leaves a cookie that resolves to nobody, so the interface
  shows its sign-in screen.
- Project-scoped `sectile` overrides in `~/.claude.json`
  (`projects[<path>].mcpServers`) are not refreshed.
- Rejected: putting the key in the redirect URL (it would land in browser
  history and logs); an identity-provider SDK or the device-code flow (it
  contradicts ADR 0008 and needs a provider change); replacing the key of a
  workstation matched by label (labels collide and change); re-issuing the
  cookie on every use (the session is read several times per request, where
  no response writer is at hand); keeping the safeStorage `secret` (option E2:
  an agent started by hand would work only after its own pairing).
