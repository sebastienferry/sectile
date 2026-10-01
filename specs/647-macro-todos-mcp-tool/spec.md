# Specification #647 - A macro skill writes the macro's todos through MCP

- Ticket: https://github.com/sebastienferry/sectile/issues/647
- Branch: `feat/batch-393-497-498-584-647-648`
- Clarification: `docs/clarifications/647.md` (rounds 1 and 2)
- Framework: Spec Kit

> **Superseded in part (2026-10-01).** #663 (PR #673) merged its own
> `get_macro` and `update_macro_todos` MCP tools into `main` while this batch
> was open. That full-list replace is the tool now, and this batch dropped its
> own implementation of US1 to US3 (append mode, omitted fields kept) in its
> favour. Still delivered here: US4, `prepare_macro_worktree` always returning
> `todos`. Not delivered: the refusal to drop a todo linked to a story
> (US2.2), which the owner chose in the clarification of #647 but which the
> merged tool does not do. It is left to the owner as a follow-up decision.

## Summary

A new MCP tool, `update_macro_todos`, writes a macro's slicing lines without
touching its shaping. It replaces the list or appends to it, keeps the lines
it is told to keep, and refuses to drop a line already linked to a story.
`prepare_macro_worktree` always returns the todos, even when there are none.

## Scope

In scope: the MCP tool, the stdio bridge catalog, the always-present `todos`
of `prepare_macro_worktree`, the `refine-macro` skill fragment, tests and the
changelog.

Out of scope: tracker writes (todos are Sectile's own), story creation (#634),
the marketplace `specify-macro` skill.

## User stories

### US1 (P1) - A skill deposits its slicing

1. Given a macro with no todos, when a session calls `update_macro_todos`
   with three lines and no ids, then the macro holds the three lines, each
   with a new id, and the answer returns them.
2. Given a macro with a description, when the tool writes todos, then the
   description, the framing, the horizon and the title are unchanged.

### US2 (P1) - Replace keeps what it is told to keep

1. Given a macro with lines L1 (story PE-1) and L2, when the tool replaces
   the list with `{id: L1, text: "new text"}` and a new line, then L1 keeps
   its id and its story with the new text, L2 is gone, and the new line has
   a new id.
2. Given a macro with line L1 linked to a story, when a replace omits L1,
   then the call is refused and the error names L1's id and text; the list is
   unchanged.
3. Given a line naming an id the macro does not hold, then the call is
   refused and nothing is written.

### US3 (P2) - Append adds lines

1. Given a macro with lines L1 and L2, when the tool appends two lines, then
   the macro holds L1, L2 and the two new lines, in that order.
2. Given an appended line that carries an id, then the call is refused.

### US4 (P2) - The read always says how many todos there are

1. Given a macro with no todos, when `prepare_macro_worktree` answers, then
   it carries `todos: []`.

## Functional requirements

- FR1. `update_macro_todos(projectId, macroKey, todos, mode)`; `mode` is
  `replace` (default) or `append`. `projectId` and `macroKey` are required.
- FR2. Each line has `text` (required, non-empty), and optional `id`, `done`,
  `storyKey`, `targetProjectId`, `targetTrackerProject`; an unset optional
  field of a kept line keeps its stored value.
- FR3. The write goes through the local macro save; nothing reaches the
  tracker.
- FR4. The caller must be identified.
- FR5. The stdio bridge accepts the new catalog of fourteen tools.

## Acceptance criteria

- AC1. US1 to US4 are covered by tests.
- AC2. The `refine-macro` skill names the tool for depositing a slicing the
  user confirmed.
- AC3. An `Added` line in `CHANGELOG.md`.

## Open points

- Whether PE-1078 really holds 22 stored lines: checked on the live server by
  the owner; the always-present `todos` makes the answer unambiguous either
  way.
