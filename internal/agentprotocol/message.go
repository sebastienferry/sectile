package agentprotocol

import (
	"encoding/json"
	"time"
)

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
