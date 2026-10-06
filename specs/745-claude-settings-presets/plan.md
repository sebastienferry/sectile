# Plan #745 - Claude settings and preset rules

Behaviour: `spec.md`. This file holds the implementation choices. It builds on
`specs/730-global-sandbox-settings/plan.md`, ADR 0048 and ADR 0050.

## Stack

- Desktop (vanilla JS, Electron): a new `desktop/src/claude-presets.mjs`,
  `desktop/src/sandbox-settings.mjs`, `desktop/src/main.js`,
  `desktop/src/style.css`.
- Agent (Go): user-visible strings only, in `internal/agent`.
- No server, database, tracker, web, settings-layout or endpoint change.

## Facts checked on 2026-10-06 (Claude Code 2.1.289, code.claude.com docs)

- `Bash(cmd *)` is the documented form, and `:*` is equivalent only at the
  end. A `*` may appear anywhere: `Bash(gcloud * delete *)` is valid. Claude
  Code warns at startup only for an *allow* rule with a `*` before the
  subcommand, and the catalogue has none.
- Deny rules match every subcommand of a compound command, past leading
  variable assignments, and apply in every permission mode. They do not
  match `sh -c '…'` or an absolute program path: a guardrail.
- `ls`, `cat`, `grep`, `find`, `head`, `wc` and read-only `git` forms run
  without a prompt in every mode, so they need no allow rule.
- In a `--settings` file, `Read(/path)` anchors at the file's directory, so
  the catalogue uses `./` (cwd) and `~/` anchors only. A `Read` deny rule from
  `--settings` also denies the sandbox's reads.
- `sandbox.network.allowedDomains` accepts a leading `*.` wildcard.
  `proxy.golang.org` serves module zips itself (HTTP 200, no redirect), so
  `storage.googleapis.com` is not needed and is not in the catalogue.
- Sandboxed commands can write the working directory and a per-user temp
  directory by default. Toolchain caches in `$HOME` need `allowWrite`.

## Rename

Labels only. The category `id` stays `'Sandbox'`, so `#settings-tab-Sandbox`,
`#project-tab-Sandbox`, `openSettings('Sandbox')` and the panel ids are
unchanged.

| File | Text today | Text after |
| --- | --- | --- |
| `desktop/src/main.js` `SETTINGS_CATEGORIES`, `PROJECT_SETTINGS_CATEGORIES` | `label:'Sandbox'` | `label:'Claude settings'` |
| `desktop/src/main.js` `workstationSandboxPanel` | `Save Sandbox settings`, `Sandbox settings saved`, `Sandbox settings are unavailable…`, `…edit the workstation Sandbox settings here.`, `Unable to read the Sandbox…` | `Claude settings` in each |
| `desktop/src/sandbox-settings.mjs` | `Projects the Sandbox values apply to`, `From the workstation Sandbox settings`, `…moved to the workstation Sandbox settings.`, `Inherited from the workstation Sandbox settings`, the scope text (`A project’s own Sandbox values`) and the two coverage texts | `Claude settings` / `the workstation Claude settings` |
| `internal/agent/agent_conversation_approvals.go` (3 lines) | `the project's Sandbox settings` | `the project's Claude settings` |
| `internal/agent/agent_claude_sandbox.go` | `the project's Sandbox settings could not be written` | `the project's Claude settings could not be written` |
| `internal/agent/agent_desktop_sandbox.go` (2) | `Invalid Sandbox settings` | `Invalid Claude settings` |
| `internal/agent/agent.go` (2 log lines) | `…workstation Sandbox settings…`, `Sandbox entry left…` | `…workstation Claude settings…`, `Claude settings entry left…` |
| `internal/agent/agent_headless.go:306` | `autorisez-le dans les réglages Sandbox du projet` | `autorisez-le dans les réglages Claude du projet` (stays French, per AGENTS.md) |
| `docs/CAPABILITIES.md` (2 lines) | `**Sandbox** category` | `**Claude settings** category` |

The comments and identifiers that say "Sandbox" stay: they name Claude Code's
sandbox values, and renaming them is churn with no reader benefit. Every
test that asserts one of these texts follows (see tasks.md).

## Catalogue module

New `desktop/src/claude-presets.mjs`, pure (no DOM), in the style of
`sandbox-settings.mjs`:

```js
// One preset: a fixed set of entries for the four lists of the workstation
// Claude settings (#745). Applying one copies its entries; nothing records it.
// {id, name, description, allow:[], deny:[], allowedDomains:[], allowWrite:[]}
export const PRESETS=[/* Common, Go, Node, Python, Java/Kotlin, Rust, .NET,
  Infrastructure, Dangerous actions: exactly spec.md › Catalogue */]
export const RECOMMENDED=['common','dangerous']
const LISTS=['allowedDomains','allowWrite','allow','deny']

// applicableLists: the lists this platform applies (#700): the rules alone on
// Windows, where Claude Code's sandbox does not run.
export function applicableLists(platformSandbox)
// presetApplied: every applicable entry of the preset is in values.
export function presetApplied(values,preset,platformSandbox)
// presetHasEntries: the preset has at least one applicable entry.
export function presetHasEntries(preset,platformSandbox)
// applyPreset: values with the missing applicable entries appended, in
// catalogue order. Never reorders or removes.
export function applyPreset(values,preset,platformSandbox)
// removePreset: values without the preset's entries, except those another
// applied preset (judged before the removal) also holds.
export function removePreset(values,preset,platformSandbox,presets=PRESETS)
// listsEmpty: the four lists are empty (the state is not considered).
export function listsEmpty(values)
```

- `values` is the panel's shape (`fromStored`): `{state, allowedDomains,
  allowWrite, allow, deny}`. The functions return a new object and never
  mutate their input.
- Entries are compared exactly, after the trim `addEntry` applies. The
  catalogue holds trimmed entries only.

## Panel

`sandboxSettings` (`desktop/src/sandbox-settings.mjs`), at workstation level
only (`workstation:true`):

- A "Presets" row, built with `settingRow('Presets',{stacked:true},box)`, is
  the first of `sections`. In `main.js` it renders right after the whitelist,
  above the state row.
- Each preset is a `<details>` element whose `<summary>` holds the name, the
  description, an "Applied" badge, and an "Apply" or "Remove" button. Its
  body lists the entries by list, as `<code>`. On Windows the body says that
  domains and writable paths do not apply. A button click does not toggle the
  `<details>` (`event.preventDefault()` on the summary click when the target
  is the button).
- "Apply recommended" is a button in the row's hint area, shown while
  `listsEmpty(values)`. It applies `RECOMMENDED` in order.
- Apply and remove compute the new `values` and push each list back into its
  editor with the existing `listEditor.set(list, inheritedEntries)`. The
  workstation has no inherited entries, so it passes `[]`. They then call
  `render()`. `render()` refreshes each preset's badge and button and the
  visibility of "Apply recommended", so a hand edit (US2.4) is reflected.
- No change to `payload`, `base`, `set` or the save flow. Presets are edits
  like any other, persisted by "Save Claude settings".

## Changelog

`CHANGELOG.md` `## [Unreleased]`:

- The #730 and #700 lines are unreleased, so they are edited in place to say
  **Claude settings** instead of **Sandbox**. No released user ever saw the
  old name, so no "renamed" line is needed.
- One new `### Added` line for the presets (#745).

## Rejected alternatives

- **Presets stored by reference** (an applied-preset id list resolved at
  launch): rejected by the owner (clarification decision 1). It also needs a
  storage key, a merge rule and a settings layout.
- **Renaming the category id and storage keys**: breaks deep links, test
  selectors and stored settings for a label change.
- **A preset JSON file the owner edits**: the lists themselves are editable,
  so a second editable source adds nothing.
- **A dedicated ADR**: the decision (copy, not reference) changes no
  architecture of ADR 0048 or ADR 0050. It is recorded here and in the
  clarification.
