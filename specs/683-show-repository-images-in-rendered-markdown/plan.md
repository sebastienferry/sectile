# Plan #683 - Show repository images in the rendered Markdown view

Behaviour: `spec.md`. This file holds the implementation choices.

## Stack

- Agent (Go): `internal/runner/worktree_diff.go` (inspection),
  `internal/agent/agent_diff.go` and `internal/agent/agent_desktop.go`
  (capability).
- Desktop (vanilla JS, Electron): `desktop/electron/main.cjs` (IPC),
  `desktop/src/markdownView.mjs` (renderer), `desktop/src/gitDiff.js`
  (Changes panel), `desktop/src/style.css`, `desktop/index.html` (CSP).
- New Go dependency: `github.com/yuin/goldmark` (MIT, no transitive
  dependency), to find image references.
- No server, database, migration or tracker change.

## Why the images travel in the same response

`inspectWorktree` builds its snapshot as loose objects under a temporary
directory removed when the request ends (`defer os.RemoveAll(temp)`). The
images are therefore read in the same request, right after
`attachDiffDocuments`, before the state re-checks that already guard the
documents. A later on-demand request would need the snapshot kept alive, with
a lifetime and an eviction policy, for no visible gain (rejected).

## Data contract

Additions to `internal/runner/worktree_diff.go`:

```go
// Images referenced by the documents travel with their own bounds, apart
// from the patch and document budgets (#683).
const diffImageLimit = 2 << 20
const diffImageBudget = 8 << 20

type WorktreeDiff struct {
    // ...existing fields...
    // Images holds the repository images the documents reference, keyed by
    // "<side>:<path>" so that one image is carried once (#683).
    Images map[string]DiffImage `json:"images,omitempty"`
}

type DiffDocument struct {
    // ...existing fields...
    // Images lists, in reference order and without duplicates, the
    // repository paths the document's images resolve to.
    Images []DiffImageRef `json:"images,omitempty"`
}

type DiffImageRef struct {
    Path          string `json:"path"`            // resolved repository path
    Image         string `json:"image,omitempty"` // key into WorktreeDiff.Images
    OmittedReason string `json:"omittedReason,omitempty"`
}

type DiffImage struct {
    MimeType string `json:"mimeType"` // image/png, image/jpeg, image/gif, image/webp, image/svg+xml
    Data     []byte `json:"data"`     // Base64 in JSON (encoding/json default)
}
```

- A ref has exactly one of `Image` and `OmittedReason`.
- Only refs that resolve to a repository path are listed. A target with a
  scheme, an empty target and a target that leaves the repository are not:
  Desktop recognizes them with the same resolver (below) and renders them
  itself (no tooltip, or "This image path leaves the repository.").
- The budget is reserved per distinct key, in file order then reference order,
  at the size check, before the bytes are read. A reserved image whose content
  is then rejected (unsupported format) still consumed its reservation: this
  bounds the bytes read, not only the bytes sent.
- Response size: 8 MiB of images is about 10.7 MiB of Base64, added to the
  4 MiB patch and 4 MiB document bounds. `api()` in `main.cjs` reads JSON with
  no size cap and a 15 s timeout; keep the timeout and check it on a 10 MiB
  fixture in the UI test.

## Agent

### Finding references (`internal/runner/markdown_images.go`, new)

`markdownImageTargets(content []byte) []string` parses the document with
goldmark (CommonMark, `extension.Table` and `extension.Strikethrough`, as the
Desktop renderer enables `table` and `strikethrough`), walks the AST, and
returns the `Destination` of each `*ast.Image` in document order. Reference
definitions are resolved by goldmark itself. Raw HTML nodes are ignored.

Rejected: a hand-written scanner. CommonMark link syntax (code spans, nested
brackets, escapes, angle-bracket destinations, reference definitions inside
containers) is where such a scanner drifts from markdown-it. A drift between
goldmark and markdown-it is harmless by construction: a path Desktop looks up
and does not find falls back with no tooltip.

### Resolving a target (`resolveImageTarget(documentPath, target string) (path, reason string)`)

1. Empty, or matching `^[A-Za-z][A-Za-z0-9+.-]*:` (a scheme): no path, no
   reason (not a repository image).
2. Cut at the first `?` or `#`.
3. `url.PathUnescape`; a decoding error: no path, no reason.
4. Leading `/`: join to the root; otherwise to `path.Dir(documentPath)`.
5. `path.Clean`; a result of `..`, starting with `../`, or `.`: reason "This
   image path leaves the repository." (Desktop shows it; the agent omits the
   ref).
6. A result containing `\n` or `\x00`: ref with reason "This path cannot be
   rendered."

Parity: a shared table `internal/runner/testdata/markdown_image_targets.json`
(`[{document, target, path, reason}]`) is read by the Go test and by the
Desktop node test, so both resolvers stay identical. markdown-it percent-encodes
`src` (`mdurl.encode`) and goldmark does not; decoding first makes both inputs
equal (`<my shot.png>` → `my%20shot.png` / `my shot.png` → `my shot.png`).

### Reading (`attachDiffImages(snapshot diffGit, result *WorktreeDiff, omitted map[string]WorktreeDiffFile, ancestor, tree string) error`)

Called from `inspectWorktree` right after `attachDiffDocuments`, under the
same comment about the snapshot's lifetime.

1. For each file in `result.Files` order whose `Document` has `Content` and no
   `OmittedReason`: side `new` reads from `tree`, `old` from `ancestor`;
   collect refs (deduplicated per document by path) and distinct keys
   `side + ":" + path` (first occurrence wins the budget order).
2. On the `new` side, a path in `omitted`: reason from `omitted[path]`.
3. Modes: one `ls-tree -z <treeish> -- <paths...>` per side, through
   `snapshot.command` (already `agentexec.Hidden`). Mode `100644`/`100755`
   with type `blob`: candidate. `120000`: "Symbolic links are not followed."
   `160000`, `tree`, or absent: "Image not found in the inspected state."
   `diffGit.command` already passes `--literal-pathspecs`, so `*` or `:` in
   a name is not interpreted.
4. Sizes: `cat-file --batch-check=%(objecttype) %(objectsize)` on
   `<treeish>:<path>` for the candidates, as `attachDiffDocuments` does. Over
   `diffImageLimit`: "Image exceeds the 2 MiB display limit." Past the budget:
   "Image skipped: the 8 MiB image budget was reached." (once exhausted,
   every later key is skipped, as for documents).
5. Contents: one `cat-file --batch` for the reserved keys, parsed and checked
   like the documents (a header mismatch is `checkout_changed`).
6. Sniffing (`sniffImage(content []byte) (mime string, ok bool)`):
   - PNG `\x89PNG\r\n\x1a\n`; JPEG `\xFF\xD8\xFF`; GIF `GIF87a` / `GIF89a`;
     WebP `RIFF` + 4 bytes + `WEBP`.
   - SVG: valid UTF-8 (BOM allowed), decoded with `encoding/xml` token by
     token, skipping the declaration, comments, processing instructions,
     directives and whitespace; the first start element's local name is
     `svg`. Strict decoding, entity expansion off (`Decoder.Strict = true`,
     no custom `Entity` map), so a hostile file costs at most its 2 MiB.
   - Otherwise "This file is not a supported image." (an LFS pointer is
     text and lands here).
7. Write `result.Images[key]` and set each ref's `Image` or `OmittedReason`.

`attachDiffImages` returns an error only for `checkout_changed` and for a Git
failure of the snapshot commands; an image problem is a reason (FR-013).

### Capability

`internal/agent/agent_diff.go`:

```go
// markdownImagesCapability is announced once the git-diff result carries the
// repository images its Markdown documents reference (#683).
const markdownImagesCapability = "markdown-images"
```

Added to the capability list of `internal/agent/agent_desktop.go:173` and to
the capability test of `internal/agent/agent_desktop_editor_test.go:141`.

## Desktop

### IPC (`desktop/electron/main.cjs`, `git-diff` handler)

Add `markdownImages:!!status.capabilities.includes('markdown-images')` next to
`markdownDocuments`. When it is false, the renderer ignores any `images`
field.

### CSP (`desktop/index.html`)

`default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'none'; img-src 'self' data:`

`'self'` keeps `./assets/icon.svg` and the other bundled assets working; no
`blob:`, no remote origin. Check that the packaged build (`desktop/dist`)
keeps the meta tag as written (Vite copies `index.html`).

### Renderer (`desktop/src/markdownView.mjs`)

- Model: the image leaf gains `title` (`token.attrGet('title')`), so
  `{type:'image',src,alt,title}`.
- New export `resolveImageTarget(documentPath, src)` returning
  `{path}` | `{reason}` | `null` (not a repository image), implementing the
  same steps as the Go resolver.
- `renderMarkdown(model,{document,openLink,image})`: `image` is optional; the
  conversation view does not pass it, so its rendering is unchanged.
  `image(item)` returns `null` (fallback, no tooltip), `{reason}` (fallback
  with `title=reason`), or `{mimeType,data}`.
- Display: `<img class="md-picture">` with `alt`, `title` when present,
  `decoding="async"`, `src="data:<mimeType>;base64,<data>"`, created with
  `createElement` and properties only. The renderer re-checks `mimeType`
  against the five supported types before building the URL. `onerror`
  replaces the element with the fallback span titled "This image could not be
  displayed."
- The header comment ("no image or frame is ever created") is updated: images
  are created only from data the caller supplies, never from a URL.

### Changes panel (`desktop/src/gitDiff.js`)

In `showDocument`, when `result.markdownImages`, pass

```js
image:item=>{
 const target=resolveImageTarget(file.path,item.src)
 if(!target)return null
 if(target.reason)return target
 const ref=file.document.images?.find(r=>r.path===target.path)
 if(!ref)return null
 return ref.omittedReason?{reason:ref.omittedReason}:result.images?.[ref.image]||null
}
```

Without the capability, no `image` option is passed.

### Style (`desktop/src/style.css`)

```css
.diff-rendered .md-picture{max-width:100%;height:auto}
```

Scoped to `.diff-rendered`: the conversation view never creates one.

## Documentation

- `CHANGELOG.md`, `## [Unreleased]` / `Added`: "The rendered view of a Markdown
  file in the Desktop Changes panel shows the images the document references
  from the repository (PNG, JPEG, GIF, WebP, SVG). (#683)"
- ADR `docs/adrs/0053-desktop-shows-repository-images-as-data-urls.md`: the
  CSP gains `img-src 'self' data:`, images travel with the inspection rather
  than on demand, and goldmark is added to find references. Rejected:
  `<canvas>` with an unchanged CSP (no SVG, frozen GIFs), a kept-alive
  snapshot, a hand-written scanner.
- `README.md` / `docs/`: check whether the Changes panel is documented
  (`grep -rn "Rendered" docs README.md`); update the paragraph if it is.

## Target files

| File | Change |
| --- | --- |
| `go.mod`, `go.sum` | add `github.com/yuin/goldmark` |
| `internal/runner/worktree_diff.go` | contract, constants, call to `attachDiffImages` |
| `internal/runner/markdown_images.go` (new) | targets, resolver, reading, sniffing |
| `internal/runner/markdown_images_test.go` (new) | unit tests |
| `internal/runner/worktree_diff_test.go` | end-to-end inspection cases |
| `internal/runner/testdata/markdown_image_targets.json` (new) | shared resolver table |
| `internal/agent/agent_diff.go`, `agent_desktop.go`, `agent_desktop_editor_test.go` | capability |
| `desktop/electron/main.cjs` | `markdownImages` flag |
| `desktop/index.html` | CSP |
| `desktop/src/markdownView.mjs` | title, resolver, `image` option |
| `desktop/src/gitDiff.js` | image lookup |
| `desktop/src/style.css` | `.md-picture` |
| `desktop/tests/markdown-view.test.mjs` | model, resolver table, render |
| `desktop/tests/git-diff.ui.cjs`, `desktop/tests/markdown-view.ui.cjs` | UI cases |
| `CHANGELOG.md`, `docs/adrs/0049-…` | documentation |

## Risks

- **Response size.** Up to about 19 MiB of JSON in the worst case; parsing in
  the main process and the IPC copy are both bounded. Acceptable for a local,
  on-demand request.
- **Parser drift.** goldmark and markdown-it may disagree on an exotic image;
  the image then falls back with no tooltip. No security impact.
- **SVG.** Shown through `<img>`, where Chromium runs no script and loads no
  subresource; the CSP adds a second barrier. The agent never serves it as a
  document.
- **Test memory notes.** Desktop UI tests need `npx vite build` first and run
  unsandboxed; restore the webui `.gitkeep` if a build deleted it; Go tests
  with `httptest` run unsandboxed (see project memory).
