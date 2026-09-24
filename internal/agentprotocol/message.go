package agentprotocol

import (
	"encoding/json"
	"time"
)

// RunNotOwned prefixes the status an agent returns when it is asked to cancel
// an execution it does not have. It is a stable marker rather than free text
// because the server acts on it: an agent that answers this is reachable and
// states it is running nothing, which is what tells the two apart from an
// agent that cannot be reached at all.
const RunNotOwned = "run-not-owned"

// Message is the envelope for all messages exchanged between the remote
// server and a connected local agent over the agent WebSocket relay.
type Message struct {
	MsgID   string          `json:"msgId"`
	Type    string          `json:"type"` // dispatch_step, pty_input, pty_resize, step_status, pty_output, heartbeat, pull_tasks, running_tasks, run_waiting, error
	TaskID  string          `json:"taskId,omitempty"`
	UserID  string          `json:"userId,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RunWaitingType is the message the server sends to a run owner's agent when the
// run's waiting mark changes. An agent that predates it logs the unknown type
// and carries on, which is why it needs no new protocol version.
const RunWaitingType = "run_waiting"

// RunWaiting is the payload of RunWaitingType. A nil WaitingSince clears the
// mark. The server's instant is sent rather than the agent's, so the desktop and
// the board count the wait from the same moment.
type RunWaiting struct {
	RunID        string     `json:"runId"`
	WaitingSince *time.Time `json:"waitingSince"`
}

// RunningTask represents an active task execution reported by an agent.
type RunningTask struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	TaskKey   string    `json:"taskKey"`
	ProjectID string    `json:"projectId"`
	Skill     string    `json:"skill"`
	Status    string    `json:"status"` // "queued", "running"
	CreatedAt time.Time `json:"createdAt"`
	StartedAt time.Time `json:"startedAt,omitzero"`
	Branch    string    `json:"branch,omitempty"`
	Directory string    `json:"directory,omitempty"`
}
