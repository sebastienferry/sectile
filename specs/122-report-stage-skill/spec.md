# #122 — Introduce a skill to operate transition and reporting

Ticket: https://github.com/sebastienferry/sectile/issues/122
Description: "and remove this part from other skills."
Branch: `feat/122`.
Clarification: [`docs/clarifications/122.md`](../../docs/clarifications/122.md).

## Context

Every task-scoped generated skill (clarify, specify, implement, adjust, handoff,
create_pr, pickup, pickup_issues, rewrite_story) renders two long blocks that say
how to talk to Sectile: "Sectile task access" at the top and "Execution and
ticket state" at the bottom. At source level they are already one fragment each,
but in the rendered SKILL.md and slash-command files they are repeated in every
skill, and the per-skill gate (when this skill may transition) is buried inside
the generic mechanics.

This ticket introduces one dedicated skill, `report-stage` (`/report-stage`),
that owns those mechanics — task access, the run indicator (`start_run` /
`finish_run`), stage transitions (`transition_stage`, `prUrl`), ticket comments
(`add_comment`) and the managed-run result-contract rule — and replaces both
blocks in the other skills with a pointer to it, a short safety invariant and the
skill's own gate.

Out of scope: the MCP tools and their server-side validation, the managed-run
result-file mechanism, the per-stage `## Report` bullets, the "Project pull
request policy" appended per project, and the macro-scoped `refine_macro` skill.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled in the clarification (Rounds 1 and 2); none reopened here.

1. **Scope of extraction (Q1).** Both the task-access block and the execution
   and ticket-state block move into `report-stage`. The session-title block stays
   inline in every skill.
2. **Pointer plus invariant (Q2).** Each task-scoped skill keeps a pointer to
   `/report-stage` that names its file in the agent's own skill directory, a 2–3
   line invariant, and its own one-line gate.
3. **Exposure (Q3).** `report-stage` is provisioned like any other skill (the provider's
   user-level skills directory, e.g. `~/.claude/skills`) and listed in the skill editor,
   but it is not a board launch action.
4. **D1.** The `## Report` bullets stay in each skill.
5. **D2.** The gate stays in each skill; the helper is generic and takes the
   stage, note, branch and optional `prUrl` as inputs.
6. **D4.** `refine_macro` is unchanged.

---

## User stories

### US1 — An agent running a stage skill finds the reporting rules in one place (P1)

As an agent running a task-scoped skill, I want the skill to tell me where the
reporting rules live and what my own gate is, so that I read the generic
mechanics once and do not confuse them with my stage's condition.

**Acceptance**

- **Given** any rendered task-scoped skill other than `report-stage`, in either
  SDD framework and in both SKILL.md and slash-command form,
  **when** I read it,
  **then** it contains a `## Sectile reporting` section that names `/report-stage`
  and `report-stage/SKILL.md`, and it no longer contains a `## Sectile task access`
  or `## Execution and ticket state` section, nor `localhost:8090`.
- **Given** the same document, **when** I read its `## Sectile reporting` section,
  **then** it states: call `start_run` before work and `finish_run` when the whole
  invocation ends, reusing a supplied runId; a nested skill never finishes the
  outer run; a managed run submits only through its result contract; and the
  skill's gate.
- **Given** pickup or pickup_issues, **when** I count the reporting sections,
  **then** there is exactly one `## Sectile reporting` section, not one per
  embedded stage.

### US2 — The gate stays with the stage (P1)

As a reviewer of the generated skills, I want each skill's transition condition
to stay visible in that skill, so that moving the mechanics out does not loosen
any stage.

**Acceptance**

- **Given** clarify, **then** its gate says to transition new → clarified only when
  the exit condition is met and contains "Never transition new → clarified while".
- **Given** specify, implement or handoff, **then** its gate names
  `transition_stage`, its target stage and the actual branch, and says to call it
  only when this step is complete.
- **Given** adjust, **then** its gate says to record reviewed with `prUrl` set to the
  verified pull request URL.
- **Given** pickup or pickup_issues, **then** its gate says to record clarified,
  specified and implemented after each corresponding step and reviewed with the PR
  URL after PR verification, and for a batch to use the same branch and combined PR
  URL and never mark unfinished work reviewed.
- **Given** create_pr or rewrite_story, **then** its gate says it does not change the
  workflow stage, and the document does not contain `transition_stage`.

### US3 — The helper carries everything generic (P1)

As an agent that loads `/report-stage`, I want every rule that used to live in the
two removed blocks, so that nothing was lost in the move.

**Acceptance**

- **Given** the rendered `report-stage` skill, **when** I read it, **then** it
  contains the task-access rules (local agent interface first, full task ID and
  external URL, the `http://localhost:8090` fallback and bug reporting, no direct
  database or tracker writes, verify each mutation) and the execution rules
  (managed run vs standalone; `start_run` / `finish_run` lifecycle with runId reuse,
  nested reuse, per-task tracking in a batch, never starting a run merely to read;
  `transition_stage` with task key, completed stage, structured note and actual
  branch, checking its result; the ordered set of pull requests behind `prUrl`;
  `add_comment`; MCP unavailable → preserve work and report the pending transition;
  reuse the worktree, never merge or delete remote objects).
- **Given** it, **then** it states its inputs — task key, stage, note, branch,
  optional `prUrl`, optional comment — and that the gate belongs to the calling skill.
- **Given** it, **then** it has no stage line, no session-title section and no
  pointer to itself.

### US4 — A human can record a stage by hand, but not from the board (P2)

As an operator, I want `/report-stage` available as a slash command and visible in
the skill editor, but not offered as a board launch action, so that it does not
look like a workflow step.

**Acceptance**

- **Given** a project provisioned by the agent, **then** `report-stage/SKILL.md`
  is installed in the provider's skills directory like any other skill
  (the agent configuration carries it with directory `report-stage`).
- **Given** `GET /api/skills` (the board and command palette catalogue), **then**
  `report_stage` is absent, and launching it as a job fails with "skill not found".
- **Given** the project skill editor, **then** `report_stage` is listed with its
  default content and can be customised like the other skills.
- **Given** `get_project_context`, **then** its `skills` list includes `report_stage`
  with directory `report-stage` and command `/report-stage`.

### US5 — The macro skill is untouched (P3)

**Acceptance**

- **Given** `refine_macro` in both frameworks, **then** its rendered SKILL.md and
  slash command are byte-identical to before this change.

---

## Functional requirements

- **FR1** — The catalogue gains one entry: id `report_stage`, directory and command
  `report-stage` / `/report-stage`, no from/to stage, task scope, hidden from the
  board. The `report-stage` alias resolves to it.
- **FR2** — Every task-scoped skill except `report_stage` renders, right after the
  session-title section, a `## Sectile reporting` section built from one templated
  fragment: pointer, invariant, gate. The task-access and execution blocks are no
  longer rendered in them.
- **FR3** — `report_stage` renders the task-access rules and the generic execution
  rules, which no longer carry per-skill branches.
- **FR4** — `refine_macro` keeps rendering the task-access block as today and gets no
  reporting pointer.
- **FR5** — The board catalogue excludes skills hidden from the board; the skill
  editor, provisioning and `get_project_context` do not.
- **FR6** — Golden files are regenerated and new tests pin FR1–FR5.

## Open requirements

None. Every product question was answered in Round 2 of the clarification.
