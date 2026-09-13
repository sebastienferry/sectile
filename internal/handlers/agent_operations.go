package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
)

type pendingOperation struct {
	agent  *AgentConn
	result chan agentprotocol.Result
}

func (d *AgentDispatcher) CallOperation(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
	ac := d.Lookup("default", op.ProjectID)
	if ac == nil {
		return nil, fmt.Errorf("no local agent connected for project %s", op.ProjectID)
	}
	if op.Action == "execute_skill" || op.Action == "open_terminal" {
		action := op.SkillID
		if op.Action == "open_terminal" {
			action = "open_terminal"
		}
		err := d.DispatchAndWait(ctx, ac.UserID, ac.ProjectID, op.TaskID, agentconfig.Dispatch{SchemaVersion: agentconfig.Version, TaskID: op.TaskID, TaskKey: op.TaskID, ProjectID: op.ProjectID, SkillID: op.SkillID, Action: action, Prompt: op.Prompt, RunID: op.RunID})
		return json.RawMessage("null"), err
	}
	raw, err := json.Marshal(op)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	pending := &pendingOperation{ac, make(chan agentprotocol.Result, 1)}
	d.mu.Lock()
	if d.operations == nil {
		d.operations = map[string]*pendingOperation{}
	}
	d.operations[id] = pending
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.operations, id); d.mu.Unlock() }()
	if err = ac.Send(AgentMessage{MsgID: id, Type: "workspace_request", TaskID: op.TaskID, Payload: raw}); err != nil {
		return nil, err
	}
	select {
	case res := <-pending.result:
		if res.Error != "" {
			return nil, fmt.Errorf("local agent: %s", res.Error)
		}
		return res.Value, nil
	case <-ac.done:
		return nil, fmt.Errorf("agent disconnected before confirming local operation; check the local agent before retrying")
	case <-ctx.Done():
		_ = ac.Send(AgentMessage{MsgID: id, Type: "workspace_cancel"})
		return nil, fmt.Errorf("local operation not confirmed; check the agent before retrying: %w", ctx.Err())
	}
}
func (d *AgentDispatcher) ReportOperation(ac *AgentConn, msg AgentMessage) {
	var res agentprotocol.Result
	if json.Unmarshal(msg.Payload, &res) != nil || (res.Error == "" && len(res.Value) == 0) {
		return
	}
	d.mu.RLock()
	pending := d.operations[msg.MsgID]
	d.mu.RUnlock()
	if pending == nil || pending.agent != ac {
		return
	}
	select {
	case pending.result <- res:
	default:
	}
}
