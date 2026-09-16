# Validation

## Automated
- `go build ./...`, `go vet ./...`, `go test ./...`.
- `cmd/agent`: a keepalive probe arriving while a frame flush holds the connection for longer
  than gorilla's one-second budget is still answered — this test fails without the fix; repeated probes do not accumulate one pending
  reply each; the heartbeat keeps at least four attempts below the server read timeout.
- `internal/handlers`: an operation succeeds when the agent registers after the call started
  waiting; an absent agent produces an error naming the reconnection and the retry; the wait
  never outlives the caller's deadline; a dropped connection carries close code 4002 and a
  reason naming the silence; the read timeout keeps room for several missed pings.

## Manual — the thirty-minute idle requirement
Thirty minutes of wall clock cannot be asserted in the Go suite. Verify it by observation:

1. Start the server and the desktop app, and wait for the agent to register.
2. Record `connectedAt`: `curl -s localhost:8090/api/agent/status | jq '.agents[0].connectedAt'`.
3. Leave the machine idle for thirty minutes, sampling every five minutes.
4. `connectedAt` must be unchanged at every sample, and `lastPingAt` must keep advancing.

A changed `connectedAt` means the connection was re-established; the agent log then names the
close code and reason, which is the diagnostic this change adds.

## Manual — the reconnection window
1. Stop the agent process, then start it again.
2. Within the first seconds, request a stage transition.
3. It either succeeds once the agent is back, or fails with an error naming the retry — never
   with the bare "no local agent connected".
