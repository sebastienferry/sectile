# Specification #698 - Add a project from the command palette

- Ticket: https://github.com/sebastienferry/sectile/issues/698
- Branch: `feat/698`
- Clarification: `docs/clarifications/698.md` (round 2, the owner chose the
  recommended scope)
- Framework: Spec Kit

## Summary

The Sectile Desktop command palette offers an **Add project** command. It opens
the Add project dialog the sidebar `+` button already opens, so a project of the
Sectile server can be added to the workstation from the keyboard, even when the
sidebar is collapsed.

## Scope

In scope: the Desktop command palette, the Desktop README paragraph that lists
the palette's commands, and the changelog.

Out of scope:

- Creating a project on the Sectile server from Desktop (only the web app does
  that).
- Any change to the content or behaviour of the Add project dialog itself.
- The web app's command palette.
- Any server, agent, API or database change.

## Vocabulary

- **Command palette**: the Desktop dialog opened with Cmd+K (macOS), Ctrl+K
  (Windows, Linux) or the `⌘K` header button, listing actions filtered by a
  search field.
- **Add project dialog**: the dialog titled "Add project" opened today by the
  `+` button next to PROJECTS in the sidebar. It lists the server's projects:
  one not yet added opens its configuration, one already added is shown
  disabled with "Already added", one hidden from the sidebar offers "Hidden,
  show in sidebar".

## User stories

### US1 (P1) - Add a project from the keyboard

As a Desktop user, I press Cmd+K, type "project" and press Enter, and the Add
project dialog opens, so I connect a server project to this workstation without
reaching for the sidebar.

### US2 (P2) - Add a project with the sidebar collapsed

As a Desktop user who keeps the sidebar collapsed, I open the palette and choose
**Add project**, so I do not have to expand the sidebar to find a button that
only shows on hover.

## Functional requirements

- **FR1** The command palette lists a command labelled exactly **Add project**,
  after **Quick add task** and **Tasks list**.
- **FR2** Running **Add project** from the palette, by clicking it or by pressing
  Enter while it is the first matching command, opens the Add project dialog in
  place of the palette.
- **FR3** The dialog opened from the palette is the one the sidebar `+` button
  opens: same title, same explanatory text, same project list with the same
  states and actions, and the same error when the server's projects cannot be
  read.
- **FR4** The palette's search matches **Add project** on any part of its label,
  case-insensitively, as for the other commands; a search that does not match it
  hides it.
- **FR5** The sidebar `+` button keeps its behaviour.
- **FR6** The command works whether or not the sidebar is collapsed and whether
  or not a project is selected.

## Acceptance scenarios

1. **Given** Desktop connected to a server with two projects, one already added,
   **when** the user opens the palette, types "project" and presses Enter,
   **then** the "Add project" dialog shows both projects, the added one disabled
   with "· Already added".
2. **Given** the palette is open, **when** the user types "tasks", **then**
   **Add project** is hidden and **Tasks list** is shown.
3. **Given** the palette is open, **when** the user clicks **Add project** and
   then a project not yet added, **then** that project's configuration opens, as
   from the `+` button.
4. **Given** a project hidden from the sidebar, **when** the user opens **Add
   project** from the palette and chooses "Hidden, show in sidebar", **then**
   the project is back in the sidebar and the dialog closes.
5. **Given** the sidebar is collapsed, **when** the user runs **Add project**
   from the palette, **then** the dialog opens without expanding the sidebar.
6. **Given** the server's projects cannot be read, **when** the user runs **Add
   project** from the palette, **then** the same error is shown as from the `+`
   button.
7. **Given** the sidebar `+` button, **when** the user clicks it, **then** the
   dialog opens as before.

## Edge cases

- The server has no project: the dialog says "No projects available on the
  server.", as today.
- The palette is opened from the setup screen before the workspace shows: the
  command behaves as the `+` button would, showing the dialog and its loading
  error if the agent is not reachable.

## Changelog

One `Added` line under `[Unreleased]` in `CHANGELOG.md`: the Desktop command
palette offers **Add project**, which opens the same dialog as the sidebar `+`.

## Open points

None. The clarification settled the only product question (scope: today's
dialog, no server-side creation).
