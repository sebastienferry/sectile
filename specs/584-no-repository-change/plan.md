# Plan #584 - A task that changed no repository can be recorded as implemented

## Stack

Go: `internal/taskmcp`, `internal/db` (stage transition and PR validation),
`internal/skills` (fragments and golden files).

## Design

- `transitionInput.NoRepositoryChange bool` (`noRepositoryChange`) and its
  schema entry with a description that names the justification.
- `db.TransitionTaskStageWithPRs` keeps its signature. A new entry point,
  `TransitionTaskStageWithoutRepositoryChange`, shares the inner
  `transitionTaskStage` with it, so existing callers are untouched. The
  handler refuses the statement next to `prUrl` or `prUrls` before calling it.
- With the statement, `noRepositoryChangeEvidence` runs in place of
  `validateStagePRs`. When the stage requires evidence:
  - refuse when any URL is given;
  - refuse when `task.PrLinks` holds a link recorded on the transition's
    branch (or with no branch);
  - refuse when `taskChangedRepositories(project, task)` is not empty;
  - otherwise return `stagePRSet{notice: noRepositoryChangeNotice}` with no
    lookup.
  When the stage requires no evidence, the flag is accepted and ignored.
- `noRepositoryChangeNotice = "No pull request: this task changed no
  repository."`, appended to the note by the existing notice path.
- Skills: the implement, adjust, pickup, pickup-issues and handoff fragments
  get one line on the path; golden files regenerated with the project's own
  command.

## Target files

- `internal/taskmcp/server.go`
- `internal/db/stage.go`, `internal/db/stageprs.go`
- `internal/db/no_repository_change_test.go` (new)
- `internal/skills/fragments/{implement,adjust,pickup,pickup_issues,handoff}/*.md`,
  `internal/skills/testdata/golden/*`
- `CHANGELOG.md`
