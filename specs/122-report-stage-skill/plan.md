# #122 — Technical plan

Behaviour and acceptance criteria live in [`spec.md`](./spec.md). This file records
the implementation choices only.

## Stack and constraints

- Go only: `internal/skills` (catalogue, fragments, rendering), `internal/models`
  (`SkillDirNames`), `internal/db` (board catalogue). No web, MCP, migration or
  HTTP route change.
- `internal/skills` stays free of storage dependencies (ADR 0006).
- Golden files are regenerated with `UPDATE_GOLDEN=1 go test ./internal/skills/`.
  On Windows the skills tests need Linux (embed paths via `filepath.Join`, CRLF), so
  they run under WSL.

## Shape of the change

```
internal/models/models.go            SkillDirNames += report_stage, report-stage
internal/skills/catalog.go
  StageSkill                         + HideFromBoard bool
  StageSkills                        + report_stage entry (no stage, task scope)
  StageSkillByID                     + "report-stage" alias
  renderTaskAccessContract           now rendered for macro scope and report_stage only
  renderTicketTransitionContract     → renderExecutionContract: generic, report_stage only
  renderReportingPointer (new)       contracts/report-pointer.md, every other task skill
  RenderSkillContent                 section order below
internal/skills/fragments/
  contracts/transition.md            per-skill branches removed (they move to the pointer)
  contracts/report-pointer.md        new: pointer + invariant + gate, templated by ID/stages
  report_stage/{goal,read-first,steps,guard,report}.md   new
internal/db/db.go GetAvailableSkills skip HideFromBoard
```

## Rendered section order

| Skill kind | Sections |
| --- | --- |
| task-scoped (not report_stage) | stage line, Session title, **Sectile reporting**, Goal, Read first, Steps, guard, Report |
| report_stage | Sectile task access, Goal, Read first, Steps, guard, Report, Execution and ticket state |
| refine_macro | unchanged: Sectile task access, Session title, Goal, …, Report |

The pointer sits right after the session title so the agent loads the helper before
its first ticket read, which is where the task-access rules used to be. The session
title stays first (clarification Q1).

`report_stage` gets no session title: it is loaded from inside another skill, and a
rename there would overwrite the caller's title (for example a batch's `#47 (+2)`).

## Pointer fragment (`contracts/report-pointer.md`)

```
## Sectile reporting
- Task access, the run indicator, stage transitions, pull request links and ticket
  comments follow `/report-stage`: read `report-stage/SKILL.md` in the same skills
  directory as this skill, for example `~/.claude/skills/report-stage/SKILL.md`. Load
  it before the first ticket read.
- If it cannot be loaded, these rules still hold. Standalone: call start_run before
  work (reuse a supplied SECTILE_RUN_ID or launch runId) and finish_run when the whole
  invocation ends, including errors or stopping for input; a nested skill never
  finishes the outer run. Managed run: submit only through the supplied result
  contract, and call no transition or comment tool.
- Gate: <per skill>
```

Gate lines, templated on `.ID`, `.FromStage`, `.ToStage`:

- pickup / pickup_issues: today's pickup sentence.
- clarify: today's clarify sentence (keeps "Never transition new → clarified while").
- adjust: "call `transition_stage` … reviewed … only when this step is complete, with
  prUrl set to the verified pull request URL".
- other staged skills: "call `transition_stage` with stage `<to>`, the report note and
  the actual branch only when this step is complete (`<from>` → `<to>`)".
- no stage (create_pr, rewrite_story): "this skill does not change the workflow stage;
  never record a stage transition." — the word `transition_stage` does not appear, the
  existing contract test forbids it in non-workflow skills.

## Board hiding

`StageSkill.HideFromBoard` is the catalogue flag. `GetAvailableSkills` skips it, which
removes it from `/api/skills` (board, command palette) and from `CreateJob`'s lookup
(`internal/db/db.go` `targetSkill` loop), so it cannot be launched as a job.
`ProjectSkillTemplates`, `ListProjectSkillEditor`, agent provisioning
(`EffectiveProjectSkills` → `agentconfig`) and `get_project_context` iterate the full
catalogue and keep it. A dedicated flag was chosen over a new `Scope` value because
`Scope` already drives session-title wording, macro checks and the editor badge.

## Rejected alternatives

- Keeping the per-skill branches inside `transition.md` and rendering it in
  `report_stage` with `.ID = report_stage`: every branch would be dead text there.
- Duplicating the task-access text into `report_stage/steps.md`: the macro skill still
  needs it, so it stays a contract fragment rendered in both.
- A new `Scope: "utility"`: see Board hiding.

## Tests

- `catalog_test.go`: new tests for FR1–FR4 (pointer present, blocks absent, gates,
  helper content, single pointer in pickup, refine_macro untouched, alias lookup,
  hidden flag); `TestGeneratedSkillsRenameTheSessionAfterTheWorkItem` skips
  `report_stage`; `TestSkillFragmentsIntegrity` requires `report-pointer.md`.
- `internal/db`: `GetAvailableSkills` omits `report_stage`; `ListProjectSkillEditor`
  lists it.
- Golden files regenerated; `refine_macro` goldens must not change.
