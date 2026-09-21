# #267 — Implementation plan

## Stack

Unchanged: Go server (`internal/handlers`, `internal/db`, SQLite through `d.conn`), Go
local agent (`internal/agent`), React + TypeScript interface (`web/src`). One new pure Go
package, `internal/marketplace`, holds the format: it has no network and no database, so
the whole parser is unit-testable from `testdata/`.

## Architecture decisions

### D1 — The format lives in one pure package

`internal/marketplace` owns everything about the Claude plugin marketplace layout:

- `ParseMarketplace(root string) (Marketplace, error)` — reads
  `<root>/.claude-plugin/marketplace.json` into `{Name, Owner, Description, Plugins[]}`.
- `ResolvePlugin(root string, plugin PluginRef) (Pack, []Ignored, error)` — locates the
  plugin directory from its `source`, reads `plugin.json` at the plugin root or under
  `.claude-plugin/`, resolves the skills directory (`"skills"` key, else `./skills/`), reads
  every `<dir>/SKILL.md`, and splits the result into accepted entries (directory name is one
  of the ten) and ignored ones.
- `WorkflowDirNames()` — the ten canonical directory names, derived from
  `models.SkillDirNames` values so a rename in the catalogue cannot leave a second list
  behind.

The agent calls it; the server never does. Rejected alternative: parsing on the server from
a tarball fetched over HTTP. It would put network access back on the server, which is the
one thing `spec_install` and `sync_config` established should live on the workstation.

Rejected alternative: shelling out to `claude plugin marketplace add`. It would make every
project depend on the Claude CLI and would put the content under `~/.claude/plugins/`, where
Sectile controls neither the refresh nor the revision (clarification, round 2).

### D2 — Two new agent actions, cache under `~/.taskflow/marketplaces`

`agentprotocol.Operation` gains `Marketplace`, `Plugin`, `Kind`, `Locator` and `Commit`.
Two actions join the allow-list in `internal/agent/agent_operations.go:94`:

- **`marketplace_catalog`** — ensures the cache for the marketplace, then returns
  `models.MarketplaceCatalog{Name, Owner, Description, Commit, Plugins[]}`. Each plugin
  carries its accepted workflow directories and its ignored ones, so the picker of US2 needs
  a single call.
- **`marketplace_pack`** — resolves one plugin and returns
  `models.SkillPack{Marketplace, Plugin, Version, Commit, Bodies map[dirName]string,
  Ignored[], Rejected[], Warnings[]}`. With `Commit` set on the operation it resolves that
  revision instead of the head, which is how a pinned project re-reads its own pack offline.

Cache: `~/.taskflow/marketplaces/<sanitized-name>/`, beside the existing
`~/.taskflow/agent-connection.json`. `github`/`git` kinds clone on first use
(`git clone --filter=blob:none`) and `git fetch` + `git checkout --detach <rev>` afterwards;
a `path` kind is read in place and never cloned. The resolved commit is
`git rev-parse HEAD` in the cache, and is empty for a `path` source that is not a git
repository — the preview then states the pin is not reproducible. `operationTimeout`
(`internal/db/agentoperations.go:93`) gives both actions 3 minutes, the network budget
`spec_install` already establishes; neither is a `localInspections` entry.

### D3 — One row per (project, skill), two body columns

`project_skills` keeps its primary key and gains three columns, in the `ensureProjectSkillsTable`
style already used for `mode`:

```sql
ALTER TABLE project_skills ADD COLUMN pack_content TEXT NOT NULL DEFAULT '';
ALTER TABLE project_skills ADD COLUMN pack_origin  TEXT NOT NULL DEFAULT '';
```

`content` stays the project's own edit, `pack_content` is the marketplace baseline,
`pack_origin` is `"<marketplace>/<plugin>@<version>+<sha7>"` for display. The clarification
asked for one `source` column; a single column cannot hold an edit *and* the pack it sits on
top of, and the precedence of FR6 requires both. The origin is therefore derived — pack body
present means the baseline is a marketplace one, a non-empty `content` means the effective
body is the project's edit — which cannot go out of sync with itself. The exact column shape
was explicitly left to this specification.

The pin is a project fact, not a per-skill one, so it gets its own small table rather than
two more columns on the already wide `projects` row (which would mean touching every scan
and update in `internal/db/db.go` for a value only the skills screen reads):

```sql
CREATE TABLE IF NOT EXISTS project_skill_packs (
    project_id   TEXT PRIMARY KEY,
    marketplace  TEXT NOT NULL,
    plugin       TEXT NOT NULL,
    version      TEXT NOT NULL DEFAULT '',
    commit_sha   TEXT NOT NULL DEFAULT '',
    applied_at   TEXT NOT NULL,
    applied_skills TEXT NOT NULL DEFAULT '[]'
);
```

The registry is deployment-wide and mirrors `known_marketplaces.json`:

```sql
CREATE TABLE IF NOT EXISTS skill_marketplaces (
    name        TEXT PRIMARY KEY,
    kind        TEXT NOT NULL,              -- github | git | path
    locator     TEXT NOT NULL,              -- owner/repo, git URL, or absolute path
    owner       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    last_commit TEXT NOT NULL DEFAULT '',
    last_fetched_at TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
```

Both DDL statements join the schema list in `internal/db/db.go:314`.

### D4 — A pack body enters the existing renderer, it does not replace it

`internal/db/skilltemplates.go` gains one function:

```go
// RenderSkillWithBody renders a skill whose prose comes from a pack: Sectile's
// header and contracts, the pack body in between.
func RenderSkillWithBody(s StageSkill, specFramework, body string) string
```

It reuses `RenderSkillContent(s, specFramework)` for the header (frontmatter, title, stage
line, `renderTaskAccessContract`, `renderSessionTitleContract`) and the tail
(`renderTicketTransitionContract`), and splices the pack body — frontmatter stripped with
the same helper `adjustmentCustomContent` already uses — between the two under a
`## Project instructions` heading. This is deliberately the shape
`adjustmentCustomContent` established for a reconciled `adjust` skill: one way of putting
foreign prose inside a Sectile contract, not two.

`pickup` and `pickup_issues` are the exception to the splice: `renderPickupSteps` keeps
composing them from the resolved bodies of `clarify`, `specify`, `implement` and `adjust`.
It therefore takes the resolved bodies as an argument instead of reading `StageSkillByID`
directly, so a pack that updates `clarify` also updates the batch skills (FR8, US5).

### D5 — Resolution reads one more layer, in one place

`EffectiveProjectSkills` and `ListProjectSkillEditor` both already start from
`ProjectSkillTemplates(framework)` and then apply the override map. A single new helper sits
between them:

```go
// resolvedSkillBaselines returns, per skill id, the content a project would get
// without its own edits: the built-in template, replaced by the pack body when
// the project pinned one that supplies that skill.
func (d *DB) resolvedSkillBaselines(projectID, framework string) []ProjectSkillTemplate
```

`EffectiveProjectSkills` starts from it instead of `ProjectSkillTemplates`, and
`ListProjectSkillEditor` sets `DefaultContent` from it — which is what makes `IsCustom`,
the reset button and the diverged-on-disk comparison keep their meaning (FR6, US4). The
framework rule of US8 lives here: a pack body for `specify-issue` or `refine-macro` applies
to both frameworks, and an absent one leaves the built-in variant in place.

### D6 — Fetch and preview are read-only; only apply writes

Three DB entry points in a new `internal/db/skillmarketplace.go`:

- `MarketplaceCatalog(name)` → `callAgent(marketplace_catalog)`, updates `last_commit` and
  `last_fetched_at` on the registry row and returns the catalogue. It writes nothing on any
  project.
- `PreviewSkillPack(projectID, marketplace, plugin)` → `callAgent(marketplace_pack)`, then
  builds `models.SkillPackPreview{Pack, Entries[]{SkillID, DirName, Current, Proposed,
  Changed}, Ignored, Rejected, Missing, Warnings}` by rendering each pack body through D4
  against the project's framework. Nothing is persisted.
- `ApplySkillPack(projectID, marketplace, plugin, commit)` → re-resolves at the previewed
  commit, writes `pack_content` / `pack_origin` per skill, clears `pack_content` for the
  skills the new pack does not supply (US7), upserts `project_skill_packs`, then calls
  `WriteAllProjectSkillsToRepo` exactly as an edit does, and records an activity through a
  `recordSkillPackActivity` modelled on `recordSpecFrameworkActivity`
  (`internal/db/specframework.go:100`) with `skillId: apply_skill_pack`.
- `UnpinSkillPack(projectID)` → deletes the pin row, clears every `pack_content`, reinstalls.

A project with a pin is never fetched implicitly: installation, launch and the skills editor
all read the stored `pack_content` (FR11, US6).

### D7 — A deployment default is a pre-selection, not an effective body

The clarification asks for the project → settings fallback that `resolveSpecFrameworkTarget`
uses. Applied literally to a prompt that runs with `--dangerously-skip-permissions`, a
settings-level pack would become effective on a project nobody reviewed it for, which
contradicts the explicit-apply decision. So the fallback is a **selection** fallback: the
deployment may name a default marketplace and plugin (two deployment settings keys,
`skillPackMarketplace` and `skillPackPlugin`), which is what the project's picker opens on.
A project with no pin of its own runs the built-in catalogue until someone applies one.

### D8 — Authorization and routes

Registry writes go through `h.requireAdmin` (`internal/handlers/authz.go:87`), like a
deployment setting; reads are open to any signed-in principal. New routes, registered in
`cmd/server/main.go` beside `/api/spec-framework/...`:

| Route | Method | Effect |
|---|---|---|
| `/api/skill-marketplaces` | `GET` | the registry |
| `/api/skill-marketplaces` | `POST` | register (admin); resolves once before storing |
| `/api/skill-marketplaces/{name}` | `DELETE` | remove (admin); reports pinning projects |
| `/api/skill-marketplaces/{name}/catalog` | `GET` | plugins of one marketplace |
| `/api/projects/{id}/skill-pack` | `GET` | the pin, or none |
| `/api/projects/{id}/skill-pack/preview` | `POST` | `{marketplace, plugin}` → preview |
| `/api/projects/{id}/skill-pack` | `POST` | `{marketplace, plugin, commit}` → apply |
| `/api/projects/{id}/skill-pack` | `DELETE` | unpin |

The project-scoped ones live in `HandleProjectDetail`
(`internal/handlers/handlers.go:1007`, next to `skill-editor`), which already splits on
`parts[1]`.

### D9 — Validation rules, and how a failure surfaces

Enforced in `internal/marketplace`, reported per entry and never as a silent drop:

1. `marketplace.json` must parse and carry a non-empty `name` and at least one plugin.
2. A plugin `source` is a path relative to the marketplace root; `..`, an absolute path or a
   symlink leaving the root rejects the plugin.
3. A skill entry needs `SKILL.md` with a frontmatter block (`---\n…\n---\n`) and a non-empty
   body after it.
4. A `SKILL.md` over 256 KiB is rejected: it is a prompt, not an asset.
5. Directory names outside `WorkflowDirNames()` are `Ignored{Dir, Reason}`.
6. Zero accepted entries is an error carrying what was found, so US2's last case is not an
   empty success.

Rejections and ignores travel in the preview payload and into the activity output, which is
where the existing UI already shows a `spec_install` failure.

### D10 — Interface

`web/src/components/SkillsView.tsx` gains, above the skill list, a pack strip: the pinned
pack (`marketplace / plugin @ version · commit`, with the applied date) or a "no pack" state,
and the actions Choose, Update, Unpin. Choosing opens the preview as a panel over the same
screen — the diff per skill reuses the existing content/`repoContent` comparison the
diverged state already renders, so this is one more state on the screen the clarification
named, not a new screen. The badge row on each entry gains a `Marketplace` badge next to the
existing custom / mode / diverged badges, fed by the new `origin` field of
`SkillEditorEntry`. The registry is edited in the settings surface, in the deployment
section, next to the SDD framework.

## Data contracts

`internal/models/models.go`:

```go
type SkillMarketplace struct {
    Name          string `json:"name"`
    Kind          string `json:"kind"`    // github | git | path
    Locator       string `json:"locator"`
    Owner         string `json:"owner,omitempty"`
    Description   string `json:"description,omitempty"`
    LastCommit    string `json:"lastCommit,omitempty"`
    LastFetchedAt string `json:"lastFetchedAt,omitempty"`
}

type MarketplacePlugin struct {
    Name        string   `json:"name"`
    Source      string   `json:"source"`
    Description string   `json:"description,omitempty"`
    Version     string   `json:"version,omitempty"`
    Skills      []string `json:"skills"`            // accepted workflow directories
    Ignored     []string `json:"ignored,omitempty"` // other directories, never installed
}

type MarketplaceCatalog struct {
    Name    string              `json:"name"`
    Owner   string              `json:"owner,omitempty"`
    Commit  string              `json:"commit,omitempty"`
    Plugins []MarketplacePlugin `json:"plugins"`
}

type SkillPack struct {
    Marketplace string            `json:"marketplace"`
    Plugin      string            `json:"plugin"`
    Version     string            `json:"version,omitempty"`
    Commit      string            `json:"commit,omitempty"`
    Bodies      map[string]string `json:"bodies"`             // skill id -> pack body
    Ignored     []string          `json:"ignored,omitempty"`
    Rejected    map[string]string `json:"rejected,omitempty"` // directory -> reason
    Warnings    []string          `json:"warnings,omitempty"`
}

type SkillPackPreviewEntry struct {
    SkillID  string `json:"skillId"`
    DirName  string `json:"dirName"`
    Current  string `json:"current"`
    Proposed string `json:"proposed"`
    Changed  bool   `json:"changed"`
}

type SkillPackPreview struct {
    Pack     SkillPack               `json:"pack"`
    Entries  []SkillPackPreviewEntry `json:"entries"`
    Missing  []string                `json:"missing,omitempty"` // workflow skills the pack does not supply
}
```

`SkillEditorEntry` gains:

```go
    Origin      string `json:"origin,omitempty"`      // "builtin" or "marketplace"
    PackOrigin  string `json:"packOrigin,omitempty"`  // "<marketplace>/<plugin>@<version>+<sha7>"
```

`SkillPackPin` (the `project_skill_packs` row) is returned by `GET .../skill-pack`, with
`orphaned: true` when its marketplace is no longer registered (US1).

The TypeScript mirrors go in `web/src/types/index.ts`, and the five calls
(`fetchSkillMarketplaces`, `fetchMarketplaceCatalog`, `previewSkillPack`, `applySkillPack`,
`unpinSkillPack`) beside `fetchSkillEditor` in `web/src/context/AppContext.tsx:2578`.

## Target files

| File | Change |
|---|---|
| `internal/marketplace/marketplace.go` (new) | format parsing, plugin resolution, validation |
| `internal/marketplace/testdata/` (new) | a marketplace fixture: good plugin, unknown dirs, broken `SKILL.md` |
| `internal/agentprotocol/operations.go` | `Marketplace`, `Plugin`, `Kind`, `Locator`, `Commit` on `Operation` |
| `internal/agent/agent_operations.go` | `marketplace_catalog`, `marketplace_pack`, cache ensure/refresh |
| `internal/db/agentoperations.go` | 3-minute budget for both actions |
| `internal/db/db.go` | `skill_marketplaces` and `project_skill_packs` DDL |
| `internal/db/projectskills.go` | `pack_content` / `pack_origin` columns, `resolvedSkillBaselines`, editor origin |
| `internal/db/skilltemplates.go` | `RenderSkillWithBody`, `renderPickupSteps` over resolved bodies |
| `internal/db/skillmarketplace.go` (new) | registry CRUD, catalog, preview, apply, unpin, activity |
| `internal/handlers/handlers.go` | `/api/skill-marketplaces…` and the `skill-pack` sub-actions |
| `cmd/server/main.go` | route registration |
| `internal/models/models.go` | the structs above, `SkillEditorEntry` origin fields |
| `web/src/types/index.ts`, `web/src/context/AppContext.tsx` | types and API calls |
| `web/src/components/SkillsView.tsx` | pack strip, preview panel, marketplace badge |
| `web/src/locales/translations.ts` | the new labels, fr + en |
| `docs/adrs/0016-skills-can-come-from-a-marketplace.md` (new) | the format choice and the explicit-apply rule |

## Rejected alternatives

- **A Sectile-native manifest** (`sectile-skills.json`): recommended in round 1, dropped by
  the owner in round 2 in favour of the official Claude plugin marketplace format.
- **Installing plugins through the Claude CLI**: ties an agent-agnostic product to one CLI
  and surrenders control of the revision (D1).
- **A parallel `project_marketplace_skills` table**: the resolver already reads one row per
  (project, skill); a second table would double every read for no gain (D3).
- **Adopting `autoUpdate`**: refused deliberately — a skill body is a prompt executed with
  `--dangerously-skip-permissions` (`internal/runner/runner.go:531`).
