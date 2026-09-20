# Tasks — #202

## Phase 1: Baseline Freeze & Golden Generation
1. [x] Create temporary or initial test in `internal/db/skills_test.go` to generate golden files in `internal/db/testdata/golden/` from the current unrefactored renderer for all 10 skills across `speckit` and `openspec`.
2. [x] Verify that all golden files (`*.skill.md` and `*.command.md`) are cleanly generated and version-controlled.

## Phase 2: Markdown Fragments Extraction
3. [x] Create `internal/db/skills/contracts/` and extract shared contracts: `task-access.md`, `session-title.md`, and `transition.md`.
4. [x] Extract fragments for stage skills:
   - `clarify`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
   - `specify`: `goal.md`, `read-first.md`, `read-first.openspec.md`, `steps.speckit.md`, `steps.openspec.md`, `guard.md`, `report.md`.
   - `implement`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
   - `adjust`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
   - `handoff`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
5. [x] Extract fragments for utility and composite skills:
   - `create_pr`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
   - `rewrite_story`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
   - `refine_macro`: `goal.md`, `read-first.md`, `steps.md`, `guard.md`, `report.md`.
   - `pickup`: `goal.md`, `read-first.md`, `report.md` (steps and guards composed from stages).
   - `pickup_issues`: `goal.md`, `read-first.md`, `report.md` (composed from stages).

## Phase 3: Renderer Refactoring
6. [x] Mount `//go:embed skills/*` in `internal/db/`.
7. [x] Remove prose fields (`goal`, `readFirst`, `stepsBody`, `guard`, `report`) from `StageSkill` struct in `internal/db/skilltemplates.go`.
8. [x] Implement fragment loading, template substitution, and composite assembly in `RenderSkillContent` and `RenderSkillCommand`.

## Phase 4: Validation & CI Checks
9. [x] Run golden file regression tests asserting 100% byte-for-byte identity across all skills and framework variants.
10. [x] Add automated CI checks in `skills_test.go` verifying YAML frontmatter validity and fragment completeness.
11. [x] Run `make test` and full repository checks to verify zero regression.

## Test plan

- **Golden Parity**: Every skill file rendered by the new engine matches its pre-refactor golden counterpart byte-for-byte.
- **Custom Detection Integrity**: `projectskills.go` tests (`TestProjectSkillTemplatesCustomDetection`) pass without deviation.
- **CI / Build**: `go test -v ./internal/db/...` and `make test` succeed green.
