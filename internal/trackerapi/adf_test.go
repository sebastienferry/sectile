package trackerapi

import (
	"encoding/json"
	"strings"
	"testing"
)

// The converter is what decides whether a clarification report is readable in
// Jira. Each case is one construct in, the ADF shape out, and back to Markdown.
func TestMarkdownToADFCoversTheReportSubset(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		// contains are fragments the JSON must carry: node types, marks, attrs.
		contains []string
		// back is the Markdown the round trip must give; empty keeps the input.
		back string
	}{
		{
			name:     "headings and paragraphs",
			markdown: "# Title\n\nA line.\nSecond line.\n\n### Deep",
			contains: []string{`"type":"heading"`, `"level":1`, `"level":3`, `"type":"hardBreak"`},
		},
		{
			name:     "bullet list with nesting",
			markdown: "- one\n- two\n  - nested\n- three",
			contains: []string{`"type":"bulletList"`, `"type":"listItem"`},
		},
		{
			name:     "ordered list",
			markdown: "1. first\n2. second",
			contains: []string{`"type":"orderedList"`},
		},
		{
			name:     "code block with language",
			markdown: "```go\nfmt.Println(\"hi\")\n```",
			contains: []string{`"type":"codeBlock"`, `"language":"go"`, `fmt.Println(\"hi\")`},
		},
		{
			name:     "inline marks",
			markdown: "Use `go test`, **bold**, *italic* and ~~gone~~.",
			contains: []string{`"type":"code"`, `"type":"strong"`, `"type":"em"`, `"type":"strike"`},
		},
		{
			name:     "link",
			markdown: "See [the spec](https://example.com/spec).",
			contains: []string{`"type":"link"`, `"href":"https://example.com/spec"`},
		},
		{
			name:     "quote and rule",
			markdown: "> quoted\n\n---\n\nafter",
			contains: []string{`"type":"blockquote"`, `"type":"rule"`},
		},
		{
			name:     "table",
			markdown: "| a | b |\n| --- | --- |\n| 1 | 2 |",
			contains: []string{`"type":"table"`, `"type":"tableHeader"`, `"type":"tableCell"`},
		},
		{
			name:     "underscore inside a word is not emphasis",
			markdown: "snake_case_name stays",
			contains: []string{`snake_case_name stays`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := MarkdownToADF(tc.markdown)
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range tc.contains {
				if !strings.Contains(string(raw), fragment) {
					t.Errorf("missing %s in %s", fragment, raw)
				}
			}
			back := ADFToMarkdown(raw)
			want := tc.back
			if want == "" {
				want = tc.markdown
			}
			if back != want {
				t.Errorf("round trip:\n got %q\nwant %q", back, want)
			}
		})
	}
}

func TestADFToMarkdownFlattensWhatItDoesNotKnow(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[
		{"type":"paragraph","content":[
			{"type":"text","text":"Ping "},
			{"type":"mention","attrs":{"id":"5b10","text":"@Ada"}},
			{"type":"text","text":" about "},
			{"type":"inlineCard","attrs":{"url":"https://acme.atlassian.net/browse/PE-1"}}
		]},
		{"type":"panel","attrs":{"panelType":"info"},"content":[{"type":"paragraph","content":[{"type":"text","text":"inside a panel"}]}]},
		{"type":"table","content":[
			{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"col"}]}]}]},
			{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"cell"}]}]}]}
		]},
		{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"x"}}]},
		{"type":"paragraph","content":[{"type":"text","text":"done","marks":[{"type":"strong"}]}]}
	]}`)
	got := ADFToMarkdown(raw)
	for _, want := range []string{"Ping @Ada about https://acme.atlassian.net/browse/PE-1", "inside a panel", "| col |", "| cell |", "**done**"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if ADFToMarkdown(json.RawMessage(`"plain string"`)) != "plain string" {
		t.Error("a plain string body must pass through")
	}
	if ADFToMarkdown(nil) != "" || ADFToMarkdown(json.RawMessage("null")) != "" {
		t.Error("an absent body is empty")
	}
}

func TestMarkdownToADFEmptyIsAnEmptyDocument(t *testing.T) {
	doc := MarkdownToADF("")
	if doc["type"] != "doc" {
		t.Fatalf("doc: %#v", doc)
	}
	if content, ok := doc["content"].([]any); !ok || len(content) != 0 {
		t.Fatalf("content: %#v", doc["content"])
	}
}
