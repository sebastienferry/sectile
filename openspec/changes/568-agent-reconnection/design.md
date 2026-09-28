# Design

## Evidence and limits

The ticket shows a single abnormal WebSocket closure and retry attempt 27. It lacks the connection start time, deployed builds, server logs, and recurrence interval, so neither Scaleway ingress nor dependency installation is a proven cause. The repository's Scaleway container configuration does not set a request timeout; changing it without deployment evidence could produce an unrelated behavior change.

## Decisions

### Count consecutive failed connection attempts

`connect` owns a WebSocket session after a successful dial. Record whether it reached that point, and reset the loop's failure counter when such a session ends. A failure before the dial remains consecutive and retains exponential backoff. A session that drops still waits a short retry delay to avoid a busy loop. Rejected: resetting only on a nil return, since the read loop normally returns an error on network loss and the count never resets.

### Wait before recording a launch

The task launch handler currently calls `Route` and returns a conflict immediately when it is nil. Resolve the route with the dispatcher's existing recent-agent grace before recording a run or launch activity. A route that never existed remains an immediate conflict. The wait respects request cancellation; on timeout the handler records no run or activity and gives a reconnect-specific French error if the agent was recently connected. Recheck the route at dispatch so a connection changing between lookup and send can be used; do not automatically repeat a command once sent, because a missing acknowledgment cannot prove it did not launch.

Rejected: queueing launches or replaying them after a lost acknowledgment. Either could start a side-effecting skill twice.

### Classify close frames accurately

WebSocket close code 1006 is synthetic: it cannot be sent in a close frame. Treat it as transport loss even if gorilla attaches `unexpected EOF` text. Keep explicit server close code and reason reporting for received frames, including session rebind and keepalive timeout. Runtime user-facing text remains French.

## Verification

Test failed dial versus an established connection, short versus repeated failures, abnormal versus explicit close reasons, and launch recovery or expiry without a ghost run. Run OpenSpec validation, Go build, vet and tests. Inspect the final diff against the clarification. A deployed recurrence remains observable only with correlated agent/server logs and connected-session timestamps.
