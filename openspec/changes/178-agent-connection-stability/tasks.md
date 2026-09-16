# Tasks

## 1. Keep the keepalive reply off the busy write path
- [ ] 1.1 Install an explicit ping handler on the agent connection that hands the reply to a
      dedicated sender and returns immediately, instead of gorilla's default one-second write.
- [ ] 1.2 Run the sender for the connection's lifetime, with a one-slot queue so repeated probes
      collapse into a single pending reply.
- [ ] 1.3 Give the sender a write budget sized against the server read timeout, and log a reply
      it cannot write.

## 2. Separate the keepalive period from the drop deadline
- [ ] 2.1 Move the agent heartbeat period to its own named constant and set the margin.
- [ ] 2.2 Raise the server read timeout to keep several missed keepalives of slack.
- [ ] 2.3 State the margin in a comment at both constants.

## 3. Name the disconnection reason
- [ ] 3.1 Close a timed-out agent connection with a distinct close code and a reason naming the
      silence window.
- [ ] 3.2 Log the received close code and reason on the agent before reconnecting.

## 4. Tolerate a reconnecting agent
- [ ] 4.1 Wait for an agent to register in `CallOperation`, bounded by a grace period and the
      caller's deadline.
- [ ] 4.2 Report the absence as a reconnection that can be retried.

## 5. Tests
- [ ] 5.1 A probe sent while a long write holds the connection still receives its reply.
- [ ] 5.2 Repeated probes do not accumulate pending replies.
- [ ] 5.3 A silent connection is still dropped, and closed with the reason.
- [ ] 5.4 An operation succeeds when the agent reconnects within the grace period.
- [ ] 5.5 An operation with no agent fails with the retry wording, and never outlives the
      caller's deadline.

## 6. Gates
- [ ] 6.1 Build, vet and the Go suite.
- [ ] 6.2 `openspec validate 178-agent-connection-stability --strict`.
- [ ] 6.3 Record the manual 30-minute idle verification procedure with the change.
