package trackerapi

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// Atlassian Document Format is what Jira Cloud wants for a description or a
// comment, and what it answers with. Sectile writes Markdown: reports, comments,
// rewritten stories. The two functions below convert a bounded subset both ways:
// paragraphs, headings, bullet and ordered lists with one level of nesting,
// fenced code blocks, quotes, rules, inline code, bold, italics, strike and
// links. Anything else is flattened to its text rather than dropped, so a
// report never loses a sentence, only a decoration.

// adfNode is the generic tree node. Attrs and Marks stay loose: the format has
// many node kinds and only a few of them matter here.
type adfNode struct {
	Type    string         `json:"type"`
	Version int            `json:"version,omitempty"`
	Text    string         `json:"text,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []adfMark      `json:"marks,omitempty"`
	Content []adfNode      `json:"content,omitempty"`
}

type adfMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// MarkdownToADF converts Markdown into an ADF document, as a value ready to be
// JSON-encoded in an issue payload. An empty input gives an empty document,
// which Jira accepts as "no description".
func MarkdownToADF(markdown string) map[string]any {
	doc := adfNode{Type: "doc", Version: 1, Content: parseMarkdownBlocks(normaliseNewlines(markdown))}
	raw, _ := json.Marshal(doc)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]any{"type": "doc", "version": 1, "content": []any{}}
	}
	if _, ok := out["content"]; !ok {
		out["content"] = []any{}
	}
	return out
}

// ADFToMarkdown converts the ADF a work item or a comment carries into
// Markdown. A plain JSON string (Jira Server, or an already flattened value)
// is returned as is.
func ADFToMarkdown(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var node adfNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	return strings.TrimSpace(renderADFBlocks(node.Content, ""))
}

func normaliseNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

var (
	mdHeading  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdBullet   = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdOrdered  = regexp.MustCompile(`^(\s*)\d+[.)]\s+(.*)$`)
	mdFence    = regexp.MustCompile("^```\\s*([A-Za-z0-9_+#.-]*)\\s*$")
	mdRule     = regexp.MustCompile(`^\s*(-{3,}|\*{3,}|_{3,})\s*$`)
	mdQuote    = regexp.MustCompile(`^>\s?(.*)$`)
	mdTableSep = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$`)
)

// parseMarkdownBlocks walks the lines once and emits block nodes.
func parseMarkdownBlocks(markdown string) []adfNode {
	lines := strings.Split(markdown, "\n")
	var blocks []adfNode
	var paragraph []string

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		blocks = append(blocks, adfNode{Type: "paragraph", Content: inlineLines(paragraph)})
		paragraph = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			flushParagraph()
			continue
		}
		if m := mdFence.FindStringSubmatch(trimmed); m != nil {
			flushParagraph()
			var code []string
			j := i + 1
			for ; j < len(lines); j++ {
				if mdFence.MatchString(strings.TrimSpace(lines[j])) {
					break
				}
				code = append(code, lines[j])
			}
			node := adfNode{Type: "codeBlock"}
			if m[1] != "" {
				node.Attrs = map[string]any{"language": m[1]}
			}
			if text := strings.Join(code, "\n"); text != "" {
				node.Content = []adfNode{{Type: "text", Text: text}}
			}
			blocks = append(blocks, node)
			i = j
			continue
		}
		if m := mdHeading.FindStringSubmatch(trimmed); m != nil {
			flushParagraph()
			blocks = append(blocks, adfNode{Type: "heading", Attrs: map[string]any{"level": len(m[1])}, Content: parseInline(strings.TrimSpace(m[2]))})
			continue
		}
		if mdRule.MatchString(trimmed) && len(paragraph) == 0 {
			blocks = append(blocks, adfNode{Type: "rule"})
			continue
		}
		if mdBullet.MatchString(line) || mdOrdered.MatchString(line) {
			flushParagraph()
			list, next := parseMarkdownList(lines, i, 0)
			blocks = append(blocks, list)
			i = next - 1
			continue
		}
		if m := mdQuote.FindStringSubmatch(trimmed); m != nil {
			flushParagraph()
			var quoted []string
			j := i
			for ; j < len(lines); j++ {
				q := mdQuote.FindStringSubmatch(strings.TrimSpace(lines[j]))
				if q == nil {
					break
				}
				quoted = append(quoted, q[1])
			}
			blocks = append(blocks, adfNode{Type: "blockquote", Content: parseMarkdownBlocks(strings.Join(quoted, "\n"))})
			i = j - 1
			continue
		}
		if strings.HasPrefix(trimmed, "|") && i+1 < len(lines) && mdTableSep.MatchString(lines[i+1]) {
			flushParagraph()
			table, next := parseMarkdownTable(lines, i)
			blocks = append(blocks, table)
			i = next - 1
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	flushParagraph()
	return blocks
}

// inlineLines joins the lines of one paragraph with hard breaks: a report
// written for GitHub relies on single newlines being visible.
func inlineLines(lines []string) []adfNode {
	var out []adfNode
	for i, line := range lines {
		if i > 0 {
			out = append(out, adfNode{Type: "hardBreak"})
		}
		out = append(out, parseInline(line)...)
	}
	return out
}

func listIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// parseMarkdownList reads consecutive list lines at the given indentation and
// nests the deeper ones under the item above. It returns the list and the
// index of the first line that is not part of it.
func parseMarkdownList(lines []string, start, indent int) (adfNode, int) {
	ordered := mdOrdered.MatchString(lines[start]) && !mdBullet.MatchString(lines[start])
	list := adfNode{Type: "bulletList"}
	if ordered {
		list.Type = "orderedList"
	}
	i := start
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// A blank line ends the list unless another item of the same list follows.
			if i+1 < len(lines) && (mdBullet.MatchString(lines[i+1]) || mdOrdered.MatchString(lines[i+1])) && listIndent(lines[i+1]) >= indent {
				i++
				continue
			}
			break
		}
		isBullet := mdBullet.MatchString(line)
		isOrdered := mdOrdered.MatchString(line) && !isBullet
		if !isBullet && !isOrdered {
			break
		}
		lineIndent := listIndent(line)
		if lineIndent < indent {
			break
		}
		if lineIndent > indent {
			// Deeper item: nest under the previous item.
			nested, next := parseMarkdownList(lines, i, lineIndent)
			if len(list.Content) == 0 {
				list.Content = append(list.Content, adfNode{Type: "listItem"})
			}
			last := &list.Content[len(list.Content)-1]
			last.Content = append(last.Content, nested)
			i = next
			continue
		}
		if isOrdered != ordered {
			break
		}
		var text string
		if isBullet {
			text = mdBullet.FindStringSubmatch(line)[2]
		} else {
			text = mdOrdered.FindStringSubmatch(line)[2]
		}
		list.Content = append(list.Content, adfNode{Type: "listItem", Content: []adfNode{{Type: "paragraph", Content: parseInline(strings.TrimSpace(text))}}})
		i++
	}
	return list, i
}

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func parseMarkdownTable(lines []string, start int) (adfNode, int) {
	table := adfNode{Type: "table", Attrs: map[string]any{"isNumberColumnEnabled": false, "layout": "default"}}
	header := splitTableRow(lines[start])
	row := adfNode{Type: "tableRow"}
	for _, cell := range header {
		row.Content = append(row.Content, adfNode{Type: "tableHeader", Content: []adfNode{{Type: "paragraph", Content: parseInline(cell)}}})
	}
	table.Content = append(table.Content, row)
	i := start + 2
	for ; i < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
			break
		}
		row := adfNode{Type: "tableRow"}
		for _, cell := range splitTableRow(lines[i]) {
			row.Content = append(row.Content, adfNode{Type: "tableCell", Content: []adfNode{{Type: "paragraph", Content: parseInline(cell)}}})
		}
		table.Content = append(table.Content, row)
	}
	return table, i
}

// parseInline turns one line of Markdown into text nodes with marks. It handles
// `code`, **bold**, *italic* / _italic_, ~~strike~~ and [text](url); everything
// else stays literal.
func parseInline(s string) []adfNode {
	var out []adfNode
	emit := func(text string, marks []adfMark) {
		if text == "" {
			return
		}
		node := adfNode{Type: "text", Text: text}
		if len(marks) > 0 {
			node.Marks = append([]adfMark{}, marks...)
		}
		out = append(out, node)
	}
	var parse func(s string, marks []adfMark)
	parse = func(s string, marks []adfMark) {
		var buf strings.Builder
		i := 0
		for i < len(s) {
			// Inline code wins over every other mark and takes no nesting.
			if s[i] == '`' {
				if end := strings.IndexByte(s[i+1:], '`'); end >= 0 {
					emit(buf.String(), marks)
					buf.Reset()
					emit(s[i+1:i+1+end], append(append([]adfMark{}, marks...), adfMark{Type: "code"}))
					i += end + 2
					continue
				}
			}
			if s[i] == '[' {
				if close := strings.IndexByte(s[i:], ']'); close > 0 && i+close+1 < len(s) && s[i+close+1] == '(' {
					if end := strings.IndexByte(s[i+close+1:], ')'); end > 0 {
						text := s[i+1 : i+close]
						href := s[i+close+2 : i+close+1+end]
						emit(buf.String(), marks)
						buf.Reset()
						parse(text, append(append([]adfMark{}, marks...), adfMark{Type: "link", Attrs: map[string]any{"href": href}}))
						i += close + 1 + end + 1
						continue
					}
				}
			}
			for _, delim := range []struct {
				token string
				mark  string
			}{{"**", "strong"}, {"~~", "strike"}, {"*", "em"}, {"_", "em"}} {
				if strings.HasPrefix(s[i:], delim.token) {
					rest := s[i+len(delim.token):]
					end := strings.Index(rest, delim.token)
					// An underscore inside a word is a word character, not emphasis.
					if delim.token == "_" && i > 0 && isWordByte(s[i-1]) {
						end = -1
					}
					if end > 0 {
						emit(buf.String(), marks)
						buf.Reset()
						parse(rest[:end], append(append([]adfMark{}, marks...), adfMark{Type: delim.mark}))
						i += len(delim.token)*2 + end
						goto next
					}
				}
			}
			buf.WriteByte(s[i])
			i++
		next:
		}
		emit(buf.String(), marks)
	}
	parse(s, nil)
	return out
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// renderADFBlocks renders block nodes; prefix is what each line of a nested
// list gets in front of it.
func renderADFBlocks(nodes []adfNode, prefix string) string {
	var b strings.Builder
	for _, node := range nodes {
		switch node.Type {
		case "paragraph":
			b.WriteString(prefix + renderADFInline(node.Content) + "\n\n")
		case "heading":
			level := 1
			if lv, ok := node.Attrs["level"].(float64); ok && lv >= 1 && lv <= 6 {
				level = int(lv)
			}
			b.WriteString(strings.Repeat("#", level) + " " + renderADFInline(node.Content) + "\n\n")
		case "bulletList", "orderedList":
			b.WriteString(renderADFList(node, prefix))
			b.WriteString("\n")
		case "codeBlock":
			lang, _ := node.Attrs["language"].(string)
			b.WriteString("```" + lang + "\n" + adfText(node.Content) + "\n```\n\n")
		case "blockquote":
			inner := strings.TrimRight(renderADFBlocks(node.Content, ""), "\n")
			for _, line := range strings.Split(inner, "\n") {
				b.WriteString("> " + line + "\n")
			}
			b.WriteString("\n")
		case "rule":
			b.WriteString("---\n\n")
		case "table":
			b.WriteString(renderADFTable(node))
		case "mediaSingle", "mediaGroup", "media":
			// Attachments have no Markdown counterpart the reader could open.
			if text := adfText(node.Content); text != "" {
				b.WriteString(text + "\n\n")
			}
		default:
			// Panels, expands, layouts and whatever comes next: keep the words.
			if len(node.Content) > 0 {
				b.WriteString(renderADFBlocks(node.Content, prefix))
			} else if text := renderADFInline([]adfNode{node}); strings.TrimSpace(text) != "" {
				b.WriteString(prefix + text + "\n\n")
			}
		}
	}
	return b.String()
}

func renderADFList(list adfNode, prefix string) string {
	var b strings.Builder
	for i, item := range list.Content {
		marker := "- "
		if list.Type == "orderedList" {
			marker = strconv.Itoa(i+1) + ". "
		}
		first := true
		for _, child := range item.Content {
			switch child.Type {
			case "bulletList", "orderedList":
				b.WriteString(renderADFList(child, prefix+"  "))
			default:
				text := strings.TrimRight(renderADFBlocks([]adfNode{child}, ""), "\n")
				if first {
					b.WriteString(prefix + marker + text + "\n")
					first = false
				} else {
					b.WriteString(prefix + "  " + text + "\n")
				}
			}
		}
		if first {
			b.WriteString(prefix + marker + "\n")
		}
	}
	return b.String()
}

func renderADFTable(table adfNode) string {
	var b strings.Builder
	for i, row := range table.Content {
		var cells []string
		for _, cell := range row.Content {
			cells = append(cells, strings.TrimSpace(strings.ReplaceAll(strings.TrimRight(renderADFBlocks(cell.Content, ""), "\n"), "\n", " ")))
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if i == 0 {
			seps := make([]string, len(cells))
			for j := range seps {
				seps[j] = "---"
			}
			b.WriteString("| " + strings.Join(seps, " | ") + " |\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

// renderADFInline renders inline nodes with their marks.
func renderADFInline(nodes []adfNode) string {
	var b strings.Builder
	for _, node := range nodes {
		switch node.Type {
		case "text":
			b.WriteString(applyADFMarks(node.Text, node.Marks))
		case "hardBreak":
			b.WriteString("\n")
		case "mention":
			text, _ := node.Attrs["text"].(string)
			if text == "" {
				text, _ = node.Attrs["id"].(string)
			}
			if !strings.HasPrefix(text, "@") {
				text = "@" + text
			}
			b.WriteString(text)
		case "emoji":
			if text, _ := node.Attrs["text"].(string); text != "" {
				b.WriteString(text)
			} else if short, _ := node.Attrs["shortName"].(string); short != "" {
				b.WriteString(short)
			}
		case "inlineCard":
			if href, _ := node.Attrs["url"].(string); href != "" {
				b.WriteString(href)
			}
		case "status":
			if text, _ := node.Attrs["text"].(string); text != "" {
				b.WriteString("[" + text + "]")
			}
		case "date":
			if ts, _ := node.Attrs["timestamp"].(string); ts != "" {
				b.WriteString(ts)
			}
		default:
			if len(node.Content) > 0 {
				b.WriteString(renderADFInline(node.Content))
			} else if node.Text != "" {
				b.WriteString(node.Text)
			}
		}
	}
	return b.String()
}

func applyADFMarks(text string, marks []adfMark) string {
	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "link":
			if href, _ := mark.Attrs["href"].(string); href != "" && href != text {
				text = "[" + text + "](" + href + ")"
			}
		}
	}
	return text
}

// adfText collects the text of a subtree, for nodes rendered verbatim.
func adfText(nodes []adfNode) string {
	var b strings.Builder
	for _, node := range nodes {
		if node.Type == "hardBreak" {
			b.WriteString("\n")
		}
		b.WriteString(node.Text)
		b.WriteString(adfText(node.Content))
	}
	return b.String()
}
