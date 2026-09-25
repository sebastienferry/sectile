# Specification #443 - Specifications folder

- Ticket: https://github.com/sebastienferry/sectile/issues/443
- Branch: `feat/443`
- Clarification: `docs/clarifications/443.md` (rounds 1 and 2, all product
  questions settled)
- Framework: Spec Kit

## Summary

The folder that carries a project's macro specifications becomes a single,
per-workstation setting, edited in the desktop project settings. The
server-side "Dépôt des spécifications" of the web project options disappears.
On a mono-repo project the setting inherits the local code checkout and can be
overridden; on a multi-repo project it must be set, and macro operations refuse
to run without it. The folder may be a Git checkout or a plain folder; Sectile
detects which and says so. The web slicing import, which read files on the
server's disk, now asks the requesting user's local agent to read them.

## Scope

In scope: the macro workflow only, meaning the slicing import, the macro
worktree, macro skill launches (`realign-macro`) and the
`prepare_macro_worktree` tool.

Out of scope:

- Task stages (clarify, specify, implement): they keep writing
  `docs/clarifications/<n>.md` and `specs/<n>-*` in the code repository, on the
  task branch, reviewed with the pull request.
- The code repository mapping (`Local repository`), the SDD framework choice
  (still a server setting), the mono-repo / multi-repo setting (still a server
  setting), and the layout of the specifications (`specs/`, `openspec/changes`).

## Definitions

- **Specifications folder**: the directory on a workstation under which the
  macro specifications are looked for (`specs/` or `openspec/changes`). It is
  either a **Git folder** (inside a Git checkout, then normalised to its top
  level) or a **plain folder** (not inside any Git checkout).
- **Inherited value**: on a mono-repo project, the local code checkout, used
  when no override is stored.
- **Override**: a folder the user explicitly chose on this workstation. Only
  overrides are stored.

## User stories (prioritised)

### US1 - One place to declare the specifications folder (P1)

As a user, I declare where a project's specifications live once, on my
workstation, in the desktop project settings, and every macro operation uses
it. The web no longer asks for a server path that nothing on my machine reads.

**Acceptance**

- **Given** the web project options of any project, **then** there is no
  "Dépôt des spécifications" field, and saving the options sends no
  specifications path.
- **Given** a server whose projects had a specifications path stored, **when**
  it is upgraded, **then** that value is discarded (it named a directory on the
  server) and no workstation setting is created from it.
- **Given** an API client that still sends `specRepoPath` on project create or
  update, **then** the field is ignored and the request succeeds.
- **Given** the desktop project settings, **then** the field is labelled
  "Specifications folder", and choosing or typing a folder then saving stores it
  for that project on this workstation only.

### US2 - Mono-repo inherits the code checkout (P1)

**Acceptance**

- **Given** a mono-repo project with no override, **when** the desktop project
  settings open, **then** the field is empty, its placeholder shows the local
  code checkout path, and the hint says the value is inherited from the local
  repository.
- **Given** a mono-repo project with no override, **when** a macro operation
  runs, **then** it uses the local code checkout, and it keeps following that
  checkout if the local repository is later changed.
- **Given** an override, **when** the user clears the field and saves, **then**
  the override is removed and the inherited value applies again.
- **Given** an override equal to the code checkout, **then** it is still stored
  as an override (no silent deduplication).

### US3 - Multi-repo requires the folder (P1)

**Acceptance**

- **Given** a multi-repo project with no folder set, **when** the desktop
  project settings open, **then** the field is flagged as required, with a
  message saying that macro operations need it; the settings can still be
  saved with the field empty.
- **Given** a multi-repo project with no folder set, **when** a macro skill is
  launched, `prepare_macro_worktree` is called, or the slicing is imported from
  the web, **then** the operation fails with a message naming the
  "Specifications folder" setting of the desktop project settings, and nothing
  falls back to the code checkout.

### US4 - A plain folder is accepted and the kind is shown (P1)

**Acceptance**

- **Given** the user saves a folder that is not inside any Git checkout,
  **then** it is accepted and stored as given (absolute, cleaned).
- **Given** the user saves a folder inside a Git checkout, **then** it is
  stored as that checkout's top level, as today.
- **Given** a relative path, or a path that does not exist or is not a
  directory, **then** saving is refused with a message saying why.
- **Given** the desktop project settings, **then** next to the field Sectile
  shows the detected kind of the effective folder: "Git repository", "Folder
  (not a Git repository)", or, when the stored folder no longer exists,
  "Not found". There is no toggle to declare the kind.
- **Given** the kind is shown, **when** the settings are saved, **then** the
  kind shown is refreshed for the saved value.

### US5 - Macro operations on a plain folder (P2)

**Acceptance**

- **Given** a plain specifications folder, **when** a macro skill is launched
  or `prepare_macro_worktree` is called, **then** no worktree and no branch are
  created, the answer (and the launch environment) names the folder itself,
  an empty branch, `worktree: false`, and a warning saying the folder is not a
  Git repository so nothing will be committed or pushed.
- **Given** a plain folder, **when** `realign-macro` runs, **then** it writes
  in the folder directly, does not check a branch, does not commit and does not
  push, and its report says so.
- **Given** a Git specifications folder, **then** the worktree-per-macro
  behaviour of #426 is unchanged (own worktree at `.tasks/worktrees/<KEY>`, on
  the macro branch, reused as is; with worktrees off, the checkout itself).

### US6 - The web slicing import reads through the local agent (P1)

**Acceptance**

- **Given** a user whose local agent is connected, **when** they import the
  slicing of a macro from its tasks.md or spec.md in the web, **then** the file
  is read from that user's specifications folder on their workstation, the
  slicing is merged as today (existing lines keep their identifier, checkbox
  and story), and the origin shown is the path actually read on the
  workstation (or `branch:path`).
- **Given** a Git specifications folder where the macro's folder is absent
  from the working tree but present on the macro branch, **then** the file is
  read from the branch, as today.
- **Given** a plain specifications folder where the macro's folder is absent,
  **then** the import fails saying no specification folder was found for the
  macro in that folder, and does not mention a branch.
- **Given** no local agent is connected for the requesting user, **then** the
  import fails with a message saying the import reads the specifications on
  the user's workstation and the desktop app must be connected.
- **Given** a connected agent too old to know the operation, **then** the
  import fails with a message saying the desktop app must be updated.
- **Given** the "stories" slicing source, **then** it is unaffected: it reads
  the tracker, not files, and needs no agent.

## Functional requirements

- **FR1** The server stores no specifications path. The web project options
  show none, and project create/update requests ignore `specRepoPath`.
- **FR2** The only declaration is the workstation's per-project setting in the
  desktop project settings, labelled "Specifications folder".
- **FR3** Resolution of the effective folder: the override when stored; else,
  for a mono-repo project, the local code checkout; else (multi-repo) none, and
  every macro operation fails with a message naming the setting.
- **FR4** A saved override must be an absolute path to an existing directory.
  A folder inside a Git checkout is normalised to that checkout's top level; a
  folder outside any Git checkout is accepted as is.
- **FR5** The desktop settings show the detected kind of the effective folder
  (Git repository, plain folder, not found), the inherited value on mono-repo,
  and a required flag on multi-repo when unset.
- **FR6** On a plain folder, macro operations create no worktree and no branch
  and report an empty branch with a warning; `realign-macro` writes without
  committing or pushing.
- **FR7** The web slicing import from tasks.md or spec.md is served by the
  requesting user's local agent, which reads the file in the effective folder
  (working tree first, then, on a Git folder only, the macro branch). Failures
  for a missing agent or an outdated agent are explicit.
- **FR8** Task stages are unchanged.
- **FR9** User-facing runtime messages stay in French on the server and web,
  and in English in the desktop UI, as each surface already speaks.
- **FR10** `CHANGELOG.md` records the change under `[Unreleased]` (`Changed`:
  the specifications folder moved to the desktop settings and accepts plain
  folders; `Removed`: the web "Dépôt des spécifications" option).

## Edge cases

- An override pointing to a folder deleted since: the desktop shows
  "Not found"; macro operations fail naming the folder, as today.
- A server newer than the agent: macro launches and `prepare_macro_worktree`
  keep working with the old agent's behaviour; only the web slicing import
  fails, with the "update the desktop app" message.
- An agent newer than a server that does not report the repository layout: the
  project is treated as mono-repo (the server's own default), so nothing that
  worked stops working.
- Several workstations of the same user: the import is served by the agent
  the server already routes that user's local operations to.

## Open requirements

None. The clarification settled every product question; the points it left to
the specification are resolved in `plan.md`.
