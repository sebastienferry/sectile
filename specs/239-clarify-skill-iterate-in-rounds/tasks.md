# #239 — Implementation checklist

Ordered implementation checklist for implementing the round-based clarification loop in the `clarify-issue` skill, unit tests, and documentation.

---

## 1. Skill Template & Contract Invariants (`internal/db/skilltemplates.go`)

- [ ] **T1** In `internal/db/skilltemplates.go`, update `StageSkills[0]` (`ID == "clarify"`):
  - Update `Description` and `frontmatterDesc` to reflect iterative rounds and confirmation of satisfaction.
  - Update `Steps` to enumerate round-based iterations in `docs/clarifications/<n>.md`, stopping without transition if product questions remain open, and transitioning only after confirmation.
  - Update `goal` to emphasize the iterative dialogue loop with the owner until explicit confirmation of satisfaction.
  - Update `readFirst` to instruct reading `docs/clarifications/<n>.md` and latest comments via `get_task`.
  - Update `stepsBody` with explicit guidance for Round 1 (restatement, ambiguities, dependencies, reversible choices, product questions with recommendations, commit format `docs(spec): clarify #<n> (round 1)`, asking questions interactively or via `add_comment`) and Round N (reading owner answers, dated header `## Round N - answers from the owner (<date>)`, recording settled and reversed choices, committing `docs(spec): clarify #<n> (round N)`).
  - Define the unambiguous exit condition: rounds continue until owner confirms clarification is satisfactory (or zero open product questions in unattended pickup).
  - Update `guard` (`Do not`): forbid transitioning `new → clarified` while any product question or decision remains open; forbid inventing answers in unattended runs; forbid discarding earlier rounds.
  - Update `report`: include report path `docs/clarifications/<n>.md`, round number, exit status, and transition status.
- [ ] **T2** In `internal/db/skilltemplates.go`, update `renderTicketTransitionContract`:
  - For `s.ID == "clarify"`, add the explicit rule: `Transition new → clarified only when the exit condition is met: the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never transition new → clarified while any product question or decision remains open.`
- [ ] **T3** In `internal/db/skilltemplates.go`, verify `renderPickupSteps`:
  - Ensure the embedded clarification step in `renderPickupSteps` preserves the rule that autonomous pickup proceeds when zero product questions are open, and halts when questions require owner input.

---

## 2. Unit Test Suite Expansion (`internal/db/skills_test.go`)

- [ ] **T4** In `internal/db/skills_test.go`, extend `TestGeneratedSkillContracts`:
  - When `stage.ID == "clarify"`, assert that both `RenderSkillContent` and `RenderSkillCommand` contain:
    - `docs/clarifications/<n>.md` or `docs/clarifications/`
    - `docs(spec):` commit format
    - `Round 1` and `Round N`
    - `satisfactory` exit condition
    - Prohibition rule against transitioning while product questions remain open.
- [ ] **T5** In `internal/db/skills_test.go`, add dedicated test `TestClarifySkillRoundLoopInvariants`:
  - Verify that `StageSkillByID("clarify")` generates templates that enforce the round loop across all supported frameworks (`speckit`, `openspec`).
  - Verify that `RenderSkillCommand` preserves all round loop instructions and arguments placeholder `$ARGUMENTS`.
  - Run `go test -v ./internal/db/ -run TestGeneratedSkillContracts` and `go test -v ./internal/db/ -run TestClarifySkillRoundLoopInvariants`.

---

## 3. Platform & Pipeline Documentation (`docs/`)

- [ ] **T6** In `docs/CAPABILITIES.md`, update `Stage 1: Clarification (clarify-issue / /clarify)`:
  - Document the iterative round loop concept (analogous to adjustment on code).
  - Document the report location (`docs/clarifications/<n>.md`) and commit convention (`docs(spec): clarify #<n> (round <r>)`).
  - Document the exit condition: owner states the clarification is satisfactory.
  - Document unattended vs. interactive behavior: interactive asks in-session; unattended posts to ticket via `add_comment` and pauses without transitioning; autonomous pickup proceeds only when zero questions are open.
  - Reference reference case: #180, rounds 1 and 2 in `docs/clarifications/180.md`.
- [ ] **T7** In `docs/README.md`, update section 2 (`Core Capabilities & Workflows`):
  - Mention round-based clarification until owner confirmation in the Autonomous AI Skill pipeline description.

---

## 4. Verification & Validation

- [ ] **T8** Run `go test -v ./internal/db/...` to verify all database and skill template tests pass cleanly.
- [ ] **T9** Run `go vet ./...` to ensure no lint regressions.
- [ ] **T10** Review git diff with `git diff` to ensure changes strictly match the settled scope of ticket #239.
