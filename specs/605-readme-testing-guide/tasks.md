# Tasks #605 - README lists the testing guide

Ordered checklist. One commit.

## 1. README (FR1, FR2, FR3, FR4)

- [ ] T1.1 `README.md`: insert the Testing guide bullet after UX components
  and before Desktop guide.

## 2. Verification

- [ ] T2.1 `git diff --stat origin/main -- README.md` shows 2 insertions and
  0 deletions.
- [ ] T2.2 `test -f docs/TESTING.md` succeeds (link target exists).
- [ ] T2.3 No line of the new bullet exceeds 80 columns.
