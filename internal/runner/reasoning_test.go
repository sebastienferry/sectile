package runner_test

import (
	"strings"
	"testing"

	"tasks/internal/runner"
)

// The reasoning stream is the output of a program, not a contract. These tests
// pin the two halves of that: what Sectile shows when it understands a line,
// and the fact that it never fails a run when it does not.

func TestAThinkingBlockIsRenderedAsReasoning(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"The file has to be read before it is written.","signature":"abc"}]}}`

	events, result, done := runner.ParseReasoningLine(line)
	if done || result != "" {
		t.Fatalf("an assistant message is not the end of the stream, got done=%v result=%q", done, result)
	}
	if len(events) != 1 || events[0].Kind != runner.ReasoningThinking {
		t.Fatalf("expected one thinking event, got %+v", events)
	}
	if events[0].Text != "The file has to be read before it is written." {
		t.Errorf("the reasoning was not read: %q", events[0].Text)
	}
}

func TestATextBlockIsRenderedAsProse(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"I fixed the function."}]}}`

	events, _, _ := runner.ParseReasoningLine(line)
	if len(events) != 1 || events[0].Kind != runner.ReasoningText {
		t.Fatalf("expected one text event, got %+v", events)
	}
	if events[0].Text != "I fixed the function." {
		t.Errorf("the prose was not read: %q", events[0].Text)
	}
}

func TestAToolCallCarriesItsNameAndWhatItDid(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/internal/db/db.go","offset":40}}]}}`

	events, _, _ := runner.ParseReasoningLine(line)
	if len(events) != 1 || events[0].Kind != runner.ReasoningTool {
		t.Fatalf("expected one tool event, got %+v", events)
	}
	if events[0].Text != "Read" {
		t.Errorf("the tool was not named: %q", events[0].Text)
	}
	if events[0].Detail != "/repo/internal/db/db.go" {
		t.Errorf("the detail did not say what was read: %q", events[0].Detail)
	}
}

// The real shapes, taken from the traces on the machine: the detail is what is
// read, "Bash" and "Skill" on their own say nothing.
func TestEachToolShowsTheArgumentThatSaysWhatItDid(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"Bash", `{"command":"go test ./internal/...","description":"Run the tests","timeout":600000}`, "go test ./internal/..."},
		{"PowerShell", `{"command":"Get-ChildItem","description":"List files"}`, "Get-ChildItem"},
		{"Edit", `{"file_path":"/repo/main.go","old_string":"a","new_string":"b","replace_all":false}`, "/repo/main.go"},
		{"Write", `{"file_path":"/repo/new.go","content":"package main"}`, "/repo/new.go"},
		{"Glob", `{"pattern":"**/*.tsx"}`, "**/*.tsx"},
		{"ToolSearch", `{"query":"select:Read","max_results":5}`, "select:Read"},
		{"WebFetch", `{"url":"https://example.test/x","prompt":"summarise"}`, "https://example.test/x"},
		{"Skill", `{"skill":"specify-issue","args":"#261"}`, "specify-issue #261"},
		{"Agent", `{"description":"Explore the panel","subagent_type":"Explore","prompt":"a very long prompt"}`, "Explore · Explore the panel"},
		// An MCP tool: the name carries the server, the detail carries the question.
		{"mcp__atlassian__getJiraIssue", `{"cloudId":"abc","issueIdOrKey":"AUC-1234"}`, "AUC-1234"},
		// Nothing readable in the arguments: the name stays, the detail is empty.
		{"AskUserQuestion", `{"questions":[{"header":"x"}]}`, ""},
		{"TodoWrite", `{}`, ""},
	}

	for _, c := range cases {
		line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"` + c.name + `","input":` + c.input + `}]}}`
		events, _, _ := runner.ParseReasoningLine(line)
		if len(events) != 1 {
			t.Fatalf("%s: expected one event, got %+v", c.name, events)
		}
		if events[0].Detail != c.want {
			t.Errorf("%s: detail = %q, expected %q", c.name, events[0].Detail, c.want)
		}
	}
}

// An MCP tool is named mcp__<server>__<tool>, which is most of a panel's width
// spent on plumbing.
func TestAnMCPToolIsNamedByItsToolAndNotItsServer(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__plugin_tempo-tracing_tempo__traceql-search","input":{"query":"{ .service = \"x\" }"}}]}}`

	events, _, _ := runner.ParseReasoningLine(line)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", events)
	}
	if events[0].Text != "traceql-search" {
		t.Errorf("name = %q, expected the tool without its server", events[0].Text)
	}
}

// A heredoc, a patch or a whole file arrive here entire, and the trace is read
// at a glance.
func TestALongDetailIsCutToOneReadableLine(t *testing.T) {
	long := strings.Repeat("x", 400)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"echo ` + long + `"}}]}}`

	events, _, _ := runner.ParseReasoningLine(line)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", events)
	}
	if runes := []rune(events[0].Detail); len(runes) > 161 {
		t.Errorf("detail is %d runes, expected it clamped", len(runes))
	}
	if !strings.HasSuffix(events[0].Detail, "…") {
		t.Errorf("a cut detail must say it was cut: %q", events[0].Detail)
	}
}

// A command spanning several lines fits on one line in the trace.
func TestAMultilineDetailIsFlattened(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cd web\n  npm run test\n"}}]}}`

	events, _, _ := runner.ParseReasoningLine(line)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", events)
	}
	if events[0].Detail != "cd web npm run test" {
		t.Errorf("detail = %q, expected one flattened line", events[0].Detail)
	}
}

func TestARunThatNeverThinksShowsNothing(t *testing.T) {
	stream := []string{
		`{"type":"system","subtype":"init","cwd":"/repo"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Done."}]}}`,
		`{"type":"result","subtype":"success","result":"Done.","is_error":false}`,
	}
	for _, line := range stream {
		events, _, _ := runner.ParseReasoningLine(line)
		for _, ev := range events {
			if ev.Kind == runner.ReasoningThinking {
				t.Errorf("no thinking was emitted, yet one was read from %q", line)
			}
		}
	}
}

func TestALineThatIsNotJSONYieldsNothingAndNoError(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"Welcome to Claude Code!",
		"npm warn deprecated something@1.0.0",
		"{",
	}
	for _, line := range cases {
		events, result, done := runner.ParseReasoningLine(line)
		if len(events) != 0 || result != "" || done {
			t.Errorf("%q: expected nothing, got events=%+v result=%q done=%v", line, events, result, done)
		}
	}
}

// A stream cut short leaves a last incomplete line. It does not read, and that
// is all: the run produced what it produced.
func TestAStreamCutMidObjectYieldsNothingAndNoError(t *testing.T) {
	truncated := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"I am start`

	events, result, done := runner.ParseReasoningLine(truncated)
	if len(events) != 0 || result != "" || done {
		t.Errorf("expected nothing, got events=%+v result=%q done=%v", events, result, done)
	}
}

func TestARecognisedTypeWithAnUnexpectedBodyYieldsNothing(t *testing.T) {
	cases := []string{
		`{"type":"assistant"}`,
		`{"type":"assistant","message":{"content":[]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":""}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}`,
		`{"type":"a_type_nobody_planned_for","payload":42}`,
	}
	for _, line := range cases {
		events, _, done := runner.ParseReasoningLine(line)
		if len(events) != 0 || done {
			t.Errorf("%q: expected nothing, got events=%+v done=%v", line, events, done)
		}
	}
}

func TestTheResultMessageCarriesTheAnswerAndEndsTheStream(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":false,"result":"### Report\n\nThe function is fixed."}`

	events, result, done := runner.ParseReasoningLine(line)
	if !done {
		t.Fatal("the result message ends the stream")
	}
	if len(events) != 0 {
		t.Errorf("the answer is not shown as reasoning, got %+v", events)
	}
	if !strings.Contains(result, "The function is fixed.") {
		t.Errorf("the answer was not read, got %q", result)
	}
}

func TestSeveralBlocksInOneMessageAreReadInOrder(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[` +
		`{"type":"thinking","thinking":"Read it first."},` +
		`{"type":"tool_use","name":"Read"},` +
		`{"type":"text","text":"There."}]}}`

	events, _, _ := runner.ParseReasoningLine(line)
	if len(events) != 3 {
		t.Fatalf("expected three events, got %+v", events)
	}
	want := []string{runner.ReasoningThinking, runner.ReasoningTool, runner.ReasoningText}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Errorf("event %d: expected %s, got %s", i, kind, events[i].Kind)
		}
	}
}

// An empty block of each kind produces no entry: the trace must not fill with
// blank lines when the engine said nothing.
func TestBlocksWithNothingInThemProduceNoEntry(t *testing.T) {
	cases := []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"   "}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"  "}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"  "}]}}`,
	}
	for _, line := range cases {
		events, _, _ := runner.ParseReasoningLine(line)
		if len(events) != 0 {
			t.Errorf("%q: expected no entry, got %+v", line, events)
		}
	}
}
