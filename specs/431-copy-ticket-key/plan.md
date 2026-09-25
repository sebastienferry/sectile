# Plan: copy a ticket's key from its page

Behaviour is in `spec.md`; this file is how to build it.

## Stack

Web client only: React 19, TypeScript, Tailwind, `lucide-react` icons. Tests
use `node --test` for pure modules (TypeScript imported directly, as in
`web/tests/backdropDismiss.test.mjs`) and a Playwright `*.browser.mjs`
harness for the component (as in `web/tests/issue-detail-layout.browser.mjs`).
No server, API, database or desktop change.

## Architecture

### 1. A clipboard helper that reports failure

New module `web/src/lib/clipboard.ts`:

```ts
export interface ClipboardWriter { writeText(text: string): Promise<void> }

/** Writes text to the clipboard; resolves false instead of throwing when the
 *  clipboard is missing or refuses the write. */
export async function copyText(
  text: string,
  clipboard: ClipboardWriter | undefined = globalThis.navigator?.clipboard,
): Promise<boolean>
```

- `clipboard` undefined (insecure context, old browser) → `false`.
- `writeText` rejects → `false`.
- resolves → `true`.

The injectable parameter keeps it testable under `node --test` without a DOM.
`handleCopySpec` keeps its current code: changing it is out of scope (FR 9).

### 2. Copy state in `TaskDetailModal`

- One state, `copiedKey: string | null`, holding the key whose button shows
  the check mark. Two buttons, one state: the other button reverts
  immediately when a new key is copied, which FR 6 allows (only the clicked
  button changes).
- A ref holding the pending timeout. Each successful copy clears the previous
  timeout before starting a new 2 000 ms one (FR 8); the timeout is cleared on
  unmount, and `copiedKey` is reset when `selectedTask.id` changes so a check
  mark never carries over to another ticket.
- Handler `handleCopyKey(key: string)`:
  1. `const ok = await copyText(key)`;
  2. on `ok`: `setCopiedKey(key)`, restart the timeout, then
     `addToast({ type: 'success', title: 'Identifiant copié', description: `${key} a été copié dans le presse-papiers.` })`;
  3. otherwise: `addToast({ type: 'error', title: 'Copie impossible', description: "Le presse-papiers n'est pas accessible depuis ce navigateur." })`,
     no state change.

Strings stay inline in French, like `handleCopySpec` and the rest of this
component; `translations.ts` is not used by these neighbours.

### 3. The button inside `renderTaskRef()`

`renderTaskRef()` (`web/src/components/TaskDetailModal.tsx`, ~L481) is the
badge shared by the panel (~L1572) and modal (~L1784) layouts, so changing it
covers FR 1 once.

A local render helper `renderCopyKeyButton(key: string)`:

```tsx
<button
  type="button"
  onClick={() => handleCopyKey(key)}
  title={`Copier ${key}`}
  aria-label={`Copier ${key}`}
  className="ml-1 inline-flex items-center self-center rounded p-0.5 opacity-60 hover:opacity-100 hover:bg-[var(--bg-tertiary)] focus-visible:opacity-100"
>
  {copiedKey === key ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
</button>
```

(`Check` and `Copy` are already imported; `--bg-tertiary` is the hover token
the other icon buttons of this component use.)

Placement:

- Parent: right after the parent `<a>` / `<span>`, before the `/` separator,
  only inside the existing `selectedTask.parentKey &&` branch (FR 3).
- Task: right after the task `<a>` / `<span>` (after the closing `</a>`, never
  inside it, so a click cannot follow the link, FR 4). No
  `stopPropagation` is needed because the button is not a descendant of the
  anchor.

`copiedKey === key` compares strings; a parent and a task never share a key,
so the comparison identifies the button.

## Data contracts

None new. Reads `selectedTask.key` and `selectedTask.parentKey` (already on
the `Task` type). The copied text is the value as is, never trimmed or
reformatted.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/clipboard.ts` | new: `copyText` |
| `web/src/components/TaskDetailModal.tsx` | `copiedKey` state + timeout ref, `handleCopyKey`, `renderCopyKeyButton`, two insertions in `renderTaskRef()` |
| `web/tests/clipboard.test.mjs` | new: unit tests of `copyText` |
| `web/tests/task-ref-copy.browser.mjs` | new: component test in panel and modal layouts |
| `CHANGELOG.md` | one `Added` line under `[Unreleased]` |

## Test plan

### Unit (`web/tests/clipboard.test.mjs`, `node --test`)

- a writer that resolves → `true`, and it received exactly the text;
- a writer that rejects → `false`, no throw;
- `undefined` writer → `false`.

### Component (`web/tests/task-ref-copy.browser.mjs`, Playwright)

Reuse the harness of `issue-detail-layout.browser.mjs` (mocked `useApp`,
`addToast` spy recorded in `window.calls`). Run with the real clipboard
granted (`context.grantPermissions(['clipboard-read', 'clipboard-write'])`)
and read it back with `navigator.clipboard.readText()`.

For each layout (`panel`, `modal`):

1. GitHub task `#431`, parent `M-7`: two buttons named `Copier M-7` and
   `Copier #431`; the `#431` link keeps `target="_blank"` and its `href`.
2. Click `Copier #431`: clipboard is `#431`; that button contains the check
   icon, `Copier M-7` still the copy icon; `addToast` called once with
   `type: 'success'` and a description containing `#431`; no popup opened
   (`page.on('popup')` counter stays 0).
3. Click `Copier M-7`: clipboard is `M-7`.
4. After ~2.2 s, both buttons show the copy icon again.
5. Jira task `SFE-123` without `parentKey`: exactly one `Copier …` button.
6. Local task without tracker URL: its button copies its key.
7. Failure: override `navigator.clipboard.writeText` to reject before the
   click; `addToast` called with `type: 'error'`, no success toast, no check
   icon.

Browser tests need
`PLAYWRIGHT_MODULE` set to the main checkout's `#`-free Playwright path, and
worktrees may lack `node_modules` (symlink the main checkout's, then remove
it).

### Gates

`npm test`, `npx tsc -b` and `npx oxlint` in `web/`; both browser tests
(`task-ref-copy` and the existing `issue-detail-layout`, which covers the same
header) pass.

## Rejected alternatives

- **One button after the whole badge.** It would not say which key it copies
  once the parent has its own copy (clarification Round 2).
- **Making the key itself copy on click.** The key already opens the tracker;
  one element cannot do both.
- **Reusing `handleCopySpec`'s pattern as is.** It ignores the clipboard
  promise and would show a false confirmation on failure (FR 7).
- **Refactoring `handleCopySpec` onto `copyText`.** Correct but out of scope;
  it can follow separately.
