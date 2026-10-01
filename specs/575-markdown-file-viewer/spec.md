# #575: Viewer for Markdown files

Ticket: https://github.com/sebastienferry/sectile/issues/575
Type: Feature: "When check the diff on a markdown file, a toggle button allow to see it
with markdown rendering."
Branch: `feat/575`.
Clarification: [`docs/clarifications/575.md`](../../docs/clarifications/575.md).

## Context

The Sectile Desktop "Changes" panel lists the files an execution changed and shows the
raw unified diff of the selected one. Many tickets change documentation, specifications
and changelogs, all written in Markdown, and a raw diff of a Markdown document is hard
to read as a document.

This ticket adds a toggle to the "Changes" panel that shows the selected Markdown file
rendered, as it reads at the inspected state. Out of scope: editing the file, a Markdown
preview anywhere else (web app, ticket descriptions, conversation view), rendering other
formats (HTML, AsciiDoc, MDX, notebooks), a rich diff that highlights rendered additions
and deletions, and a general-purpose file browser.

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Decisions being specified

Settled by the owner in Round 2 of the clarification, all on the recommended option.

1. **Rendered content:** the whole Markdown file at the inspected state, rendered,
   without highlighting what changed. A deleted file renders its old version, labelled
   as such.
2. **Toggle behaviour:** a Markdown file opens on the raw diff, as today. Once the reader
   switches to "Rendered", that choice sticks for every other Markdown file selected in
   the same Desktop session; it is not persisted and resets to the raw diff when Desktop
   restarts. Non-Markdown files always show the raw diff.
3. **Links and images:** `http`, `https` and `mailto` links open in the default browser.
   Relative links and in-page anchors are displayed but inert. Images are never loaded:
   each is replaced by its alt text and target path.

Technical choices resolved during the clarification and kept: Desktop "Changes" panel
only; `.md` and `.markdown` files, case-insensitive, `.mdx` excluded; raw HTML never
interpreted; the rendered content describes the same state as the diff; CommonMark plus
GFM tables, task lists, strikethrough and fenced code, without syntax highlighting.

## Definitions

- **Markdown file:** a changed entry of kind `text` whose path ends in `.md` or
  `.markdown`, compared case-insensitively. For a renamed file the new path decides.
- **Inspected state:** the state of the worktree the "Changes" panel describes when it
  last loaded: committed, staged, unstaged and untracked changes together.
- **Rendered view:** the Markdown file's whole content at the inspected state, displayed
  as a formatted document in place of the raw diff.

## User stories

### US1 (P1): Read a changed Markdown file as a document

As a reviewer in Sectile Desktop, I switch the selected Markdown file from its raw diff
to its rendered form, so that I read the document as its readers will.

- **Given** the "Changes" panel lists a modified file `docs/guide.md`,
  **When** I select it,
  **Then** the raw diff is shown, as today, and a "Rendered" toggle is offered, not
  pressed.
- **Given** the raw diff of `docs/guide.md` is shown,
  **When** I press "Rendered",
  **Then** the whole file at the inspected state is shown formatted (headings, lists,
  emphasis, tables, block quotes, code blocks), the toggle reads as pressed, and the
  file information line still names the file, its status and its counts.
- **Given** the rendered view is shown,
  **When** I press the toggle again,
  **Then** the raw diff of the same file is shown again.
- **Given** a file added, modified or renamed in the execution,
  **When** it is rendered,
  **Then** its new version is shown, including changes not yet committed or staged.
- **Given** a deleted Markdown file,
  **When** it is rendered,
  **Then** its last version before the deletion is shown, and the view states that it is
  the old version of a deleted file.
- **Given** a file named `README.MARKDOWN` or `notes.Md`,
  **When** it is selected,
  **Then** the toggle is offered.
- **Given** a selected file named `page.mdx`, `index.html` or `main.go`,
  **When** it is shown,
  **Then** no toggle is offered and the raw diff is shown.

### US2 (P1): The rendered view is safe

As a Desktop user, I can render any repository's Markdown without the document running
code, loading content or moving the Desktop window away from Sectile.

- **Given** a Markdown file containing raw HTML, such as `<script>`, `<img onerror>`,
  `<iframe>` or `<b>`,
  **When** it is rendered,
  **Then** the HTML appears as literal text and nothing in it is interpreted.
- **Given** a document with a link to `https://example.com`, `http://example.com` or
  `mailto:someone@example.com`,
  **When** I activate that link,
  **Then** it opens in the default browser or mail client, and the Desktop window stays
  on the "Changes" panel.
- **Given** a document with a relative link such as `../plan.md` or an anchor such as
  `#usage`,
  **When** it is rendered,
  **Then** the link text is displayed, marked as a link that cannot be followed, with its
  target available to read, and activating it does nothing.
- **Given** a document with a link using any other scheme, such as `javascript:`,
  `file:` or `data:`,
  **When** it is rendered,
  **Then** it is displayed like a relative link and cannot be followed.
- **Given** a document with an image, local or remote,
  **When** it is rendered,
  **Then** no image is loaded, and its alt text and target path are shown in its place.

### US3 (P2): The choice of view follows the reader

As a reviewer going through several Markdown files, I switch to the rendered view once.

- **Given** I pressed "Rendered" on one Markdown file,
  **When** I select another Markdown file, in the same execution or in another one,
  **Then** it opens rendered.
- **Given** I pressed "Rendered" on one Markdown file,
  **When** I select a non-Markdown file,
  **Then** its raw diff is shown, and selecting a Markdown file afterwards opens it
  rendered again.
- **Given** I pressed "Rendered" and refresh the "Changes" panel,
  **When** the selected Markdown file is still listed,
  **Then** it stays rendered, with the refreshed content.
- **Given** I left the rendered view on and restart Sectile Desktop,
  **When** I select a Markdown file,
  **Then** it opens on the raw diff.

### US4 (P2): A file that cannot be rendered says why

As a reviewer, I understand why a Markdown file is not rendered.

- **Given** a Markdown file whose content exceeds the rendering size limit, is not valid
  UTF-8, or could not be included within the inspection budget,
  **When** I select it,
  **Then** the toggle is disabled and the reason is shown; the raw diff, when available,
  is shown as today.
- **Given** a Markdown file whose raw diff is omitted for size but whose content is
  within the rendering limit,
  **When** I press "Rendered",
  **Then** the rendered view is shown.
- **Given** a Markdown file of kind `binary`, `symlink` or `submodule`,
  **When** it is selected,
  **Then** no toggle is offered.
- **Given** the rendered view is on for the session and the selected Markdown file cannot
  be rendered,
  **When** it is shown,
  **Then** its raw diff is shown with the reason, and the session choice is kept for the
  next Markdown file.

### US5 (P3): An older local agent degrades cleanly

As a Desktop user whose local agent predates this feature, I am told what to do.

- **Given** a local agent that does not offer Markdown content,
  **When** I select a Markdown file,
  **Then** the raw diff is shown, the toggle is disabled, and the reason reads that the
  local agent must be updated and restarted to render Markdown.
- **Given** the same agent,
  **When** I use the "Changes" panel on any file,
  **Then** everything else behaves as before this feature.

## Functional requirements

- **FR-001** The "Changes" panel offers a "Rendered" toggle only for a selected Markdown
  file, as defined above.
- **FR-002** The toggle is a button that reports its pressed state to assistive
  technology, as the Console and Changes buttons do.
- **FR-003** Pressing the toggle replaces the raw diff with the rendered view of the same
  file; pressing it again restores the raw diff. The file list and the file information
  line are unchanged.
- **FR-004** The rendered view shows the whole file at the inspected state: the new
  version for an added, modified or renamed file, the old version for a deleted one.
  It is the same state the raw diff describes, never a later read of the disk.
- **FR-005** A deleted file's rendered view is labelled as the old version of a deleted
  file.
- **FR-006** Supported syntax: CommonMark, plus GFM tables, task lists (shown as
  checkboxes that cannot be changed), strikethrough and fenced code blocks. Code blocks
  are monospaced without syntax highlighting; Mermaid and math blocks are shown as code.
- **FR-007** Raw HTML, block or inline, is shown as literal text and never interpreted.
- **FR-008** `http`, `https` and `mailto` links open outside Sectile Desktop, in the
  system's default handler. No element of the rendered view navigates the Desktop
  window.
- **FR-009** Relative links, in-page anchors and links with any other scheme are
  displayed as inert, with their target readable.
- **FR-010** Images are never loaded; each is replaced by its alt text and target.
- **FR-011** The view choice is held for the Desktop session across files, executions and
  refreshes, applies only to Markdown files, is not persisted, and starts on the raw diff
  at every Desktop start.
- **FR-012** A Markdown file whose content is not available shows a disabled toggle and
  the reason; its raw diff behaves as today.
- **FR-013** Adding Markdown content never reduces what the raw diff view lists or shows
  today: no changed file disappears from the list and no patch is omitted because of
  it.
- **FR-014** With a local agent that does not provide Markdown content, the toggle is
  disabled with a message asking to update and restart the local agent.
- **FR-015** The rendered view is scrollable and reachable from the keyboard, like the
  raw diff, and follows the Desktop light and dark appearances.

## Non-functional requirements

- **NFR-001** Rendering a Markdown file at the size limit completes without a visible
  freeze of the Desktop window.
- **NFR-002** Loading the "Changes" panel on an execution without Markdown files is not
  slower than today.

## Acceptance criteria

- **AC-001** Every Given/When/Then above passes.
- **AC-002** A malicious document (scripts, event handlers, `javascript:` links, remote
  images, iframes) renders with no script run, no network request and no navigation of
  the Desktop window.
- **AC-003** A Markdown file changed but not yet committed renders its uncommitted
  content.
- **AC-004** An execution whose raw diff stays within today's limits lists the same files
  and patches with this feature as without it.
- **AC-005** `CHANGELOG.md` has one `Added` line under `## [Unreleased]` describing the
  rendered Markdown view in the Desktop "Changes" panel.

## Open points

None. The clarification left no product question open.
