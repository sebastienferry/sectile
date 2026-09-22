# ADR 0019: The machine surfaces have no open mode

Status: Accepted

Supersedes one decision of
[ADR 0011](0011-one-api-key-for-agent-and-mcp.md): *"the open mode of a server
without `SECTILE_SERVER_TOKEN` lasts until the first key is issued and ends
there"*.

## Context

`resolveAgentCredential` ended with a legacy open mode. When a bearer credential
named no device credential, `SECTILE_SERVER_TOKEN` was unset and
`HasDeviceCredentials()` reported no credential at all, it returned
`agentCredential{UserID: ImplicitUser}`: any syntactically valid token
authenticated as `default` on the agent WebSocket, on `/api/v1/agent/*` and on
`/mcp`.

ADR 0011 arrived at that shape deliberately. A personal deployment that had
never created a key had agents configured with an arbitrary string, and the
upgrade was not allowed to cut them off. Ending the mode at the first issued key
was what made revocation mean something, and counting revoked keys too was what
kept a revoked value from passing again as "any nonempty token".

Two things have changed since.

**Sign-in became mandatory.** [ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md)
removed the implicit browser mode and rejected keeping it behind a flag, as
"the same hole with a switch in front of it". The open mode is that same hole on
the machine surfaces, and `default` is now an ordinary account that may well
hold the admin role — so a deployment that has never issued a key hands admin to
whoever sets one header.

**The condition became re-armable.** `HasDeviceCredentials()` is a live count of
`device_credentials`. Revocation only marks a row, so the count never used to
fall; account deletion removes rows outright. Deleting the last account that
holds a workstation key therefore empties the table and silently puts the
deployment back into the open mode it had left behind, years of keys later. A
security boundary that a routine roster edit can reopen is not a boundary.

## Decision

**The open mode is removed, not narrowed.** `resolveAgentCredential` resolves a
device credential or a configured `SECTILE_SERVER_TOKEN`, and nothing else. A
credential that matches neither names nobody, on a fresh deployment as on an
established one.

**No count decides anything.** `HasDeviceCredentials` is deleted rather than
replaced by a stored "this deployment has issued a key" marker. A marker would
be correct about the re-arming and still wrong about the fresh deployment, which
is exactly the case ADR 0015 closed on the browser side.

**`SECTILE_SERVER_TOKEN` stays, unchanged, as the upgrade path.** It is still
accepted for one release with the same startup warning and still resolves to
`ImplicitUser`. A deployment whose agents ran on an arbitrary string sets that
string as `SECTILE_SERVER_TOKEN` and keeps running while its owner pairs a
workstation. It is one pinned value an operator chose, not a door that takes
anything.

**Pairing is untouched, so nothing is locked out.**
`POST /api/v1/agent/pair` is the only agent endpoint that is not itself
authenticated — the single-use code is the proof — so a deployment with no key
at all still reaches one: sign in, create a key or spend a pairing code.

## Consequences

An agent started with an invented token against a keyless server now fails its
handshake with `401` instead of registering as `default`. That is the intended
break, and it is the last behaviour ADR 0011 deferred.

Tests that reached a machine surface with a made-up bearer now issue a real key
first. Several did, which is the same reading of the hole from the other side:
the open mode was load-bearing in the suite without any test naming it.
`internal/handlers/mcp_sessions_test.go` was pinning `TASKFLOW_SERVER_TOKEN`, a
name the server stopped reading long ago, and passed only because of the open
mode.

`webSessionUser` is unaffected: it already required `credential.Device != nil`,
so neither the open mode nor the shared token ever stood in for a session on the
interface API.

## Alternatives rejected

- **A stored marker recording that a key was once issued.** It fixes the
  re-arming and keeps the hole on a deployment that has issued no key, which is
  the case ADR 0015 exists to close. It also adds a row whose only purpose is to
  keep a removed mode reachable.
- **Keying the mode on "no account exists yet" instead of "no key exists yet".**
  The same objection, with a condition an admin can restore by deleting the last
  account.
- **Removing `SECTILE_SERVER_TOKEN` in the same change.** Its retirement is a
  separate, already-announced step; taking both at once would leave a deployment
  on arbitrary tokens with no bridge at all.
