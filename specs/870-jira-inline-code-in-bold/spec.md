# Spec #870 - Jira refuses a comment whose Markdown has inline code inside bold text

Ticket: https://github.com/sebastienferry/sectile/issues/870
Clarification: `docs/clarifications/870.md` (rounds 1 and 2, settled)
Branch: `feat/870`

## Problem

On a Jira tracker, a comment, a stage report or an issue description whose
Markdown holds inline code inside bold, italic or struck-through text is
refused with `HTTP 400 INVALID_INPUT`: the converter emits a text node that
carries the `code` mark next to `strong`, `em` or `strike`, which the ADF
schema forbids. When that text is the note of `transition_stage`, the stage is
applied but the report never reaches the ticket, and the agent that recorded
the stage is told the call succeeded.

## Scope

In:

- The Markdown-to-ADF converter, for every Jira write it feeds (issue
  creation, description update, comment creation and update).
- The result of the `transition_stage` MCP tool, and the shared skill
  contract that tells the agents how to read it.

Out:

- The list-continuation defect of the same converter (#252).
- Other ADF mark combinations Jira accepts today.
- Retrying a refused report automatically.
- GitHub and GitLab trackers, which receive Markdown.

## User stories

### US1 - Inline code inside emphasis reaches Jira (P1)

As someone whose agent writes to a Jira ticket, I want inline code inside bold,
italic or struck-through text to be accepted, so that no report is lost to a
formatting detail.

- **US1.1** Given the Markdown `**a `b` c**`, when it is converted to ADF, then
  `a ` and ` c` carry `strong` and `b` carries `code` only.
- **US1.2** Given `*a `b` c*`, `_a `b` c_` or `~~a `b` c~~`, when converted,
  then `b` carries `code` only and the surrounding text keeps its mark.
- **US1.3** Given `[a `b` c](https://x)`, when converted, then `b` carries
  `link` and `code`, and the link is kept on all three nodes.
- **US1.4** Given `**[a `b`](https://x)**`, when converted, then `b` carries
  `link` and `code` and nothing else.
- **US1.5** Given any Markdown document, when converted, then no text node
  carries `code` together with a mark other than `link`.

### US2 - The caller of `transition_stage` learns that its report was refused (P1)

As an agent recording a stage, I want to know whether the report reached the
tracker, so that I can post it again instead of claiming it was published.

- **US2.1** Given a tracker that accepts the report, when `transition_stage`
  returns within the wait, then `trackerSync` is `done`.
- **US2.2** Given a tracker that refuses the report, when `transition_stage`
  returns, then `trackerSync` is `failed`, `trackerError` carries the
  tracker's error, the stage is applied, and the call is not an MCP error.
- **US2.3** Given a tracker job still running when the wait ends, when
  `transition_stage` returns, then `trackerSync` is `queued`, as before.
- **US2.4** Given the `transition_stage` tool description, when an agent reads
  it, then it names the three values of `trackerSync`.
- **US2.5** Given the rendered workflow skills, when an agent reads their
  execution contract, then it is told to read `trackerSync` and, on `failed`,
  to report the error and post the note with `add_comment`.

## Functional requirements

- **FR1** A text node carrying `code` keeps only `code` and `link`.
- **FR2** `transition_stage` commits the stage before it waits; the wait never
  undoes it.
- **FR3** The wait is bounded (15 seconds) and ends early when the caller's
  request is canceled, in which case the result is `queued`.
- **FR4** `transition_stage` with `noRepositoryChange` follows the same rule.
- **FR5** `ADFToMarkdown` is unchanged.

## Changelog

User-visible: a `Fixed` line under `[Unreleased]` for the Jira refusal, and one
for the silent loss of the stage report.
