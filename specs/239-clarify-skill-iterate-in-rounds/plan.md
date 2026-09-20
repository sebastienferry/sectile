# #239 — Implementation plan

## Stack

- **Go 1.x backend and skill template generator**: `internal/db/skilltemplates.go` (`StageSkills["clarify"]`, `renderTicketTransitionContract`, `renderPickupSteps`, `RenderSkillContent`, `RenderSkillCommand`, `SkillDirsFor`), `internal/models/models.go` (`SkillAgentDirs`).
- **Go test suite**: `internal/db/skills_test.go` (`TestGeneratedSkillContracts`, new dedicated invariant tests for `clarify`).
- **Technical & workflow documentation**: `docs/CAPABILITIES.md`, `docs/README.md`.
- **Worktree & Git context**: Work branch `feat/239` in `/Users/sferry/Sources/sectile/.tasks/worktrees/#239`.

---

## Architecture decisions

### D1 — Explicit Round Loop in `StageSkills["clarify"]` (`internal/db/skilltemplates.go`)

The master template definition for `clarify` in `StageSkills` is upgraded from a single-pass instruction set to an explicit round-based state machine:

```go
{
    ID:          "clarify",
    Name:        "Clarify",
    DirName:     models.SkillDirNames["clarify"],
    Command:     "/clarify-issue",
    FromStage:   "new",
    ToStage:     "clarified",
    Description: "Résout les ambiguïtés réversibles et mène des rounds d'échange jusqu'à confirmation du cadrage.",
    Icon:        "HelpCircle",
    Color:       "amber",
    Steps: []string{
        "Lecture du ticket et du code concerné",
        "Itération en rounds (Round 1, Round N) consignés dans docs/clarifications/<n>.md",
        "Questions bloquantes posées à l'interlocuteur ou en commentaire de ticket",
        "Arrêt sans transition tant que des choix produit restent ouverts",
        "Label 'clarified' et transition posés uniquement après confirmation",
    },
    title:           "Clarify Issue",
    frontmatterDesc: "Analyse a ticket against the code, iterate in clarification rounds, and resolve open questions until the owner confirms satisfaction.",
    goal: `Turn an ambiguous ticket into a settled specification baseline. Clarification is an
iterative dialogue with the work item's owner: execute in rounds until the owner
explicitly confirms that the clarification is satisfactory.`,
    readFirst: `- The ticket: title, description, comments via get_task, parent epic if present.
- The existing clarification report if one exists: docs/clarifications/<n>.md on the assigned work branch.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.`,
    stepsBody: `1. Re-read the assigned branch and worktree. If docs/clarifications/<n>.md already exists,
   this run continues an existing clarification into Round N. If not, this is Round 1.
2. In Round 1:
   a. Restate the request in two sentences, including what is out of scope.
   b. List ambiguities, worst first. Only list an ambiguity if two readings lead to different code.
   c. Name critical dependencies: other services, migrations, missing data, third-party limits.
   d. Resolve reversible technical choices using existing code and project conventions.
   e. Formulate essential product questions that alter acceptance criteria, with your recommended option.
   f. Write docs/clarifications/<n>.md, commit with docs(spec): clarify #<n> (round 1).
   g. Ask the questions (interactively in-session if the owner is present; as a ticket discussion
      comment via add_comment when unattended).
3. In Round N (follow-up after owner answers):
   a. Read the owner's answers from the interactive prompt or ticket comments via get_task.
   b. Append a dated section: "## Round N - answers from the owner (<date>)" to docs/clarifications/<n>.md.
   c. Explicitly record settled choices and any reversed prior assumptions.
   d. Address newly surfaced ambiguities or dependencies.
   e. Commit updates with docs(spec): clarify #<n> (round N).
   f. If follow-up product questions remain, ask them and stop without transitioning.
4. Exit condition:
   Rounds continue until the owner confirms that the clarification is satisfactory (or zero open
   product questions remain in unattended pickup). Never transition new → clarified while product
   questions remain open.
5. Persist the settled scope, decisions, and assumptions in the report before concluding.`,
    guardTitle: "Do not",
    guard: `- Do not transition new → clarified while any product question or decision remains open.
- Do not invent answers to essential product questions in unattended runs; record them and ask.
- Do not write production code or start the technical specification at this stage.
- Do not discard previous round sections when writing Round N; append each round chronologically.
- Do not switch branches or create a new branch: reuse the assigned feat/<n> branch.`,
    report: `- The report path: docs/clarifications/<n>.md.
- Current round number and whether the exit condition was met.
- Settled decisions and reversed assumptions.
- Numbered open questions (if any) and who is expected to answer them.
- Stage transition status (applied or blocked awaiting answers).`,
}
```

### D2 — Transition Contract Guard in `renderTicketTransitionContract` (`internal/db/skilltemplates.go`)

In `renderTicketTransitionContract`, the standalone execution instructions for `clarify` must explicitly mandate the transition prohibition:

```go
if s.ID == "clarify" {
    b.WriteString("Transition new → clarified only when the exit condition is met: the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never transition new → clarified while any product question or decision remains open.\n")
} else {
    fmt.Fprintf(&b, "Transition %s → %s only when this step is complete.\n", s.FromStage, s.ToStage)
    if s.ID == "adjust" {
        b.WriteString("Include prUrl with the verified pull request URL.\n")
    }
}
```

This prevents any agent from mechanically executing `transition_stage` at the end of Round 1 when questions are still awaiting user answers.

### D3 — Autonomous Pickup Guard in `renderPickupSteps` (`internal/db/skilltemplates.go`)

In `renderPickupSteps`, composite workflows (`pickup-issue`, `pickup-issues`) inherit the clarify steps. We ensure the clarification guidance embedded in pickup explicitly details the autonomous vs. pause condition:
- If Round 1 resolves all ambiguities with zero open product questions, proceed immediately to specification.
- If any product question remains open, record it in `docs/clarifications/<n>.md`, post via `add_comment`, and stop the pipeline without transitioning.

### D4 — Slash Command & Mirror Distribution Consistency

- `RenderSkillCommand` wraps the updated `clarify` content with `---\ndescription: ...\nargument-hint: ...\n---\n` and adds `$ARGUMENTS`.
- When Sectile generates or checks out skill files across agent directories (`SkillDirsFor` covering `.claude`, `.agents`, `.gemini`, `.agy`, `.skills`), they are produced from `RenderSkillContent(StageSkills["clarify"], framework)`.
- Existing mirror templates in the project checkouts and documentation are kept completely aligned with the Go source generator.

### D5 — Core Architecture Documentation Updates

1. **`docs/CAPABILITIES.md`**:
   - Update `Stage 1: Clarification (clarify-issue / /clarify)` section:
     - Document that clarification operates as an iterative feedback loop in numbered rounds (analogous to adjustment on code).
     - Specify the report path convention: `docs/clarifications/<n>.md`.
     - Specify the commit convention: `docs(spec): clarify #<n> (round <r>)`.
     - Document the exit condition: owner states the clarification is satisfactory.
     - Document the unattended / autonomous rule: halt and post questions via `add_comment` if product choices are open; continue autonomously only if zero product questions remain.
     - Document that each round is an independent Sectile run (`start_run` / `finish_run`).
2. **`docs/README.md`**:
   - Update the Autonomous AI Skill pipeline summary to note that `clarify-issue` iterates in rounds until owner confirmation.

### D6 — Contract Invariants & Unit Tests (`internal/db/skills_test.go`)

Add comprehensive assertions to `internal/db/skills_test.go`:
1. **Extend `TestGeneratedSkillContracts`**:
   - For `stage.ID == "clarify"`, assert that the generated content contains:
     - `docs/clarifications/<n>.md` (or `docs/clarifications/`)
     - `docs(spec):`
     - `Round 1` and `Round N`
     - `satisfactory`
     - Prohibition against transitioning while product questions remain open.
2. **Add `TestClarifySkillRoundLoopInvariants`**:
   - Directly verify that `StageSkillByID("clarify")` produces content that enforces:
     - Round-based execution steps.
     - Ticket comments via `add_comment` and `get_task`.
     - Unattended stop condition.
     - Both `speckit` and `openspec` frameworks render these invariants identically.

---

## Data contracts & file targets

### 1. `internal/db/skilltemplates.go`
- Modify `StageSkills[0]` (`ID == "clarify"`):
  - `Description`: emphasize iterative rounds and owner confirmation.
  - `Steps`: list round-based process and owner confirmation.
  - `frontmatterDesc`: emphasize round loop and owner confirmation.
  - `goal`: explain dialogue loop and exit condition.
  - `readFirst`: include `docs/clarifications/<n>.md` and ticket comments via `get_task`.
  - `stepsBody`: complete 5-step round loop (Round 1, Round N, exit condition, unattended vs interactive).
  - `guard`: strict prohibition against transitioning while product questions are open.
  - `report`: round details, report path, transition status.
- Modify `renderTicketTransitionContract`:
  - Enforce explicit guard for `clarify` stage transition.

### 2. `docs/CAPABILITIES.md`
- Revise Section `Stage 1: Clarification (clarify-issue / /clarify)` to document the round loop, file conventions, exit criteria, and unattended pickup behavior.

### 3. `docs/README.md`
- Update skill pipeline description to mention round-based clarification.

### 4. `internal/db/skills_test.go`
- Add invariant checks asserting round loop, report path, commit format, exit condition, and transition guard.

---

## Verification & testing plan

1. **Unit Tests**:
   - Run `go test -v ./internal/db/ -run TestGeneratedSkillContracts`
   - Run `go test -v ./internal/db/ -run TestClarifySkill`
   - Run full database test suite: `go test -v ./internal/db/...`
2. **End-to-End Skill Generation Verification**:
   - Verify that `RenderSkillContent(clarifySkill, "speckit")` and `RenderSkillContent(clarifySkill, "openspec")` generate valid markdown with frontmatter and all round loop sections.
   - Verify `RenderSkillCommand(clarifySkill, "speckit")` produces expected slash command format.
3. **Lint & Build Verification**:
   - Run `go vet ./...`
   - Run `git diff` to confirm strict adherence to scope.
