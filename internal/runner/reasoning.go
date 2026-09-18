package runner

import (
	"encoding/json"
	"strconv"
	"strings"
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
}

// reasoningLine is the outer shape of a stream line. Only the fields Sectile
// reads are declared, so an engine adding one changes nothing here.
type reasoningLine struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
			Name     string          `json:"name"`
			Input    json.RawMessage `json:"input"`
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
		return nil, parsed.Result, true

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
				events = append(events, ReasoningEvent{
					Kind:   ReasoningTool,
					Text:   shortToolName(block.Name),
					Detail: toolDetail(block.Name, block.Input),
				})
			}
		}
		return events, "", false
	}

	// system, user, and everything an engine will add: nothing to show.
	return nil, "", false
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
