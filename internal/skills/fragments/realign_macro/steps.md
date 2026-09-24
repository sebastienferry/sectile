1. Start the run (see "Macro run" below), then resolve where to write: `SECTILE_SPEC_REPO` and `SECTILE_SPEC_BRANCH`, or the answer of `prepare_macro_worktree`. Never write on the default branch.
2. Read the slicing and the specification folder. An entry is a numbered user story under the user stories heading of `spec.md`, or a level-two group heading of `tasks.md`.
3. Compare, line by line, and classify. There are exactly four cases, and each one has one answer:
   - A line whose `sourceEntry` matches an entry, with the same text: nothing to do.
   - A line whose `sourceEntry` matches an entry, with a different text: rename the entry's title, and leave its body untouched.
   - A line with no `sourceKind`: add an entry for it, with a title and the minimum body Spec Kit requires. Say in the report that its body is a stub.
   - An entry no line points at any more: mark it, do not delete it.
4. Mark an orphan entry by appending ` (to be removed: no longer in the slicing)` to its title. Do not remove its acceptance criteria, do not empty its group.
5. Keep the numbering and the priorities consistent: an added story takes the next free number, and a marked one keeps the one it had. Never renumber the file: that would break every reference to it.
6. When you wrote something, commit it in `SECTILE_SPEC_REPO` and push the macro branch:

       git -C "${SECTILE_SPEC_REPO}" add -A
       git -C "${SECTILE_SPEC_REPO}" commit -m "<MACRO-KEY>: realign the specification with the slicing"
       git -C "${SECTILE_SPEC_REPO}" push -u origin HEAD

   Push nothing when you wrote nothing, and never push the default branch. Opening the pull request is the human's gesture: say the branch is pushed and stop there.