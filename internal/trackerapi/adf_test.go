package trackerapi

import (
	"encoding/json"
	"strconv"
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
			name:     "list item continued after a blank line",
			markdown: "- a\n\n  continued\n- b",
			contains: []string{`"type":"bulletList"`},
		},
		{
			name:     "ordered item continued after a blank line",
			markdown: "1. a\n\n   continued\n2. b",
			contains: []string{`"type":"orderedList"`},
		},
		{
			name:     "list item holding a code block",
			markdown: "- step\n\n  ```sh\n  cmd\n  ```\n- next",
			contains: []string{`"type":"codeBlock"`, `"language":"sh"`},
		},
		{
			name:     "list item continued after a nested list",
			markdown: "- a\n  - x\n\n  para\n- b",
			contains: []string{`"type":"bulletList"`},
		},
		{
			name:     "lazy continuation comes back indented",
			markdown: "- a\ncontinued\n- b",
			contains: []string{`"type":"hardBreak"`},
			back:     "- a\n  continued\n- b",
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

// A list indented under a sentence is a list, not a list inside an empty
// bullet. It was read as one level deeper than its parent and hung under a
// listItem with no paragraph; Jira's validator refuses a listItem whose content
// does not begin with one, so the site answered 400 and the whole description
// or comment was lost.
func TestAnIndentedListIsNotNestedUnderAnEmptyBullet(t *testing.T) {
	doc := MarkdownToADF("Intro:\n  - x\n  - y\n")
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `{"type":"listItem"}`) {
		t.Fatalf("a listItem must carry a paragraph: %s", raw)
	}

	var parsed struct {
		Content []struct {
			Type    string `json:"type"`
			Content []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			} `json:"content"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Content) != 2 || parsed.Content[0].Type != "paragraph" || parsed.Content[1].Type != "bulletList" {
		t.Fatalf("a paragraph then one list: %s", raw)
	}
	items := parsed.Content[1].Content
	if len(items) != 2 {
		t.Fatalf("both bullets belong to the same list: %s", raw)
	}
	for _, item := range items {
		if item.Type != "listItem" || len(item.Content) == 0 || item.Content[0].Type != "paragraph" {
			t.Fatalf("every listItem opens on a paragraph: %s", raw)
		}
	}
}

// adfTree is the decoded shape of a converted document, for the tests that
// assert on its structure rather than on fragments of its JSON.
type adfTree struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Content []adfTree      `json:"content"`
}

func decodeADF(t *testing.T, doc map[string]any) adfTree {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var tree adfTree
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	return tree
}

// shape writes a tree on one line: a block is its type followed by its
// children in brackets, a text node is its quoted text, a hard break is "/".
// It keeps the expected structure of a test readable at a glance.
func shape(nodes []adfTree) string {
	var parts []string
	for _, node := range nodes {
		switch node.Type {
		case "text":
			parts = append(parts, strconv.Quote(node.Text))
		case "hardBreak":
			parts = append(parts, "/")
		default:
			part := node.Type
			if lang, _ := node.Attrs["language"].(string); lang != "" {
				part += ":" + lang
			}
			if len(node.Content) > 0 {
				part += "[" + shape(node.Content) + "]"
			}
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

// everyListItemOpensOnAParagraph walks a tree for the #180 invariant: Jira
// refuses a listItem whose content does not begin with a paragraph.
func everyListItemOpensOnAParagraph(t *testing.T, node adfTree) {
	t.Helper()
	if node.Type == "listItem" && (len(node.Content) == 0 || node.Content[0].Type != "paragraph") {
		t.Errorf("a listItem must open on a paragraph: %s", shape([]adfTree{node}))
	}
	for _, child := range node.Content {
		everyListItemOpensOnAParagraph(t, child)
	}
}

// An item whose text goes on past its marker line is one item: after a blank
// line when the text is indented to the item's content column, and without a
// blank line whatever its indent. It used to close the list, emit the rest as
// a top-level paragraph and open a second list, so Jira showed a broken list.
func TestAListItemKeepsWhatContinuesIt(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		want     string
	}{
		{
			name:     "paragraph after a blank line",
			markdown: "- a\n\n  continued\n- b",
			want:     `bulletList[listItem[paragraph["a"] paragraph["continued"]] listItem[paragraph["b"]]]`,
		},
		{
			name:     "ordered item continued at its own content column",
			markdown: "1. a\n\n   continued\n2. b",
			want:     `orderedList[listItem[paragraph["a"] paragraph["continued"]] listItem[paragraph["b"]]]`,
		},
		{
			name:     "several blank lines",
			markdown: "- a\n\n\n  continued",
			want:     `bulletList[listItem[paragraph["a"] paragraph["continued"]]]`,
		},
		{
			name:     "unindented text after a blank line ends the list",
			markdown: "- a\n\ncontinued",
			want:     `bulletList[listItem[paragraph["a"]]] paragraph["continued"]`,
		},
		{
			name:     "text short of the content column after a blank line ends the list",
			markdown: "1. a\n\n  continued",
			want:     `orderedList[listItem[paragraph["a"]]] paragraph["continued"]`,
		},
		{
			name:     "indented line without a blank line",
			markdown: "- a\n  continued\n- b",
			want:     `bulletList[listItem[paragraph["a" / "continued"]] listItem[paragraph["b"]]]`,
		},
		{
			name:     "lazy line without a blank line",
			markdown: "- a\ncontinued\n- b",
			want:     `bulletList[listItem[paragraph["a" / "continued"]] listItem[paragraph["b"]]]`,
		},
		{
			name:     "lazy line joins the innermost item",
			markdown: "- a\n  - x\n  more",
			want:     `bulletList[listItem[paragraph["a"] bulletList[listItem[paragraph["x" / "more"]]]]]`,
		},
		{
			name:     "code block after a blank line",
			markdown: "- step\n\n  ```sh\n  cmd\n\n    indented\n  ```\n- next",
			want:     `bulletList[listItem[paragraph["step"] codeBlock:sh["cmd\n\n  indented"]] listItem[paragraph["next"]]]`,
		},
		{
			name:     "code block right under the text",
			markdown: "- step\n  ```sh\n  cmd\n  ```\n- next",
			want:     `bulletList[listItem[paragraph["step"] codeBlock:sh["cmd"]] listItem[paragraph["next"]]]`,
		},
		{
			name:     "code block short of the content column ends the list",
			markdown: "- step\n```sh\ncmd\n```",
			want:     `bulletList[listItem[paragraph["step"]]] codeBlock:sh["cmd"]`,
		},
		{
			name:     "text at the content column after the item's code block",
			markdown: "- step\n  ```\n  cmd\n  ```\n  after",
			want:     `bulletList[listItem[paragraph["step"] codeBlock["cmd"] paragraph["after"]]]`,
		},
		{
			name:     "unindented text after the item's code block ends the list",
			markdown: "- step\n  ```\n  cmd\n  ```\nafter",
			want:     `bulletList[listItem[paragraph["step"] codeBlock["cmd"]]] paragraph["after"]`,
		},
		{
			name:     "a paragraph at the outer column belongs to the outer item",
			markdown: "- a\n  - x\n\n  para\n- b",
			want:     `bulletList[listItem[paragraph["a"] bulletList[listItem[paragraph["x"]]] paragraph["para"]] listItem[paragraph["b"]]]`,
		},
		{
			name:     "a paragraph at the nested column belongs to the nested item",
			markdown: "- a\n  - x\n\n    para",
			want:     `bulletList[listItem[paragraph["a"] bulletList[listItem[paragraph["x"] paragraph["para"]]]]]`,
		},
		{
			name:     "a nested list after a continuation paragraph",
			markdown: "- a\n\n  para\n  - x",
			want:     `bulletList[listItem[paragraph["a"] paragraph["para"] bulletList[listItem[paragraph["x"]]]]]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := decodeADF(t, MarkdownToADF(tc.markdown))
			if got := shape(tree.Content); got != tc.want {
				t.Errorf("shape:\n got %s\nwant %s", got, tc.want)
			}
			everyListItemOpensOnAParagraph(t, tree)
		})
	}
}

// ADF does not allow a heading, a quote, a rule or a table inside a listItem,
// so a line that opens one ends the list instead of joining the item's text,
// indented or not.
func TestABlockAListItemCannotHoldEndsTheList(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		want     string
	}{
		{"heading", "- a\n  # h", `bulletList[listItem[paragraph["a"]]] heading["h"]`},
		{"quote", "- a\n> q", `bulletList[listItem[paragraph["a"]]] blockquote[paragraph["q"]]`},
		{"rule", "- a\n---", `bulletList[listItem[paragraph["a"]]] rule`},
		{"indented heading after a blank line", "- a\n\n  ## h", `bulletList[listItem[paragraph["a"]]] heading["h"]`},
		{"table", "- a\n| x |\n| --- |", `bulletList[listItem[paragraph["a"]]] table[tableRow[tableHeader[paragraph["x"]]]]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := decodeADF(t, MarkdownToADF(tc.markdown))
			if got := shape(tree.Content); got != tc.want {
				t.Errorf("shape:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// A description is read from Jira, edited in Sectile and written back. An item
// holding several paragraphs or a code block must come back as Markdown that
// converts to the same item: its later children were written on the very next
// line with only their first line indented, and read back as one paragraph.
func TestAMultiBlockListItemSurvivesTheJiraRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[
		{"type":"bulletList","content":[
			{"type":"listItem","content":[
				{"type":"paragraph","content":[{"type":"text","text":"first"},{"type":"hardBreak"},{"type":"text","text":"line two"}]},
				{"type":"paragraph","content":[{"type":"text","text":"second"},{"type":"hardBreak"},{"type":"text","text":"its line two"}]},
				{"type":"codeBlock","attrs":{"language":"sh"},"content":[{"type":"text","text":"cmd\n\n  indented"}]},
				{"type":"paragraph","content":[{"type":"text","text":"after the code"}]}
			]},
			{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"next"}]}]}
		]},
		{"type":"orderedList","content":[
			{"type":"listItem","content":[
				{"type":"paragraph","content":[{"type":"text","text":"step"}]},
				{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"detail"}]}]}]},
				{"type":"paragraph","content":[{"type":"text","text":"then"}]}
			]}
		]}
	]}`)
	markdown := ADFToMarkdown(raw)
	want := "- first\n  line two\n\n  second\n  its line two\n\n  ```sh\n  cmd\n\n    indented\n  ```\n\n  after the code\n- next\n\n" +
		"1. step\n   - detail\n\n   then"
	if markdown != want {
		t.Errorf("markdown:\n got %q\nwant %q", markdown, want)
	}

	var original adfTree
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	back := decodeADF(t, MarkdownToADF(markdown))
	if got, want := shape(back.Content), shape(original.Content); got != want {
		t.Errorf("round trip:\n got %s\nwant %s", got, want)
	}
	everyListItemOpensOnAParagraph(t, back)
}
