# The waiting glyph clears once the question is answered

Scope restated from the clarification (`docs/clarifications/475.md`, confirmed
by the owner after two rounds, with an addendum). The implementation choices
live in `plan.md`.

## Context

A skill that needs its owner declares a wait (`report_waiting` with
`waiting: true`). The run then shows the `?` "Waiting for your answer" glyph on
its desktop skill badge, the "Waiting for you" run state and banner in the
desktop, and the waiting badge on the web board. Today the glyph can stay long
after the owner answered, for three reasons observed in the clarification:

- **A.** The session resumed after the answer but has not made a Sectile call
  yet. Only such a call ends the wait.
- **B.** The server ended the wait, but the owner's desktop never learned it.
- **C.** The session made Sectile calls and the wait still did not end
  (observed on 2026-09-25 across a server connection reset).

## User stories

### P1: Answering in the console ends the wait

As the owner of a run that asked me a question, I want the waiting glyph to
disappear as soon as I answer in that run's console, so that the board tells
me the run is working again rather than still waiting for me.

### P2: A wait the server ended also ends on my desktop

As the same owner, I want my desktop to stop showing a run as waiting as soon
as the server considers it resumed, even if the server restarted or my desktop
was disconnected in between.

### P3: The session's next Sectile call always ends its wait

As the same owner, I want a run's wait to end when its session makes its next
Sectile call, whichever server instance serves that call and even if the
server restarted since the wait was declared.

## Functional requirements

1. **Enter ends the wait.** When the owner presses Enter in the console of a
   running, waiting run, its wait ends. "Enter" means that the input sent to
   the console contains a line terminator (carriage return or line feed),
   typed or pasted. This holds for the desktop console and the web terminal.
2. **Other keystrokes do not.** Input without a line terminator (arrow keys,
   characters being typed, Escape, Ctrl-C) does not end the wait.
3. **No effect elsewhere.** Enter in the console of a run that is not waiting
   changes nothing. A headless run is never marked, so it is not affected.
4. **One truth.** A wait ended by Enter is ended on the server. The desktop
   skill badge, the desktop run state, the web board badge and the MCP
   `get_task` output all stop reporting it, the desktop within one poll of the
   keystroke.
5. **An ended wait stays ended.** After a wait has ended, neither a
   reconnection of the desktop agent nor a delayed notification brings the
   glyph back. Only a new `report_waiting(true)` declares a new wait, with its
   own start time.
6. **The desktop catches up.** When the desktop agent reconnects to the
   server, every run it holds shows the wait state the server records for it:
   a wait the server ended while the desktop was disconnected, or before a
   server restart, disappears. A wait still in force stays.
7. **The next Sectile call ends the wait.** Any Sectile tool call from the
   session that declared the wait, other than `report_waiting` itself, ends
   it. This holds whichever server instance serves the call, and even if the
   server instance that recorded the wait restarted since, as long as the
   session keeps its identity.
8. **Existing endings are unchanged.** `report_waiting(false)`, `finish_run`,
   canceling or stopping the run, and the end of the session still end the
   wait as they do today.
9. **Compatibility.** A desktop agent older than this change keeps working
   with a newer server: its console Enter does not end the wait (requirement 7
   and the existing endings still do). A newer agent talking to an older server
   keeps working: the glyph may disappear locally on Enter, and the older
   server ignores the signal.
10. **Out of scope, unchanged:** how a wait is declared, the agent CLI's own
    permission prompts, the glyphs, labels, colours and banners themselves,
    and the notification raised when a run starts waiting.
11. **Changelog.** `CHANGELOG.md` carries one line under `## [Unreleased]`, in
    the section `Fixed`: the waiting glyph now clears as soon as the question
    is answered.

## Acceptance scenarios

- **Given** a running interactive run that called `report_waiting(true)`,
  **when** the owner types an answer in its desktop console and presses Enter,
  **then** within one desktop poll the `?` glyph and the "Waiting for you"
  state are gone, the web board badge no longer shows the wait, and `get_task`
  returns the run without `waitingSince`, although the session has made no
  Sectile call since.
- **Given** the same run, **when** the owner answers from the web terminal,
  **then** the same holds.
- **Given** a waiting run, **when** the owner presses an arrow key or types
  characters without Enter, **then** the run is still shown as waiting.
- **Given** a running run that is not waiting, **when** the owner presses
  Enter in its console, **then** nothing changes on the server or the desktop.
- **Given** a waiting run whose desktop agent is disconnected, **when** the
  session makes a Sectile call and the agent reconnects afterwards, **then**
  the desktop stops showing the run as waiting within one poll of the
  reconnection.
- **Given** a waiting run, **when** the owner presses Enter while the desktop
  agent is disconnected and the agent reconnects later, **then** the glyph
  does not come back and the server records the wait as ended.
- **Given** a wait declared by a session, **when** the server instance that
  recorded it restarts and the same session then calls `get_task`, **then**
  the wait ends.
- **Given** a wait declared by a session that the server's live session
  registry does not know, **when** that session makes any Sectile call other
  than `report_waiting`, **then** the wait ends.
- **Given** a wait ended by Enter, **when** the session calls
  `report_waiting(true)` again, **then** a new wait starts with a new
  `waitingSince` and the glyph shows again.
- **Given** a headless run, **when** input reaches its process, **then**
  nothing is marked or cleared.

## Open requirements

None. Every product decision was settled in the clarification.

One residual behaviour is accepted rather than open: a session that loses its
identity (its MCP client re-initializes after a server restart) cannot end its
old wait by its next call, because that call comes from a different session.
Enter in the console (requirement 1) and `finish_run` still end it.
