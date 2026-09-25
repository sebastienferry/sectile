# Plan #443 - Specifications folder

Behaviour and acceptance criteria are in `spec.md`; this file holds the
implementation choices.

## Stack and boundaries

Go server (`internal/db`, `internal/handlers`, `internal/taskmcp`), Go local
agent (`internal/agent`, `internal/agentconfig`, `internal/agentprotocol`),
desktop Electron UI (`desktop/src/main.js`, vanilla JS), web React UI
(`web/src`). The server must not import the agent's filesystem helpers
(`internal/workspace` runtime boundary test); the agent must not import
`internal/db`. Shared file lookup therefore moves to a new leaf package.

## Architecture

```
web "Importer la découpe" (tasks | spec)
  └─ POST /api/projects/{id}/macros/{key}/slicing      (handlers.go)
       └─ db.TodosFromSDD(ctx, userID, project, key, source)
            └─ callAgentContext(Operation{Action:"macro_spec_file",
                   UserID, ProjectID, MacroKey, Framework, SpecFile})
                 └─ agent.executeOperation → macroSpecFileFor
                      ├─ fetchConfig → MonoRepo, SpecFramework
                      ├─ localSpecRepo(overrides, id, root, monoRepo)  ← single resolver
                      └─ sddfiles.Read(folder, framework, key, file)
                           ├─ working tree: <folder>/<root>/<key>-*/<file>
                           └─ Git folder only: macro branch  <branch>:<path>
                 ← {"content", "origin"}
            └─ ExtractTaskGroups / ExtractSpecRequirements → merge → SaveMacroMeta

desktop project settings "Specifications folder"
  ├─ GET  project info (agent desktop API) → specPath (override), specDefault, specKind
  └─ POST map-project {specPath}      → validate (abs, dir), Git top-level or as is

macro launch / prepare_macro_worktree
  └─ localSpecRepo(...) → ensureMacroWorktree(folder, …)
        Git folder  : unchanged (#426)
        plain folder: {path: folder, branch: "", worktree: false, warning}
```

## Decisions (the points the clarification left to the specification)

1. **New agent operation `macro_spec_file`.** Payload: the existing
   `Operation` fields `projectId`, `userId`, `macroKey`, `framework` (the
   project's `SpecFramework`) plus a new field `specFile` (`"tasks.md"` or
   `"spec.md"`). Answer: `{"content": "...", "origin": "<path>|<branch>:<path>"}`.
   Errors come back as the operation's `error`, in French, and are shown to the
   user verbatim. It is a read-only local inspection: 15 s budget in
   `localInspections`. The server keeps the parsing and the merge; only the file
   read moves. Rejected: sending the parsed entries from the agent (duplicates
   parsing logic on two sides and version-couples them); keeping a server-side
   read (the server is remote).
2. **Too-old and missing agents.** An agent that predates the operation answers
   `unknown local operation "macro_spec_file"`; the handler maps that text to
   « Votre app desktop Sectile est trop ancienne pour importer la découpe :
   mettez-la à jour puis réessayez. » A missing agent (`ErrNoAgentConnected`,
   or the dispatcher's "no local agent connected" error, which the operation
   path must wrap with `ErrNoAgentConnected` if it does not already) maps to
   « L'import de la découpe lit les spécifications sur votre poste : connectez
   l'app desktop Sectile puis réessayez. » The mapping lives in the handler,
   since `ErrNoAgentConnected` belongs to `internal/handlers`. No protocol
   version bump: an unknown action is already a clean refusal.
3. **Repository layout reaches the agent.** `agentconfig.Config` gains
   `MonoRepo *bool` (`json:"monoRepo,omitempty"`), filled by `DB.AgentConfig`.
   Absent (older server) reads as mono-repo, the server's own default, so a
   new agent never starts refusing against an old server. `localSpecRepo`
   becomes `localSpecRepo(overrides, projectID, root string, monoRepo bool)`:
   override → validated override; else mono-repo → `root`; else the error
   « Aucun dossier des spécifications déclaré sur ce poste pour ce projet
   multi-dépôt : renseignez « Specifications folder » dans les réglages du
   projet de l'app desktop. » The desktop reads `monoRepo` from the payload it
   already receives (`GET` project info, `project.MonoRepo`).
4. **Plain folder in `ensureMacroWorktree`.** When `git rev-parse --git-dir`
   fails, instead of refusing: return
   `macroWorkspace{Path: folder, Branch: "", Worktree: false, Warning: "…"}`
   with the warning « <folder> n'est pas un dépôt Git : la spécification est
   écrite directement dans ce dossier, sans branche, sans commit ni push. »
   No lock, no fetch, nothing created. Launch environment:
   `SECTILE_SPEC_BRANCH=""`, `SECTILE_SPEC_WORKTREE=false`. `dispatchCommand`
   receives an empty branch; verify it renders no dangling branch text.
5. **`realign-macro` with an empty branch.** Fragments
   (`read-first*.md`, `steps*.md`, `report.md`, `guard.md`) gain one rule:
   when `branch` is empty the folder is not a Git repository; write in `path`,
   skip the branch check, do not run any `git` command, do not commit or push,
   and report "not a Git repository, nothing committed". "on the macro branch"
   in the "no such folder" stop becomes "in `path` (or on the macro branch)".
   Golden files regenerated. `refine-macro` does not touch spec files and is
   unchanged.
6. **Shared lookup package `internal/sddfiles`** (new, imports only stdlib and
   `internal/models`): `SearchRoots(framework)`, `FindMacroDir(folder,
   framework, key)`, `Read(ctx, folder, framework, key, file) (content,
   origin string, err error)`, with the branch fallback (`findMacroBranch`,
   `readFromBranch`) run only when the folder is a Git checkout. Moved from
   `internal/db/sddslicing.go`, whose file-reading functions are deleted; the
   extractors (`ExtractTaskGroups`, `ExtractSpecRequirements`, …) stay in `db`.
   Refusal texts no longer point to the web option: « … ce dossier n'est pas
   celui qui les porte (le « Specifications folder » se règle dans les réglages
   du projet de l'app desktop) ». On a plain folder the message omits the
   branch hypothesis.
7. **Migration.** A new numbered migration drops `projects.spec_repo_path`
   (`ALTER TABLE projects DROP COLUMN spec_repo_path`, SQLite ≥ 3.35 and
   PostgreSQL). Its number is the next free one when implementing (14 on the
   current `main`; re-check against `origin/main` and the open PR stack). The
   baseline and migration 10 stay untouched. Tests that dropped the column to
   simulate an old schema (`migrations_test.go`, `activerun_test.go`) are
   adjusted to the new last migration.
8. **ADR.** New `docs/adrs/0027-specifications-folder-is-a-workstation-setting.md`,
   status Accepted, superseding the "The specifications repository has two
   declarations" paragraph of ADR 0026; ADR 0026 gets a one-line
   "Superseded in part by 0027" note on that paragraph.
9. **Desktop kind detection.** `GET` project info adds `specDefault` (the code
   checkout on mono-repo, `""` on multi-repo) and `specKind` for the effective
   folder: `"git"`, `"folder"`, `"missing"` or `"unset"` (multi-repo, empty).
   Detection is `git rev-parse --show-toplevel` then `os.Stat`. The UI renders
   it next to the field and refreshes it from the fresh payload after a save
   (the dialog already re-reads it). No live detection while typing.
10. **API compatibility.** `specRepoPath` disappears from `Project`,
    `CreateProjectRequest` and `UpdateProjectRequest`; Go's JSON decoding
    ignores unknown fields, so older clients still succeed (FR1).

## Data contracts

- Agent settings (unchanged shape): `specRepos: {projectId: absolutePath}`,
  overrides only.
- `agentconfig.Config`: `+ monoRepo?: boolean`.
- `agentprotocol.Operation`: `+ specFile?: string`.
- Desktop project info: `+ specDefault: string`, `+ specKind: "git"|"folder"|"missing"|"unset"`.
- `macro_worktree` answer: unchanged fields; `branch` may be `""`.
- `Project` (server API, web types): `- specRepoPath`.

## Target files

| File | Change |
| --- | --- |
| `internal/sddfiles/sddfiles.go` (new) + test | Lookup and read moved from `db/sddslicing.go`; branch fallback on Git folders only; new refusal texts. |
| `internal/db/sddslicing.go` | Delete `macroSpecRepoPath`, `FindMacroSpecDir`, `readSDDFile*`, `findMacroBranch`, `gitOutput`, `sddSearchRoots`; `TodosFromSDD(ctx, userID, projectID, key, source)` gets content via `callAgentContext`. |
| `internal/db/agentoperations.go` | `"macro_spec_file": 15s` in `localInspections`. |
| `internal/db/db.go` | Drop `spec_repo_path` from the project SELECTs, scans, INSERT, UPDATE and `UpdateProject` patch. |
| `internal/db/migrations.go` | New migration dropping the column. |
| `internal/db/agentconfig.go` | Fill `Config.MonoRepo`. |
| `internal/models/models.go` | Remove `SpecRepoPath` from `Project` and both requests. |
| `internal/handlers/handlers.go` | Slicing handler passes `r.Context()` and `h.webSessionUser(r)`; maps no-agent / too-old errors. |
| `internal/handlers/agent_dispatcher.go` | Operation path wraps `ErrNoAgentConnected` if not already. |
| `internal/agentconfig/config.go` | `MonoRepo *bool`. |
| `internal/agentconfig/local.go` | Comment of `SpecRepos`: override of the specifications folder. |
| `internal/agentprotocol/operations.go` | `SpecFile` field. |
| `internal/agent/agent_operations.go` | Route `macro_spec_file` before the task-shaped switch, like `macro_worktree`. |
| `internal/agent/agent_macro_dispatch.go` | `localSpecRepo` with `monoRepo`; `macroSpecFileFor`; callers updated. |
| `internal/agent/agent_macro_worktree.go` | Plain-folder branch (decision 4). |
| `internal/agent/agent_desktop.go` | Map-project accepts plain folders (abs + existing dir; Git top level when inside a checkout); GET adds `specDefault`, `specKind`; English errors. |
| `internal/skills/fragments/realign_macro/*` + golden files | Empty-branch rule (decision 5). |
| `internal/taskmcp/server.go` | `prepare_macro_worktree` description: "specifications folder", branch may be empty. |
| `desktop/src/main.js` | Label "Specifications folder"; placeholder = `specDefault` or "Required for a multi-repo project"; hint per layout; kind badge; required flag on multi-repo. |
| `web/src/components/ProjectModal.tsx`, `web/src/types/index.ts`, `web/src/locales/translations.ts` | Remove the field, state, payload key, type and strings. |
| `docs/contracts/server-agent-v1.md` | Document `macro_spec_file`, `monoRepo` in the config, empty `branch` of `macro_worktree`, `SECTILE_SPEC_BRANCH` empty. |
| `docs/adrs/0027-…md` (new), `docs/adrs/0026-…md` | Decision 8. |
| `CHANGELOG.md` | FR10 lines under `[Unreleased]`. |

## Risks

- Removing the column is irreversible for the stored values; accepted by the
  clarification (they name server paths).
- The web import now depends on the desktop app being connected; the error
  message says so explicitly.
- Test flakes known in this repo (keepalive, browser tests in `#` worktrees)
  are unrelated; see project memory.
