# ADR 0053: Desktop shows repository images as data URLs

- Status: Accepted
- Date: 2026-10-06
- Issue: [#683](https://github.com/sebastienferry/sectile/issues/683)
- Follows: #575, the rendered Markdown view of the Changes panel

## Context

The Rendered view of a Markdown file in the Desktop Changes panel showed every
image as its alt text and target. The images a document references by a
repository path should be shown as they are in the state the document
describes, without the window fetching anything: the Desktop content security
policy keeps `default-src 'self'` and `connect-src 'none'`, and the local agent
is the only reader of the checkout.

Three constraints shaped the choice. The inspection snapshot lives in a
temporary object directory removed when the git-diff request ends. The CSP
allowed no `data:` or `blob:` image. The agent and the renderer parse Markdown
with different libraries, goldmark in Go and markdown-it in Desktop.

## Decision

**The images travel with the inspection.** `attachDiffImages` runs right after
`attachDiffDocuments`, in the same request, and reads each referenced image
from the tree of its document's side: the inspected tree for a new or modified
document, the merge base for a deleted one. The result carries one `images`
map keyed by `<side>:<path>`, so an image referenced several times is read and
sent once, and each document lists the paths it references with the key or the
reason the image is not shown.

**Their own bounds.** An image over 2 MiB is never read, and the images of one
result share an 8 MiB budget, counted before Base64 and reserved in file order
then reference order, apart from the patch and document bounds. An image
problem is a reason, never a failed inspection.

**The type comes from the content.** PNG, JPEG, GIF and WebP by signature; SVG
by a strict XML decoding up to the first element, which must be `svg`, with no
entity expansion. A Git LFS pointer, an HTML file or anything else is refused,
whatever its name.

**The CSP gains `img-src 'self' data:` and nothing else.** Desktop shows an
image as an `<img>` whose source is a `data:` URL built from the bytes the agent
sent, after checking the MIME type against the five supported ones. An SVG in an
`<img>` runs no script and loads no resource. Remote images stay blocked, and
the renderer never turns a document's own target into a URL.

**goldmark finds the references, set up like markdown-it.** CommonMark with
tables and strikethrough, and without the HTML block and raw HTML parsers,
since the renderer runs with `html:false`. Both sides resolve a target with the
same steps, checked against one shared table
(`internal/runner/testdata/markdown_image_targets.json`), and against one
shared document for the references. A drift between the parsers is harmless:
an image the agent did not list keeps its alt text.

**Gated by the `markdown-images` capability.** Without it, Desktop passes no
image option to the renderer and every image keeps its alt text, as the
conversation view always does.

## Consequences

- A git-diff response can reach about 19 MiB of JSON (4 MiB of patches, 4 MiB
  of documents, about 10.7 MiB of Base64 images). It is local, on demand and
  bounded.
- `github.com/yuin/goldmark` becomes a dependency of the agent (MIT, no
  transitive dependency).
- The CSP no longer matches the one #575 recorded; any later directive change is
  a decision of its own.

## Rejected alternatives

- **Drawing decoded bitmaps on a `<canvas>` with the CSP unchanged.** No SVG,
  animated GIFs frozen on their first frame, and the alt text moved to an ARIA
  label.
- **Fetching images on demand.** The snapshot would have to outlive the
  request, with a lifetime and an eviction policy, for no visible gain; reading
  the working directory later would break the rule that an image describes the
  same state as its document.
- **A hand-written reference scanner.** Code spans, nested brackets, escapes,
  angle-bracket destinations and reference definitions are where such a scanner
  drifts from CommonMark.
