# Tasks #683 - Show repository images in the rendered Markdown view

Order matters: each step builds on the previous one. Tests go with the step
they cover. Behaviour in `spec.md`, choices in `plan.md`.

## 1. Shared resolver table

- [ ] T1.1 Write `internal/runner/testdata/markdown_image_targets.json`:
      relative, `./`, `../` inside the repository, `../` leaving it, leading
      `/`, query, fragment, percent-encoded, angle-bracket form as markdown-it
      reports it (`my%20shot.png`), empty, `http:`, `https:`, `data:`,
      `file:`, `mailto:`, a newline after decoding, a document at the root and
      one nested.

## 2. Agent: references and resolution

- [ ] T2.1 Add `github.com/yuin/goldmark` to `go.mod` (`go get`, `go mod
      tidy`).
- [ ] T2.2 `markdownImageTargets` in `internal/runner/markdown_images.go`
      (goldmark, table and strikethrough extensions, `*ast.Image` walk).
- [ ] T2.3 `resolveImageTarget(documentPath, target)`.
- Tests (`markdown_images_test.go`): inline, reference style, image in a
  table cell, in a list, in a link; inside a code span and a fenced block (not
  found); raw HTML `<img>` (not found); document order kept; the shared table.

## 3. Agent: sniffing

- [ ] T3.1 `sniffImage` for PNG, JPEG, GIF87a/89a, WebP and SVG.
- Tests: each signature; a PNG named `.svg`; an SVG with BOM, XML
  declaration, comment and doctype; an HTML file; a Git LFS pointer; an XML
  document whose root is not `svg`; truncated headers; an SVG declaring an
  entity (decoding stays bounded, no expansion).

## 4. Agent: reading from the snapshot

- [ ] T4.1 Contract: `diffImageLimit`, `diffImageBudget`,
      `WorktreeDiff.Images`, `DiffDocument.Images`, `DiffImageRef`,
      `DiffImage` in `worktree_diff.go`.
- [ ] T4.2 `attachDiffImages`: collection in file then reference order, keys
      `side:path`, `omitted` on the `new` side, `ls-tree` modes, size check,
      budget reservation, one `cat-file --batch`, sniffing, reasons.
- [ ] T4.3 Call it from `inspectWorktree` right after
      `attachDiffDocuments`, before the state re-checks.
- [ ] T4.4 Make the two bounds overridable in tests (package variables or a
      parameter) without changing their production values.
- Tests (`worktree_diff_test.go`, real Git repositories as the existing tests
  build them):
  - modified document, unchanged image (US1.2); modified image (US1.3);
    untracked image (US1.4); root-relative target (US1.5);
  - deleted document reads the merge-base image (US2.1); image only in the
    inspected state for a deleted document (US2.2);
  - missing path, directory, submodule, symbolic link (US3.1, US3.2);
  - unsupported content and LFS pointer (US3.4);
  - over the per-image limit (US3.5); budget exhaustion in file then
    reference order, with a shared image counted once (US3.6, FR-007);
  - image left out of the snapshot keeps the snapshot's reason (US3.7);
  - the same image referenced twice by one document and by two documents is
    carried once;
  - the files, patches and documents are byte-identical to the same
    inspection with images disabled (acceptance criterion);
  - a document with an `OmittedReason` carries no image.

## 5. Agent: capability

- [ ] T5.1 `markdownImagesCapability` in `internal/agent/agent_diff.go`,
      announced in `agent_desktop.go`.
- [ ] T5.2 Add it to the capability test in `agent_desktop_editor_test.go`.
- [ ] T5.3 Agent handler test: `/desktop/git-diff` returns `images` with
      Base64 data and the refs on the document.

## 6. Desktop: IPC and CSP

- [ ] T6.1 `markdownImages` flag in the `git-diff` handler of
      `desktop/electron/main.cjs`.
- [ ] T6.2 Add `img-src 'self' data:` to the CSP of `desktop/index.html`,
      nothing else; check the built `desktop/dist/index.html` keeps it.

## 7. Desktop: renderer

- [ ] T7.1 Image leaf carries `title`.
- [ ] T7.2 `resolveImageTarget` export, same steps as Go.
- [ ] T7.3 `image` option of `renderMarkdown`: `<img class="md-picture">`
      from `{mimeType,data}` with MIME re-check, fallback with `title=reason`,
      `onerror` fallback "This image could not be displayed."
- [ ] T7.4 Update the header comment of `markdownView.mjs`.
- Tests (`desktop/tests/markdown-view.test.mjs`): the shared JSON table
  through `resolveImageTarget`; model carries `title`; without the `image`
  option the output is unchanged (conversation view, US5.2); a refused MIME
  type falls back.

## 8. Desktop: Changes panel and style

- [ ] T8.1 `gitDiff.js`: pass the `image` lookup when `result.markdownImages`.
- [ ] T8.2 `.diff-rendered .md-picture{max-width:100%;height:auto}`.
- UI tests (`desktop/tests/git-diff.ui.cjs`, after `npx vite build`, run
  unsandboxed):
  - an image is shown with its alt and title (US1.1, US1.9);
  - a reason is the fallback's tooltip (US3);
  - a remote target falls back with no tooltip and the window makes no
    request (US3.9, acceptance criterion);
  - an undecodable payload falls back after `error` (US3.8);
  - a wide image is no wider than the view, a small one keeps its size
    (US4.1);
  - an image inside an external link opens it through `openLink` (US4.2);
  - an SVG is shown (US4.4);
  - without `markdownImages`, every image falls back with no tooltip (US5.1);
  - a 10 MiB image payload renders within the request timeout.
- `desktop/tests/markdown-view.ui.cjs`: the conversation view still shows the
  fallback for an image (US5.2).

## 9. Documentation

- [ ] T9.1 `CHANGELOG.md`: the `Added` line of `plan.md` under
      `## [Unreleased]`.
- [ ] T9.2 ADR `docs/adrs/0049-desktop-shows-repository-images-as-data-urls.md`.
- [ ] T9.3 `README.md` and `docs/`: update a description of the Changes
      panel's Rendered view if one exists.

## 10. Checks

- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/runner/...
      ./internal/agent/...` (unsandboxed for `httptest`).
- [ ] `node --test desktop/tests/markdown-view.test.mjs`.
- [ ] `cd desktop && npx vite build`, then the two UI suites, serially.
- [ ] A manual check in Desktop on a real execution whose document references
      a PNG, an SVG and a remote image.
