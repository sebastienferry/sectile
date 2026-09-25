# Tasks: copy a ticket's key from its page

Ordered checklist. Each task names what proves it done. Requirement numbers
refer to `spec.md`, design to `plan.md`.

## 0. Before starting

- [ ] `git fetch` and compare `feat/431` with `origin/main`: another agent may
      have touched `renderTaskRef()` or the detail header. Merge
      `origin/main` (do not rebase a pushed spec branch).
- [ ] Check `[ -e web/node_modules ]`; if missing, symlink the main
      checkout's for the gates and remove the symlink afterwards.

## 1. Tests first (red)

- [ ] `web/tests/clipboard.test.mjs`: resolve → `true` with the exact text,
      reject → `false`, missing writer → `false`. Fails: module missing.
- [ ] `web/tests/task-ref-copy.browser.mjs`: the scenarios of `plan.md`
      § Component, in panel and modal layouts. Fails: no `Copier …` button.

## 2. Clipboard helper

- [ ] `web/src/lib/clipboard.ts`: `copyText(text, clipboard?)`.
- [ ] Check: `node --test tests/clipboard.test.mjs` passes.

## 3. Buttons on the badge (FR 1-8)

- [ ] `TaskDetailModal.tsx`: `copiedKey` state, timeout ref cleared on each
      copy, on unmount and on ticket change.
- [ ] `handleCopyKey(key)`: success toast + check mark, or error toast only.
- [ ] `renderCopyKeyButton(key)`: `type="button"`, `title` and `aria-label`
      `Copier ${key}`, `Copy`/`Check` icon.
- [ ] Insert it after the parent key (inside the `parentKey` branch, before
      `/`) and after the task key (outside the `<a>`).
- [ ] Check: the browser test passes in both layouts; the existing
      `issue-detail-layout.browser.mjs` still passes (no layout overflow at
      390 px).

## 4. Changelog (FR 10)

- [ ] `CHANGELOG.md`, `[Unreleased]` → `Added`: one line, for example
      "**Copy a ticket's key from its page.** A button next to the ticket's
      key, and one next to its parent's key, copies that key to the
      clipboard and confirms it, or says the clipboard is not available.
      (#431)"

## 5. Gates

- [ ] In `web/`: `npm test`, `npx tsc -b`, `npx oxlint`.
- [ ] Both browser tests, with `PLAYWRIGHT_MODULE` pointing at the main
      checkout's `#`-free Playwright path.
- [ ] Manual check in a running web app (never a branch server on the dev
      database): click each button, paste, verify the text and the toast.
- [ ] Working tree clean (restore `webui/.gitkeep` if a build deleted it).
