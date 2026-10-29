# Electron and Tauri desktop UX audit

Date: 2026-10-29
Status: source corrections are in progress; matched visual comparison and installed-app refresh remain open.

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
| A local-only project shows tracker controls even though it has no remote tickets. | In the live Tauri window, `Documents` has a local path and zero tickets, while the board toolbar still says “Statuts du tracker · lecture seule” and offers tracker grouping/search. `App.tsx` renders this toolbar for every project. | Make the empty local-project state describe local work and hide tracker-specific controls until the project has a tracker-backed ticket collection. Keep “Nouveau prompt” as the single primary action. |
| The local-project empty state repeats the primary prompt action. | The project toolbar already has “Nouveau prompt”; the empty board adds “Commencer une conversation”. The live screenshot shows both at once. | Keep the global project action and remove the duplicate empty-state button; use the empty state to explain that the local project is ready for prompts. |
| The project toolbar offers navigation to the view already open. | The live board screenshot shows a “Tableau” button beside the current project; `App.tsx` always renders it, and its handler navigates to the same board view. | Hide the current-view action while the board is open. Keep “Tableau” available from prompts and execution views so navigation remains direct. |
| Attached folders were hidden behind a disclosure in the prompt context. | `PromptFolders` listed the active directory in the toolbar, while the additional project folders were only visible after opening “Dossiers”. | Show attached folder names under the active directory with full paths available on hover; keep detailed roles and paths in the existing disclosure. |
| The current live Electron/Tauri windows are not a valid populated-state visual comparison. | Both windows were captured at the same desktop size, but Electron was showing a long historical conversation and orchestration sidebar while Tauri showed the empty Local `Documents` board. | Use these captures only for shell/navigation comparison. Before tuning typography, spacing or colors, capture equivalent screens, themes and data states; do not infer style changes from the mismatched content. |

## Current source and runtime status

- Trackerless board controls and the duplicate empty-state prompt action are
  corrected in Tauri source and covered by targeted UI regressions. The current
  board navigation action is hidden while already on the board. These changes
  are in commits `bf4a22a`, `9457e14` and `25f9cac`.
- The open installed Tauri window was inspected after those source commits and
  still displayed the older board toolbar and duplicate prompt action. Treat
  that window as a stale bundle, not as evidence that the committed source
  correction failed. Rebuild/relaunch in an isolated profile before claiming
  visual qualification; the computer became locked before the Electron window
  could be inspected in this continuation.
- Prompt context now surfaces attached folder names immediately. The full path
  and role remain available in the details disclosure. Typecheck and production
  build passed for this change; a matched live visual review is still pending.

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

1. Rebuild/relaunch the current Tauri source and visually verify the contextual
   toolbar and trackerless board states. Recheck local projects, tracker-backed
   projects with tickets, tracker-backed projects with no tickets, and
   failed/stale ticket reads.
2. Capture matched light and dark screenshots of Electron and Tauri at the same
   window size and with equivalent project/activity data. Normalize typography,
   surface contrast, control height, spacing and selected/hover states from
   evidence, not from a wholesale restyle.
3. Reduce settings page density and align labels, section names and primary
   actions with Electron while retaining native OAuth and filesystem flows.
4. Audit each Electron command-palette action and shortcut against Tauri's
   command inventory; only expose actions Tauri can actually perform.
5. Recheck prompt, workflow and conversation layouts at narrow and normal window
   sizes, including empty, active, error and waiting states.

Each completed item should preserve tracker-authoritative ticket state, the
existing Go execution/MCP contracts, accessible focus behavior, the no-tabs
decision and the fast standalone Local path.
