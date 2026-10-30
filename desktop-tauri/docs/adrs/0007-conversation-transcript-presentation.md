# ADR 0007: Present conversations with safe Markdown and grouped tools

- Status: Accepted
- Date: 2026-10-28

## Decision

Keep the React conversation controller and native Go contracts. Presentation is
handled by `ConversationTranscript` and `Markdown` components, independent of
execution state. Use Marked's GFM lexer and entities' pure text decoder, rendering
supported tokens as React elements. Do not insert generated HTML into the DOM.
Raw HTML stays text, remote image tokens become labels, and only explicit HTTP(S)
links can open through a narrow native command in the system browser. Code is
rendered literally with a copy button; no syntax-highlighting dependency is added.

A dedicated transcript viewport follows output while the reader is within 48px
of its bottom. User scroll-up suspends following; a return button or sending a
message resumes it. Layout and content resize are observed so streamed Markdown
and expanding cards do not leave following behind. Resize observers are removed
on unmount. The composer and approvals live outside the scrolling transcript.

Tool results are attached by tool identity, never by the nearest preceding card.
Consecutive calls share a collapsed group. Late results can update earlier cards
without hiding assistant messages. Orphan results remain separate; missing
results are never marked successful. Approval controls are not collapsed with
ordinary tool output.

## Validation

UI tests cover Markdown structure and code copying, inert HTML and images,
unsafe link schemes, partial fences, entity decoding, out-of-order tool results,
orphan results and automatic scroll suspension/resumption. Existing conversation
controller and approval tests continue to apply.

The packaged macOS desktop was checked with a real Claude reply containing a
heading, bold text, bullets, a 25-line Python fence and a table. The rendered
reply and completion marker were visible with the composer pinned below the
transcript. Scrolling up exposed the return-to-bottom button; clicking it returned
to the latest output. Existing real tool calls were restored as completed,
collapsed cards. Final checks passed: 35 UI tests, four domain tests, 12 Rust tests
and the desktop build. The lazy conversation module is about 82.5 kB (33.8 kB gzip).
