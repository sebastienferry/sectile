# #248 — Plan

Spec: [`spec.md`](spec.md). Checklist: [`tasks.md`](tasks.md).

## Stack and target files

- `.gitignore` (root), line 7 — the only file changed.
- Read, not changed: `web/.gitignore` (already has `node_modules`), `Makefile`
  (`desktop-deps`: `cd desktop && npm ci`).

## Decisions

- **D1 — Drop the trailing slash.** Root `.gitignore:7` becomes `node_modules`. A pattern
  without a slash matches any entry of that name at any depth, whatever its type; with the
  trailing slash it matches directories only, which is the gap. This is the form
  `web/.gitignore:10` already uses. Covers FR1, FR2.
  - Rejected: an explicit `desktop/node_modules` entry. It closes one path and leaves the
    next package directory exposed.
  - Rejected: `**/node_modules`. Equivalent to `node_modules` for a slash-free pattern, only
    noisier.
- **D2 — Update the section comment** only if it describes the rule as directory-only. It
  does not (`# Node / Web Dependencies`), so it stays.
- **D3 — No history rewrite.** The symlink blob remains in past commits; it is inert.

## Side effects

Any tracked or future *file* named `node_modules` becomes ignored. `git ls-files` lists no
`node_modules` entry today (FR3), so nothing tracked changes status (FR4).

## Verification

1. `git check-ignore -v --no-index desktop/node_modules web/node_modules` reports the root
   rule for `desktop/` (and a rule for `web/`).
2. Create a symlink at `desktop/node_modules`, confirm `git status --porcelain` does not
   list it, then remove it.
3. `git ls-files | grep node_modules` is empty.
4. `git clone` the branch into a temporary directory and run `npm ci --prefix desktop`; it
   succeeds and leaves a real `desktop/node_modules` directory.
5. `go build ./...` and `go test ./...` still pass (no code change, baseline check).
