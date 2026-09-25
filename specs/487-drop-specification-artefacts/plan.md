# Plan #487 - Drop the specification artefacts

Implementation choices for `spec.md`. Behaviour lives there; this file says
where and how.

## Stack and precedents

- Go server (`internal/db`, `internal/handlers`, `internal/taskmcp`), Go local
  agent (`internal/agent`, `internal/agentconfig`), React web
  (`web/src/components/ProjectModal.tsx`), Electron desktop
  (`desktop/src/main.js`).
- The closest precedent is `useWorktrees`: a `projects` column, projected into
  `agentconfig.Config`, overridden per workstation by `Overrides.Worktrees`
  with an `inheritWorktrees` flag on save, shown in the web "Local agent
  execution defaults" panel and in the desktop as a segmented row with a
  reset control. The new setting copies that shape, except that it is a
  string enum rather than a boolean, so that the stored override can say
  *keep* or *drop* and its absence says *follow the server*.
- The local exclude file is already written by `excludeTaskWorktrees`
  (`internal/agent/agent_macro_worktree.go`), which resolves
  `git rev-parse --git-common-dir` and appends to `info/exclude`.

## Decisions

### 1. Data contract

| Place | Name | Values |
| --- | --- | --- |
| `projects` column | `spec_artifacts TEXT NOT NULL DEFAULT 'keep'` | `keep`, `drop` |
| `models.Project`, create/update requests | `SpecArtifacts` / `specArtifacts` (`*string` in the update request) | same |
| `agentconfig.Config` | `SpecArtifacts string` `json:"specArtifacts,omitempty"` | same; empty read as `keep` |
| `agentconfig.Overrides` | `SpecArtifacts map[string]string` `json:"specArtifacts,omitempty"` | per project, `keep` or `drop` |
| desktop save input | `specArtifacts *string`, `inheritSpecArtifacts bool` | as `useWorktrees` / `inheritWorktrees` |
| desktop project GET | `specArtifacts` (effective), `specArtifactsOverride` (bool), `specArtifactsTracked` (int) | |
| agent operation | `spec_artifacts` -> `{"mode": "keep"|"drop"}` | effective value |

A `models.NormalizeSpecArtifacts(string) string` helper returns `drop` for
`drop` (case-insensitive, trimmed) and `keep` for everything else. Project
create and update in `internal/db/db.go` validate the raw value first
(`keep`, `drop`, or empty meaning `keep` on create and unchanged on update)
and refuse anything else with "specArtifacts must be keep or drop", as they
already do for `prCreationStage`.

`ApplyOverrides` sets `c.SpecArtifacts` from the override when one is stored,
so every consumer of the applied config reads the effective value (FR3).
`localProjectRoot` copies the new key like `Worktrees` (see
`.agents/MEMORY.md`: a new `Overrides` key needs the merge, and
`WriteSettings` must null an emptied map).

### 2. Migration

A new numbered migration in `internal/db/migrations.go` (23 is taken on
`origin/main`; check again before writing): `ALTER TABLE projects ADD COLUMN
spec_artifacts TEXT NOT NULL DEFAULT 'keep';`. The frozen baseline in
`db.go` is not touched. The SELECT/INSERT/UPDATE column lists of `projects`
in `db.go` gain the column. The db test helpers that rewind migrations drop
the new column (the `dropRepositoryColumns` pattern).

### 3. Exclude block

New file `internal/agent/specexclude.go`:

```text
# >>> sectile: dropped specification artefacts, project <projectID> >>>
/docs/clarifications/487.md
/openspec/changes/487-*/
/specs/487-*/
# <<< sectile: dropped specification artefacts, project <projectID> <<<
```

- `artefactPatterns(key string) []string`: strips a leading `#`, accepts only
  `^[A-Za-z0-9][A-Za-z0-9._-]*$` (returns nil otherwise), and returns the three
  anchored patterns, plus their lower-case forms when they differ.
- `ensureSpecExclusions(ctx, checkout, projectID, key string) error`: reads
  `<git-common-dir>/info/exclude`, finds the project's block, adds the missing
  patterns, sorts and dedups them, and writes the file back atomically
  (temporary file in the same directory, then rename). No change, no write.
- `removeSpecExclusions(ctx, checkout, projectID string) error`: removes the
  block and its markers, leaves every other byte as it was, no write when
  absent.
- A marker without its closing line is treated as the end of the file for the
  project's own block only when the opening marker is Sectile's; lines after
  it are kept.
- Reuse the common-dir resolution of `excludeTaskWorktrees` through a small
  shared `excludeFilePath(ctx, repo)` helper.

Callers, all under `prepareMu`:

- `prepareDispatchLocked` (`internal/agent/agent_config.go`), right after
  `ensureLocalWorktree`, on `root` (the primary checkout): `drop` ->
  `ensureSpecExclusions`, `keep` -> `removeSpecExclusions`. This covers stage
  launches, terminals and the board's `prepare_workspace`. A failure is
  returned: a launch that should drop must not start committing.
- The desktop project save (`agent_desktop.go`, the handler that writes
  `overrides.Worktrees`): after `WriteSettings`, when the effective value is
  `keep`, `removeSpecExclusions` on the project's code checkout and on every
  checkout `overrides.Repositories` maps for the project's repositories. A
  failure is logged, not returned: settings are already saved and the next
  launch retries.

### 4. Skills

The rendered skills are installed per user, not per project
(`agentconfig.Scaffold`), and the effective value depends on the workstation,
so the skill text cannot carry the value. It carries a rule instead, and Git
is the source of truth:

- `internal/skills/fragments/clarify/steps.md`, steps 2f and 3e: before
  committing, run `git check-ignore -q docs/clarifications/<n>.md`; when it
  succeeds, the project drops its artefacts: write the file, do not commit
  it, never `git add -f`, and put the settled decisions in full in the
  transition note. Define `<n>` as the task key without its leading `#`.
- `internal/skills/fragments/specify/steps*.md` (the three variants): the
  same rule for the specification directory, and the report carries the
  requirements and open points.
- `internal/skills/fragments/implement/read-first.md` and `steps.md`:
  ignored specification files are read, never committed or force-added; when
  the project drops its artefacts (the launch notice says so, or the paths
  are ignored) and the specification is missing, stop and report it.
- `internal/skills/fragments/adjust/steps.md`: never force-add an ignored
  specification artefact.
- The pickup skills embed these bodies (`renderPickupSteps`), so they follow.
- Regenerate every golden file under `internal/skills/testdata/golden/`
  with `UPDATE_GOLDEN=1 go test ./internal/skills/`, and review the diff.

### 5. Launch notice

In the dispatch path of `internal/agent/agent.go`, next to the "Preserve
accepted artifacts" line, for skills `clarify`, `specify`, `implement`,
`adjust`, `pickup` and `pickup_issues`, when `config.SpecArtifacts` is
`drop`:

```text
Specification artefacts are dropped on this workstation: write them in the worktree, never commit or push them (they are excluded through .git/info/exclude).
```

No environment variable is added: the skill decides from Git, and the notice
is for the human reading the session.

### 6. Pull request at specified (FR9)

- Server: in `validateStagePR` (`internal/db/adjustment.go`), when
  `skillID == "specify"`, the creation owner is `specify` and the transition
  carries no `prUrl`, call the actor's agent with the new `spec_artifacts`
  operation. On `drop`, accept the transition without a pull request and
  return the notice "Pull request deferred to the implemented stage: the
  specification artefacts are dropped on this workstation." On `keep`, on an
  unknown-operation error or on any other error, continue with today's lookup.
- Agent: `spec_artifacts` joins the operation list of `executeOperation`
  (`internal/agent/agent_operations.go`) and returns the effective value of
  the applied config. Add it to `localInspections` (15 s) in
  `internal/db/agentoperations.go`.
- The implemented stage already requires its pull request
  (`stagePRRequired`), so nothing changes there.
- `internal/db/projectskills.go`: the `timing == "specified"` policy text
  gains one sentence: "When the specification files are ignored by Git
  (dropped artefacts), open no PR at this stage, say so in the report, and
  create the draft after implementation."

### 7. Tracked warning (FR10)

The desktop project GET computes `specArtifactsTracked` with
`git -C <root> ls-files -- specs openspec/changes docs/clarifications | wc -l`
equivalent (count the lines in Go), only when the mapping is valid; errors
count as 0. The desktop row shows the warning text of US5 under the hint when
the effective value is `drop` and the count is positive.

### 8. Web

`ProjectModal.tsx`, "Local agent execution defaults" panel, below the
worktree checkbox: a checkbox "Keep specifications out of the repository"
bound to `specArtifacts === 'drop'`, and the help text "Clarifications and
specifications stay in the task worktree and are never committed. Those
already committed stay in the history." The panel is English today, so the
new strings are English too. `web/src/types/index.ts` gains
`specArtifacts?: 'keep' | 'drop'`; the save payload sends it.

### 9. Desktop

`desktop/src/main.js`, project settings, after the Worktrees row: a
"Specifications" `settingRow` with a `Keep` / `Drop` segmented control, reset
label "Reset specifications to server default", hint built like the
Worktrees one from `config.specArtifacts`. The save call sends
`specArtifacts` and `inheritSpecArtifacts`.

## Rejected alternatives

- **Committed `.gitignore`, separate folder**: rejected in clarification.
- **Rendering the value into the skills**: skills are installed per user and
  shared by projects; the value is per project and per workstation.
- **`SECTILE_SPEC_ARTIFACTS` environment variable**: redundant with Git and
  absent from standalone runs.
- **Exposing the setting in `get_project_context`**: it would give the server
  value, which a workstation override can contradict.
- **Pruning the rules of finished tasks at handoff cleanup**: worktrees are
  removed by several paths (the handoff skill, `remove_workspace`, multi-repo
  cleanup); stale rules are harmless, the block goes when the option is off.
- **Ignoring the whole `specs/` tree**: would also ignore specifications a
  human adds by hand, and the clarification bounded the rules to the task's
  artefacts.

## Target files

- `internal/db/migrations.go`, `internal/db/db.go`, db test rewind helpers
- `internal/models/models.go`
- `internal/db/agentconfig.go`, `internal/agentconfig/config.go`,
  `internal/agentconfig/local.go`, `internal/agent/agent_config.go`
- `internal/agent/specexclude.go` (new), `internal/agent/agent_macro_worktree.go`
- `internal/agent/agent.go`, `internal/agent/agent_desktop.go`,
  `internal/agent/agent_operations.go`
- `internal/db/adjustment.go`, `internal/db/agentoperations.go`,
  `internal/db/projectskills.go`
- `internal/skills/fragments/{clarify,specify,implement,adjust}/*.md`,
  `internal/skills/testdata/golden/*`
- `web/src/types/index.ts`, `web/src/components/ProjectModal.tsx`
- `desktop/src/main.js`
- `CHANGELOG.md`
