# Specification #735 - Roadmap slicing import from an uploaded file

- Ticket: https://github.com/sebastienferry/sectile/issues/735
- Branch: `feat/735`
- Clarification: `docs/clarifications/735.md` (rounds 1 to 3, settled by the
  owner on 2026-10-05)
- Framework: Spec Kit

## Summary

From the framing panel of a macro in the Roadmap view, a user picks a
`tasks.md` or `spec.md` file on their own machine. Sectile produces the
macro's slicing from that file, exactly as it does today from the file the
local agent finds in the workstation's specifications folder. The import does
not need Sectile Desktop or a connected agent, and it remembers nothing.

## Scope

In scope: one upload control per file source in the framing panel's import
row, the server accepting the file content on the existing slicing request,
the refusals of the upload path, and the changelog.

Out of scope:

- Choosing, remembering or overriding a specifications folder, per project or
  per macro (#736 covers the folders).
- Any change to the agent-read import (`tasks.md`, `spec.md` buttons) or to
  the "stories" import.
- Drag and drop onto the panel.
- Translating existing French messages.

## Vocabulary

- **File source**: `tasks` (group headings of a `tasks.md`) or `spec`
  (requirements or prioritised user stories of a `spec.md`), as today.
- **Agent-read import**: today's import, where the local agent finds and reads
  the file.
- **Upload import**: the new import, where the user picks the file and the
  browser sends its content.
- **Origin**: the text the success notification shows under its title, saying
  where the slicing was read from.

## User stories

### US1 (P1) - Slice a macro from a tasks.md picked on my machine

As a user framing a macro, I pick a `tasks.md` from my disk and get the
slicing from its groups, without the file being in the specifications folder
Sectile knows about.

1. Given a macro selected in the framing panel, when I look at the import row,
   then an upload icon button sits next to the `tasks.md` button, and its
   tooltip says that it imports a local file as a `tasks.md`.
2. Given I pick a file with three task groups, when the import completes, then
   the slicing holds those three lines, each with `tasks` as its source kind,
   and a success notification names the macro.
3. Given the notification, then its description reads
   `imported file: <file name>`, with the existing suffix about lines already
   linked to their story when some are.
4. Given the macro already holds lines, when I upload a file whose groups match
   some of them on their normalised text, then the matched lines keep their
   identifier, checkbox and story, and the unmatched existing lines are kept,
   as with the agent-read import.
5. Given a file not named `tasks.md` (for example `plan-v2.md`), when I pick it
   with the `tasks.md` upload button, then it is parsed as a `tasks.md`.

### US2 (P1) - Slice a macro from a spec.md picked on my machine

1. Given a macro selected, then an upload icon button also sits next to the
   `spec.md` button.
2. Given I pick a file with two requirements, when the import completes, then
   the slicing holds those two lines with `spec` as their source kind, and the
   origin reads `imported file: <file name>`.
3. Given a Spec Kit `spec.md` with prioritised user stories, then the lines are
   those user stories, as with the agent-read import.

### US3 (P1) - Import with no desktop app

1. Given no agent is connected for the user, when I upload a file, then the
   slicing is produced; the import never asks for the desktop app.
2. Given the agent-read buttons, then they keep their current behaviour and
   messages when no agent is connected.

### US4 (P2) - Clear refusals, slicing untouched

Each refusal shows the existing "Slicing not produced" notification with the
message as its description, and leaves the slicing as it was.

1. Given a file larger than 1 MiB, when I pick it, then nothing is sent and
   the message says the file exceeds the 1 MiB limit.
2. Given a file that is not UTF-8 text, when I pick it, then nothing is sent
   and the message says the file is not UTF-8 text.
3. Given a file in which no unit of its source is found (an empty file, or a
   `tasks.md` with no group heading), when the import runs, then the message
   names the file and what was looked for, in English, for example
   `imported file: notes.md has no task group: the slicing is left unchanged`.
4. Given a request sent outside the web client with content larger than 1 MiB,
   containing a NUL character, or with the `stories` source, then the server
   refuses it with a message saying why, in English.

### US5 (P2) - Same guards as the other import buttons

1. Given an import of any source is running for the macro, then both upload
   buttons are disabled, and the one that started the running upload shows a
   spinner.
2. Given a batch holds the macro's slicing (`slicingLocked`), then both upload
   buttons are disabled.
3. Given I pick the same file twice in a row, then the second pick runs the
   import again.

## Functional requirements

- FR-1: The framing panel offers one upload control per file source, attached
  to the `tasks.md` and `spec.md` buttons. There is none for the "stories"
  source.
- FR-2: The file is parsed as the source of the control used, whatever its
  name or extension.
- FR-3: The picker suggests `.md`, `.markdown`, `.txt`, `text/markdown` and
  `text/plain`. The browser refuses a file over 1 MiB, or one that is not
  valid UTF-8, before sending anything.
- FR-4: The server produces the slicing from the uploaded content with the
  same extraction, merge, save and tracker mirror as the agent-read import, and
  never calls the agent for it.
- FR-5: The server accepts uploaded content only with the `tasks` or `spec`
  source, at most 1 MiB, with no NUL character. Anything else is refused with
  an HTTP 400 and an English message.
- FR-6: The origin of an upload import is `imported file: <file name>`, the
  file name being the base name sent by the browser, or the source's usual
  file name when none is sent.
- FR-7: The upload import writes no setting, no file and no repository change,
  and the agent-read import stays the default.
- FR-8: New user-facing strings are written in English by default. Web labels
  are added to the French and English catalogs.

## Success criteria

- A user with no Sectile Desktop can produce a macro's slicing from a local
  `tasks.md` or `spec.md` in one pick.
- The slicing produced from an uploaded file is identical to the one the
  agent-read import produces from the same content.
- Every refusal names its cause, and no refusal changes the slicing.
