# Electron and Tauri desktop UX audit

Date: 2026-10-29
Status: source corrections and a current-source visual check are complete; matched visual comparison remains open.

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
| Tauri's workspace polish sheet overrode the canonical surface palette. | The dark/light colors for `--bg`, `--surface`, `--sunken` and `--selected` differed from Electron's `desktop/src/style.css` and duplicated the values in `App.css`. | Use `App.css` as the single source for shared background, surface and selection tokens; keep only workspace-specific tokens in the polish sheet. |
| The Tauri sidebar was fixed width; Electron has a pointer and keyboard resizer. | Electron `desktop/src/main.js` wires `#sidebar-resizer`; Tauri originally rendered a fixed-width `.sidebar`. | Added a bounded, keyboard-accessible divider and persistent width. |
| Settings are a full native page in Tauri and a categorized dialog in Electron. | Tauri's `LocalSettings` has separate engine, profile, connection, project and service sections; Electron groups workstation/project settings in its dialog. | Keep native Tauri setup, but align section naming, compact spacing and progressive disclosure. Do not force the dialog layout into a native page. |
| Fleet configuration was labeled as Local in the settings header and service navigation. | `LocalSettings` used “VOTRE ESPACE LOCAL” and “Services locaux” for every mode, despite Fleet profile changes going to `/api/me` and the Fleet agent remaining remotely managed. | Identify Fleet in the settings heading, call the section “Connexion et services” in Fleet mode, and describe the profile according to its backend owner. |
| A local-only project shows tracker controls even though it has no remote tickets. | In the live Tauri window, `workspace` has a local path and zero tickets, while the board toolbar still says “Statuts du tracker · lecture seule” and offers tracker grouping/search. Go represents a local workspace with a synthetic `local` tracker row, but the desktop board projection omitted the provider, so the renderer could not distinguish it from a remote tracker. | Include tracker providers in the additive desktop board projection. Hide remote controls when a project has no trackers or only the synthetic local tracker; preserve remote controls for GitHub, GitLab and Jira. Keep “Nouveau prompt” as the single primary action. |
| A failed tracker refresh left previously read tickets visible without identifying them as stale. | `ProjectPane.refresh` preserves the last successful `tasks` on error, which is preferable to an empty board, but only displayed the transport error. | Keep the previous data visible and identify it as coming from the previous successful read until refresh succeeds. |
| The local-project empty state repeats the primary prompt action. | The project toolbar already has “Nouveau prompt”; the empty board adds “Commencer une conversation”. The live screenshot shows both at once. | Keep the global project action and remove the duplicate empty-state button; use the empty state to explain that the local project is ready for prompts. |
| The project toolbar offers navigation to the view already open. | The live board screenshot shows a “Tableau” button beside the current project; `App.tsx` always renders it, and its handler navigates to the same board view. | Hide the current-view action while the board is open. Keep “Tableau” available from prompts and execution views so navigation remains direct. |
| The prompt folder action obscured its project-wide scope. | `PromptFolders` listed the active directory and project attachments together; its “Ajouter un dossier” action did not explain that the selected folder is shared with every project conversation. | Call the action “Attacher au projet,” label the disclosure “Dossiers du projet,” and state that the attachment is shared across conversations. Keep detailed roles and paths in the disclosure. |
| The current live Electron/Tauri windows are not a valid populated-state visual comparison. | Both windows were captured at the same desktop size, but Electron was showing a long historical conversation and orchestration sidebar while Tauri showed the empty Local `Documents` board. | Use these captures only for shell/navigation comparison. Before tuning typography, spacing or colors, capture equivalent screens, themes and data states; do not infer style changes from the mismatched content. |

## Current source and runtime status

- Trackerless board controls and the duplicate empty-state prompt action are
  corrected in Tauri source and covered by targeted UI regressions, including
  the synthetic local tracker and a pre-provider agent compatibility case. The
  Go board projection now supplies provider metadata. The current
  board navigation action is hidden while already on the board. These changes
  are in commits `bf4a22a`, `9457e14` and `25f9cac`.
- On 2026-10-29 the Tauri debug bundle was rebuilt from current source and
  launched in the existing workspace. The visible board uses the single
  persistent agent status in the sidebar, omits the redundant connection
  indicator and footer claim, hides the current-board navigation action, and
  keeps refresh beside the board it updates. The project screen puts the local
  folder, default prompt profile and primary prompt action before advanced
  options. This is a source verification, not a matched Electron/Tauri visual
  comparison.
- A live review then selected the `workspace` project: its persisted project
  mapping contains only a synthetic `local` tracker, while the older running
  agent does not expose tracker providers in its board metadata. That explains
  why the existing runtime still showed remote controls despite the earlier
  source guard. A new UI regression covers both the additive current-agent
  provider field and fallback through `issueTracker` for an older agent. An
  isolated current-source package was then launched with a separate identity
  and port. A native folder picker created a temporary local project whose DB
  tracker provider was `local`; its board visibly showed “Votre projet local
  est prêt” and omitted ticket search, grouping and remote status. The 52964
  companion, app data, folder, bundle and worktrees were removed afterward; the
  production Local companion on 52963 was left running.
- Electron's searchable settings dialog and Tauri's native settings page were
  inspected side by side. Electron exposes General, Appearance, Agent
  connection, Execution defaults, AI engines, Skills, Deployment, Agent logs
  and Changelog; Tauri exposes AI engines, Profile, Accounts and trackers,
  Projects and Local services. The settings architectures differ as intended,
  but the terminology and information hierarchy still deserve a focused
  comparison. Electron was in English/dark mode while Tauri was in French/light
  mode. Tauri's dark theme was briefly selected to inspect its palette, then its
  original light preference was restored. The Electron screenshot resolved to
  an active task rather than the settings dialog shown in its accessibility
  tree, so the screenshots are not comparable and matched language, color and
  spacing review remains open.
- Prompt context now surfaces attached folder names immediately. The full path
  and role remain available in the details disclosure. Typecheck and production
  build passed for this change; a matched live visual review is still pending.
- The prompt action now names its project-wide scope before opening the native
  picker, and the disclosure says attached folders are shared across project
  conversations. Focused UI tests verify the label and scope copy.
- The Electron/Tauri command inventory was compared. Tauri keeps one project
  board action for both Electron's task-list and project-board destinations;
  it omits task creation and web-interface actions because the desktop does
  not own tracker tasks or the web planning surface. Project, prompt, settings,
  orchestration, folder and recent-run navigation remain available. No
  unsupported commands were added.
- Native settings now name the active Local or Fleet owner. The Fleet services
  section describes the Fleet agent, while the Local mode retains its local
  companion wording. UI regressions cover Fleet pairing and the initial
  unconfigured state.
- A failed ticket refresh now preserves the last successful board and labels it
  as previous data alongside the read error. A UI regression exercises a
  successful read followed by a failed refresh.
- The duplicate workspace color overrides have been removed. Tauri now uses
  the same canonical dark/light background, surface and selection colors as
  Electron; exact appearance still needs review in the rebuilt app once the
  desktop session is available.

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

1. Recheck the contextual toolbar for a tracker-backed project with no tickets
   and for failed/stale ticket reads in an isolated app. The Local-only board
   state is now visually verified against a rebuilt Go bundle, and UI
   regressions cover Local and Jira-backed projects.
2. Capture matched light and dark screenshots of Electron and Tauri at the same
   window size and with equivalent project/activity data. Normalize typography,
   surface contrast, control height, spacing and selected/hover states from
   evidence, not from a wholesale restyle.
3. Reduce settings page density and align labels, section names and primary
   actions with Electron while retaining native OAuth and filesystem flows.
4. Complete the command-palette audit by comparing shortcuts and keyboard
   focus behavior in matched live windows; the action inventory and unsupported
   Electron-only actions have been reviewed.
5. Recheck prompt, workflow and conversation layouts at narrow and normal window
   sizes, including empty, active, error and waiting states.

Each completed item should preserve tracker-authoritative ticket state, the
existing Go execution/MCP contracts, accessible focus behavior, the no-tabs
decision and the fast standalone Local path.
