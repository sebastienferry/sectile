# Electron and Tauri desktop UX audit

Date: 2026-10-29
Status: initial audit; navigation inconsistencies corrected, with remaining visual convergence work listed below.

## Reference and scope

Electron's renderer is in Sectile's Go repository under `desktop/src/`. The
Tauri renderer is in this repository under `src/`. Electron is the reference for
the established task, conversation, settings and navigation behavior; Tauri
keeps its native folder picker, separate Go companion, single workspace and
tracker-backed task state. This audit does not reintroduce tabs or copy
Electron's runtime architecture.

The existing [parity delivery plan](desktop-parity-plan.md) records the feature
work already carried over: project folders, provider preparation, the complete
ticket workflow, stage grouping/review and orchestration configuration. Its
verification record remains the evidence for those flows.

## Concrete findings

| Finding | Evidence | Decision |
| --- | --- | --- |
| Connection state was repeated in the header, workspace footer and project sidebar. | `App.tsx` rendered `connection-pill`, `statusbar` and `sidebar-footer` from the same `connected` value. | Keep the persistent, labelled agent status in the sidebar, matching Electron's status placement. Remove the other two copies. |
| The workspace footer claimed “Données et exécution locales” regardless of which backend supplied tracker state. | The footer was static while the project board reads remote tracker tickets. | Remove the claim instead of presenting topology as business-data ownership. |
| A global refresh action only refreshed the project list, while the board had a separate refresh for ticket data. | Header called `refreshProjects`; the board toolbar called its own task refresh. | Remove the misleading global refresh. Keep project refresh in project/settings flows and task refresh beside the board it updates. |
| Electron's searchable command palette is missing from Tauri. | `desktop/src/main.js` indexes actions, projects and executions; Tauri had only direct sidebar/header controls. | Add a small React palette on Cmd/Ctrl+K with actions, projects and runs; do not add a UI dependency. |
| Styling is split between base rules and a second override sheet. | `App.css` owns foundational controls/layout while `workspace-polish.css` redefines tokens, radii, spacing and component density after it. | Consolidate tokens and component rules after the navigation behavior settles. Avoid broad cosmetic changes before checking light/dark screenshots. |
| The Tauri sidebar was fixed width; Electron has a pointer and keyboard resizer. | Electron `desktop/src/main.js` wires `#sidebar-resizer`; Tauri originally rendered a fixed-width `.sidebar`. | Added a bounded, keyboard-accessible divider and persistent width. |
| Settings are a full native page in Tauri and a categorized dialog in Electron. | Tauri's `LocalSettings` has separate engine, profile, connection, project and service sections; Electron groups workstation/project settings in its dialog. | Keep native Tauri setup, but align section naming, compact spacing and progressive disclosure. Do not force the dialog layout into a native page. |

## First corrections

- Removed duplicated connection indicators and the unqualified local-data
  footer claim. The sidebar's labelled agent state remains the single persistent
  connection indicator.
- Removed the header refresh button, which refreshed projects rather than the
  visible ticket board.
- Added Cmd/Ctrl+K navigation across actions, projects and executions, with
  filtering, arrow-key selection, Enter to open and Escape to close.
- Added a pointer- and keyboard-resizable sidebar, bounded between 200 and 480
  pixels, with its width retained locally.

## Remaining convergence order

1. Capture matched light and dark screenshots of Electron and Tauri at the same
   window size. Normalize typography, surface contrast, control height, spacing
   and selected/hover states from evidence, not from a wholesale restyle.
2. Reduce settings page density and align labels, section names and primary
   actions with Electron while retaining native OAuth and filesystem flows.
3. Audit each Electron command-palette action and shortcut against Tauri's
   command inventory; only expose actions Tauri can actually perform.
4. Recheck prompt, workflow and conversation layouts at narrow and normal window
   sizes, including empty, active, error and waiting states.

Each completed item should preserve tracker-authoritative ticket state, the
existing Go execution/MCP contracts, accessible focus behavior, the no-tabs
decision and the fast standalone Local path.
