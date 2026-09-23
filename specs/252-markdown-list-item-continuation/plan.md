# #252 — Technical plan

Behaviour and acceptance criteria live in [`spec.md`](./spec.md). This file records the
implementation choices only.

## Stack and constraints

- Go, package `internal/trackerapi`, file `adf.go`; tests in `adf_test.go`
  (`go test ./internal/trackerapi/`).
- No new dependency, no migration, no route, no shared type. `MarkdownToADF` and
  `ADFToMarkdown` keep their signatures; their callers (`jira.go`, `jira_mapping.go`)
  are untouched.
- The converter stays a single-pass, bounded-subset parser: no CommonMark library.

## Shape of the change

```
adf.go
  parseMarkdownBlocks   [A] fence reading moved to fencedCodeBlock(lines, i, lang, strip)
  fencedCodeBlock       [B] new: shared by the top level (strip 0) and list items
  endsListItem          [C] new: heading / quote / rule / table start → the list ends
  dedent                [D] new: strip up to n columns of leading indent
  parseMarkdownList     [E] rewritten loop: content column, open paragraph, blank flag
  renderADFList         [F] blank line before later children, indent every line
adf_test.go             [G] new tests next to TestAnIndentedListIsNotNestedUnderAnEmptyBullet
```

## Decisions

### P1 — Item state in `parseMarkdownList`

The loop keeps, for the last item of the list:

- `content`, its content column: `len(line) - len(text)` where `text` is the marker
  regex's text group (the `\s+` after the marker is greedy, so the group starts at the
  first character of the text);
- `paragraph []string` and `open bool`, the lines of its open paragraph. The marker
  line opens it (`open` with an empty text too, so an empty item still gets its
  paragraph);
- `blank`, whether a blank line separates the line being read from the previous one.

`flush()` appends the open paragraph to the last item (`inlineLines(paragraph)`) and
closes it. It runs before anything else is appended to the item — a nested list, a
code block, a new paragraph — and before a new item or the end of the list, so the
marker-line paragraph is always the item's first child (FR8). The item node is
created with no content and filled by `flush`.

### P2 — Blank lines look ahead to the next non-blank line

A blank line skips to the next non-blank line and continues the list only if FR1
holds; otherwise the list ends at the first blank line (the caller already skips
blank lines). Looking past one blank line is what lets `- a\n\n\n  continued` work;
it also lets a sibling marker after two blank lines continue the list, which it did
not before — harmless, and what GitHub renders.

### P3 — Continuation rules, in order, for a non-blank non-marker line

1. `endsListItem(lines, i)` → break (FR5). It checks the trimmed line against
   `mdHeading`, `mdQuote`, `mdRule` and the table start used by `parseMarkdownBlocks`
   (a `|` line followed by `mdTableSep`).
2. A fence (`mdFence` on the trimmed line) indented to `content` → `flush`, then
   `fencedCodeBlock(lines, i, lang, content)` appended to the item (FR4). Not indented
   enough → break.
3. After a blank line: indented to `content` → new paragraph (FR2), else break (the
   look-ahead already guarantees the indent; the check stays for safety).
4. No blank line: open paragraph → append (FR3, lazy or indented); no open paragraph →
   new paragraph if indented to `content`, else break.

List markers keep their current branches (FR6); the nested branch calls `flush`
first and resets `open`, so a line after a nested list never joins the outer item's
first paragraph. A lazy line right under a nested item is consumed by the nested
call, which is the innermost open paragraph, as CommonMark does.

### P4 — `fencedCodeBlock`

Extracted from `parseMarkdownBlocks` unchanged except for `strip`: every code line
goes through `dedent(line, strip)`, which removes up to `strip` leading spaces or
tabs and nothing else. The top level passes 0, so its output is byte-identical to
today's. An unclosed fence still runs to the end of the input.

### P5 — `renderADFList`

- `pad := prefix + strings.Repeat(" ", len(marker))` is the item's content column.
- The first non-list child is written after the marker; its later lines (hard breaks,
  code lines) are indented with `pad`.
- Every later non-list child is preceded by a blank line and all its lines are
  indented with `pad`. Empty lines stay empty (no trailing whitespace).
- Nested lists are rendered with `pad` as their prefix instead of `prefix + "  "`.
  For bullets both are two spaces; for ordered items the nested list now sits at the
  content column, which the parser nests either way and GitHub reads as nested.
- No blank line precedes a nested list: the parser nests a deeper marker straight
  after a paragraph, and adding one would change today's output for
  `- two\n  - nested`.

A later child must be preceded by a blank line because a line right under a nested
list would otherwise be read as a lazy continuation of the nested item.

## Rejected alternatives

- **Collect the item's lines, de-indent them and call `parseMarkdownBlocks` on
  them.** Closer to CommonMark, but it would let headings, quotes and tables into a
  `listItem` (invalid ADF, D5) unless filtered after the fact, and it changes the
  indentation rules of nested lists (today a marker one column deeper nests).
- **Lazy continuation only for indented lines.** Rejected by the owner (D2).
- **Leave `renderADFList` alone.** Rejected by the owner (D3): a multi-paragraph item
  edited in Sectile would come back to Jira as one paragraph with hard breaks.

## Test plan

In `adf_test.go`, next to `TestAnIndentedListIsNotNestedUnderAnEmptyBullet`:

- a structural test for each acceptance of US1–US4, asserting node types and texts
  of the ADF tree (a small decoding helper shared by the new tests);
- round-trip cases added to `TestMarkdownToADFCoversTheReportSubset` for the ticket
  example, the ordered variant, the code block variant, the nested variant (verbatim)
  and the lazy variant (`back` = indented form);
- a US5 test: an ADF `listItem` with two paragraphs and a code block (plus an ordered
  list with a nested list) → `ADFToMarkdown` → `MarkdownToADF` gives the same
  structure;
- an invariant check that every `listItem` of every new case starts with a
  `paragraph`.

Then `go vet ./internal/trackerapi/` and `go test ./internal/trackerapi/`, and the full
`go test ./...` under WSL (the skills tests do not run on Windows).
