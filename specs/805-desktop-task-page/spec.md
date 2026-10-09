# Spec #805 - Desktop: Add task / Edit task

Ticket: https://github.com/sebastienferry/sectile/issues/805
Clarification: `docs/clarifications/805.md` (rounds 1 and 2, settled)
Branch: `feat/805`

## Problem

Sectile Desktop creates a task only through a small modal (project, title, a
plain-text description) and cannot edit a task at all: changing a title, a
description, an assignee or a pull request link means opening the web app. The
owner wants one full page, Notion-like, to create and to edit a task, with a
rich Markdown description, its pull request links and its assignee.

## Scope

In: one **task page** shown in the Desktop workspace, in a creation mode and an
edition mode; a WYSIWYG Markdown description editor with a raw Markdown toggle;
the assignee; the ordered pull request links; the project and tracker at
creation; explicit save with an unsaved-changes guard; the entry points
(Cmd+N / Ctrl+N, palette "New task", the Tickets pane row menu, the run
header); the removal of the quick add dialog; the agent routes and capability
the page needs; tests and user documentation.

Out: the web task modal; every other task field (status, stage, priority,
labels, due date, sprint, team, epic, branch, repository); comments; moving a
task to another project; autosave; conflict detection between concurrent
editors (last write wins); the sidebar pencil that renames a local run (it keeps
renaming the local name only); `markdownView.mjs` and the Changes panel.

## User stories

### US1 - Create a task in a page (P1)

- **US1.1** Given the agent is connected, when I press Cmd+N (macOS) or Ctrl+N
  (other platforms), or pick **New task** in the command palette, or press the
  project's create button, then the workspace shows the task page in creation
  mode, titled **New task**. No modal dialog opens.
- **US1.2** The page shows, top to bottom: **Project** (a select of the
  configured projects), **Tracker** (only when the chosen project has more than
  one tracker), **Title**, **Assignee**, **Pull requests** and **Description**.
- **US1.3** The project starts on: the project the entry point was opened from,
  else the selected project, else the one last used for a creation, else the
  only configured project, else none ("Select a project"). The tracker starts
  on the project's first tracker.
- **US1.4** Focus starts in **Title** when a project is preselected, in
  **Project** otherwise.
- **US1.5** Given a title and a project, when I press **Create task** or
  Cmd+Enter / Ctrl+Enter, then the task is created on the server and its
  tracker with the title, the description and the chosen tracker; when an
  assignee or pull request links were entered, they are then saved on the
  created task. While this runs the page says
  `Creating the task on the server and its tracker…` and the save controls are
  disabled.
- **US1.6** Given the creation succeeds, then the page turns into the edition
  page of the created task, shows `Created <key> · <title>` and offers
  **Clarify now** (default action), **Launch task**, **Add another** and
  **Done**, with the same effects as the quick add dialog had: Clarify now
  launches `clarify` on the task; Launch task opens the Tickets pane of the
  project filtered on the task; Add another opens a new creation page on the
  same project; Done leaves the page for the view shown before it opened.
- **US1.7** Given the creation fails, then nothing is lost: the page stays in
  creation mode with every field as typed, shows the error, and the save
  controls are usable again.
- **US1.8** Given the task was created but saving its assignee or pull request
  links afterwards failed, then the page is in edition mode on the created task,
  says `Created <key>, but <assignee|pull requests> could not be saved: <error>`,
  and keeps those fields marked as unsaved, so **Save** retries them.
- **US1.9** Without a title or a project the create action does nothing and the
  missing field is marked required.

### US2 - Edit a task in the same page (P1)

- **US2.1** Given the Tickets pane is open, when I open a row's **…** menu, then
  it offers **Edit task**, which shows the task page in edition mode for that
  task.
- **US2.2** Given the current run belongs to a server task (not a free console,
  not a macro run), then the run header offers **Edit task**, with the same
  effect.
- **US2.3** The edition page is titled with the task key and shows **Title**,
  **Assignee**, **Pull requests** and **Description**, loaded fresh from the
  server when the page opens, never from a cached list. The project and the
  tracker are shown read-only. A link opens the task's tracker page when it has
  one.
- **US2.4** While the task loads, the page says so; when it cannot be loaded,
  the page shows the error and a **Retry** action, and no field is editable.
- **US2.5** When I press **Save** or Cmd+Enter / Ctrl+Enter, then only the
  fields I changed are sent. A save with no change sends nothing and is not
  offered (the button is disabled).
- **US2.6** Given the save succeeds, then the page reloads the saved task, says
  `Saved`, and the unsaved mark disappears. An open Tickets pane of the project
  shows the new title and assignee the next time it renders its rows.
- **US2.7** Given the save fails, then the edits stay on the page, the error is
  shown, and the unsaved mark stays.

### US3 - Rich Markdown description (P1)

- **US3.1** The description is edited WYSIWYG: formatting renders as I type
  (Markdown input rules such as `# `, `- `, `1. `, `[ ] `, `> `, ``` ``` ```,
  `**bold**`), and is stored as Markdown.
- **US3.2** The supported content is: headings 1 to 6, paragraphs, bold, italic,
  strikethrough, inline code, code blocks, bullet, numbered and task lists
  (nested), block quotes, links, tables and horizontal rules. Nothing that
  Markdown cannot store is offered (no embeds, colours, callouts, columns).
- **US3.3** Typing `/` at the start of an empty block opens a block menu
  (Text, Heading 1-3, Bullet list, Numbered list, Task list, Quote, Code block,
  Table, Divider), filtered by what I type after `/`, usable with the arrow keys,
  Enter and Escape.
- **US3.4** Selecting text shows an inline toolbar: bold, italic,
  strikethrough, inline code, link. Cmd/Ctrl+B, Cmd/Ctrl+I, Cmd/Ctrl+Shift+X
  (strikethrough), Cmd/Ctrl+E (inline code) and Cmd/Ctrl+K (link) do the same.
  Cmd/Ctrl+Z and Cmd/Ctrl+Shift+Z undo and redo.
- **US3.5** Hovering a block shows a handle to drag it elsewhere and a `+` to
  insert a block below it.
- **US3.6** A **Markdown** toggle on the description switches to the raw
  Markdown source in a plain text area, and back. Switching back re-renders the
  source; content the WYSIWYG view cannot represent (raw HTML, footnotes) is
  kept in the source as typed, and is only altered once the user edits the
  WYSIWYG view.
- **US3.7** Opening a task and saving it without touching the description never
  sends the description, even when the editor would write the same content with
  different Markdown (list markers, emphasis characters, blank lines).
  Touching it and undoing back to the loaded content does not send it either.
- **US3.8** Clicking a link inside the editor does not navigate. Cmd/Ctrl+click
  opens a web or mail link in the system browser through the existing external
  link opener; any other link is not opened.
- **US3.9** Pasting HTML (from a browser, a document) inserts its supported
  subset as rich content; scripts, styles, iframes and unsupported elements are
  dropped, and nothing is inserted as raw HTML. Pasting plain text that is
  Markdown is parsed as Markdown.
- **US3.10** An image in a description is not loaded from the network: it is
  shown as a placeholder carrying its alt text and address, and stays in the
  Markdown unchanged. A `data:` image is shown.
- **US3.11** The editor follows the Desktop appearance (light and dark) and
  grows with its content; the page scrolls, not the editor.

### US4 - Assignee (P1)

- **US4.1** Given a task whose tracker supports assignee lookup (Jira, GitLab),
  when I focus **Assignee** with nothing typed, then the ticket's team is
  proposed; typing searches the tracker (the web's `/assignable` behaviour).
  Picking a person stores their display name and account id. A **Clear**
  action unassigns.
- **US4.2** Given a tracker without assignee lookup (GitHub), then **Assignee**
  is free text (a login); an empty value unassigns.
- **US4.3** Given creation mode and a tracker with assignee lookup, then
  **Assignee** is disabled with the hint
  `Search becomes available once the task is created.` (see open point O1).
  Given creation mode and a tracker without lookup, the free text is saved on
  the task right after it is created (US1.5).
- **US4.4** A failing lookup says so under the field and leaves the current
  assignee unchanged.

### US5 - Pull request links (P1)

- **US5.1** **Pull requests** lists the task's pull request links in their
  stored order, each with its URL, its state when known, and actions to move it
  up, move it down and remove it. The last one is marked **Current**: it is the
  task's `prUrl`.
- **US5.2** An input and an **Add** action append a link. Add is refused, with
  a reason under the input, for an empty value, a value that is not an absolute
  `http:` or `https:` URL, a URL with credentials, and a URL already in the
  list.
- **US5.3** Saving sends the whole ordered set when, and only when, it differs
  from the loaded one (added, removed or reordered). Removing every link
  detaches them all. Links that were loaded keep their stored state and branch.
- **US5.4** A link opens in the system browser on Cmd/Ctrl+click, through the
  external link opener.

### US6 - Unsaved changes (P1)

- **US6.1** As soon as a field differs from what was loaded (or, in creation
  mode, from the empty form), the page title carries an unsaved mark (`•`) and
  the save button is enabled.
- **US6.2** Given unsaved changes, when I leave the page by any in-app means
  (selecting a run or a project in the sidebar, opening the Tickets pane, the
  palette, settings, Cmd+N, **Done**, **Add another**, another **Edit task**),
  then a confirmation asks `Discard unsaved changes to <key or "the new task">?`
  with **Discard** and **Keep editing** (default). Keep editing leaves me on the
  page untouched.
- **US6.3** Given unsaved changes, when I close the Desktop window or quit, then
  the same confirmation is shown by the system, and Keep editing cancels the
  close.
- **US6.4** Without unsaved changes, leaving asks nothing.

### US7 - Outdated agent (P2)

- **US7.1** Given the running agent does not report the `task-page`
  capability, when the page loads a task, saves edits, or looks up assignees,
  then it says `The running local agent is outdated. Stop it, then start the
  rebuilt agent before editing a task. Closing the desktop alone does not
  restart the agent.` Creating a task with a title, a project and a description
  still works as it does today (`create-task`); the tracker select, the assignee
  and the pull requests are then hidden in creation mode.

## Functional requirements

- **FR1** One page serves creation and edition; the quick add dialog and its
  styles are removed. Every former entry point of the quick add opens the page.
- **FR2** The page is shown in the workspace area as the Tickets pane is, and
  restores the previous workspace view when left through **Done** or after a
  confirmed discard with no other destination.
- **FR3** The stored description is Markdown. The value sent is the editor's
  Markdown serialization, or the raw source when the Markdown toggle is on.
- **FR4** A description, title, assignee or pull request set is sent only when
  changed (US2.5, US3.7, US5.3). The description change is decided by comparing
  editor documents, not Markdown text: the loaded description is parsed once
  as the baseline, and the description is changed when the current document is
  not equal to it. In raw Markdown mode, the description is changed when the
  source text differs from the source shown on entering that mode, or when the
  WYSIWYG document had already changed.
- **FR5** Titles are trimmed, required and limited to 500 characters. A
  description is limited to 60,000 characters; above it, the save is refused
  with `The description is too long (<n> of 60,000 characters).`
- **FR6** Pull request URLs are validated by the page (US5.2) and again by the
  agent, which refuses the save with `Invalid pull request link: <url>` and
  forwards nothing.
- **FR7** Every write goes through the local agent with the paired user's
  token, as task creation does; the tracker write is attributed to that user.
- **FR8** Nothing in the editor executes scripts, loads a remote resource or
  needs a Content-Security-Policy change: `desktop/index.html` keeps
  `script-src 'self'`, `connect-src 'none'` and `img-src 'self' data:`.
- **FR9** The new user-facing strings are English. The page is fully usable
  with the keyboard: every control is reachable with Tab, the block menu and
  the assignee results with the arrow keys.
- **FR10** The editor dependency is added to `desktop/package.json` and bundled
  by Vite; the packaged app needs no network access to use it.

## Acceptance criteria

1. Cmd+N / Ctrl+N, palette **New task** and the project's create button open
   the page in creation mode; no element of the former quick add dialog exists
   in the DOM (US1.1, FR1).
2. Creating a task sends `create-task` with the title, description and tracker,
   then one update with the assignee and the pull request links when given; the
   page then shows the post-creation actions, and each one behaves as listed in
   US1.6.
3. A failed creation keeps every field; a failed follow-up update leaves the
   page in edition mode with those fields unsaved (US1.7, US1.8).
4. **Edit task** from the Tickets row menu and from the run header open the
   edition page with fresh data (US2.1-US2.3).
5. Changing only the title sends only `title`; opening a task whose description
   uses `*` bullets, `__bold__` and two blank lines, then saving after a title
   change, sends no `description` (US2.5, US3.7).
6. Typing `/`, choosing **Task list** and typing an item yields `- [ ] item` in
   the saved description; selecting a word and pressing Cmd/Ctrl+B yields
   `**word**` (US3.1-US3.4).
7. Raw Markdown toggle round trip keeps a `<details>` HTML block and a footnote
   unchanged when only the title is edited (US3.6).
8. Pasting `<p>a<script>x</script><b>b</b></p>` inserts `a**b**` and no script
   (US3.9). The UI test run records no Content-Security-Policy violation (FR8).
9. On Jira and GitLab tasks the assignee picker searches through the agent and
   sends display name and account id; on GitHub tasks the free text login is
   sent (US4).
10. Adding, reordering and removing pull request links sends the full ordered
    set; an invalid or duplicate URL is refused with its reason; the last link
    is shown as **Current** (US5).
11. Leaving the page with unsaved changes asks for confirmation, both in-app and
    on window close; Keep editing keeps the edits (US6).
12. With an agent lacking `task-page`, edition shows the outdated-agent message
    and basic creation still works (US7).
13. `CHANGELOG.md` gains an `Added` line under `## [Unreleased]`, and
    `docs/USER_GUIDE.md` describes the task page instead of **Quick add task**.

## Open points

- **O1 - Assignee lookup before the task exists.** The tracker lookup
  (`GET /api/tasks/<id>/assignable`) is per ticket, so a Jira or GitLab task
  cannot search assignees before it is created. This specification disables the
  field in creation mode on those trackers (US4.3) and leaves the search to the
  edition page the creation turns into. Searching before creation needs a new
  project-scoped lookup on the server and in the Jira and GitLab adapters; it is
  left to the owner to ask for it as a follow-up. It blocks nothing here.
  Accepted by the owner on 2026-10-09.
