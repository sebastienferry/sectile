# Feature Specification: Dark/Light mode for Sectile Desktop

**Ticket**: [#507](https://github.com/sebastienferry/sectile/issues/507)
**Branch**: `feat/507`
**Clarification**: [docs/clarifications/507.md](../../docs/clarifications/507.md)
**Status**: Specified

## Summary

Sectile Desktop only renders in dark colours today. It gains a light mode and a
workstation setting with three values, System, Dark and Light, System being the
default. The whole window follows the chosen mode, the terminal included, and
the mode switches live, without a restart, when the setting or the OS
appearance changes.

Out of scope: the web interface's theme (including its unimplemented `system`
value) and any synchronisation between the desktop setting and the web
profile's theme.

## User stories

### US1 - The desktop matches my OS appearance (P1)

As a user whose OS is set to light, I want Sectile Desktop to render in light
colours without configuring anything, so that it looks like the rest of my
workstation.

**Acceptance scenarios**

1. **Given** a workstation whose settings carry no appearance choice (a fresh
   install or an existing one after the update), **when** the app starts
   while the OS appearance is light, **then** the window, the title bar area
   and the terminal render in light colours from the first frame, without a
   dark flash.
2. **Given** the same workstation, **when** the OS appearance is dark,
   **then** the app renders in dark colours, as it does today.
3. **Given** the appearance is System, **when** the OS appearance changes while
   the app is open, **then** the app, the terminal and the title bar overlay
   follow without a restart.

### US2 - I choose the appearance myself (P1)

As a user, I want to force Dark or Light regardless of the OS, so that I keep
the look I prefer.

**Acceptance scenarios**

1. **Given** the settings dialog is open, **when** I select the "Appearance"
   category, **then** a segmented control offers System, Dark and Light, and
   the current choice is pressed.
2. **Given** the Appearance panel, **when** I press Light (or Dark), **then**
   the whole app switches to that mode immediately, the choice is saved on the
   workstation, and the OS appearance no longer affects the app.
3. **Given** I chose Light and quit, **when** I start the app again, **then** it
   opens in light colours whatever the OS appearance.
4. **Given** I press System, **then** the app follows the OS appearance again.

### US3 - Everything is readable in light mode (P1)

As a user in light mode, I want every surface to be readable, so that no part
of the app is left dark or illegible.

**Acceptance scenarios**

1. **Given** light mode, **then** the header, sidebar, toolbar, setup and
   pairing screens, dialogs and settings, run queue, ticket pane, diff view,
   tooltips, menus, notifications and footer render on light backgrounds with
   dark text.
2. **Given** light mode, **then** status colours (running, failed, waiting,
   canceled, PR states, priorities, diff additions and deletions, warnings)
   keep their meaning with tones that contrast on a light background.
3. **Given** light mode, **then** the terminal uses a light palette covering
   background, foreground, cursor, selection and the 16 ANSI colours.
4. **Given** light mode, **then** native widgets (scrollbars, `<select>`
   popups, the dialog backdrop) render light.

## Functional requirements

- **FR-1** The desktop keeps an `appearance` preference in its workstation
  settings file (`settings.json`), with the values `system`, `dark` and
  `light`. A missing or unknown value reads as `system`.
- **FR-2** The preference is applied by the main process before the window is
  created: the window background and the title bar overlay are painted in the
  resolved mode's colours.
- **FR-3** The settings dialog has an "Appearance" category with a segmented
  control System / Dark / Light. Pressing a value saves and applies it at once;
  there is no separate Save step.
- **FR-4** A change of the preference, or of the OS appearance while on
  System, repaints the stylesheet, the terminal, the window background and the
  title bar overlay without a restart.
- **FR-5** Every colour of the desktop stylesheet comes from a named colour
  token defined once for dark and once for light; no surface keeps a dark
  literal in light mode.
- **FR-6** The terminal has a dark and a light theme, each defining
  background, foreground, cursor, selection and the 16 ANSI colours. The dark
  theme keeps today's background and foreground.
- **FR-7** The dark mode keeps today's appearance.
- **FR-8** The desktop neither reads nor writes the server's user settings for
  this preference.
- **FR-9** `CHANGELOG.md` gains one `Added` line under `[Unreleased]`.

## Edge cases

- A `settings.json` written by an older version, or with an invalid
  `appearance`, behaves as System and is not rewritten until the user picks a
  value.
- The settings file cannot be written: the choice is not applied and the error
  is shown the way other settings errors are.
- macOS draws its own traffic lights and has no title bar overlay colours to
  update; Windows and Linux repaint the overlay.
- The setup and pairing screens, shown before any agent connection, follow the
  mode too: the preference does not depend on the agent.

## Success criteria

- In light mode, no element of the desktop stylesheet keeps a colour from the
  dark set (checked by a test on the stylesheet).
- Switching mode takes effect in the running window with no restart (checked
  by a UI test).
- An existing dark-OS install looks the same after the update.

## Open requirements

None. Every product question was settled in the clarification.
