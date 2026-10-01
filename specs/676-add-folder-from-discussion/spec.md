# Specification #676 - Add a folder from a discussion

- Ticket: https://github.com/sebastienferry/sectile/issues/676
- Branch: `feat/676`
- Clarification: `docs/clarifications/676.md` (rounds 1 to 3; the owner
  answered every product question)
- Framework: Spec Kit

## Summary

While talking to Claude in Sectile Desktop, the user attaches another folder
of the workstation without leaving the discussion. The folder joins the
project's attached folders (ADR 0036) and stays there, so the discussion in
progress sees it at once and every later execution of the project sees it
too. Discussions also start receiving the folders already attached to the
project, which they do not today.

## Scope

In scope:

- the Claude conversation view (ADR 0042) and its composer;
- the ticket discussion ("Discussion (no skill)") running in a Sectile
  terminal;
- the launch of ticket discussions (in Sectile and in the native terminal)
  and of free "Project prompt" consoles;
- the changelog.

Out of scope:

- a folder list kept on the server (rejected by ADR 0036);
- the web app (it has no attached-folder interface and never holds a
  workstation path, ADR 0003);
- detaching a folder from a discussion (the project settings already do it);
- a folder visible to one discussion only;
- drag and drop of a folder;
- a typed `/add-dir` in the conversation composer intercepted by Sectile;
- an add-folder button on a free "Project prompt" PTY console or on a
  discussion running in the native terminal (see Open points).

## Vocabulary

- **Attached folder**: a folder of this workstation attached to the project
  from its settings (#484, ADR 0036). It is listed in the workstation's
  project settings only.
- **Present attached folder**: an attached folder that exists on disk. A
  missing one is named to the agent but never passed as a folder option.
- **Project folders**: the folders an execution of the project is given
  beside its working directory: the mapped folders of the project's other
  repositories, the specifications folder when it is a separate folder, and
  the present attached folders.
- **Conversation**: the Claude conversation view without a terminal, opened
  from "Project prompt" when Appearance → Claude consoles is set to
  Conversation, or from "Claude chat (test)" on an execution. Each message is
  one turn.
- **Ticket discussion**: an execution of a ticket with "Discussion (no
  skill)", showing the engine's interactive session in a Sectile terminal.
- **Free console**: an execution opened from "Project prompt" in a terminal,
  with no ticket.
- **Add-folder action**: the "Add folder…" button this specification adds,
  which opens the native directory picker.

## User stories

### US1 (P1) - Add a folder from a conversation

As a person talking to Claude in a conversation, I attach a folder I need
and my next message can use it, without starting over.

1. Given a conversation that is ready, when the person uses the add-folder
   action and picks a folder, then the folder is attached to the project and
   the composer status says it was attached.
2. Given that folder attached, when the person sends their next message,
   then Claude can read and edit files in it during that turn, and the
   conversation keeps its history.
3. Given the picked folder is a checkout of one of the project's
   repositories, then it becomes that repository's folder, as in the project
   settings, and the status says so.
4. Given the picked folder is refused (already attached, the project's local
   repository, the specifications folder, already a repository's folder),
   then nothing is attached and the status gives the reason.
5. Given the person closes the picker without choosing, then nothing
   changes.
6. Given Claude is working on a turn, then the add-folder action is still
   available, and the folder is given from the next turn on.
7. Given a read-only conversation (history after an agent restart), then the
   add-folder action is unavailable.

### US2 (P1) - A conversation receives the project's folders

As a person who attached folders to a project, I expect a conversation to
see them like any other execution.

1. Given a project with present attached folders, when a conversation turn
   starts, then Claude is given every one of them, and every other project
   folder, as additional folders.
2. Given a folder attached or removed from the project settings while a
   conversation is open, then the next turn reflects the change.
3. Given an attached folder that no longer exists on disk, then the turn
   runs without it and does not fail.
4. Given a turn, then the agent session also receives the project folders
   in `SECTILE_REPOSITORIES`, as skill runs do.
5. Given the project folders cannot be read when a turn starts (for example
   the server is unreachable), then the turn still runs, without additional
   folders, and the conversation shows a status line saying so.

### US3 (P1) - Add a folder from a running ticket discussion

As a person in a ticket discussion with Claude Code, I attach a folder and
Claude sees it immediately, keeping the context of the session.

1. Given a running ticket discussion whose engine is Claude Code, when the
   person uses the add-folder action in the execution toolbar and picks a
   folder, then the folder is attached to the project, and Sectile types
   `/add-dir <path>` followed by Enter into the session.
2. Given that line typed, then the toolbar status says the folder was
   attached and typed into the session.
3. Given Claude Code asks, in the session, to confirm the directory, then
   the person answers it in the terminal as for any other Claude prompt.
4. Given a path containing spaces, then the typed line quotes it so Claude
   Code reads one path.
5. Given a path Sectile cannot type safely (it contains a line break or
   another control character), then the folder is attached, nothing is
   typed, and the status says it applies at the next launch of the
   discussion.
6. Given the engine of the discussion is not Claude Code (Codex, Antigravity,
   a custom engine), then the folder is attached, nothing is typed, and the
   status says it applies at the next launch of the discussion.
7. Given the attach is refused (same reasons as US1.4), then nothing is
   typed into the session and the status gives the reason.
8. Given Claude is still printing when the folder is attached, then Sectile
   waits for the session to settle, for a bounded time, before typing.
9. Given the discussion has ended, then the add-folder action is not
   offered on it.

### US4 (P2) - Discussions and free consoles start with the project's folders

As a person who attached folders to a project, I expect every interactive
session I open on it to see them from the start.

1. Given a project with present attached folders and a Claude Code or Codex
   engine, when a ticket discussion starts, in Sectile or in the native
   terminal, then the engine is launched with an `--add-dir` option for each
   project folder of the ticket.
2. Given the same project, when a free console starts with Claude Code or
   Codex, then the engine is launched with an `--add-dir` option for each
   project folder, and the session receives `SECTILE_REPOSITORIES`.
3. Given an engine whose `--add-dir` option is not attested (Antigravity, a
   custom engine), then the launch line is unchanged.
4. Given a project with no project folders, then the launch line is the one
   used today.

## Functional requirements

- **FR1** The add-folder action is offered in the conversation composer and
  in the execution toolbar of a running ticket discussion, and nowhere else
  in this change. It is labelled "Add folder…", like the project settings
  button, and opens the same native directory picker.
- **FR2** The add-folder action attaches the folder through the existing
  attach path, with its existing checks and its existing outcomes: attached,
  mapped as a repository's folder, or refused with a reason. Nothing is
  stored on the server.
- **FR3** The outcome of the add-folder action is shown in a status beside
  the action, in English like the rest of the Desktop interface (the
  clarification said French; the Desktop, its "Add folder…" button and its
  composer already speak English): attached;
  attached and typed into the session; attached, applies at the next launch;
  mapped as the folder of a repository; refused with the reason.
- **FR4** Each conversation turn reads the project folders when it starts
  and passes each present one to Claude Code as an additional folder, with
  the same folder map in `SECTILE_REPOSITORIES` as a skill run of the
  project. A failure to read them never fails the turn; it is reported in
  the conversation.
- **FR5** In a running ticket discussion launched with Claude Code, a
  successful add-folder action types one `/add-dir` line with the folder's
  path, quoted when needed, then Enter, once the session output has been
  quiet briefly or a bounded wait has elapsed. Nothing is typed for another
  engine, for a refused attach, or for a path that cannot be typed safely.
- **FR6** When the attach maps the folder as a repository's folder, the
  typed `/add-dir` line (FR5) names that folder too, since it becomes a
  project folder.
- **FR7** Every launch of a ticket discussion and of a free console passes
  one `--add-dir` option per project folder for an engine whose option is
  attested (Claude Code, Codex), and none for any other engine.
- **FR8** A free console's session receives `SECTILE_REPOSITORIES` with the
  project folder map.
- **FR9** The Desktop offers the add-folder action only when the local agent
  announces support for it; an older agent hides it.
- **FR10** `CHANGELOG.md` has one line under `## [Unreleased]` → `Added`
  describing the action, and one under `Changed` or `Fixed` saying that
  discussions and project consoles now receive the attached folders.

## Acceptance criteria

- **AC1** In a conversation, attaching a folder and sending a message lets
  Claude list a file of that folder in the same conversation (US1.1-2).
- **AC2** A conversation turn's Claude Code command carries one `--add-dir`
  per present project folder and none for a missing one; a folder attached
  between two turns appears on the second (US2.1-3).
- **AC3** A refused attach from a conversation or a discussion shows the
  agent's reason and changes nothing (US1.4, US3.7).
- **AC4** In a running Claude Code ticket discussion, attaching a folder
  types exactly one `/add-dir` line into the session; a path with a space is
  quoted; a Codex discussion receives no line (US3.1, 3.4, 3.6).
- **AC5** A ticket discussion and a free console launched with Claude Code
  on a project with attached folders carry their `--add-dir` options; with
  Antigravity they do not (US4).
- **AC6** With an agent lacking the new capability, the Desktop shows no
  add-folder action in the composer or the toolbar (FR9).
- **AC7** `CHANGELOG.md` carries the lines of FR10.

## Open points

- **Add-folder action on a free PTY console and on a native-terminal
  discussion.** The clarification placed the action in the conversation and
  the ticket discussion only. Free consoles receive the folders at launch
  (US4) but get no action; a native-terminal discussion runs outside Sectile
  and cannot be typed into. This blocks nothing; a follow-up ticket can add
  the action to free PTY consoles if the owner wants it.
- **Claude Code's own `/add-dir` behaviour** (whether it asks to confirm,
  how it reads a quoted path) is not verified here. The specification
  requires only that the typed line name one path and that the person can
  answer any prompt in the terminal; the implementation verifies the quoting
  against the installed Claude Code (see plan, "Risks").
