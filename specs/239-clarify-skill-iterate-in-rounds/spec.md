# #239 — Clarify skill: iterate in rounds until the owner confirms the clarification

## Context

While clarifying ticket #180, the `clarify-issue` skill executed as a single-pass process: it produced one report and immediately transitioned the task from `new` to `clarified`. When the owner answered the recorded open questions, widened the scope, and adjusted initial assumptions, handling those answers required an ad-hoc second pass without formal guidance. The second pass added a new run, appended an informal follow-up section in `docs/clarifications/<n>.md`, and posted a ticket comment, but revealed that the skill had no defined rule specifying when the `clarified` stage is actually achieved.

Clarification is inherently an iterative feedback loop between the agent and the work item's owner—analogous to how `adjust-issue` iterates on code review feedback during implementation. The clarification stage must not be considered complete after a single mechanical pass if product questions remain open or if the owner has not confirmed that the clarification is satisfactory.

This specification describes behaviour only. Technical choices and data contracts are detailed in `plan.md`, and the ordered implementation checklist is in `tasks.md`.

---

## Decisions being specified

1. **Iterative Round Loop**:
   - Clarification executes in distinct, numbered **rounds** (Round 1, Round 2, ..., Round N).
   - **Round 1**: Analyzes the ticket and affected code, identifies ambiguities, dependencies, and questions. Reversible choices are settled; essential product questions affecting acceptance criteria are asked. Produces `docs/clarifications/<n>.md` with stage header `Stage: new → clarified (Round 1)`, restated request, files read, ambiguities, dependencies, numbered questions with recommendations, and settled scope. Commits with `docs(spec): clarify #<n> (round 1)`.
   - **Round N**: Triggered when the owner provides answers. Reads existing report and ticket discussion via `get_task`. Appends a dated `## Round N - answers from the owner (<date>)` section. Documents settled answers, notes reversed or updated decisions explicitly, and addresses any newly surfaced ambiguities. Commits with `docs(spec): clarify #<n> (round N)`.
2. **Exit Condition**:
   - Clarification completes only when the exit condition is met: the owner confirms that the clarification is satisfactory (e.g. states "the clarification is satisfactory", or confirms all requirements and answers).
   - In unattended/autonomous pickup mode, if Round 1 identifies zero open product questions (all ambiguities resolved via existing code and conventions), Round 1 concludes clarification without waiting for an interactive prompt.
3. **Unambiguous Transition Rule**:
   - The task must **never** be transitioned `new → clarified` while any product question or decision remains open.
   - An unattended run that encounters open product questions records them in `docs/clarifications/<n>.md`, posts them to the ticket via `add_comment`, and terminates its run without calling `transition_stage`.
4. **Session & Activity Lifecycle**:
   - Each round is an independent Sectile execution reported via `start_run` and `finish_run`.
   - A round stopping to await owner answers finishes its run with status `completed` (recording that it is awaiting user answers) and does not call `transition_stage`.
   - When answers arrive, a new run starts for Round N, picking up context from the existing branch and `docs/clarifications/<n>.md`.
5. **Worktree & Branch Persistence**:
   - The assigned worktree and branch (`feat/<n>`) are strictly reused across all rounds.
   - Reports and skill outcomes are committed incrementally (`docs(spec): clarify #<n> (round <r>)`) and posted to the ticket discussion via `add_comment`.
6. **Synchronized Distribution**:
   - The master template in `internal/db/skilltemplates.go` (`StageSkills["clarify"]`), slash command generators, and all agent skill directory mirrors (`.claude`, `.agents`, `.gemini`, `.agy`, `.skills`) reflect identical round loop contracts.
7. **Documentation Updates**:
   - Core architecture documentation (`docs/CAPABILITIES.md` and `docs/README.md`) describes round-based clarification, the exit condition, and unattended pickup behavior.

---

## User stories

### US1 — Round 1 identifies ambiguities, records questions, and halts without transitioning if questions remain open (P1)

**As a** product owner filing a work item with open architectural or business choices,  
**I want** the clarification agent to analyze the problem, formulate specific questions, and stop without marking the task clarified,  
**So that** unvetted assumptions do not leak into specification and implementation.

- **Given** a new task in stage `new` with ambiguous requirements or open product decisions
- **When** the agent runs `/clarify-issue`
- **Then** it reads the ticket description, comments, and relevant codebase files.
- **And** it writes `docs/clarifications/<n>.md` with restated request, ambiguities, dependencies, numbered questions with recommended options, and settled scope.
- **And** it commits the report on the task branch with message `docs(spec): clarify #<n> (round 1)`.
- **And** if run interactively, it presents the questions to the owner; if run unattended, it posts the questions to the ticket via `add_comment`.
- **And** because product questions remain open, it does **not** call `transition_stage` to `clarified`.
- **And** it finishes the run with `finish_run` reporting that it is awaiting owner answers.
- **And** the task remains in stage `new`.

### US2 — Subsequent rounds incorporate owner answers until satisfaction is confirmed (P1)

**As a** product owner reviewing the clarification findings,  
**I want** to answer the recorded questions and have the clarification agent update the report in a dated follow-up section,  
**So that** my decisions are officially recorded and the ticket advances only when I am satisfied.

- **Given** an existing clarification report `docs/clarifications/<n>.md` on branch `feat/<n>` with open questions
- **When** the owner provides answers (in session or via ticket comments) and runs `/clarify-issue`
- **Then** the agent reads the existing report and latest ticket comments via `get_task`.
- **And** it appends a dated section `## Round N - answers from the owner (<date>)` to `docs/clarifications/<n>.md`.
- **And** it explicitly records which decisions are settled and which prior assumptions were reversed.
- **And** if new ambiguities or unresolved choices remain, it asks the follow-up questions, commits `docs(spec): clarify #<n> (round N)`, posts a discussion comment, and halts without transitioning.
- **And** when the owner confirms that the clarification is satisfactory and zero product questions remain open, it commits `docs(spec): clarify #<n> (round N)`.
- **And** it invokes `transition_stage` to move the task from `new` to `clarified`.
- **And** it logs a summary comment via `add_comment` and finishes the run as completed.

### US3 — Unattended pickup proceeds autonomously when zero questions are open, or pauses when questions arise (P1)

**As a** developer running autonomous batch or pickup execution (`/pickup-issue`, `/pickup-issues`),  
**I want** tickets with fully decidable scope to advance to specification automatically, while tickets with genuine product ambiguities pause for human input,  
**So that** batch autonomy is preserved without making unauthorized business decisions.

- **Given** a task processed via `/pickup-issue` or `/pickup-issues`
- **When** Round 1 clarification finds all ambiguities can be resolved via existing code patterns and conventions (zero open product questions)
- **Then** the agent records the settled scope in `docs/clarifications/<n>.md`.
- **And** it commits `docs(spec): clarify #<n> (round 1)`.
- **And** it advances the task to `clarified` via `transition_stage` and continues to the specification stage autonomously.

- **Given** a task processed via `/pickup-issue` or `/pickup-issues`
- **When** Round 1 clarification discovers essential product decisions that alter acceptance criteria or missing dependencies
- **Then** the agent records the questions in `docs/clarifications/<n>.md`.
- **And** it commits `docs(spec): clarify #<n> (round 1)`.
- **And** it posts the questions to the ticket via `add_comment`.
- **And** it stops execution without transitioning `new → clarified`, reporting the concrete blocker.

### US4 — Master skill template and generated files maintain strict contract invariants (P1)

**As a** developer maintaining the Sectile agent platform,  
**I want** the Go skill template generator and all repository/workstation skill files to enforce the round loop and transition guards,  
**So that** any agent CLI running clarification adheres to the exact same workflow invariants.

- **Given** the built-in skill catalog defined in `internal/db/skilltemplates.go`
- **When** `RenderSkillContent(StageSkillByID("clarify"), framework)` or `RenderSkillCommand(...)` is evaluated
- **Then** the rendered document contains explicit instructions for Round 1 and Round N.
- **And** the document specifies the report path `docs/clarifications/<n>.md`.
- **And** the document specifies the commit format `docs(spec): clarify #<n> (round <r>)`.
- **And** the document specifies the exit condition ("owner confirms that the clarification is satisfactory").
- **And** the execution state section explicitly forbids calling `transition_stage` to `clarified` while any product question is open.
- **And** automated unit tests in `internal/db/skills_test.go` assert these invariants for both `speckit` and `openspec` frameworks.

### US5 — Architectural documentation describes round-based clarification (P2)

**As an** engineer reading the Sectile platform documentation,  
**I want** `docs/CAPABILITIES.md` and `docs/README.md` to document the round-based clarification workflow,  
**So that** the developer documentation matches the real-world operational lifecycle.

- **Given** `docs/CAPABILITIES.md` and `docs/README.md`
- **When** a user reads the description of Stage 1 (Clarification)
- **Then** `docs/CAPABILITIES.md` describes the iterative round loop, the report structure in `docs/clarifications/<n>.md`, the exit criteria, and the distinction between interactive and unattended runs.
- **And** `docs/README.md` references round-based clarification in the autonomous AI skill pipeline overview.
