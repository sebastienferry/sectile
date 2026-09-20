# Implementation plan — #202

## Stack

- **Language**: Go 1.22+ (`embed.FS`, `text/template`, `testing`).
- **Target packages**: `internal/db`.
- **Validation**: `go test ./internal/db/...`, `make test`.

## Architecture & Directory Layout

### 1. Embedded File System
An `embed.FS` instance in `internal/db/skills.go` (or `skilltemplates.go`) mounts all fragments:
```go
//go:embed skills/*
var embeddedSkillsFS embed.FS
```

### 2. Fragment Directory Tree
```
internal/db/skills/
├── contracts/
│   ├── task-access.md
│   ├── session-title.md
│   └── transition.md
├── clarify/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
├── specify/
│   ├── goal.md
│   ├── read-first.md
│   ├── read-first.openspec.md
│   ├── steps.speckit.md
│   ├── steps.openspec.md
│   ├── guard.md
│   └── report.md
├── implement/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
├── adjust/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
├── handoff/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
├── create_pr/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
├── pickup/
│   ├── goal.md
│   ├── read-first.md
│   └── report.md
├── rewrite_story/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
├── refine_macro/
│   ├── goal.md
│   ├── read-first.md
│   ├── steps.md
│   ├── guard.md
│   └── report.md
└── pickup_issues/
    ├── goal.md
    ├── read-first.md
    └── report.md
```

### 3. Rendering Engine
- Replace hardcoded string concatenation in `RenderSkillContent` and `RenderSkillCommand` with a clean renderer that loads fragments via `embeddedSkillsFS.ReadFile`.
- Fragment fallback logic: check for `<name>.<framework>.md`, fall back to `<name>.md`.
- Dynamic parameters (`Scope`, `Command`, `Title`, `Batch`, `FromStage`, `ToStage`) evaluated using `text/template`.
- Composite skills (`pickup`, `pickup_issues`) load `clarify`, `specify`, `implement`, and `adjust` fragments directly from their respective folders.

### 4. Golden Test Baseline Harness
- Golden fixtures saved in `internal/db/testdata/golden/`:
  - `<skill>.<framework>.skill.md`
  - `<skill>.<framework>.command.md`
- Generated before changing the Go rendering code to freeze the exact expected output.
- Unit test iterates over all combinations and asserts `diff == ""`.

## Target files

| File | Change |
| --- | --- |
| `internal/db/skills/**` | Create all 10 skill fragment directories, markdown files, and shared contract fragments. |
| `internal/db/skilltemplates.go` | Strip prose fields from `StageSkill`; replace builders with embedded FS loader and `text/template`. |
| `internal/db/testdata/golden/**` | Store baseline golden rendering outputs for all 10 skills across `speckit` and `openspec`. |
| `internal/db/skills_test.go` | Add golden parity tests and CI frontmatter / fragment integrity validation. |

## Data contracts

- No database schema or HTTP API contracts modified.
- Internal Go types: `StageSkill` drops `goal`, `readFirst`, `stepsBody`, `guard`, `report` string fields.

## Risks & Mitigations

- **Risk**: Whitespace drift (trailing newlines, indentation) causes `isCustom` to detect false overrides for existing projects.
  - *Mitigation*: Golden file tests assert byte-for-byte exact equality against current renderer before making any Go edits.
- **Risk**: Missing framework variant when resolving `openspec` vs `speckit`.
  - *Mitigation*: Fallback cascade (`<file>.<framework>.md` -> `<file>.md`) and explicit coverage in golden test suite.
