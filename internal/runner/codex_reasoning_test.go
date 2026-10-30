package runner_test

import (
	"strings"
	"tasks/internal/runner"
	"testing"
)

func TestCodexExecTraceShowsCommandsMCPAndCompletedAnswers(t *testing.T) {
	cases := []struct{ line, tool, detail string }{
		{`{"type":"item.started","item":{"id":"cmd-1","type":"command_execution","command":"python3 -m unittest","status":"in_progress"}}`, "Bash", "python3 -m unittest"},
		{`{"type":"item.started","item":{"id":"mcp-1","type":"mcp_tool_call","server":"sectile_companion","tool":"get_task","arguments":{"taskId":"fixture-1"}}}`, "mcp__sectile_companion__get_task", "fixture-1"},
		{`{"type":"item.completed","item":{"id":"file-1","type":"file_change","status":"completed","changes":[{"path":"greeting.py","kind":"add"}]}}`, "Edit", "greeting.py"},
	}
	for _, tc := range cases {
		events, result, done := runner.ParseReasoningLine(tc.line)
		if done || result != "" || len(events) != 1 || events[0].Tool != tc.tool || events[0].Detail != tc.detail || events[0].ToolID == "" {
			t.Fatalf("unexpected Codex trace: %+v %q %v", events, result, done)
		}
	}
	events, result, done := runner.ParseReasoningLine(`{"type":"item.completed","item":{"id":"msg-1","type":"agent_message","text":"Tests passed; MR ready."}}`)
	if !done || result != "Tests passed; MR ready." || len(events) != 1 || events[0].Kind != runner.ReasoningText {
		t.Fatal("completed Codex answer was lost")
	}
}

func TestCodexExecTraceKeepsFailuresAndBoundsMCPArguments(t *testing.T) {
	_, fatal, failed := runner.ParseReasoningLine(`{"type":"error","message":"MCP initialization failed"}`)
	if !failed || fatal != "MCP initialization failed" {
		t.Fatal("top-level Codex error was lost")
	}

	_, result, done := runner.ParseReasoningLine(`{"type":"turn.failed","error":{"message":"Provider connection failed"}}`)
	if !done || result != "Provider connection failed" {
		t.Fatal("Codex failure was lost")
	}
	events, _, _ := runner.ParseReasoningLine(`{"type":"item.completed","item":{"id":"cmd-1","type":"command_execution","command":"false","status":"failed"}}`)
	if len(events) != 1 || !strings.Contains(events[0].Text, "erreur") {
		t.Fatal("failed command looks successful")
	}
	events, _, _ = runner.ParseReasoningLine(`{"type":"item.started","item":{"id":"mcp-1","type":"mcp_tool_call","server":"sectile_companion","tool":"get_task","arguments":{"taskId":"fixture-1","large":"` + strings.Repeat("x", runner.ToolInputLimit) + `"}}}`)
	if len(events) != 1 || events[0].Input != nil || events[0].Detail != "fixture-1" {
		t.Fatal("oversized MCP arguments were retained or useful detail was lost")
	}
	for _, line := range []string{`{"type":"item.started"`, `{"type":"item.completed","item":{"type":"unknown","text":"ignore"}}`, `{"type":"item.started","item":{"type":"agent_message","text":"unfinished"}}`, `{"type":"item.started","item":{"type":"mcp_tool_call"}}`} {
		events, result, done := runner.ParseReasoningLine(line)
		if len(events) != 0 || result != "" || done {
			t.Fatalf("incomplete/unknown event became an answer: %q", line)
		}
	}
}
