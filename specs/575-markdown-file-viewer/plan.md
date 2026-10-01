# #575: Implementation plan

Behaviour: [`spec.md`](spec.md). Checklist: [`tasks.md`](tasks.md).

## Stack and surfaces

| Layer | Files | Change |
| --- | --- | --- |
| Agent, inspection | `internal/runner/worktree_diff.go` | capture Markdown contents from the snapshot trees |
| Agent, HTTP | `internal/agent/agent_desktop.go` | advertise the `markdown-documents` capability |
| Desktop, main process | `desktop/electron/main.cjs`, `desktop/electron/preload.cjs` | pass the capability through; new `open-link` IPC |
| Desktop, renderer | `desktop/src/markdownView.mjs` (new), `desktop/src/gitDiff.js`, `desktop/src/main.js`, `desktop/src/style.css` | Markdown model and DOM builder; toggle and rendered view |
| Dependency | `desktop/package.json`, `desktop/package-lock.json` | `markdown-it` ^14 (MIT) |
| Docs | `CHANGELOG.md` | one `Added` line |

No server change, no database migration, no tracker impact, no web change.

## 1. Agent: Markdown contents in the inspection result

### Contract

`WorktreeDiffFile` gains one optional field, present only on Markdown files (kind `text`,
path ending in `.md` or `.markdown`, case-insensitive, the new path for a rename):

```go
type DiffDocument struct {
	Side          string `json:"side"`                    // "new", or "old" for a deleted file
	Content       string `json:"content,omitempty"`
	OmittedReason string `json:"omittedReason,omitempty"`
}

type WorktreeDiffFile struct {
	// existing fields unchanged
	Document *DiffDocument `json:"document,omitempty"`
}
```

`OmittedReason` is set whenever `Content` is unavailable. Older Desktop builds ignore the
field. Every user-facing reason is in English, like the existing `omittedReason` strings
of this file.

### Where the content comes from

The snapshot is a private index and object directory removed when `inspectWorktree`
returns, and uncommitted blobs exist only there. The content is therefore read inside
`inspectWorktree`, after `write-tree` and before the temporary directory is removed,
through `snapshot.command`: one `cat-file --batch-check` call reads the sizes, then one
`cat-file --batch` call reads only the blobs within the bounds below, so an oversized
blob is never read into memory:

- new side: `<tree>:<path>` where `tree` is the snapshot tree returned by `write-tree`;
- old side (status `deleted`): `<ancestor>:<path>`, the merge base the patch compares
  against.

Reading the blobs of the same trees the patch compares guarantees FR-004: both views
describe the same state, and the existing `before`/`after` stability checks already
reject an inspection whose inputs moved. `cat-file` runs with the same hardened prefix
and environment as every other call, through `agentexec.Hidden` (the existing
`diffGit.command`), so no console window opens on Windows. No filters, textconv or
hooks run: the blob is the raw worktree bytes `writeDiffObject` stored.

A path containing a newline cannot be addressed on a `--batch` line; such a file gets
`OmittedReason: "This path cannot be rendered."`.

### Bounds

- `diffDocumentLimit = 512 << 10` per document. A larger blob gets
  `"File exceeds the 512 KiB rendering limit."`. The size is read by
  `cat-file --batch-check`, and both requests ask for the documents in path order.
- `diffDocumentBudget = 4 << 20` for all documents of one response, **separate from**
  `diffResponseLimit`. The existing file-list truncation loop runs first and is left
  untouched, so FR-013 holds; documents are attached afterwards, in path order, to the
  files that survived truncation, until the budget would be exceeded. Later ones get
  `"Rendering skipped: the 4 MiB rendering budget was reached."`.
- Content that is not valid UTF-8 gets `"Non-UTF-8 contents cannot be rendered."`.
- A Markdown file listed in `omitted` (size, snapshot budget) gets the reason of its
  omission when its content was never written to the snapshot.

The worst-case response grows from 4 MiB to 8 MiB plus metadata. It travels over
loopback and Electron IPC once per refresh, which both handle.

When the execution has no Markdown file, no `cat-file` process starts (NFR-002).

An empty Markdown file carries a document with neither `Content` nor `OmittedReason`:
the absence of a reason is what tells the Desktop it can render.

### Capability

`agent_desktop.go` appends `markdownDocumentsCapability = "markdown-documents"` to the
`/desktop/status` capabilities list, next to `git-diff`.

## 2. Desktop main process

- `git-diff` handler: return `{...result, markdownDocuments: status.capabilities.includes('markdown-documents')}`.
  The status is already fetched there; no extra request.
- New `open-link` handler, exposed as `api.openLink(url)` in `preload.cjs`: parse with
  `new URL`, accept only `http:`, `https:` (without username or password) and
  `mailto:`, then `shell.openExternal(url.href)`. Anything else throws. This mirrors
  `open-pr`; the renderer filters too, the main process is the authority.
- The window guards (`will-navigate` prevented, `setWindowOpenHandler` deny) and the CSP
  in `desktop/index.html` stay as they are. `img-src` needs no change because no image
  element is ever created.

## 3. Desktop renderer

### `desktop/src/markdownView.mjs` (new)

Two exports, split so the parsing rules are testable under `node --test` without a DOM:

- `markdownModel(source)`: parses with `markdown-it` configured as
  `new MarkdownIt('commonmark', {html: false, linkify: false})` then
  `.enable(['table', 'strikethrough'])`, calls `md.parse(source, {})` and converts the
  token stream into a plain tree of `{type, children, text, href, alt, src, checked,
  align, info}` nodes. With `html: false`, raw HTML is emitted as text tokens, which
  FR-007 needs. Task lists are recognised in the converter: a list item whose first
  inline text starts with `[ ] `, `[x] ` or `[X] ` becomes `{type: 'task', checked}`.
  Each link node carries `external: true` only when its href parses as an absolute URL
  with an `http:`, `https:` or `mailto:` scheme; every other href, relative, anchor or
  other scheme, is `external: false`.
- `renderMarkdown(model, {document, openLink})`: builds DOM nodes with
  `createElement`, `textContent` and `setAttribute` only. It never assigns `innerHTML`,
  `outerHTML` or `insertAdjacentHTML`, never creates `img`, `iframe`, `script`, `style`,
  `object` or `embed`, and never sets an `href` attribute:
  - external link: `<a role="link" tabindex="0" class="md-link" title="<href>">`,
    whose click and Enter call `openLink(href)`;
  - inert link: `<span class="md-link-inert" title="<href>">` plus a muted
    ` (<href>)` suffix;
  - image: `<span class="md-image">[<alt>] (<src>)</span>`;
  - task item: a disabled `<input type="checkbox">`;
  - code block: `<pre><code>` with `textContent`.

Rejected: `marked` (no option that keeps raw HTML inert; would need a sanitizer and
`innerHTML`); `micromark` with GFM extensions (string output again, larger tree of
packages); extending the hand-written subset of `renderConversationText` (CommonMark
lists, tables and nesting are where a hand parser goes wrong); rendering the HTML string
of `markdown-it` into a sandboxed `iframe` (needs a CSP and frame-navigation story for a
gain the token walk already gives).

### `desktop/src/gitDiff.js`

- Closure state `rendered=false`, at the level of `createGitDiff`, so it survives
  `select(id)` and refreshes and dies with the window (FR-011). Never written to
  settings or `localStorage`.
- Template: a `<button type="button" class="diff-render-toggle" aria-pressed="false"
  hidden>Rendered</button>` in `.diff-detail` before `.diff-file-info`, a
  `<p class="diff-render-note" hidden>` for the reason or the "old version" label, and a
  `<div class="diff-rendered" tabindex="0" aria-label="Rendered Markdown" hidden>`
  after `.diff-patch`.
- `isMarkdown(file)`: `file.kind==='text'` and `/\.(md|markdown)$/i` on `file.path`.
- `showFile()`:
  - not Markdown: toggle hidden, rendered hidden, patch shown (current behaviour);
  - Markdown, `result.markdownDocuments` false: toggle shown and disabled, note
    "Update and restart the local agent to render Markdown.", patch shown;
  - Markdown, `file.document.omittedReason`: toggle disabled, note shows the reason,
    patch shown, `rendered` unchanged;
  - Markdown with content: toggle enabled, `aria-pressed=String(rendered)`; when
    `rendered`, hide the patch, fill `.diff-rendered` with
    `renderMarkdown(markdownModel(content), {document, openLink: api.openLink})`, and
    when `document.side==='old'` show the note "Old version: this file is deleted.".
- Toggle click: `rendered=!rendered; showFile()`.
- `clear()` empties `.diff-rendered` and hides the note and toggle.

`markdownModel` runs only for the selected file, on demand (NFR-001, NFR-002).

### `desktop/src/style.css`

`.diff-rendered` reuses the `.diff-patch` scroll box (`overflow:auto`,
`max-height:65vh`, `background:var(--bg)`, padding) with a proportional font; rules for
headings, lists, tables (borders from existing tokens), block quotes, `pre`,
`.md-link`, `.md-link-inert` and `.md-image` use only existing colour tokens so both
appearances work (FR-015). The toggle reuses `[aria-pressed="true"]` styling of the
view buttons.

## Tests

- Go, `internal/runner/worktree_diff_test.go`: a new `TestWorktreeDiffMarkdownDocuments`
  in a temporary repository covering added, committed-modified, unstaged, staged,
  untracked, renamed (`.txt` to `.md` and back), deleted (old side), `.MARKDOWN`,
  `.mdx` and `.txt` (no document), oversized (reason), non-UTF-8 (reason), budget
  exhaustion (later documents carry the budget reason, every file still listed with its
  patch), and content equal to the bytes on disk at inspection time.
- Go, `internal/agent`: the status capabilities contain `markdown-documents`, following
  `agent_desktop_editor_test.go`.
- Node, `desktop/tests/markdown-view.test.mjs`: `markdownModel` on headings, nested
  lists, tables with alignment, task lists, strikethrough, fenced code with info string,
  raw HTML as text, link classification for `https`, `http`, `mailto`, relative,
  `#anchor`, `javascript:`, `file:`, `data:`, uppercase scheme, and images.
- Electron UI, `desktop/tests/git-diff.ui.cjs` (or a new `markdown-view.ui.cjs`): stubbed
  agent payload with Markdown and non-Markdown files; toggle presence, `aria-pressed`,
  rendered content, session stickiness across files, executions and refresh, deleted
  file label, disabled toggle with reason, old agent without the capability, a hostile
  document (no `img`, `script` or `iframe` in `.diff-rendered`, no `href` attribute, no
  navigation), and an external link calling a stubbed `shell.openExternal` as
  `pr-display.ui.cjs` does. The window URL is unchanged afterwards.

UI tests need `npx vite build` first and the sandbox off (see project memory).

## Documentation

- `CHANGELOG.md`, `## [Unreleased]` → `### Added`: "**Markdown files render in the
  Desktop Changes panel.** A **Rendered** toggle shows the selected `.md` file as a
  formatted document at the inspected state, the old version for a deleted file; web
  links open in the browser and images are not loaded. (#575)"
- No ADR: the change extends the #81 inspection contract without a new architectural
  choice; the dependency choice is recorded here.
