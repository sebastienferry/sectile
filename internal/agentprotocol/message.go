package agentprotocol

import "encoding/json"

// Message is the envelope for all messages exchanged between the remote
// server and a connected local agent over the agent WebSocket relay.
type Message struct {
	MsgID   string          `json:"msgId"`
	Type    string          `json:"type"` // dispatch_step, pty_input, pty_resize, step_status, pty_output, heartbeat, error
	TaskID  string          `json:"taskId,omitempty"`
	UserID  string          `json:"userId,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}
