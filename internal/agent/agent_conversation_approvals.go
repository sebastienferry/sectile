package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
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

// conversationDecisions are the answers the owner can give a tool call;
// "answer" is the one an AskUserQuestion call takes.
var conversationDecisions = map[string]bool{"allow": true, "always": true, "deny": true, "answer": true}

// askUserQuestion is the tool Claude asks the owner questions through.
const askUserQuestion = "AskUserQuestion"

// conversationAnswersLimit bounds what an owner's answers may weigh.
const conversationAnswersLimit = 16 * 1024

// approvalResponse is the control response that carries a decision. An
// answer allows the AskUserQuestion call with its input plus the answers,
// keyed by question text, which is how Claude reads them.
func approvalResponse(approval conversationApproval, decision string, answers map[string]string) map[string]any {
	answer := map[string]any{"behavior": "deny", "message": "The user denied this tool call."}
	if decision == "answer" {
		input := map[string]any{}
		_ = json.Unmarshal(approval.Input, &input)
		input["answers"] = answers
		answer = map[string]any{"behavior": "allow", "updatedInput": input}
	} else if decision != "deny" {
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
func decideApprovalLocked(run *controlledRun, id, decision string, answers map[string]string) error {
	c := run.conversation
	for i, approval := range c.approvals {
		if approval.ID != id {
			continue
		}
		if c.input == nil {
			return fmt.Errorf("the turn has ended")
		}
		if err := checkAnswers(approval, decision, answers); err != nil {
			return err
		}
		if err := c.input.send(approvalResponse(approval, decision, answers)); err != nil {
			return err
		}
		c.approvals = append(c.approvals[:i:i], c.approvals[i+1:]...)
		markApprovalWaitLocked(run)
		event := conversationEvent{Kind: "approval", Text: decision, Tool: approval.Tool, ToolID: approval.ToolUseID}
		if decision == "answer" {
			event.Detail = answersDetail(answers)
		}
		conversationWriteEvent(run.trace, event)
		return nil
	}
	return fmt.Errorf("no tool call is waiting for that decision")
}

// checkAnswers keeps answers to AskUserQuestion calls, and holds them to one
// non-empty answer per question asked, within a size bound.
func checkAnswers(approval conversationApproval, decision string, answers map[string]string) error {
	if decision != "answer" {
		if len(answers) > 0 {
			return fmt.Errorf("answers only go with an answer")
		}
		return nil
	}
	if approval.Tool != askUserQuestion {
		return fmt.Errorf("only a question takes answers")
	}
	var input struct {
		Questions []struct {
			Question string `json:"question"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(approval.Input, &input)
	asked, size := map[string]bool{}, 0
	for _, question := range input.Questions {
		asked[question.Question] = true
	}
	for question, answer := range answers {
		size += len(question) + len(answer)
		if !asked[question] || strings.TrimSpace(answer) == "" {
			return fmt.Errorf("an answer does not match a question asked")
		}
	}
	if len(answers) != len(asked) || size > conversationAnswersLimit {
		return fmt.Errorf("answer every question, briefly")
	}
	return nil
}

// answersDetail is how the trace keeps the owner's answers, one per line.
func answersDetail(answers map[string]string) string {
	questions := make([]string, 0, len(answers))
	for question := range answers {
		questions = append(questions, question)
	}
	sort.Strings(questions)
	lines := make([]string, 0, len(questions))
	for _, question := range questions {
		lines = append(lines, question+" → "+answers[question])
	}
	return strings.Join(lines, "\n")
}
