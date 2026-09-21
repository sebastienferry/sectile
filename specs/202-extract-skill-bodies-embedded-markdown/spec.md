# #202 — Extract the skill bodies out of Go into embedded markdown fragments

## Context

`internal/db/skilltemplates.go` holds the prose of all 10 workflow skills (`clarify`, `specify`, `implement`, `adjust`, `handoff`, `create_pr`, `pickup`, `rewrite_story`, `refine_macro`, `pickup_issues`) as raw Go string literals inside a 738-line source file. 

Editing or reviewing skill prompts requires editing Go code, skill wording cannot be reviewed effectively in pull requests, and formatting mistakes are invisible until rendered at runtime. Furthermore, `pickup` and `pickup_issues` manually duplicate prompt steps and guard rules instead of reusing fragments by composition.

The objective is to externalize all skill prompt bodies into individual Markdown fragments embedded via Go's standard library `embed.FS`, assemble them dynamically with `text/template`, retain strictly navigation/UI metadata in `StageSkill`, and verify byte-for-byte exact rendering output against golden test baselines.

## User stories

### US1 — Maintain skill prose in Markdown files (P1)

As a developer maintaining Sectile skills, I want skill instructions, goals, and contracts to reside in standard Markdown files rather than Go string constants, so that I can format, lint, and review prompt wording with standard Markdown tooling.

- **Given** any of the 10 workflow skills in Sectile
- **When** inspecting its prose (goal, read-first, steps, guard, report, or shared contracts)
- **Then** the text resides under `internal/db/skills/` as `.md` files embedded into the Go binary at compile time via `go:embed`.

### US2 — Preserve exact runtime rendering output (P1)

As an agent executing workflow skills, I want the rendered `SKILL.md` content and slash command definitions to match the current baseline byte-for-byte, so that prompt behavior, instructions, and stage transition contracts remain completely unaffected by the refactoring.

- **Given** any skill ID (`clarify`, `specify`, `implement`, etc.) and any supported framework (`speckit`, `openspec`)
- **When** calling `RenderSkillContent(s, framework)` or `RenderSkillCommand(s, framework)`
- **Then** the rendered string output is identical byte-for-byte to the baseline generated from the unrefactored code.

### US3 — Dynamic composition without duplicated prose (P2)

As a developer adding or editing a stage rule, I want composite skills (`pickup`, `pickup_issues`) to automatically include the stage fragments (`clarify`, `specify`, `implement`, `adjust`), so that rules cannot accidentally drift between standalone stage skills and the automated pickup pipeline.

- **Given** an edit to a stage fragment (such as `clarify/steps.md` or `implement/guard.md`)
- **When** rendering `pickup` or `pickup_issues`
- **Then** the composite skill automatically includes the updated fragment content.

### US4 — Automatic validation in CI (P2)

As a contributor opening a pull request that modifies skill markdown fragments, I want automated CI tests to validate frontmatter, syntax, and fragment completeness, so that broken templates or syntax errors are caught before merging.

- **Given** a change to any embedded skill fragment
- **When** running `make test` or `go test ./internal/db/...`
- **Then** tests validate YAML frontmatter validity, required fragment presence, and golden file parity.

## Functional requirements

- **FR1**: Create embedded filesystem `//go:embed skills/*` in `internal/db/` exposing all skill fragments and shared contracts.
- **FR2**: Separate skill fragments into `internal/db/skills/<skill-id>/`:
  - `goal.md`
  - `read-first.md` (and `read-first.openspec.md` where applicable)
  - `steps.md` (and `steps.openspec.md`, `steps.speckit.md` where applicable)
  - `guard.md` (list items only)
  - `report.md`
- **FR3**: Store shared workflow contracts in `internal/db/skills/contracts/`:
  - `task-access.md`
  - `session-title.md`
  - `transition.md`
- **FR4**: Strip all prose string literals from `StageSkill` in `internal/db/skilltemplates.go`, retaining only UI/display metadata (`ID`, `Name`, `DirName`, `Command`, `FromStage`, `ToStage`, `Scope`, `Mode`, `Description`, `Icon`, `Color`, `Steps`, `Title`, `FrontmatterDesc`, `GuardTitle`).
- **FR5**: Replace `fmt.Fprintf` string builder concatenation with `text/template` rendering for dynamic parameters (scope, command names, batch indicators, framework titles).
- **FR6**: Maintain 100% backward compatibility for `ProjectSkillTemplates(framework)` and `DefaultContent`, ensuring `isCustom` calculations in `projectskills.go` are unaffected.
- **FR7**: Introduce golden test fixtures in `internal/db/testdata/golden/` comparing old vs. new rendering for all 10 skills across both `speckit` and `openspec` frameworks.

## Out of scope

- Modifying the text, wording, tone, or contract instructions of any skill.
- Changing UI labels, icons, colors, or step list strings displayed in the Sectile web or desktop interfaces.
- Modifying database schemas or table columns (`project_skills`, `tasks`, `settings`).
- Modifying client-side MCP tools, runner hooks, or agent dispatch routines.
