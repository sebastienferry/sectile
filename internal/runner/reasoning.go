package runner

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Reading the reasoning stream of an engine.
//
// Asked for it, Claude prints one JSON object per line instead of its answer
// alone: the assistant messages carry the thinking, the prose and the tool calls
// as they happen, and a final result message carries the answer. Sectile shows
// the first three in the run's console and keeps the last as the run's output.
//
// Everything here treats the stream as untrusted. It is the output of a program,
// not a contract: a line that is not JSON, a shape nobody planned for, a stream
// cut in the middle of an object are all normal endings, and none of them may
// fail a run that otherwise worked.
//
// One thing measured rather than assumed: the thinking blocks come back with an
// empty string and only a signature. Every one of them, on the machine this was
// written on — in the live stream and in the transcripts Claude Code keeps —
// even on a prompt that forces extended thinking. The reasoning is encrypted and
// the API does not surface it. So what a reader actually gets from a run is the
// prose and the tool calls, and the thinking case below is kept because the day
// it does arrive it costs nothing, not because it fires today.

// Reasoning event kinds, in the order a reader meets them.
const (
	ReasoningThinking = "thinking"
	ReasoningText     = "text"
	ReasoningTool     = "tool"
)

// ReasoningEvent is one renderable thing pulled out of the stream.
type ReasoningEvent struct {
	// Kind is one of the three constants above.
	Kind string
	// Text is the reasoning, the prose, or the name of the tool called.
	Text string
	// Detail says what a tool call was about: the command run, the file read,
	// the skill invoked. "Bash" and "Skill" on their own tell a reader that
	// something happened and nothing about what.
	Detail string
	// Tool is the full name of the tool called, and Input its arguments as
	// the engine sent them, for a reader that draws each tool its own way.
	// Input is dropped past ToolInputLimit; Detail still says what it was.
	Tool  string
	Input json.RawMessage
	// ToolID is the call's identifier, which its result names.
	ToolID string
}

// ToolInputLimit bounds the arguments kept with a tool call. A Write carries
// the whole file it writes, and a trace keeps thousands of events.
const ToolInputLimit = 64 * 1024

// reasoningLine is the outer shape of a stream line. Only the fields Sectile
// reads are declared, so an engine adding one changes nothing here.
type reasoningLine struct {
	Type string `json:"type"`
	// Subtype and IsError are how a result message reports a run that did not
	// reach an answer. They are the only thing such a message carries, so they
	// are read for the same reason Result is: the activity has to say why.
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
			Name     string          `json:"name"`
			Input    json.RawMessage `json:"input"`
			ID       string          `json:"id"`
		} `json:"content"`
	} `json:"message"`
}

// ParseReasoningLine reads one line of the stream. It returns the events the
// line carries, the final answer when the line is the result message, and
// whether it was that message.
//
// A line it cannot read yields nothing at all and no error: see the note above.
func ParseReasoningLine(line string) (events []ReasoningEvent, result string, done bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil, "", false
	}

	var parsed reasoningLine
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, "", false
	}

	switch parsed.Type {
	case "result":
		return nil, resultText(parsed), true

	case "assistant":
		for _, block := range parsed.Message.Content {
			switch block.Type {
			case "thinking":
				// An encrypted thought comes back with its signature and an
				// empty text: the API does not surface it, and there is
				// nothing to show.
				if strings.TrimSpace(block.Thinking) == "" {
					continue
				}
				events = append(events, ReasoningEvent{Kind: ReasoningThinking, Text: block.Thinking})
			case "text":
				if strings.TrimSpace(block.Text) == "" {
					continue
				}
				events = append(events, ReasoningEvent{Kind: ReasoningText, Text: block.Text})
			case "tool_use":
				if strings.TrimSpace(block.Name) == "" {
					continue
				}
				event := ReasoningEvent{
					Kind:   ReasoningTool,
					Text:   shortToolName(block.Name),
					Detail: toolDetail(block.Name, block.Input),
					Tool:   block.Name,
					ToolID: block.ID,
				}
				if len(block.Input) <= ToolInputLimit && json.Valid(block.Input) {
					event.Input = block.Input
				}
				events = append(events, event)
			}
		}
		return events, "", false
	}

	// system, user, and everything an engine will add: nothing to show.
	return nil, "", false
}

// resultText is what a result message leaves on the task activity.
//
// A run that reached an answer carries it here. A run that failed often carries
// nothing at all: the engine reports the failure in the frame's subtype and
// prints no text beside it, so recording the answer alone would leave the
// activity empty exactly where the reason belongs — and the run's own status
// only ever says the process exited non-zero. The subtype is then all the run
// has to say for itself, and it is worth more than silence.
func resultText(parsed reasoningLine) string {
	if strings.TrimSpace(parsed.Result) != "" {
		return parsed.Result
	}
	if !parsed.IsError {
		return ""
	}
	if reason := strings.TrimSpace(parsed.Subtype); reason != "" {
		return "The engine ended the run without an answer: " + reason
	}
	return "The engine ended the run without an answer."
}

// toolDetailKeys are the input fields worth showing, most specific first. A tool
// carries a dozen of them and only one says what the call was about: the command
// for a shell, the path for an edit, the pattern for a search.
//
// The list is ordered rather than per-tool because engines and MCP servers add
// tools nobody here has heard of, and they name their arguments from the same
// small vocabulary. A tool matching none of it still shows its name.
var toolDetailKeys = []string{
	"command", "skill", "file_path", "path", "pattern", "query", "url",
	"description", "prompt", "trace_id", "issueIdOrKey", "pageId", "name", "regex",
}

// toolDetail pulls the readable half of a tool call out of its arguments.
func toolDetail(name string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return ""
	}

	// Two shapes read better composed: a skill is its name and its arguments,
	// a sub-agent is its type and what it is being asked for.
	switch name {
	case "Skill":
		return clampDetail(strings.TrimSpace(scalar(args["skill"]) + " " + scalar(args["args"])))
	case "Agent", "Task":
		kind, what := scalar(args["subagent_type"]), scalar(args["description"])
		if kind != "" && what != "" {
			return clampDetail(kind + " · " + what)
		}
		return clampDetail(kind + what)
	}

	for _, key := range toolDetailKeys {
		if v := scalar(args[key]); strings.TrimSpace(v) != "" {
			return clampDetail(v)
		}
	}
	return ""
}

// scalar renders an argument that is worth showing, and nothing for one that is
// not: a list of questions or a whole file's content is not a detail.
func scalar(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

// clampDetail keeps a detail to one readable line. A heredoc, a patch or a
// generated file arrive here whole, and the trace is read at a glance.
func clampDetail(detail string) string {
	detail = strings.Join(strings.Fields(detail), " ")
	const max = 160
	if runes := []rune(detail); len(runes) > max {
		return strings.TrimSpace(string(runes[:max])) + "…"
	}
	return detail
}

// shortToolName drops the plumbing from an MCP tool's name. They are named
// mcp__<server>__<tool>, which is most of a panel's width spent on the server.
func shortToolName(name string) string {
	if !strings.HasPrefix(name, "mcp__") {
		return name
	}
	parts := strings.Split(name, "__")
	if last := parts[len(parts)-1]; strings.TrimSpace(last) != "" {
		return last
	}
	return name
}

// ToolResult is what a tool call answered, as the engine reported it back.
type ToolResult struct {
	// ToolID names the call this answers.
	ToolID string
	// Text is the answer, cut to ToolResultLimit bytes; Truncated says so.
	Text      string
	Truncated bool
	IsError   bool
}

// ToolResultLimit bounds the answer kept from a tool call. A read returns the
// whole file and a command all it printed; a reader needs the start of it.
const ToolResultLimit = 16 * 1024

// toolResultLine is the shape of the user messages that carry tool results.
type toolResultLine struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type    string          `json:"type"`
			ToolID  string          `json:"tool_use_id"`
			Content json.RawMessage `json:"content"`
			IsError bool            `json:"is_error"`
		} `json:"content"`
	} `json:"message"`
}

// ParseToolResults reads the tool results a stream line carries. Like
// ParseReasoningLine, a line it cannot read yields nothing and no error.
func ParseToolResults(line string) []ToolResult {
	var parsed toolResultLine
	if json.Unmarshal([]byte(strings.TrimSpace(line)), &parsed) != nil || parsed.Type != "user" {
		return nil
	}
	var results []ToolResult
	for _, block := range parsed.Message.Content {
		if block.Type != "tool_result" || block.ToolID == "" {
			continue
		}
		text, truncated := clampResult(toolResultText(block.Content))
		results = append(results, ToolResult{ToolID: block.ToolID, Text: text, Truncated: truncated, IsError: block.IsError})
	}
	return results
}

// toolResultText flattens a result's content: a string, or a list of blocks
// whose text is kept and whose images are named.
func toolResultText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			parts = append(parts, block.Text)
		case "image":
			parts = append(parts, "[image]")
		}
	}
	return strings.Join(parts, "\n")
}

// clampResult cuts text to ToolResultLimit bytes on a character boundary.
func clampResult(text string) (string, bool) {
	if len(text) <= ToolResultLimit {
		return text, false
	}
	cut := ToolResultLimit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut], true
}
