# Specification #683 - Show repository images in the rendered Markdown view

- Ticket: https://github.com/sebastienferry/sectile/issues/683
- Branch: `feat/683`
- Clarification: `docs/clarifications/683.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-02)
- Follow-up of #575 (rendered Markdown in the Changes panel, shipped by #677)
- Framework: Spec Kit

## Summary

In the Sectile Desktop "Changes" panel, the Rendered view of a Markdown file
shows the images the document references by a relative path to a file of the
repository. The images describe the same state as the document: the inspected
state of the checkout for a new or modified document, the baseline for a
deleted one. They travel with the inspection result from the local agent; the
window fetches nothing. An image that cannot be shown keeps today's rendering,
its alt text and path, and says why on hover.

## Scope

In scope: images written with the Markdown image syntax (inline
`![alt](path "title")` and reference style `![alt][ref]`) in a document of the
Changes panel's Rendered view; PNG, JPEG, GIF (animated), WebP and SVG files of
the repository; the size bounds; the reason shown for an image that is not
displayed; the Desktop content security policy; the changelog.

Out of scope:

- Remote images (`http:`, `https:`) and any other scheme (`data:`, `file:`):
  they keep today's rendering and are never loaded.
- Raw HTML `<img>` tags: raw HTML stays text, as today.
- Images in the conversation view, which shares the Markdown renderer: it
  keeps today's rendering for every image.
- The raw diff view, and any change to how links behave.
- The web client and the server.

## Vocabulary

- **Document**: a Markdown file (`.md`, `.markdown`) listed in the Changes
  panel whose content the agent sent (#575).
- **Side**: the state a document describes: `new` (the inspected state of the
  checkout) for an added, modified or renamed document, `old` (the merge-base)
  for a deleted one.
- **Repository image**: an image whose target, once resolved, is a path inside
  the repository.
- **Fallback**: today's rendering of an image, the text `[alt] (target)` in a
  muted italic span.
- **Image budget**: the image bytes one inspection result may carry, counted
  before encoding.

## User stories

### US1 (P1) - See the images of a changed document

As an owner reviewing an execution, I open a changed Markdown file in the
Rendered view and see its diagrams and screenshots as images, the way the
document's reader will.

1. Given a modified `docs/guide.md` containing `![Flow](images/flow.png)` and a
   PNG at `docs/images/flow.png`, when I open it rendered, then the image is
   shown in place of the reference, with `Flow` as its alternative text.
2. Given the image file is unchanged by the execution, when I open the
   document rendered, then the image is still shown (it is read from the
   inspected state, not only from the changed files).
3. Given the execution modified both the document and the image, when I open
   the document rendered, then the image shown is the modified one.
4. Given the image file is new and not yet committed, when I open the
   document rendered, then it is shown, as the document itself is.
5. Given a target starting with `/`, such as `/web/public/logo.svg`, when I
   open the document rendered, then it resolves from the repository root and
   is shown.
6. Given a reference-style image `![Logo][logo]` with `[logo]: ../logo.png`,
   when I open the document rendered, then the image is shown.
7. Given a target with a query or a fragment (`diagram.svg#dark`,
   `shot.png?raw=1`), when I open the document rendered, then the query and
   fragment are ignored and the file is shown.
8. Given a target with percent-encoded characters (`my%20shot.png`) or written
   between angle brackets (`<my shot.png>`), when I open the document
   rendered, then the file `my shot.png` is shown.
9. Given an image with a title (`![Flow](flow.png "Request flow")`), when I
   hover it, then the title is shown as its tooltip.

### US2 (P1) - A deleted document shows its own images

As an owner, I open a deleted Markdown file rendered (its old version) and see
the images as they were, not as the execution left them.

1. Given a deleted `docs/old.md` referencing `shot.png`, and `docs/shot.png`
   deleted or modified by the execution, when I open it rendered, then the
   image shown is the baseline version of `docs/shot.png`.
2. Given the image only exists in the inspected state (added by the
   execution), when I open the deleted document rendered, then the fallback is
   shown with the reason "Image not found in the inspected state."

### US3 (P1) - An image that cannot be shown says why

As an owner, I see, for an image that is not displayed, the alt text and path
as today, and on hover the reason it is not displayed.

1. Given a target to a missing file, a directory or a submodule, when I open
   the document rendered, then the fallback is shown with the tooltip "Image
   not found in the inspected state."
2. Given a target that is a symbolic link, when I open the document rendered,
   then the fallback is shown with the tooltip "Symbolic links are not
   followed." and the link is not followed.
3. Given a target whose resolution leaves the repository (`../../outside.png`
   from a document at the root), when I open the document rendered, then the
   fallback is shown with the tooltip "This image path leaves the repository."
   and nothing outside the repository is read.
4. Given a file whose content is not PNG, JPEG, GIF, WebP or SVG (whatever its
   name, a Git LFS pointer included), when I open the document rendered, then
   the fallback is shown with the tooltip "This file is not a supported
   image."
5. Given an image over 2 MiB, when I open the document rendered, then the
   fallback is shown with the tooltip "Image exceeds the 2 MiB display
   limit."
6. Given the images of the inspection exceed 8 MiB in total, when I open the
   documents rendered, then the images are taken in document order (the file
   list order), then in reference order within a document, until the budget is
   spent, and every later image shows the fallback with the tooltip "Image
   skipped: the 8 MiB image budget was reached."
7. Given an image file the inspection left out of its snapshot (over the 8 MiB
   inspection limit, past the snapshot budget), when I open the document
   rendered, then the fallback is shown with the reason the inspection gives
   for that file.
8. Given image bytes the window cannot decode, when the image fails to load,
   then the fallback replaces it with the tooltip "This image could not be
   displayed."
9. Given a remote target (`https://example.com/a.png`) or another scheme
   (`data:`, `file:`), when I open the document rendered, then the fallback is
   shown exactly as today, with no extra tooltip, and nothing is fetched.

### US4 (P2) - Layout and links

1. Given an image wider than the Rendered view, when it is shown, then it is
   scaled down to the view's width, keeping its proportions; a smaller image
   is never scaled up.
2. Given an image wrapped in an external link (`[![Badge](b.svg)](https://…)`),
   when I click it, then the link opens in the default browser, as links
   already do; an image not wrapped in a link is not clickable.
3. Given an animated GIF, when it is shown, then it plays.
4. Given an SVG image, when it is shown, then it runs no script and loads no
   external resource.

### US5 (P2) - Older agent and conversation view

1. Given Desktop connected to an agent that does not announce the image
   capability, when I open a document rendered, then every image keeps
   today's fallback, with no tooltip and no message about an update.
2. Given a conversation message containing a Markdown image, when it is
   rendered in the conversation view, then it keeps today's fallback, whatever
   the agent.

## Functional requirements

- **FR-001** The inspection result carries, for each document whose content it
  carries, the repository images the document references, read during the same
  inspection as the patch and the document.
- **FR-002** A reference is a Markdown image (inline or reference style) as
  CommonMark parses it, tables and strikethrough enabled as in the renderer.
  Raw HTML is never searched for images.
- **FR-003** A target is resolved as follows: an empty target, or one with a
  scheme, is not a repository image; the query (`?…`) and fragment (`#…`) are
  dropped; the rest is percent-decoded; a target starting with `/` is relative
  to the repository root, any other to the document's directory; `.` and `..`
  segments are applied; a result above the repository root is refused (US3.3).
  Desktop and the agent resolve a target to the same repository path.
- **FR-004** The image of a `new` document is read from the inspected state; the
  image of an `old` document from the merge-base.
- **FR-005** Only a regular file entry of the side's tree is read. A symbolic
  link, a submodule, a directory or a missing path is not; a path the
  inspection left out of its snapshot is not, on the `new` side.
- **FR-006** The format is identified from the content: PNG, JPEG, GIF, WebP
  and SVG (an XML document whose root element is `svg`). Anything else is not
  displayed. The file name plays no part.
- **FR-007** One image is at most 2 MiB. The images of one result are at most
  8 MiB, counted before encoding, apart from the patch and document bounds: an
  image never removes a file, a patch or a document from the result. An image
  referenced several times, by one or several documents of the same side, is
  carried and counted once.
- **FR-008** Each image not displayed shows the fallback with its reason as a
  tooltip, using the messages of US3. A remote or other-scheme target shows
  the fallback with no tooltip.
- **FR-009** A displayed image keeps its alternative text and title, is scaled
  down to the width of the Rendered view and never up.
- **FR-010** The Desktop content security policy allows images from the
  application itself and from `data:` URLs only (`img-src 'self' data:`).
  Every other directive is unchanged; `connect-src 'none'` stays, and remote
  images remain blocked.
- **FR-011** The agent announces the capability `markdown-images` on
  `/desktop/status`. Without it, Desktop renders every image as the fallback,
  with no tooltip.
- **FR-012** The conversation view renders images as the fallback whatever the
  agent.
- **FR-013** Reading images never fails the inspection: a problem with an image
  becomes that image's reason. A change of the checkout during the inspection
  still fails it with the existing "Refresh to retry." errors.
- **FR-014** `CHANGELOG.md` gains one `Added` line under `## [Unreleased]`.

## Acceptance criteria

- [ ] US1 to US5 hold in the Desktop UI tests and the agent tests listed in
      `tasks.md`.
- [ ] The CSP of `desktop/index.html` differs from today's by the
      `img-src 'self' data:` directive only.
- [ ] A remote image referenced by a document triggers no network request from
      the window.
- [ ] The conversation view's rendering of an image is unchanged.
- [ ] The patch, the document and the file list of an inspection with images
      are identical to those of the same inspection without the capability.
- [ ] `CHANGELOG.md` carries the `Added` line.

## Edge cases

- A document that references the same image twice carries it once and shows
  it twice.
- A renamed document resolves its images against its new path.
- A document whose content was not carried (over its own limits) carries no
  image.
- A target with a newline in its resolved path is not read: "This path cannot
  be rendered."
- An SVG without an XML declaration, with a leading byte order mark, comments
  or a doctype, is still recognized when its root element is `svg`.
- A PNG named `diagram.svg` is shown as a PNG; an HTML file named `logo.svg` is
  not shown.

## Open points

None. The three product questions of the clarification are settled (round 2).
