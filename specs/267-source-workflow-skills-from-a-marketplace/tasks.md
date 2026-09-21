# #267 — Implementation checklist

Ordered so the format parser exists before anything calls it, the resolution layer is
coherent before the interface can pin a pack, and nothing writes on a project until the
preview it is built from is proven read-only.

`make test` (`go test ./...`, then `npm test`, `tsc --noEmit`, `oxlint`) is the gate at the
end of each section.

## 1. The format — `internal/marketplace`

- [ ] **T1** Create `internal/marketplace/marketplace.go` with `ParseMarketplace`,
      `ResolvePlugin` and `WorkflowDirNames()` (D1). `WorkflowDirNames` is derived from the
      values of `models.SkillDirNames`, never re-typed.
- [ ] **T2** Implement the validation of D9: manifest shape, plugin `source` confined to the
      marketplace root, frontmatter + non-empty body, 256 KiB cap, unknown directories
      reported as `Ignored`, zero accepted entries as an error naming what was found.
- [ ] **T3** `internal/marketplace/testdata/`: one marketplace carrying a good plugin (four
      workflow skills), a plugin mixing workflow and non-workflow directories, a plugin with
      a `SKILL.md` without frontmatter, a plugin with none of the ten, and a plugin whose
      `source` points outside the root.
- [ ] **T4** `internal/marketplace/marketplace_test.go`: one case per fixture, plus
      `plugin.json` read at the plugin root and under `.claude-plugin/`, and the
      `"skills"` key overriding the default `./skills/`.

## 2. The agent — fetch and resolve on the workstation

- [ ] **T5** Add `Marketplace`, `Plugin`, `Kind`, `Locator` and `Commit` to
      `agentprotocol.Operation` (`internal/agentprotocol/operations.go`), `omitempty`, with
      the comment saying the server never dereferences them itself.
- [ ] **T6** In `internal/agent/agent_operations.go`, add `marketplace_catalog` and
      `marketplace_pack` to the action allow-list (line 94) and implement them: ensure the
      cache under `~/.taskflow/marketplaces/<sanitized-name>/`, clone or fetch for
      `github`/`git`, read a `path` kind in place, check out `Commit` when given, then call
      `internal/marketplace` and resolve the commit with `git rev-parse HEAD` (D2).
- [ ] **T7** `internal/db/agentoperations.go`: give both actions a 3-minute budget in
      `operationTimeout`, beside the `spec_install` case, and leave them out of
      `localInspections`.
- [ ] **T8** Agent tests: a `path` marketplace resolved end to end from the T3 fixtures, a
      name with a path separator refused before it reaches the cache path, and a second call
      re-using the cache without a network call.

## 3. Storage and resolution

- [ ] **T9** Add the `skill_marketplaces` and `project_skill_packs` DDL of `plan.md` to the
      schema list in `internal/db/db.go` (near line 314).
- [ ] **T10** In `ensureProjectSkillsTable` (`internal/db/projectskills.go:21`), add the
      `pack_content` and `pack_origin` columns with the same `ALTER TABLE … ADD COLUMN`
      idiom as `mode`, and read them in `projectSkillOverrides`.
- [ ] **T11** Add `RenderSkillWithBody` to `internal/db/skilltemplates.go` (D4): Sectile's
      header, the pack body under `## Project instructions` with its own frontmatter
      stripped, then the generated contracts.
- [ ] **T12** Make `renderPickupSteps` compose from the resolved bodies passed to it instead
      of reading `StageSkillByID` directly, so a pack that updates `clarify` updates
      `pickup` and `pickup_issues` too (FR8).
- [ ] **T13** Add `resolvedSkillBaselines(projectID, framework)` (D5) and start
      `EffectiveProjectSkills` from it; a pack body for `specify` or `refine_macro` applies
      to both frameworks, an absent one keeps the built-in variant (US8).
- [ ] **T14** `ListProjectSkillEditor`: `DefaultContent` becomes the resolved baseline, and
      the entry carries `Origin` / `PackOrigin`. Check that `IsCustom`, `Diverged` and the
      reset path keep their meaning (US4).
- [ ] **T15** `internal/db/skilltemplates_test.go`: an arbitrary pack body still renders the
      frontmatter, the stage line, the task-access, session-title and transition contracts;
      a pack body carrying its own frontmatter produces exactly one; `pickup` embeds the
      pack's `clarify` body.
- [ ] **T16** `internal/db/projectskills_test.go`: precedence built-in → pack → edit;
      resetting an edited skill lands on the pack body; resetting with no pack lands on the
      built-in; a pack-supplied, never-edited skill is not `isCustom`.

## 4. Registry, preview and apply

- [ ] **T17** Create `internal/db/skillmarketplace.go` with the registry CRUD
      (`ListSkillMarketplaces`, `AddSkillMarketplace`, `RemoveSkillMarketplace`), the add
      path resolving the marketplace once through the agent before it stores anything (US1).
- [ ] **T18** `MarketplaceCatalog(name)`: call `marketplace_catalog`, refresh `last_commit`
      and `last_fetched_at`, write nothing on any project.
- [ ] **T19** `PreviewSkillPack(projectID, marketplace, plugin)`: call `marketplace_pack`,
      render each body through `RenderSkillWithBody` against the project's framework, return
      `models.SkillPackPreview` with per-skill current/proposed, ignored, rejected and
      missing. No write at all (US3).
- [ ] **T20** `ApplySkillPack(projectID, marketplace, plugin, commit)`: re-resolve at the
      previewed commit, write `pack_content` / `pack_origin`, clear the bodies the new pack
      does not supply (US7), upsert `project_skill_packs`, then
      `WriteAllProjectSkillsToRepo`.
- [ ] **T21** `UnpinSkillPack(projectID)`: drop the pin, clear every `pack_content`, keep
      every `content`, reinstall.
- [ ] **T22** `recordSkillPackActivity`, modelled on `recordSpecFrameworkActivity`
      (`internal/db/specframework.go:100`), `skillId: apply_skill_pack`, output carrying the
      applied skills, the ignored directories and the rejections (D9).
- [ ] **T23** `RemoveSkillMarketplace` reports the projects that pin it and leaves their
      applied bodies alone; `GET .../skill-pack` marks such a pin `orphaned` (US1).
- [ ] **T24** `internal/db/skillmarketplace_test.go`: preview writes nothing (row count and
      content unchanged, no `sync_config` call), apply then unpin round-trips, a pack that
      drops a skill returns it to the built-in body, an offline agent surfaces the
      `callAgent` error and changes nothing (US8).

## 5. HTTP API

- [ ] **T25** Add the `/api/skill-marketplaces` routes of D8 in `internal/handlers` and
      register them in `cmd/server/main.go` beside `/api/spec-framework/...`; writes go
      through `h.requireAdmin`, reads do not.
- [ ] **T26** Add the `skill-pack` sub-actions (`GET`, `POST`, `POST /preview`, `DELETE`) to
      `HandleProjectDetail`, next to the `skill-editor` block
      (`internal/handlers/handlers.go:1007`).
- [ ] **T27** Handler tests: a member refused on a registry write and allowed on a read, an
      unknown marketplace answering 404, a preview leaving the project untouched, an apply
      answering the new pin.

## 6. Interface

- [ ] **T28** Mirror the new models in `web/src/types/index.ts` and add
      `fetchSkillMarketplaces`, `fetchMarketplaceCatalog`, `previewSkillPack`,
      `applySkillPack` and `unpinSkillPack` to `web/src/context/AppContext.tsx`.
- [ ] **T29** `SkillsView.tsx`: the pack strip (pinned coordinates and applied date, or "no
      pack") with Choose / Update / Unpin, and the `Marketplace` badge on each entry fed by
      `origin` (D10).
- [ ] **T30** The preview panel on the same screen: per-skill diff, ignored directories,
      rejected entries with their reason, skills the pack does not supply, and an Apply
      button that is the only thing that writes.
- [ ] **T31** The registry editor in the deployment section of the settings surface, next to
      the SDD framework, hidden from a member the way the other deployment settings are.
- [ ] **T32** `web/src/locales/translations.ts`: every new label in `fr` and `en`.
- [ ] **T33** `web/tests/skillPack.test.mjs`: the badge and the strip render from a pinned
      entry, and the preview panel lists ignored and missing skills.

## 7. Documentation

- [ ] **T34** `docs/adrs/0016-skills-can-come-from-a-marketplace.md`: the Claude plugin
      marketplace format, why Sectile parses it rather than driving the CLI, the
      built-in → pack → edit precedence, and why `autoUpdate` is refused.
- [ ] **T35** Update `docs/CAPABILITIES.md` and `docs/API_AND_DATA_SPEC.md` with the new
      routes, tables and vocabulary (marketplace, plugin, pack, apply) shared with #106.

## Test plan

| Level | What it proves |
|---|---|
| `internal/marketplace` unit tests (T4) | the format is read as specified, and a malformed pack is reported per entry rather than dropped |
| `skilltemplates` tests (T15) | FR7 and US5: no pack body can remove a generated contract, and composition follows the pack |
| `projectskills` tests (T16) | FR6 and US4: precedence, reset target, `isCustom` |
| `skillmarketplace` tests (T24) | US3 and US6: preview is read-only, apply is the only write, offline changes nothing |
| handler tests (T27) | D8: admin-only registry writes, project-scoped pack actions |
| agent tests (T8) | D2: cache re-use, path confinement, a `path` marketplace end to end |
| web tests (T33) | D10: origin badge and preview contents |
| manual | register a local-directory marketplace built from the T3 fixture, apply it to a scratch project, run `/clarify-issue` on a ticket and check the rendered file still carries the transition contract |
