# ADR 0016: The workflow skills can come from a marketplace

Status: Accepted

## Context

The ten workflow skills are a catalogue compiled into the binary. `StageSkills`
(`internal/skills/catalog.go`) holds their metadata, the embedded markdown
fragments under `internal/skills/fragments/` hold their prose, and
`RenderSkillContent` assembles a `SKILL.md` out of both. A project may edit any of
them — `project_skills` stores the edited body, `EffectiveProjectSkills`
resolves built-in template → project edit, and the result is installed into the
agent conventions of the checkout.

Nothing let a team share a set of skill bodies. Improving the prompts meant
either patching Sectile or re-typing the same edit in every project.

## Decision

**A third source sits underneath the project's own edits: a Claude plugin
marketplace.** A deployment registers marketplaces; a project picks one plugin
from one of them, reads the diff, and applies it. From then on the pack bodies
are that project's baseline.

**The format is the official Claude one, and Sectile parses it itself.** A
marketplace is a git repository — or a plain directory — carrying
`.claude-plugin/marketplace.json`; a plugin is a directory with an optional
`plugin.json`; a skill is `skills/<name>/SKILL.md`. Sectile never invokes the
`claude` CLI. Driving `claude plugin marketplace add` would make a project
running codex, agy, gemini, cursor or vibe depend on the Claude CLI, and would
put the content under `~/.claude/plugins/`, where Sectile controls neither the
refresh nor the revision. `internal/marketplace` is therefore a pure package:
no network, no database, no process.

**A pack supplies bodies, never a workflow step.** An entry is accepted when its
skill directory name is one of the ten workflow directories, matched against the
values of `models.SkillDirNames`. Anything else — another skill, a command, an
agent, a hook — is reported as ignored and installed nowhere. Ids, directory
names, stages and slash commands stay Sectile's, so `/clarify-issue` keeps
resolving whatever a pack is applied.

**Sectile's contracts are spliced around a pack body, never replaced by it.**
`RenderSkillWithBody` keeps the generated frontmatter, title, stage line,
task-access contract and session-title contract, puts the pack body under
`## Project instructions` — the shape `adjustmentCustomContent` already
established for a reconciled adjustment — and appends the ticket-transition
contract and the project's pull request policy. A third-party body can therefore
not detach an agent from the workflow protocol, whatever it says or omits. The
batch skills keep composing their stage sections from the *resolved* bodies, so
a pack that updates `clarify` updates `pickup` too.

**Precedence is built-in → pack → project edit, and the edit always wins.**
`resolvedSkillBaselines` is the layer in between: the editor's default content
is the resolved baseline, so `isCustom` keeps meaning "differs from what this
project would otherwise get" and a reset lands on the pack rather than on the
catalogue. `project_skills` carries both bodies — `content` is the edit,
`pack_content` the baseline — because a single column cannot hold an edit and
the pack it sits on top of.

**Applying is explicit, and nothing re-resolves on its own.** Fetching and
previewing write nothing at all. The project records the marketplace, the
plugin, the plugin version and the **resolved commit**, and a pinned project
reads its own cache: the marketplace can move without changing what the project
runs, and installing skills works with the network gone. The format's own
`autoUpdate` flag is read as information and never acted upon.

**Every read of a marketplace happens on the workstation.** Two agent actions,
`marketplace_catalog` and `marketplace_pack`, join `sync_config` and
`spec_install`; the cache lives under `~/.taskflow/marketplaces/<name>/`. The
server never reaches the network itself, and each application is recorded as an
activity carrying what was applied, ignored and refused.

## Consequences

A team publishes its prompts once and every project adopts them deliberately.
The registry is a deployment setting — administrator-only to write, readable by
anyone signed in — and applying a pack to a project is guarded like the other
decisions about what the agents run.

Removing a marketplace from the registry does not change what a project runs:
the applied bodies stay, and the pin is reported as orphaned. Unpinning restores
the built-in catalogue and leaves every hand-edited skill alone.

A plugin can only ship one `specify-issue` and one `refine-macro`, because the
format carries no framework variant. That single body then serves both Spec Kit
and OpenSpec, and a skill the pack omits keeps Sectile's framework-specific one.
A team that needs two variants publishes two plugins.

## Alternatives rejected

- **A Sectile-native manifest** (`sectile-skills.json`). It would tie teams to a
  format only Sectile reads when an official one already exists and is already
  published against.
- **Installing plugins through the Claude CLI.** It ties an agent-agnostic
  product to one CLI and surrenders control of the revision.
- **Adopting `autoUpdate`.** A skill body is a prompt executed by a CLI launched
  with `--dangerously-skip-permissions`; a silent remote update is the one
  irreversible mistake this feature could make.
- **A deployment-level pack that applies to every project.** A settings-level
  default would become effective on projects nobody reviewed it for, which is
  exactly the explicit-apply rule above. Should a deployment default be added
  later, it can only pre-select what a project's picker opens on.
- **A parallel `project_marketplace_skills` table.** The resolver already reads
  one row per (project, skill); a second table would double every read for no
  gain.
