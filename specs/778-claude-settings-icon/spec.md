# Spec #778 - Desktop: the Claude settings tab draws the Claude logo

Ticket: https://github.com/sebastienferry/sectile/issues/778
Clarification: `docs/clarifications/778.md` (rounds 1 and 2, settled)
Branch: `feat/778`

## Problem

The "Claude settings" category of the Desktop settings navigation draws a
generic shield-with-a-check icon, which says nothing about Claude. Desktop
already ships the Claude logo (#777, next to "Conversation permission mode"),
drawn in the text colour.

## Scope

In: both "Claude settings" tabs (workstation, under General; per project),
wherever the settings navigation is drawn; the colour of the Claude logo in
Desktop settings.

Out: the content and label of the Claude settings panels, every other
category's icon, the web app's Claude icon.

## User stories

### US1 - The Claude settings tab shows the Claude logo (P1)

As a Desktop user, I recognise the Claude settings category by the Claude logo.

- **US1.1** Given the workstation settings are open, when I look at the General
  group, then the "Claude settings" tab draws the Claude logo and no shield.
- **US1.2** Given a project's settings are open, when I look at its group, then
  its "Claude settings" tab draws the Claude logo and no shield.
- **US1.3** Given the configuration page lists the General group and the
  project groups, then every "Claude settings" tab it draws carries the Claude
  logo.
- **US1.4** Given any of these tabs, then the logo is a filled shape the size
  of the other category icons, and the label, tooltip and keyboard behaviour of
  the tab are unchanged.

### US2 - The Claude logo is drawn in Claude's orange (P1)

As a Desktop user, I see the Claude logo one way across the settings.

- **US2.1** Given a "Claude settings" tab, idle, hovered or selected, then its
  logo is drawn in Claude's orange (`#D97757`); the selected state is still
  shown by the tab background and label colour.
- **US2.2** Given the Appearance category, then the logo next to "Conversation
  permission mode" is drawn in the same orange.
- **US2.3** Given the light or the dark theme, then the orange is the same.

## Functional requirements

- **FR1** A settings category names the Claude logo instead of carrying an
  inline icon; the logo's path keeps a single source, the one the test compares
  with the web copy.
- **FR2** Every place that draws a settings category tab draws the icon the
  same way, so the five sites agree.
- **FR3** The orange is defined once, in the stylesheet, and applies to every
  Claude logo Desktop draws; the logo element keeps drawing in the current
  colour.
- **FR4** The shield icon is removed.
- **FR5** `CHANGELOG.md` gets one `Changed` line under `[Unreleased]`.

## Open points

None.
