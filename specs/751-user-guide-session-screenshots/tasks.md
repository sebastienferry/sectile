# Tasks #751 - The user guide's Claude desktop session section shows two screenshots

- [x] T1 Check both captures for private content.
- [x] T2 Add them under `docs/images/user-guide/`.
- [x] T3 Reference them from `### What the session shows` with descriptive alt
  text.
- [x] T4 Verify: the image paths resolve from `docs/USER_GUIDE.md`, no em dash
  added, `git diff --check` clean.

## Test plan

- Open `docs/USER_GUIDE.md` rendered on GitHub and check that both images
  display in the subsection, at the places described in the plan.
