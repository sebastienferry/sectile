# Reliable agent reconnection for launches

## Why

A Scaleway-hosted Sectile server's agent connection closed with WebSocket code 1006 and `unexpected EOF`. The agent reported this as a deliberate server disconnect and resumed at retry attempt 27 with a one-minute delay. Launches can fail while the connection is briefly absent, even though other agent operations already tolerate a short reconnect. The transport's initiating cause is not established by the available logs.

## What Changes

- Reset the consecutive retry count after any established WebSocket session ends, so old sessions do not leave later launch attempts behind a maximum backoff.
- Let a launch wait briefly for an agent that was recently connected, within the request's deadline, before deciding whether an agent is available.
- Describe code 1006 and other closures without a received close frame as transport loss, while preserving explicit server close reasons.
- Add regression coverage and a user-facing changelog entry.

## Scope

The change targets connection recovery and launches on the existing agent transport. It does not replay an unconfirmed launch, recover a lost running execution, change unrelated browser connections, or migrate hosting architecture. Keepalive intervals and infrastructure settings remain as they are unless incident evidence supports a separate correction.
