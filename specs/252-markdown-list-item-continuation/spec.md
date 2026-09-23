# #252 — A Markdown list item continued on the next line is split into three blocks in Jira

Ticket: https://github.com/sebastienferry/sectile/issues/252
Type: Bug — rendering defect of the Markdown → ADF converter used for Jira.
Branch: `feat/252`.
Clarification: [`docs/clarifications/252.md`](../../docs/clarifications/252.md).

## Context

Sectile writes Markdown (reports, comments, rewritten stories) and converts it to
Atlassian Document Format before sending it to Jira. A list item whose text goes on
past its marker line is not recognised as one item:

```
- a

  continued
- b
```

becomes a `bulletList` with `a`, a top-level paragraph `continued`, then a second
`bulletList` with `b`. Jira accepts it, but the reader sees a broken list where the
author wrote one item with two paragraphs. The same split happens without the blank
line (`- a\n  continued`) and for a fenced code block indented under an item.

The reverse converter, used every time a Jira issue or comment is read back, writes
a second paragraph of an item on the very next line and indents only the first line
of a multi-line child, so a multi-paragraph item written in Jira does not survive a
Jira → Sectile → Jira round trip.

Out of scope: the GitHub tracker (it sends raw Markdown), the inline parser, tables,
headings, quotes and rules outside lists, and any change to how Jira validates a
payload.

This file states behaviour and acceptance criteria only. Implementation choices are
in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Decisions being specified

The clarification's three product questions were answered by the owner in Round 2
("1 yes 2 yes 3 yes"), each in line with the Round 1 recommendation:

1. **D1 — Fenced code blocks inside an item.** A fenced code block indented to the
   item's content column is part of that item.
2. **D2 — Lazy continuation.** A plain-text line directly under an item's text (no
   blank line in between) joins that text as a new line, whatever its indent, as
   GitHub renders it.
3. **D3 — Round trip.** Markdown produced from a Jira list keeps each item's
   paragraphs and code blocks as separate blocks of the same item.

Technical choices settled in the clarification and applied here:

- **D4 — Content column.** An item's content column is where its text starts after
  the marker (2 for `- a`, 3 for `1. a`, 4 for `10. a`, plus the marker's own indent).
  After a blank line, only a line indented to that column belongs to the item.
- **D5 — What an item may hold.** Paragraphs, fenced code blocks and nested lists.
  A heading, quote, table or rule ends the list, indented or not, because ADF does
  not allow them inside a `listItem`.
- **D6 — Invariant from #180.** Every `listItem` still starts with a `paragraph`.

No requirement is left open.

## User stories

### US1 (P1) — A multi-paragraph item stays one item in Jira

As the author of a report or comment sent to Jira, I want an item whose text
continues after a blank line to stay one item, so the reader sees the list I wrote.

**Acceptance**

- **Given** `- a\n\n  continued\n- b`,
  **when** it is converted to ADF,
  **then** the document holds exactly one `bulletList` of two `listItem`s; the first
  holds two `paragraph`s (`a`, `continued`), the second one `paragraph` (`b`).
- **Given** `1. a\n\n   continued\n2. b`,
  **then** the same shape holds with an `orderedList`.
- **Given** `- a\n\ncontinued` (continuation not indented to the content column),
  **then** the list ends: a `bulletList` with `a`, then a top-level `paragraph`
  `continued` — as today.
- **Given** `- a\n\n\n  continued` (several blank lines),
  **then** `continued` still belongs to the item.

### US2 (P1) — A line continued without a blank line joins the item's text

**Acceptance**

- **Given** `- a\n  continued\n- b` or `- a\ncontinued\n- b`,
  **then** there is one `bulletList`; the first item's single `paragraph` is `a`,
  a `hardBreak`, then `continued`.
- **Given** `- a\n  - x\n  more`,
  **then** `more` joins the nested item `x`, the innermost open text.
- **Given** a line that opens another block right under an item — a heading
  (`# h`), a quote (`> q`), a rule (`---`) or a table header followed by its
  separator — indented or not,
  **then** it is not joined: the list ends and the block is emitted after it, as
  today.

### US3 (P2) — A code block indented under an item stays in the item

**Acceptance**

- **Given** `- step\n\n  ```sh\n  cmd\n  ```\n- next`,
  **then** there is one `bulletList`; the first item holds a `paragraph` `step` then
  a `codeBlock` with language `sh` and text `cmd` (the item's indent removed), the
  second item a `paragraph` `next`.
- **Given** the same fence right under the item's text with no blank line and
  indented to the content column,
  **then** the result is the same.
- **Given** a fence that is not indented to the content column,
  **then** the list ends and the code block is top-level, as today.
- **Given** a text line indented to the content column right after the item's code
  block, **then** it is a new `paragraph` of the item; a less indented one ends the
  list.

### US4 (P2) — Nesting is unchanged

**Acceptance**

- **Given** `- a\n  - x\n\n  para\n- b`,
  **then** `para` is a second `paragraph` of `a` (column 2), not of `x` (column 4);
  `x` stays nested under `a`; `b` is the second item of the outer list.
- **Given** the existing cases (`- one\n- two\n  - nested\n- three`,
  `Intro:\n  - x\n  - y`), **then** they produce the same ADF as before.
- **Given** any input, **then** no `listItem` starts with anything but a
  `paragraph` (D6).

### US5 (P2) — A Jira list comes back as Markdown that converts to the same list

As a user editing a Jira task in Sectile, I want an item holding several paragraphs
or a code block to keep them when the description is saved back.

**Acceptance**

- **Given** an ADF `listItem` holding a `paragraph`, a second `paragraph` and a
  `codeBlock`, **when** it is rendered to Markdown, **then** each child after the
  first is preceded by a blank line and every one of its lines is indented to the
  item's content column (the marker's width: 2 for `- `, 3 for `1. `); the
  paragraph's own lines split by a `hardBreak` are indented the same way.
- **Given** that Markdown, **when** it is converted back to ADF, **then** the
  structure is the same as the ADF it came from.
- **Given** a nested list inside an ordered item, **then** it is indented to the
  ordered item's content column.
- **Given** the Markdown in US1, US3 and US4, **then** ADF → Markdown gives it back
  verbatim; for US2 it gives the indented form (`- a\n  continued\n- b`).

## Functional requirements

- **FR1** After a blank line (or several), a list goes on when the next non-blank
  line is either a list marker indented at least as far as the list, or a
  non-marker line indented at least to the current item's content column.
  Otherwise the list ends at the blank line.
- **FR2** A plain-text continuation after a blank line opens a new `paragraph` in
  the current item.
- **FR3** A plain-text line with no blank line before it joins the item's open
  paragraph with a `hardBreak`, whatever its indent (D2). When the item has no open
  paragraph (its last block is a code block or a nested list), the line opens a new
  paragraph if indented to the content column, and ends the list otherwise.
- **FR4** A fence indented to the content column, with or without a blank line
  before it, opens a `codeBlock` child of the item that runs to the closing fence;
  each code line loses up to the content column's worth of leading indent.
- **FR5** A heading, quote, rule or table start never belongs to an item and never
  joins a paragraph: the list ends before it (D5).
- **FR6** List markers keep today's rules: a deeper marker nests under the current
  item, a shallower one ends the list, a marker of the other list kind at the same
  indent ends the list.
- **FR7** The ADF → Markdown renderer separates an item's children after the first
  (other than nested lists) with a blank line, indents all their lines to the
  item's content column, and indents nested lists to that column too.
- **FR8** Every `listItem` produced starts with a `paragraph` (D6).

## Success criteria

- The ticket's example produces one list in Jira.
- Every acceptance above is covered by a test in `internal/trackerapi/adf_test.go`.
- The existing `adf_test.go` cases and the Jira tracker tests pass unchanged.
