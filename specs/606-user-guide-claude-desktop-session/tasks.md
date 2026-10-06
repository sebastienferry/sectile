# Tasks #606 - The user guide explains how a workflow skill shows up in a Claude desktop session

- [x] T1 Re-read `internal/skills/fragments/contracts/session-title.md` on the
  merged `main` and note any wording that differs from the plan.
- [x] T2 Add `### What the session shows` at the end of **Use a prompt in
  Claude Code** in `docs/USER_GUIDE.md`: title and batch form, emoji table,
  links (GitHub / GitLab), group, chapters, diff pane, next step with the
  plugin form linked to the README, notification.
- [x] T3 In the same subsection, add the paragraph on nested skills, runs
  launched by Sectile, other CLIs and refused actions, with the Desktop guide
  pointer.
- [ ] T4 When the owner supplies them, add the two PNG captures under
  `docs/images/user-guide/`, check them for private content, and reference them
  with descriptive alt text. Otherwise report them as pending.
- [x] T5 Verify: every relative link and anchor resolves; the section renders
  (headings, table); no em dash or wording the repository's style hook flags;
  `git diff --check` is clean.

## Test plan

- Open `docs/USER_GUIDE.md` rendered (GitHub preview or Desktop's **Rendered**
  view) and read the subsection against each acceptance criterion of #606.
- Follow the README and Desktop guide links from the subsection.
- Compare each claim with `session-title.md` on `main`.
