package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
)

// A conversation turn speaks Claude's streaming input protocol, the one the
// Claude Agent SDK uses: the message goes in on stdin as a JSON line, and
// stdin stays open until the turn's result so Claude can ask, through
// control requests, whether it may use a tool. The answer goes back on the
// same stdin. Stdin is closed on the result, and the process exits.

// conversationInput is the stdin of a running turn. Its writes are
// serialized: the turn, the owner's answers and an interrupt share it.
type conversationInput struct {
	mu     sync.Mutex
	w      io.WriteCloser
	closed bool
}

func (c *conversationInput) send(message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("the turn has ended")
	}
	_, err = c.w.Write(append(data, '\n'))
	return err
}

func (c *conversationInput) close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		_ = c.w.Close()
	}
}

// conversationUserMessage is how a message reaches Claude on stdin.
func conversationUserMessage(text string) map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}}
}

// conversationControl is a control request the agent sends Claude.
func conversationControl(subtype string) map[string]any {
	return map[string]any{"type": "control_request", "request_id": uuid.NewString(), "request": map[string]any{"subtype": subtype}}
}

// conversationApproval is a tool call waiting for the owner's decision.
type conversationApproval struct {
	ID          string          `json:"id"`
	ToolUseID   string          `json:"toolUseId,omitempty"`
	Tool        string          `json:"tool"`
	Description string          `json:"description,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	// Suggestions are the rules Claude proposes for "always allow"; they
	// are handed back as they came.
	Suggestions json.RawMessage `json:"suggestions,omitempty"`
}

// conversationControlRequest is the part of Claude's control requests the
// agent reads.
type conversationControlRequest struct {
	RequestID string `json:"request_id"`
	Request   struct {
		Subtype     string          `json:"subtype"`
		ToolName    string          `json:"tool_name"`
		Input       json.RawMessage `json:"input"`
		Suggestions json.RawMessage `json:"permission_suggestions"`
		ToolUseID   string          `json:"tool_use_id"`
		Description string          `json:"description"`
	} `json:"request"`
}

// conversationDecisions are the answers the owner can give a tool call.
var conversationDecisions = map[string]bool{"allow": true, "always": true, "deny": true}

// approvalResponse is the control response that carries a decision.
func approvalResponse(approval conversationApproval, decision string) map[string]any {
	answer := map[string]any{"behavior": "deny", "message": "The user denied this tool call."}
	if decision != "deny" {
		input := approval.Input
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		answer = map[string]any{"behavior": "allow", "updatedInput": input}
		if decision == "always" && len(approval.Suggestions) > 0 && string(approval.Suggestions) != "null" {
			answer["updatedPermissions"] = approval.Suggestions
		}
	}
	return map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": approval.ID, "response": answer}}
}

// controlError answers a control request the agent does not handle, so
// Claude does not wait for it.
func controlError(requestID, message string) map[string]any {
	return map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": requestID, "error": message}}
}

// decideApprovalLocked answers a pending tool call and records the decision
// in the trace. The queue lock is held.
func decideApprovalLocked(run *controlledRun, id, decision string) error {
	c := run.conversation
	for i, approval := range c.approvals {
		if approval.ID != id {
			continue
		}
		if c.input == nil {
			return fmt.Errorf("the turn has ended")
		}
		if err := c.input.send(approvalResponse(approval, decision)); err != nil {
			return err
		}
		c.approvals = append(c.approvals[:i:i], c.approvals[i+1:]...)
		conversationWriteEvent(run.trace, conversationEvent{Kind: "approval", Text: decision, Tool: approval.Tool, ToolID: approval.ToolUseID})
		return nil
	}
	return fmt.Errorf("no tool call is waiting for that decision")
}
