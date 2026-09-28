# Specification #580 - Open the draft pull request at clarification

- Ticket: https://github.com/sebastienferry/sectile/issues/580
- Branch: `feat/580`
- Clarification: `docs/clarifications/580.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

The project setting "PR creation stage" says which workflow stage publishes
the task branch and opens its draft pull request. It offers two values today,
after implementation (default) and after specification. This change adds a
third, earlier value: after clarification. With it, the clarification stage
pushes the task branch and opens the draft pull request in its final round,
once the owner has confirmed the clarification; the later stages update that
same pull request, and it becomes ready only after adjustment, as today.

## Scope

In scope: the project setting and its validation, the clarification stage's
instructions and transition check, the stages after it (specify, implement,
adjust, pickup), the web project options, the "Create PR" recovery step on
the web board and the desktop app, and the documentation.

Out of scope:

- The six workflow stages and their order.
- Making the pull request ready before adjustment.
- Opening pull requests in context repositories at clarification.
- The default value (after implementation) and projects that use "after
  specification": their behaviour does not change.
- Intermediate clarification rounds: they open no pull request.
- The macro workflow.

## Definitions

- **Creation stage**: the value of the project setting, one of `clarified`,
  `specified`, `implemented` (default).
- **Creation owner**: the stage skill that opens the draft pull request:
  `clarify` for `clarified`, `specify` for `specified`, `implement` for
  `implemented`.
- **Final clarification round**: the round in which the owner confirms the
  clarification (interactive) or in which no product question remains open
  (unattended), and which ends with the `new -> clarified` transition.
- **Dropped artefacts**: the workstation setting of #487 under which the
  clarification report and the specification files are ignored by Git.

## User stories (prioritised)

### US1 - A project opens its pull request at clarification (P1)

As a project owner, I choose "Draft after clarification" in the web project
options, so that every task has a draft pull request as soon as its
clarification is confirmed, and reviewers can follow the specification and the
code in one place from the start.

Acceptance:

1. **Given** a project whose creation stage is `implemented`, **when** its
   owner selects "Draft after clarification" and saves, **then** the project
   reads back with creation stage `clarified`, in the web options and in the
   project context an agent reads.
2. **Given** a project whose creation stage is `clarified`, **when** the
   clarification of a task ends with a confirmed final round, **then** the task
   branch is pushed, a draft pull request exists for it, and the task reaches
   `clarified` with that pull request recorded as its current one.
3. **Given** the same project, **when** a clarification round stops with open
   product questions, **then** no pull request is opened and the task stays at
   `new`.
4. **Given** a draft pull request already open on the task branch, **when**
   the final round runs, **then** that pull request is reused and no second one
   is created.

### US2 - The server holds the clarification stage to its pull request (P1)

As a project owner, I rely on Sectile refusing a clarification that claims to
be done without the pull request the project asked for, as it already does at
specification.

Acceptance:

1. **Given** a project whose creation stage is `clarified` and a task whose
   branch has no pull request, **when** a `clarified` transition arrives
   (standalone or from a managed run) without a verifiable pull request,
   **then** it is refused with the same kind of message as a `specified`
   transition without its pull request, and the task stays at `new`.
2. **Given** the same project, **when** the transition names a pull request
   that is open on the task branch, **then** it is accepted and the pull
   request is recorded.
3. **Given** a project whose creation stage is `specified` or `implemented`,
   **when** a `clarified` transition arrives without a pull request, **then**
   it is accepted as today.

### US3 - Later stages keep the same pull request (P1)

As a reviewer, I see one pull request per task, opened at clarification and
updated by every later stage.

Acceptance:

1. **Given** a project whose creation stage is `clarified` and a task at
   `clarified` with a draft pull request, **when** the specification runs,
   **then** it pushes its commits to the same branch, the `specified`
   transition requires and records that same pull request, and it stays draft.
2. **Given** the same task, **when** implementation and adjustment run,
   **then** they behave as under `specified` today: implementation updates the
   draft, adjustment makes it ready.

### US4 - Dropped artefacts defer the pull request (P2)

As a developer whose workstation drops the specification artefacts, I can
still finish a clarification: there is nothing to push, so the pull request is
deferred rather than blocking the stage.

Acceptance:

1. **Given** a project whose creation stage is `clarified` and a workstation
   that drops the task's artefacts, **when** the final round transitions to
   `clarified` without a pull request, **then** the transition is accepted and
   its note says the pull request is deferred.
2. **Given** that deferral, **when** the specification runs, **then** it
   opens no pull request either (its artefacts are dropped too) and the
   `specified` transition is accepted without one, with the same notice.
3. **Given** that deferral, **when** implementation runs, **then** it opens
   the draft pull request, and the `implemented` transition requires it, as
   today.

### US5 - Recovering a missing pull request (P2)

As a developer, when a task reached `implemented` without a pull request, the
"Create PR" step recovers it without re-running a clarification dialogue.

Acceptance:

1. **Given** a project whose creation stage is `clarified` and a task at
   `implemented` without a pull request, **when** the board or the desktop app
   offers the next step, **then** it is "Create PR" through the implementation
   skill.
2. **Given** a project whose creation stage is `specified`, **then** the
   recovery still goes through the specification skill, as today.

## Functional requirements

- **FR1** The creation stage accepts exactly `clarified`, `specified` and
  `implemented`; an empty value on creation means `implemented`. Any other
  value is refused on project creation and update, and the stored value is
  left unchanged.
- **FR2** The web project options offer the three values, in workflow order:
  "Brouillon après la clarification" / "Draft after clarification", then after
  specification, then after implementation. The help text of the setting stays
  true for the three.
- **FR3** The project context read by agents and the effective skill contents
  carry the new value.
- **FR4** With `clarified`, the clarification skill's instructions say: in the
  final round only, after the report commit, push the task branch
  (`git push -u origin <branch>`, never force), discover and reuse its pull
  request or create a draft on confirmed absence, and report its URL in the
  `clarified` transition (standalone) or in the result (managed run). Lookup
  failure is not absence. Intermediate rounds open no pull request. When the
  report is ignored by Git, open no pull request and say so.
- **FR5** With `clarified`, a `clarified` transition requires pull request
  evidence on the task branch, verified the same way as a `specified`
  transition under `specified`. This holds for the standalone transition and
  for the post-back of a managed run.
- **FR6** With `clarified`, a `specified` transition requires the same pull
  request evidence as under `specified` (the draft is updated, not created).
- **FR7** With `clarified`, when the reporting workstation drops the task's
  artefacts, the `clarified` and `specified` transitions are accepted without a
  pull request, with a notice saying it is deferred to the implemented stage.
- **FR8** With `clarified`, "Create PR" recovery at `implemented` goes through
  the implementation skill, on the web board and in the desktop app.
- **FR9** With `specified` or `implemented`, nothing changes: no pull request
  is required at `clarified`, the clarification skill gets no pull request
  instruction, and recovery is unchanged.
- **FR10** Only the task's primary repository gets a pull request at
  clarification; secondary repositories follow the existing rules of the
  stages that change them.
- **FR11** An agent that receives a creation stage it does not know does not
  reject the whole project configuration: it treats the value as
  `implemented`. (An agent built before this change still rejects
  `clarified`; the changelog says to update the desktop app before choosing
  it.)
- **FR12** `CHANGELOG.md` gets an `Added` line under `[Unreleased]` naming
  the new option and the desktop update it needs; the server-agent contract
  and ADR 0004 describe the three values.

## Edge cases

- A task already at `clarified` when its project switches to `clarified`: no
  retroactive check; its specification requires and opens the pull request as
  the creation owner's successor would. (FR6 applies: specify requires one.)
  -> see open point OP1.
- A project switching from `clarified` back to `implemented`: tasks keep
  their recorded pull requests; later stages reuse them, as a ready or draft
  PR is preserved today.
- A clarification whose owner confirms in round 1 (no question asked): round
  1 is the final round and opens the pull request.
- A merged or closed pull request on the branch at the final round: handled
  by the existing evidence rules (a follow-up PR on the same branch is
  appended).

## Open points

- **OP1 - A task at `clarified` without a pull request when the project is
  set to `clarified`.** Under FR6 its specification must open the draft, which
  the specification instructions must then say explicitly (create on
  confirmed absence, not only reuse). The plan takes that reading; it follows
  from the existing "creation-owner recovery" rule and needs no product
  decision, but it is recorded here because the clarification did not discuss
  it.

No product question remains open.
