# Specification #710 - Pick the changed file from a drop-down

- Ticket: https://github.com/sebastienferry/sectile/issues/710
- Branch: `feat/710`
- Clarification: `docs/clarifications/710.md` (round 1, no product question)
- Framework: Spec Kit
- Type: change

## Summary

In Sectile Desktop's **Changes** panel, the changed files of the selected
execution are chosen from one drop-down instead of a stack of buttons. The
diff of the chosen file then takes the panel's full width, in the full view as
in the split view, whatever the number of changed files.

## Scope

In scope: the file list of Desktop's Changes panel, its styles, the two UI
tests that pick a file, and the changelog.

Out of scope:

- The content and order of the file list returned by the agent.
- The diff and Markdown rendering of the chosen file, the **Rendered** toggle.
- The console/changes split and its divider.
- The web app, which has no Changes panel.
- New navigation: previous/next file buttons, a file filter or search.

## Vocabulary

- **File picker**: the drop-down that lists the changed files, labelled
  "Changed file".
- **Option label**: the text of one entry, `[oldPath → ]path · status`, the
  same text as today's button.
- **Chosen file**: the file whose diff the panel shows.

## User stories

### US1 (P1) - Choose a file from the drop-down

As an owner reviewing an execution's changes, I pick a file from one
drop-down, and its diff fills the panel's width.

1. Given an execution with changed files, when Changes loads, then the file
   picker lists every changed file in the agent's order, the first one is
   chosen, and its diff is shown.
2. Given the file picker, when I choose another file, with the mouse or the
   keyboard, then that file's information line and diff are shown.
3. Given a renamed file, then its option label reads `old → new · renamed`.
4. Given the full view or the split view, then the diff starts at the
   panel's left edge, with no file list beside it.

### US2 (P1) - Keep the choice across a refresh

1. Given a chosen file, when I press **Refresh** and the file is still
   changed, then it stays chosen.
2. Given a chosen file, when I press **Refresh** and it is no longer changed,
   then the first file is chosen.
3. Given a chosen file, when I select another execution, then the first file
   of that execution is chosen.

### US3 (P2) - No changed file

1. Given a clean worktree, an error or a disconnected agent, then the file
   picker is hidden, not shown empty.

## Functional requirements

- FR1. The Changes panel shows one native drop-down, labelled "Changed
  file" for assistive technologies, in place of the stack of file buttons.
- FR2. It has one option per changed file, in the order the agent returns
  them, each labelled `[oldPath → ]path · status`, its value being the path.
- FR3. Its value is the chosen file; changing it shows that file, as clicking
  a button did.
- FR4. The choice survives a refresh while the path is still changed, else
  falls back to the first file; selecting another execution resets it.
- FR5. It sits on its own row between the summary line and the diff, spans
  the panel's width, and truncates a long label rather than widening the
  panel.
- FR6. It is hidden while there is no changed file to list.
- FR7. The diff, the rendered Markdown and the file information line take
  the panel's full width in the full and the split views.
- FR8. `CHANGELOG.md` has a `Changed` line under `[Unreleased]`.

## Success criteria

- The two Changes UI tests pick files through the drop-down and pass.
- At 700 px wide, the Changes panel does not scroll horizontally.

## Open points

None.
