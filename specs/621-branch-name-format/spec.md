# Specification #621 - Choose the format of the task branch name

- Ticket: https://github.com/sebastienferry/sectile/issues/621 (macro M-8 "Core features")
- Branch: `feat/621`
- Clarification: `docs/clarifications/621.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

When a task gets its own worktree, Sectile creates its branch as
`feat/<lower-case key slug>` (`feat/621`, `feat/auc-1234`), and nothing can
change that. This change adds a project setting, the branch name format: a
template with placeholders that Sectile renders into the branch name of each
task that has no branch yet. A project that leaves it empty keeps today's
names exactly. A Jira project can then write `{key}` and get `AUC-1234`, the
upper-case key the Jira and GitLab development integrations recognize.

## Scope

In scope: the project setting, its storage and validation, its delivery to
the agent, the branch the agent creates for a task, the project settings
modal of the web app, the project context an agent reads, and the
documentation and skill wording that assumes `feat/<n>`.

Out of scope:

- Renaming a branch a task already has.
- Projects that run without worktrees: the agent works on the checkout's
  current branch and creates none; the format does not apply.
- Macro specification branches: they keep `<KEY>-<title slug>`.
- A workstation override of the format.
- The desktop app's settings: it has no project settings form today.

## Definitions

- **Branch name format**: the value of the project setting, a template string.
  Empty means the default format.
- **Default format**: `feat/{key_lower}`, today's behaviour.
- **Assigned branch**: the branch recorded on the task (`branch_name`). Once
  set, it wins over any format.
- **Placeholders**:
  - `{key}`: the task key with its leading `#` dropped, case preserved
    (`#621` -> `621`, `AUC-1234` -> `AUC-1234`).
  - `{key_lower}`: the same, lower-cased (`621`, `auc-1234`).
  - `{title}`: the lower-case slug of the task title, at most 30 characters.

## User stories (prioritised)

### US1 - A project keeps today's branch names by default (P1)

As an existing project owner, I change nothing and my tasks keep getting the
branches they get today.

Acceptance:

1. **Given** a project whose branch name format is empty, **when** a task
   `#621` without an assigned branch gets its worktree, **then** its branch is
   `feat/621`.
2. **Given** the same project, **when** a task `AUC-1234` without an assigned
   branch gets its worktree, **then** its branch is `feat/auc-1234`.
3. **Given** a database created before this change, **when** the server
   starts, **then** every project reads back with an empty branch name format.

### US2 - A project chooses its branch name format (P1)

As a project owner, I set a branch name format in the project settings, so
that the branches Sectile creates follow my team's convention.

Acceptance:

1. **Given** a project whose format is `{key}`, **when** a task `AUC-1234`
   without an assigned branch gets its worktree, **then** its branch is
   `AUC-1234`; for a task `#621`, it is `621`.
2. **Given** a project whose format is `feat/{key}-{title}`, **when** a task
   `AUC-1234` titled "Allow the user to choose the format" gets its worktree,
   **then** its branch is `feat/AUC-1234-` followed by the title slug, bounded
   to 30 characters and cut like macro branch slugs
   (`feat/AUC-1234-allow-the-user-to-choose-the-f`).
3. **Given** a project whose format uses `{title}`, **when** the task title
   yields no slug (only punctuation, or empty), **then** `{title}` renders
   empty and the separator left dangling before it is dropped
   (`feat/{key}-{title}` gives `feat/AUC-1234`).
4. **Given** a project owner editing the format in the settings modal, **when**
   they type or pick a format, **then** the modal shows the branch it would
   give for a sample task, and offers the presets `feat/{key_lower}`
   (default), `{key}` and `feat/{key}-{title}`.
5. **Given** a saved format, **when** an agent reads the project context,
   **then** it carries the format as `branchNameFormat`.

### US3 - An unusable format is refused on save (P1)

As a project owner, I cannot save a format that would give tasks colliding or
invalid branches.

Acceptance:

1. **Given** a format containing neither `{key}` nor `{key_lower}`, **when**
   the project is saved (created or updated), **then** the save is refused
   with a 400 and a French message, and the stored format is unchanged.
2. **Given** a format with an unknown placeholder (`{id}`) or an unbalanced
   brace, **when** the project is saved, **then** it is refused the same way.
3. **Given** a format whose render for a sample task is not a valid Git
   branch name (for example `feat//{key}`, `{key}.lock`, `feat {key}`), or is
   `main`, `master` or `HEAD`, **when** the project is saved, **then** it is
   refused the same way.
4. **Given** a format made of spaces only, **when** the project is saved,
   **then** it is stored empty, which is the default format.

### US4 - Assigned branches are never renamed (P1)

As a developer with work in progress, I change the project format and my
running tasks keep their branch, worktree and pull request.

Acceptance:

1. **Given** a task whose branch `feat/621` is assigned, **when** the project
   format changes to `{key}`, **then** the task's next worktree preparation
   still uses `feat/621`.
2. **Given** a task without an assigned branch, **when** its worktree is
   prepared under a new format, **then** the rendered branch is recorded on
   the task and used by every later stage.

### US5 - The prompt and the branch agree (P2)

As a skill run, I am told the branch that was actually created.

Acceptance:

1. **Given** a project with a custom format and a task without an assigned
   branch, **when** a launch prepares the worktree and expands `{branchName}`
   in the command template, **then** it expands to the branch the worktree
   carries.
2. **Given** the server's fallback prompt for a task without an assigned
   branch, **when** it expands `{branchName}`, **then** it gives the default
   format's branch, not the legacy `<key>-<title>` name.

### US6 - Macro branches are unchanged (P3)

1. **Given** any branch name format, **when** a macro specification worktree is
   prepared, **then** its branch is `<KEY>-<title slug>` as today.

## Functional requirements

- FR-1. A project stores a branch name format, empty by default. Creating and
  updating a project accept it; reading a project returns it.
- FR-2. The format is trimmed on save; an empty result is the default format.
- FR-3. A non-empty format is refused on save unless: every `{...}` is one of
  `{key}`, `{key_lower}`, `{title}`; braces are balanced; it contains `{key}`
  or `{key_lower}`; its render for the sample task `AUC-1234` titled "Sample
  title" is a valid Git branch name and is not `main`, `master` or `HEAD`.
- FR-4. The refusal answers 400 with a French message naming the rule broken.
- FR-5. The format reaches the agent in the project configuration it already
  receives. An agent that predates it ignores it and keeps `feat/<key>`; a
  server that predates it sends nothing, which reads as the default format.
- FR-6. The agent renders the format only for a task without an assigned
  branch. The rendered branch still goes through `git check-ref-format
  --branch` before the worktree is created, as today.
- FR-7. `{key_lower}` renders exactly today's key slug, so the default format
  renders byte for byte what Sectile renders today.
- FR-8. The project context returned to agents (`get_project_context`)
  includes `branchNameFormat`.
- FR-9. The web project settings modal edits the format, with the presets and
  a live example; its text exists in French and English like the rest of the
  modal.
- FR-10. Macro branches, projects without worktrees and assigned branches
  behave as today.
- FR-11. `CHANGELOG.md` gets an `Added` line under `[Unreleased]`.
- FR-12. Skill and documentation wording that says "the assigned `feat/<n>`
  branch" says "the assigned branch".

## Success criteria

- A project that sets nothing sees no branch name change (existing branch
  tests pass unchanged).
- A Jira project with `{key}` gets `AUC-1234` branches.
- An invalid format never reaches the database.

## Open points

None. Every product question was settled in the clarification (Q1-Q4).
