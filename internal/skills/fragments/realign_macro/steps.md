1. Start the run (see "Macro run" below), then call `prepare_macro_worktree` for `path`, `branch` and `todos`. Never write on the default branch.
2. Read the slicing and the macro's folder. An entry is a numbered user story under the user stories heading of `spec.md`, or a level-two group heading of `tasks.md`.
3. Compare, line by line, within the file each line came from, and classify. Each case has one answer:
   - A line whose `sourceEntry` matches an entry, with the same text: nothing to do.
   - A line whose `sourceEntry` matches an entry, with a different text: rename the entry's title, and leave its body untouched.
   - A line whose `sourceEntry` matches no entry any more: leave it, and report it as unmatched. Do not guess which entry it was.
   - A line with no `sourceKind`: add an entry for it, with a title and the minimum body Spec Kit requires. Say in the report that its body is a stub.
   - An entry of that file that no line points at any more: mark it, do not delete it. An entry of the file the slicing was not imported from is never an orphan.
4. Mark an orphan entry by appending ` (to be removed: no longer in the slicing)` to its title. Do not remove its acceptance criteria, do not empty its group.
5. Keep the numbering and the priorities consistent: an added story takes the next free number, and a marked one keeps the one it had. Never renumber the file: that would break every reference to it.
6. When you wrote something, commit only the macro's folder in `path` and push the macro branch:

       git -C "<path>" add -- specs/<MACRO-KEY>-<slug>
       git -C "<path>" commit -m "<MACRO-KEY>: realign the specification with the slicing"
       git -C "<path>" push -u origin HEAD

   Never `git add -A`: with worktrees off, `path` is a working checkout that may hold other changes. Push nothing when you wrote nothing, and never push the default branch. Opening the pull request is the human's gesture: say the branch is pushed and stop there.