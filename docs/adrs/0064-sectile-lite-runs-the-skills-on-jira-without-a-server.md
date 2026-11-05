# ADR 0064: Sectile Lite runs the ticket skills on Jira, without a Sectile server

- Status: Proposed
- Date: 2026-11-05

## Context

Claude now provides much of what the Sectile server and the desktop add around
the workflow skills: a worktree per session, a task list, a "next step" box at
the end of every turn, sidebar groups, and shared pages (artifacts) that can
read Jira with each viewer's own connector, show who is looking, and hand a
ticket to the viewer's Claude on a click. A team that keeps its tickets on Jira
could run the Sectile workflow with nothing of its own to host.

The skills as they are cannot do that. Their bodies (goal, steps, guards,
reports) say nothing about where the ticket lives, but four contracts tie them
to a Sectile server: task access goes through the local agent, a stage is
recorded with `transition_stage`, a run is reported with `start_run` and
`finish_run`, and the specifications workspace may come from
`prepare_task_spec_worktree`. About ten lines of the steps also name a Sectile
tool.

Copying the skills into a second set would have them drift apart at the first
change.

## Decision

**A second Claude plugin, `sectile-lite`, renders the same ticket skills from
the same fragments, under its own contracts. Jira is the source of truth, and a
ticket's stage is a `sectile:<stage>` label.**

- `internal/skills` takes a `lite` flavour. A contract is read from
  `fragments/contracts/lite/` when one is there, and from the shared contract
  otherwise. Lite has its own `task-access`, `transition` and `spec-workspace`.
  The session title contract is shared, with `{{if .Lite}}` around what only
  makes sense with a Sectile launch. `RenderLiteSkillContent` and
  `RenderLitePlugin` render it; `sectile-plugin -lite` writes it.
- The steps are not touched. The Lite task access contract maps each Sectile
  tool or variable the steps name to what replaces it: `get_task` is a read of
  the Jira issue, `add_comment` a Jira comment, `transition_stage` the stage
  change, `record_pull_request` a remote link, `get_project_context` a reading
  of the repository; `start_run`, `finish_run` and `report_waiting` are
  skipped, and `SECTILE_*` variables are absent.
- **A stage is one label** among `sectile:clarified`, `sectile:specified`,
  `sectile:implemented`, `sectile:reviewed` and `sectile:finished`; none means
  `new`. Setting a stage replaces the previous `sectile:` label and leaves the
  others alone. The Jira status is never changed: it belongs to the team's own
  workflow. The stage report is one Jira comment headed `Sectile · <stage>`.
- The plugin declares no MCP server and asks nothing at install time. Jira is
  reached through the Atlassian connector of whoever runs the skill, so every
  write uses that person's own access (ADR 0059 by other means).
- Macro skills stay in the full plugin: a macro is a Sectile object with no
  Jira equivalent.
- The full plugin's output is unchanged, byte for byte; its golden files
  guard it. A test checks that each Lite skill has the same body as the
  generic one.

## Consequences

- One edit to a fragment reaches both plugins. A change to a shared contract
  must keep reading right in both flavours, or move into `contracts/lite/`.
- The board that goes with Lite is an artifact that reads the labels, not the
  Sectile web board. It shows what Jira holds and nothing more: no run in
  progress, no waiting state, no orchestration capacity.
- Without a server there is no orchestrator, no concurrency limit and no
  result validation: a stage is what the skill wrote on the ticket. A team that
  needs those keeps the full Sectile.
- The skills now depend on an Atlassian connector being available to the
  session. When it is not, they keep the work and report the pending write
  instead of reaching Jira another way.
