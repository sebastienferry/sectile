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
	Type    string          `json:"type"` // dispatch_step, pty_input, pty_resize, step_status, pty_output, heartbeat, pull_tasks, running_tasks, error
	TaskID  string          `json:"taskId,omitempty"`
	UserID  string          `json:"userId,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
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
