# #122 — Implementation checklist

Ordered so the helper exists before the other skills point to it. References:
[`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Catalogue entry (FR1)

- [ ] T1.1 `internal/models/models.go`: add `report_stage` and `report-stage` → `report-stage` to `SkillDirNames`.
- [ ] T1.2 `internal/skills/catalog.go`: add `HideFromBoard bool` to `StageSkill` with a comment.
- [ ] T1.3 Add the `report_stage` entry to `StageSkills` (no stage, `HideFromBoard: true`), and the `report-stage` alias in `StageSkillByID`.

## 2. Helper fragments (FR3)

- [ ] T2.1 Create `fragments/report_stage/{goal,read-first,steps,guard,report}.md`: inputs, direct invocation, gate belongs to the caller.
- [ ] T2.2 Strip the per-skill branches from `contracts/transition.md`, keeping every generic rule.

## 3. Pointer (FR2, FR4)

- [ ] T3.1 Create `contracts/report-pointer.md` with pointer, invariant and templated gate.
- [ ] T3.2 `RenderSkillContent`: render the pointer after the session title for task skills other than `report_stage`; task access only for macro scope and `report_stage`; execution block only for `report_stage`; no session title for `report_stage`.

## 4. Board hiding (FR5)

- [ ] T4.1 `internal/db/db.go` `GetAvailableSkills`: skip `HideFromBoard`.

## 5. Tests (FR6)

- [ ] T5.1 `catalog_test.go`: pointer/invariant/gate per skill, blocks absent, single pointer in pickup, helper content, alias and flag, refine_macro unchanged.
- [ ] T5.2 Adjust `TestGeneratedSkillsRenameTheSessionAfterTheWorkItem` and `TestSkillFragmentsIntegrity`.
- [ ] T5.3 `internal/db`: board catalogue omits `report_stage`, editor lists it.
- [ ] T5.4 Regenerate golden files; check `refine_macro` goldens are unchanged.

## 6. Verification

- [ ] T6.1 `go build ./...`, `go vet ./...`, `go test ./...` (skills tests under WSL), all green.
- [ ] T6.2 Re-read the rendered `clarify`, `pickup` and `report-stage` skills end to end.
