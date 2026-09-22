# #248 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Ignore rule (D1, FR1, FR2)

- [ ] T1.1 Change root `.gitignore:7` from `node_modules/` to `node_modules`.

## 2. Verification (FR1–FR4, US1, US2)

- [ ] T2.1 `git check-ignore -v --no-index desktop/node_modules web/node_modules` reports a
      rule for both paths.
- [ ] T2.2 With a temporary symlink at `desktop/node_modules`, `git status --porcelain`
      does not list it and `git check-ignore -v` reports the root rule. Remove the symlink.
- [ ] T2.3 `git ls-files | grep node_modules` is empty (FR3).
- [ ] T2.4 Fresh `git clone` of the committed branch into a temporary directory, then
      `npm ci --prefix desktop` succeeds and `desktop/node_modules` is a directory (US2).
- [ ] T2.5 `go build ./...` and `go test ./...` pass (FR4 baseline; no Go code changed).

## Test plan

No automated test is added: the owner chose the ignore rule as the only guard (Decision 3),
and there is no CI to run a check. The verification steps above are the replayable test plan.
