# Specification #736 - Separate Macro and Issue specifications folders on the workstation

- Ticket: https://github.com/sebastienferry/sectile/issues/736
- Branch: `feat/736`
- Clarification: `docs/clarifications/736.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-05)
- Supersedes in part: ADR 0027 (one specifications folder per project)
- Framework: Spec Kit

## Summary

A project's workstation settings carry two specifications folders instead of
one. The **Macro specifications folder** is the current "Specifications
folder", renamed, with its stored value: the macro skills, the macro worktree
and the slicing import keep using it. The **Issue specifications folder** is
new and empty by default: when it names another folder than the code checkout,
the issue skill-set writes its clarification reports and specifications there,
in a worktree and on a branch of the task, which the agent prepares as it does
for macros. Left empty, either folder is the project's code checkout, so an
existing workstation behaves exactly as before.

## Scope

In scope: the two Desktop settings and their effective values, the new
workstation key, the resolution of both folders, the task's specifications
worktree and branch in the Issue folder, the `SECTILE_SPEC_*` variables of a
task run, a `prepare_task_spec_worktree` MCP tool for skills typed by hand, the
issue skill fragments (clarify, specify, implement, adjust, handoff, pickup),
the task folder map, the dropped artefacts exclusion (#487) in the Issue
folder, a new ADR, the user guide, the agent contract and the changelog.

Out of scope:

- Any server-side or web setting: both folders stay workstation settings
  (ADR 0027). The one-off upload from the web is #735.
- Moving existing specifications from one folder to another.
- Where the macro skills write, and what the macro worktree does.
- Opening a pull request in the specifications repository: that is the human's
  gesture, as for macros.
- The hand-maintained marketplace plugin.

## Vocabulary

- **Code checkout**: the project's local repository on this workstation.
- **Macro folder**: the Macro specifications folder; empty means the code
  checkout.
- **Issue folder**: the Issue specifications folder; empty means the code
  checkout.
- **Distinct Issue folder**: an Issue folder that resolves to another directory
  than the code checkout.
- **Issue artefacts**: a task's clarification report
  (`docs/clarifications/<n>.md`) and specification (`specs/<KEY>-<slug>/` or
  `openspec/changes/<KEY>-<slug>/`).
- **Specifications worktree**: the checkout of the Issue folder an issue skill
  writes in for one task, on the **specifications branch**.

## User stories

### US1 (P1) - Keep every existing workstation as it is

As an owner who already set a Specifications folder, I upgrade and find it as
the Macro folder; issue skills behave as before.

1. Given a workstation whose settings carry `specPath`, when the agent starts,
   then the Desktop project settings show that value as the Macro
   specifications folder, and the Issue specifications folder is empty, its
   effective value being the local repository.
2. Given an empty Issue folder, when an issue skill runs, then
   `SECTILE_SPEC_REPO` and `SECTILE_SPEC_WORKTREE` name the task's own
   worktree, `SECTILE_SPEC_BRANCH` its branch, no other worktree is created,
   and the skill writes where it wrote before.
3. Given a Macro folder set, when a macro skill runs, when
   `prepare_macro_worktree` is called, or when the slicing import reads a
   macro's specification, then the Macro folder is used and the Issue folder is
   never read.

### US2 (P1) - Write issue specifications in their own repository

As an owner whose issue specifications live in a repository of their own, I
set it as the Issue folder and every issue skill writes there.

1. Given an Issue folder that is another Git checkout, when Sectile launches an
   issue skill on a task, then the agent prepares a worktree of that checkout
   under its `.tasks/worktrees/`, on a branch carrying the task's branch name,
   started from the up-to-date default branch of that checkout (or from
   `origin/<branch>` when it exists), and the run receives its path, branch and
   `worktree=true` through `SECTILE_SPEC_REPO`, `SECTILE_SPEC_BRANCH` and
   `SECTILE_SPEC_WORKTREE`.
2. Given the same setting, when the run starts, then the code worktree is the
   one it would have been without the setting, untouched.
3. Given a specifications worktree already present for the task, when another
   issue skill runs on it, then the tree is reused as it is, uncommitted work
   included.
4. Given worktrees turned off for the project, when an issue skill runs, then
   the Issue folder checkout is returned as it is with the branch it should be
   on and a warning, and nothing is created.
5. Given an Issue folder that is not a Git checkout, when an issue skill runs,
   then the folder itself is returned with an empty branch and a warning that
   nothing is committed or pushed.
6. Given an Issue folder whose remote cannot be fetched, when the worktree is
   prepared, then the preparation goes on from what is known locally and the
   warning says so.

### US3 (P1) - Skills typed by hand find the Issue folder

As an owner who types `/specify-issue #47` in a session of my own, I get the
same specifications worktree a Sectile launch would have given.

1. Given no `SECTILE_SPEC_*` variable in the session, when an issue skill
   starts, then it calls `prepare_task_spec_worktree` with the task key and
   writes in the `path` it returns, on its `branch`, repeating its `warning`.
2. Given a launch already prepared the worktree, when the tool is called, then
   it returns the same path and branch.
3. Given a workstation whose Issue folder is empty, when the tool is called,
   then it returns the task's code worktree and branch.
4. Given the task's project has no local agent connected for the caller, when
   the tool is called, then it fails with the same message as the other
   agent-backed tools.

### US4 (P1) - Publish the specifications branch, leave the pull request to me

1. Given an issue skill wrote artefacts in a distinct specifications worktree,
   when it commits, then it stages only the task's artefacts, commits on the
   specifications branch, and pushes it with a plain push (`-u` the first
   time), never forced, never the default branch.
2. Given the push is refused, when the skill reports, then it says so and keeps
   the commit; the stage is not blocked.
3. Given a code pull request is opened or updated for the task, when its
   description is written, then it names the specifications repository and
   branch.
4. Given a plain Issue folder, when the skill ends, then nothing is committed
   or pushed and the report says so.
5. Given the project drops its specification artefacts (#487), when the Issue
   folder is distinct, then the exclusion block is written in the Issue folder
   checkout, and the artefacts are written but never committed.

### US5 (P2) - Clean up at handoff

1. Given a distinct specifications worktree whose branch is merged into the
   default branch of the Issue folder repository, when `handoff-issue` runs,
   then it removes the worktree and the local branch.
2. Given that branch is not merged, when `handoff-issue` runs, then it keeps
   both and says so in its report, naming the branch.
3. Given a plain Issue folder or an empty one, when `handoff-issue` runs, then
   it has nothing more to clean than today.

### US6 (P1) - Set both folders in Desktop

1. Given the Desktop project settings, when I open them, then I see two rows,
   "Macro specifications folder" and "Issue specifications folder", each with
   its "live in the code repository" checkbox, folder field, browse button and
   effective value with its kind (Git repository, plain folder, not found).
2. Given I set the Issue folder and save, when I reopen the settings, then the
   value is shown and the Macro folder is unchanged, and the reverse.
3. Given I tick "live in the code repository" for one folder and save, then its
   stored value is removed and the effective value is the local repository
   again.
4. Given I attach a folder that is already the Macro or the Issue folder, then
   it is refused naming which one.

### US7 (P1) - A missing folder is refused by name

1. Given an Issue folder that no longer exists, when an issue skill is launched
   or `prepare_task_spec_worktree` is called, then it is refused with a message
   naming the Issue specifications folder setting and the path.
2. Given a Macro folder that no longer exists, when a macro operation runs,
   then it is refused with a message naming the Macro specifications folder
   setting and the path.

## Functional requirements

- **FR-001** The workstation settings keep the key `specPath` as the Macro
  folder and add `issueSpecPath` as the Issue folder, both per project,
  optional, merged by `localProjectRoot` and removed by `WriteSettings` when
  emptied. A project section holding only `issueSpecPath` is not empty.
- **FR-002** Each folder resolves the same way: empty is the code checkout, a
  relative path is relative to the code checkout, an absent directory is an
  error naming the setting and the path.
- **FR-003** Macro operations (macro launch, `macro_worktree`,
  `macro_spec_file`) resolve the Macro folder only.
- **FR-004** A task launch, the `task_spec_worktree` operation and the folder
  map resolve the Issue folder only.
- **FR-005** With a distinct Issue folder, the specifications worktree is
  prepared before the run starts, as the macro worktree is: bounded fetch,
  branch reuse, `.tasks/worktrees/<safe key>`, worktrees off, plain folder,
  per-repository lock.
- **FR-006** A task run always carries `SECTILE_SPEC_REPO`,
  `SECTILE_SPEC_BRANCH` and `SECTILE_SPEC_WORKTREE`; without a distinct Issue
  folder they repeat the task worktree, its branch and whether it is a linked
  worktree.
- **FR-007** The MCP tool `prepare_task_spec_worktree` takes a task key and
  returns `path`, `branch`, `worktree`, `warning`, `repository` (the Issue
  folder) and `distinct` (whether it is not the code checkout).
- **FR-008** The issue skill fragments write and read issue artefacts in the
  specifications worktree, commit and push it as US4 says, and handoff cleans
  it as US5 says.
- **FR-009** A task run's folder map lists a distinct Issue folder as its
  `spec` entry with the specifications worktree; the Macro folder is not listed
  in a task run.
- **FR-010** `attachFolder` refuses the Macro folder and the Issue folder,
  naming which.
- **FR-011** The Desktop project settings API returns and accepts
  `issueSpecPath`, with `issueSpecDefault` and `issueSpecKind` beside the
  existing `specPath`, `specDefault` and `specKind`.
- **FR-012** The dropped artefacts exclusion (#487) is written in the checkout
  holding the issue artefacts.

## Acceptance criteria

- [ ] The Desktop project settings show two folder fields, Macro and Issue,
      each with its browse button and its effective value.
- [ ] An existing workstation keeps its current folder as the Macro folder; the
      Issue folder is empty and issue skills behave as today.
- [ ] Macro operations read only the Macro folder.
- [ ] With an Issue folder set to another Git checkout, an issue skill run gets
      a task worktree and branch in it through `SECTILE_SPEC_*`, and the code
      worktree is unchanged.
- [ ] A skill typed by hand gets the same worktree through
      `prepare_task_spec_worktree`.
- [ ] A missing configured folder is refused with a message naming the
      setting.
- [ ] Docs, a new ADR and `CHANGELOG.md` (`Changed` / `Added`) are updated.

## Open points

None: the clarification settled every product question.
