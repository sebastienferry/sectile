# Specification #922 - Conversation header next-step button

Ticket: https://github.com/sebastienferry/sectile/issues/922
Clarification: `docs/clarifications/922.md` (round 1, no open questions).

## User story (P1)

As a user viewing a ticket conversation, I want its next-step button to match
the neighbouring header icons, so the toolbar is visually consistent.

## Requirements and acceptance scenarios

1. Given a ticket conversation with an eligible next workflow step, when its
   header appears, then Next step has the same visible dimensions, padding,
   icon scale and vertical alignment as Rename and Archive, with a usable
   32 by 32 pixel click target.
2. Given the control in default, hover, keyboard focus or disabled state, when
   compared with an adjacent icon button in that state, then it uses the same
   visual treatment and retains a visible keyboard focus indicator.
3. Given an available agent and an eligible step, when Next step is clicked,
   then the existing callback receives that step's skill ID. Its tooltip and
   accessible name continue to identify the next step.
4. Given an unavailable agent, then the button remains disabled. Given an
   active task run or no eligible step, then existing visibility rules remain
   unchanged.
5. Given window widths of 800, 1050 and 1280 pixels, then the control keeps its
   dimensions and alignment in the conversation header without clipping.
6. The root and Tauri changelogs include a concise Fixed entry under Unreleased,
   and the Tauri README describes the consistent header actions.

## Scope

Only the existing Tauri conversation header presentation and associated
documentation. No workflow logic, backend, Electron or web changes.

## Open requirements

None.
