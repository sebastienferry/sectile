# ADR 0056: Sectile owns the skill contracts, and overrides replace the work

- Status: Accepted
- Date: 2026-10-05
- Issue: [#732](https://github.com/sebastienferry/sectile/issues/732)

## Context

A custom skill replaced the whole `SKILL.md`, whichever way it came: a
project's skill saved in the web Skills view, a workstation's `skills` entry in
`settings.json`, or a foreign command chosen per stage in `skillCommands`
(for example specify run as `/plan-jira`). The replacement dropped everything
Sectile relies on to follow the run: task access, the session title and
status, the run lifecycle (`start_run`, `finish_run`, `report_waiting`),
`transition_stage` and its evidence, pull-request recording and the
pull-request policy. The work got done and the card stayed where it was.

A team that wanted its own steps for one stage therefore had to copy the whole
built-in skill and keep it in step with every Sectile release, and a foreign
command had no way to advance the card at all.

Each stage's exit condition (for example "never transition new → clarified
while a product question remains open") lived in the stage's guard, a work
section, so it went with any replacement. The transition contract also still
described a "managed Sectile run" that follows a result-file contract, a
mechanism retired with the server-side result-file worker (ADR 0006).

The question was clarified over three rounds in `docs/clarifications/732.md`.

## Decision

- **Every task-scope stage skill is split into Sectile contracts and work
  sections.** Sectile owns the frontmatter (name, ID, directory, command), task
  access, the session title, links, experience and status, the run lifecycle
  and waiting, `transition_stage` with pull-request recording, the
  pull-request policy, and the stage's **exit condition**. The exit condition
  moves out of the guard into the contract (`fragments/<id>/exit.md`, for
  clarify, adjust, handoff and pickup; the others read "this step is
  complete"). The work sections are `## Goal`, `## Read first`, `## Steps`,
  `## <guard>` (`Do not` by default, `Recovery and blockers` for implement) and
  `## Report`. Pickup inlines its stages' guards but not their contracts, so
  each inlined stage gains an "Exit condition before recording <stage>" line,
  and pickup's own exit condition is rendered on its transition line.
- **An override is either a full replacement or work only.** A work-only
  override is one Markdown body made of those `## ` sections, each at most
  once and none empty; headings inside code fences, and `###` or deeper
  headings, belong to their section. A preamble, frontmatter, an unknown or
  duplicate section, or an empty one is refused with a message naming the
  allowed sections. A section left out keeps the built-in one. Pickup's and
  pickup_issues' Steps are their inlined stages, overridden through each
  stage. Macro skills and `/transition` are replaced whole only.
- **Full replacement stays the compatible kind.** The server stores the kind
  in `project_skills.override_kind` (migration 51): `''` is a full replacement,
  which every existing row keeps, and `work` is work only. A save that names
  no kind creates a work-only override when the skill takes one and keeps an
  existing row's kind; an explicit `''` switches back to full replacement. An
  import from the repository is stored as a full replacement. A full adjust
  override keeps its legacy wrapping, and the adjustment contract (ADR 0038)
  is still appended to every adjust prompt. On a workstation, a plain string
  under `skills` in `settings.json` remains a full replacement, and
  `{"kind":"work","content":"…"}` is work only; a work override that does not
  parse is ignored when the settings are resolved. Nothing is migrated
  silently.
- **Precedence, per skill:**

  | Rank | Source | Effect |
  |---|---|---|
  | 1 | Workstation full replacement | Wins wholesale (unchanged). |
  | 2 | Project full replacement | Wins wholesale (unchanged); a workstation work override of that skill is ignored, since a full body has no sections. |
  | 3 | Project work override | Each section it states wins. |
  | 4 | Workstation work override | Fills the sections the project left built-in. |
  | 5 | Built-in | Every other section. |

  The project's section is a team decision recorded on the server; the
  workstation's is a personal default. **Custom project skills win**
  (ADR 0039) still decides only whether a custom skill runs instead of the
  installed one.
- **The agent composes; the server composes too.** One composer in
  `internal/skills` renders a skill from its contracts and its layered work
  sections, adjust included. The agent runs it from its embedded fragments
  for a launched run, written to the run folder, and pickup inlines each
  overridden stage's work. The server also puts its own composite in each
  skill's `content`, beside the new `overrideKind` and `workContent` fields of
  `GET /api/v1/agent/config`, so an agent older than this decision runs the
  project's composite rather than the built-in. Both fields are omitted when
  empty, so an older server reads as full replacement.
- **A foreign command carries the stage contract.** When a stage runs a
  command from `skillCommands`, the launch prompt appends a
  "Sectile stage contract": the run lifecycle, the stage transition with its
  evidence, and the exit condition, so `/plan-jira` advances the card. Inside
  pickup, a stage with a foreign command keeps the built-in work. The prompt
  also carries the specifications workspace contract when the stage reads or
  writes the task's issue artefacts, and the task access contract with the
  fallback a launched run's direct copy names. It carries the exit condition
  of the stage the command stands for, and only that one: a foreign command
  standing for pickup or pickup_issues gets that skill's own exit, not the exits
  of the stages pickup inlines, which the command must honour by itself; the
  server's `transition_stage` validation still has the last word.
- **The direct copy carries per-project variants.** The direct setup writes
  one copy per skill, shared by every project of the workstation (ADR 0039).
  When a project the workstation knows, or the workstation itself, has a
  work-only override of a skill (or of a stage pickup inlines), each
  overridden section is rendered as one
  `### When get_project_context reports projectId "<id>"` subsection per
  overriding project, sorted by ID, then `### Otherwise` with the
  workstation's section or the built-in one, whose framework variants move
  one level down. Pickup's inlined sections sit one level deeper still. A
  section nobody overrides is rendered as before. The projects are those the
  server lists, minus the ones the workstation disconnected; a project whose
  configuration cannot be fetched is skipped with a warning, so its variant
  drops out of the shared copy until the next refresh. `sectile-agent init`,
  the desktop's **Initialize** and skill installs, `sync_config` and
  `refresh_skills` all write through this step, which also applies the
  workstation's settings that the initialization used to skip.
- **A narrow `refresh_skills` operation, not a flag on `sync_config`.** After
  a project skill is saved, reset or imported, the server sends
  `refresh_skills`, in the background and best-effort, to the connected agents
  serving the project, one per user; the existing routing reaches an agent on
  another instance. The agent rewrites the direct copies of the providers
  whose copies `agent-manifest.json` already manages, backing up hand edits
  as the setup does, and touches nothing else: no MCP registration, no other
  provider, and nothing at all on a workstation without a direct setup. An
  older agent answers that it does not support the operation, which is
  ignored. `sync_config` also scaffolds every checkout, bootstraps the MCP
  server and writes every setup provider: an agent older than a flag would
  have ignored it and run all of that on every save, the unasked install
  ADR 0039 removes.
- **`/transition <taskKey> <stage>` records a stage by hand.** It reads the
  task, gathers the stage's evidence (note, actual branch, `prUrl` and
  `prUrls`, or `noRepositoryChange`), checks the exit condition, shows the
  evidence and calls `transition_stage` once the user confirms; the server's
  validation has the last word. Its evidence rules and its per-stage exit
  table are generated from the same contract fragments as the stage skills,
  so they cannot drift. It never starts or finishes a run, ships in the
  plugin and the direct setup, takes no override, is not offered as a launch
  (`GetAvailableSkills`, the skills editor), and the agent refuses to launch
  it. A CLI that does not substitute `$ARGUMENTS` reads the key and the stage
  from the invocation text, and the skill asks for whichever is missing.
- **The result-file wording is retired.** The transition, task-access and
  session-title contracts, the clarify steps and the `transition_stage` tool
  description no longer mention a managed run or a result-file contract: a
  run Sectile launched reports through `start_run` and `finish_run`, and
  records its stage when its skill moves one, like any other run. Accepted
  ADRs 0006 and 0007 stay as written; this decision records the retirement.

## Consequences

- A team overrides the steps, the guard or any other work section of a stage
  without copying the rest, and the card still advances. A Sectile release
  that changes a contract reaches every work-only override at once.
- Existing overrides keep replacing the whole skill, and show as such in the
  Skills view, which lets the user switch the kind. Desktop gets no editor,
  and `settings.json` stays the only editor of a workstation override.
- A project's full replacement does not reach the direct copy: invoked by
  hand, the skill runs its "Otherwise" content, as before. A workstation's
  full replacement never touches the direct copy either.
- `refresh_skills` reaches only connected agents. It needs no local mapping
  of the project, since the direct copies are user-level: a workstation
  without a checkout of it refreshes them from its settings folder. A
  workstation offline at the save catches up at its next
  `sectile-agent init`, **Initialize** or `sync_config`; a hand edit of `settings.json` has no event either and
  takes effect at those points. Each refresh fetches the configuration of
  every project the workstation knows, which is affordable since saves are
  rare.
- Known limits of `refresh_skills`. It rewrites the direct copies even while
  a run is active on the workstation, unlike the desktop's **Initialize**,
  which refuses with 409 until the executions stop. `directSetupConfig`
  fetches the projects' configurations before `prepareMu` is taken, so two
  quick saves can be written out of order; `sync_config` has the same shape,
  and the next save or refresh corrects it. The server's fan-out
  (`internal/handlers/skill_refresh.go`) sends one operation per user, routed
  by user and project, so a second workstation of the same user, connected
  under another key, is refreshed only at its next `sectile-agent init`,
  **Initialize** or `sync_config`.
- Adding the operation makes every agent older than this decision show as
  outdated until it is upgraded; the changelog asks to upgrade the agent
  along with the server.
- The plugin skill (`/sectile:<dir>`) is never rewritten (ADR 0039): it
  carries no override.
- Rejected: a second, internal skill per stage for the transition, with
  pickup chaining skills (a new skill per stage and a pickup that no longer
  inlines); keeping the exit condition in the guard (an override would drop
  it); a flag on `sync_config` (see above); one direct copy per project (the
  collision ADR 0039 removed); overriding a third-party plugin's skill by
  parts (out of scope).
