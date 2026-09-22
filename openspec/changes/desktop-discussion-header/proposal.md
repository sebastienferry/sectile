# Rework the discussion header around identity, state and the worktree

## Why
The header above the discussion console spends four rows saying very little. The
task title wraps because the skill-result badge and the worktree path share its
block, three controls carry full text labels (`Relaunch`, `Export log`,
`Console` / `Changes`) beside controls that are already icon-only, and the
Console/Changes switch occupies a row of its own below the toolbar. On a narrow
window the stop control wraps away from the row it belongs to.

Two things a user actually needs are the hardest to get at. The run state is
shown on the sidebar row and on the desktop notification, but not in the header
of the execution being read, so the state of what is on screen has to be read
somewhere else. And the worktree path is a caption: to open that checkout in a
shell or an editor it has to be retyped or selected by hand from a truncated
line.

## What Changes
- Split the header into an identity block and an action row: title, run state
  and skill result on the title line, the worktree path below it, every control
  on the right.
- Show the selected execution's run state beside the title, drawn from the
  shared run-state definition so the header, the sidebar row and the
  notification cannot say three different things. Spell its label out: the
  header has the room a task row does not.
- Turn the worktree path into a control. One click copies it to the clipboard,
  with a `Copied` confirmation; the text stays selectable for a manual copy.
  Copying goes through a new `copy-text` IPC, since the renderer has no
  clipboard permission of its own.
- Reduce `Relaunch`, `Export log` and the `Console` / `Changes` switch to icons,
  keeping their wording as accessible name and tooltip. Move the view switch
  into the toolbar, removing the row it had to itself.
- Keep the pull request control icon-led but keep its number: it identifies
  which pull request, not merely that one exists.
- Keep the workflow actions (`Next: <skill>`, `Mark reviewed`, `Retry`,
  `Launch anyway`) labelled: their meaning depends on the stage, and an icon
  would not carry it.
- Stop the skill-result indicator from restating the run state. It answers what
  the server knows about the skill; a run still in flight, a run that failed or
  was cancelled without a verdict, and a free console have nothing for it to
  say, so it shows nothing instead of a second glyph repeating `Running` as
  `In progress` or `Cancelled` as `Console stopped`. The one exception is a
  requested stop that has not taken effect, which the run state cannot express.
  An indicator whose result stops applying is now cleared rather than left
  behind.

## Impact
Desktop only: `desktop/src/main.js` (toolbar markup, `iconPaths`, header
rendering, worktree control, task-row badge clearing),
`desktop/src/skill-result.mjs`, `desktop/src/style.css`,
`desktop/electron/main.cjs` and `desktop/electron/preload.cjs` (clipboard IPC),
and a new `desktop/tests/discussion-header.ui.cjs`. No server, agent or tracker
behaviour changes, and nothing leaves the workstation: the clipboard write
carries the path the header already displays.
