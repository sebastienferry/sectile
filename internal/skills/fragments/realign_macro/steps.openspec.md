1. Start the run (see "Macro run" below), then call `prepare_macro_worktree` for `path`, `branch` and `todos`. Never write on the default branch.
2. Read the slicing and the macro's folder. An entry is a level-two group heading of `tasks.md`, or a `### Requirement:` of a capability's spec delta.
3. Compare, line by line, within the file each line came from, and classify. Each case has one answer:
   - A line whose `sourceEntry` matches an entry, with the same text: nothing to do.
   - A line whose `sourceEntry` matches an entry, with a different text: rename the entry's title, and leave its body untouched.
   - A line whose `sourceEntry` matches no entry any more: leave it, and report it as unmatched. Do not guess which entry it was.
   - A line with no `sourceKind`: add an entry for it, with a title and the minimum body OpenSpec requires. Say in the report that its body is a stub.
   - An entry of that file that no line points at any more: mark it, do not delete it. An entry of the file the slicing was not imported from is never an orphan.
4. Mark an orphan entry by appending ` (to be removed: no longer in the slicing)` to its title. Do not remove its requirements, do not empty its group.
5. Run `openspec validate <change-id> --strict` and fix only what it reports about what you wrote. A failure that predates your edit is reported, not fixed.
6. When you wrote something, commit only the macro's folder in `path` and push the macro branch:

       git -C "<path>" add -- openspec/changes/<MACRO-KEY>-<slug>
       git -C "<path>" commit -m "<MACRO-KEY>: realign the specification with the slicing"
       git -C "<path>" push -u origin HEAD

   Never `git add -A`: with worktrees off, `path` is a working checkout that may hold other changes. Push nothing when you wrote nothing, and never push the default branch. Opening the pull request is the human's gesture: say the branch is pushed and stop there.