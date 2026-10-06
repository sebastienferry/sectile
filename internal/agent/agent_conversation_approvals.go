package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	// Suggestions are the updates Claude proposes for "always allow". They
	// are handed back for the running conversation only, and their allow
	// rules are added to the project's (#700).
	Suggestions json.RawMessage `json:"suggestions,omitempty"`
	Reason      string          `json:"reason,omitempty"`
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
		Reason      string          `json:"decision_reason"`
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

// sessionPermissions rewrites the updates an "Always allow" hands back so
// that only the running conversation applies them: Claude proposes a
// destination such as localSettings, the worktree's .claude/settings.local.json,
// which goes with the worktree or gets committed by mistake (#700). It returns
// the allow rules among them, as Claude writes a rule, for the project's own
// allow rules, and the updates it could not read, which are passed on for
// the conversation but persist nothing.
func sessionPermissions(raw json.RawMessage) (json.RawMessage, []string, []string) {
	var updates []json.RawMessage
	if json.Unmarshal(raw, &updates) != nil {
		return raw, nil, []string{string(raw)}
	}
	var rules, unread []string
	for i, update := range updates {
		var fields map[string]any
		if json.Unmarshal(update, &fields) != nil || fields == nil {
			unread = append(unread, string(update))
			continue
		}
		if _, ok := fields["destination"]; ok {
			fields["destination"] = "session"
		}
		if fields["type"] == "addRules" && fields["behavior"] == "allow" {
			if found, ok := permissionRules(fields["rules"]); ok {
				rules = append(rules, found...)
			} else {
				unread = append(unread, string(update))
			}
		}
		if rewritten, err := json.Marshal(fields); err == nil {
			updates[i] = rewritten
		}
	}
	rewritten, err := json.Marshal(updates)
	if err != nil {
		return raw, nil, []string{string(raw)}
	}
	return rewritten, rules, unread
}

// permissionRules renders the rules of an addRules update as Claude writes
// them in its settings: the tool alone, or the tool and its content in
// parentheses. A rule without a tool name makes the update unreadable.
func permissionRules(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	var rules []string
	for _, item := range list {
		rule, _ := item.(map[string]any)
		tool, _ := rule["toolName"].(string)
		content, _ := rule["ruleContent"].(string)
		if tool = strings.TrimSpace(tool); tool == "" || strings.ContainsAny(tool, "()") {
			return nil, false
		}
		if content == "" {
			rules = append(rules, tool)
		} else {
			rules = append(rules, tool+"("+content+")")
		}
	}
	return rules, true
}

// approvalResponse is the control response that carries a decision. An
// answer allows the AskUserQuestion call with its input plus the answers,
// keyed by question text, which is how Claude reads them.
func approvalResponse(approval conversationApproval, decision string, answers map[string]string) map[string]any {
	response, _, _ := approvalDecision(approval, decision, answers)
	return response
}

// approvalDecision is approvalResponse with what an "Always allow" approved:
// the allow rules to add to the project, and the updates it could not read.
func approvalDecision(approval conversationApproval, decision string, answers map[string]string) (map[string]any, []string, []string) {
	var rules, unread []string
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
			var updates json.RawMessage
			updates, rules, unread = sessionPermissions(approval.Suggestions)
			answer["updatedPermissions"] = updates
		}
	}
	return map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": approval.ID, "response": answer}}, rules, unread
}

// controlError answers a control request the agent does not handle, so
// Claude does not wait for it.
func controlError(requestID, message string) map[string]any {
	return map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": requestID, "error": message}}
}

// decideApprovalLocked answers a pending tool call and records the decision
// in the trace. The queue lock is held.
func (d *agentDaemon) decideApprovalLocked(run *controlledRun, id, decision string, answers map[string]string) error {
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
		response, rules, unread := approvalDecision(approval, decision, answers)
		if err := c.input.send(response); err != nil {
			return err
		}
		c.approvals = append(c.approvals[:i:i], c.approvals[i+1:]...)
		markApprovalWaitLocked(run)
		event := conversationEvent{Kind: "approval", Text: decision, Tool: approval.Tool, ToolID: approval.ToolUseID}
		if decision == "answer" {
			event.Detail = answersDetail(answers)
		}
		conversationWriteEvent(run.trace, event)
		d.keepAllowRulesLocked(run, rules, unread)
		if decision == "always" {
			directories := approvedDirectories(approval.Suggestions)
			if err := d.addProjectDirectories(run.desktop.ProjectID, directories); err != nil {
				conversationWrite(run.trace, "error", "The folders could not be kept in the project’s Claude settings: "+err.Error(), "")
			} else if len(directories) > 0 {
				conversationWrite(run.trace, "notice", "Folders added to the project’s Claude settings", strings.Join(directories, "\n"))
			}
		}
		return nil
	}
	return fmt.Errorf("no tool call is waiting for that decision")
}

// keepAllowRulesLocked adds the rules an "Always allow" approved to the
// project's allow rules, so the next turns and the next tasks of the project
// apply them, and says so in the trace. An update it could not read applies
// to this turn only, which is logged and said too. The queue lock is held.
func (d *agentDaemon) keepAllowRulesLocked(run *controlledRun, rules, unread []string) {
	for _, update := range unread {
		log.Printf("[Agent] \"Always allow\" update kept for this turn only, not understood: %s", update)
		conversationWrite(run.trace, "notice", "Allowed for this turn only", "Sectile could not read this rule to keep it in the project's Claude settings: "+update)
	}
	if len(rules) == 0 {
		return
	}
	added, err := d.addProjectAllowRules(run.desktop.ProjectID, rules)
	if err != nil {
		conversationWrite(run.trace, "error", "The rule could not be kept in the project's Claude settings: "+err.Error(), "")
		return
	}
	for _, rule := range added {
		conversationWrite(run.trace, "notice", "Rule added to the project's Claude settings", rule)
	}
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

// conversationSlash is a slash command Claude offers in a conversation's
// directory, as its initialize response lists it.
type conversationSlash struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ArgumentHint string `json:"argumentHint,omitempty"`
}

// conversationSlashLimit bounds the commands kept, and the length of each
// description, for the composer's completion.
const (
	conversationSlashLimit       = 1000
	conversationSlashDescription = 300
)

// initializeCommands reads the slash commands out of the response to the
// initialize request id, or reports that the line is not that response.
func initializeCommands(line, id string) ([]conversationSlash, bool) {
	var frame struct {
		Type     string `json:"type"`
		Response struct {
			RequestID string `json:"request_id"`
			Response  struct {
				Commands []conversationSlash `json:"commands"`
			} `json:"response"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(line), &frame) != nil || frame.Type != "control_response" || frame.Response.RequestID != id {
		return nil, false
	}
	commands := make([]conversationSlash, 0, len(frame.Response.Response.Commands))
	for _, command := range frame.Response.Response.Commands {
		name := strings.TrimSpace(command.Name)
		if name == "" || strings.ContainsAny(name, " \t\n") || len(commands) == conversationSlashLimit {
			continue
		}
		description := strings.TrimSpace(command.Description)
		if runes := []rune(description); len(runes) > conversationSlashDescription {
			description = string(runes[:conversationSlashDescription]) + "…"
		}
		commands = append(commands, conversationSlash{Name: name, Description: description, ArgumentHint: strings.TrimSpace(command.ArgumentHint)})
	}
	return commands, true
}

// conversationInitialize is the initialize request a turn or a probe sends,
// with the id its response is recognised by.
func conversationInitialize() (map[string]any, string) {
	request := conversationControl("initialize")
	return request, request["request_id"].(string)
}

// approvedDirectories extracts only directory additions explicitly approved by the owner.
func approvedDirectories(raw json.RawMessage) []string {
	var updates []struct {
		Type        string   `json:"type"`
		Directories []string `json:"directories"`
	}
	if json.Unmarshal(raw, &updates) != nil {
		return nil
	}
	var directories []string
	for _, update := range updates {
		if update.Type == "addDirectories" {
			directories = append(directories, update.Directories...)
		}
	}
	return directories
}
