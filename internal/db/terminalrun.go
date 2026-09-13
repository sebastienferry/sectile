package db

import (
	"fmt"
	"tasks/internal/agentprotocol"
)

// TTYSkillLaunch confirms delegation; only the local agent owns console processes.
type TTYSkillLaunch struct {
	SessionID     string `json:"sessionId"`
	AgentLaunched bool   `json:"agentLaunched"`
	AgentRunning  bool   `json:"agentRunning"`
	Call          string `json:"call"`
	Cwd           string `json:"cwd"`
	Provider      string `json:"provider"`
	LaunchCommand string `json:"launchCommand"`
}

func TaskSessionID(taskID string) string { return "task-" + taskID }
func (d *DB) StartAgentInTTY(taskID string, force bool) (*TTYSkillLaunch, error) {
	return d.launchAgentTTY(taskID, "")
}
func (d *DB) InjectSkillInTTY(taskID, skillID string) (*TTYSkillLaunch, error) {
	return d.launchAgentTTY(taskID, skillID)
}
func (d *DB) launchAgentTTY(taskID, skillID string) (*TTYSkillLaunch, error) {
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("task not found")
	}
	action := "execute_skill"
	if skillID == "" {
		action = "open_terminal"
	}
	if err = d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: action, SkillID: skillID}, nil); err != nil {
		return nil, err
	}
	return &TTYSkillLaunch{SessionID: TaskSessionID(task.ID), AgentLaunched: true, AgentRunning: true, Call: skillID}, nil
}
