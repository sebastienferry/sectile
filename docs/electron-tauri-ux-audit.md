# Electron and Tauri desktop UX audit

Date: 2026-10-29
Status: source corrections and a current-source visual check are complete. The
convergence plan has passed an independent design challenge; matched
populated-state screenshots remain a hard gate before UI implementation.

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

## Electron experience and Tauri direction

The reference is a workbench organized around a compact project/task rail and
one active work surface. Its project groups expose recent runs, workflow stage,
run state, execution history and orchestration. The main surface changes with
the selected item: ticket board/details, workflow action, provider conversation,
console or diff. The execution toolbar keeps task identity, working directory,
PR, next workflow action and stop/review controls close to the work. Electron's
conversation view keeps the transcript readable above a composer, groups tool
events, and exposes model/mode, questions, approval and folder context. Settings
are grouped by workstation and project; less frequent permissions, sandbox,
provider and workflow policy remain available without occupying the normal
work surface.

Source inspection shows Electron's visual language uses a 68 px header, a
resizable project/task rail, semantic dark/light tokens, compact task rows and
restrained card borders. The token set distinguishes surfaces, muted text,
tracker stage, run state, approval/waiting, diff and provider color. Tauri shares
some core background and surface colors and has the resizable rail, but its
light selection/accent values and semantic status-token set differ. Its separate
project board, prompt, execution, workflow and settings pages create a different
navigation rhythm. Its large first-run prompt introduction, section headings,
settings cards and flexible padding also make the same work feel more like a
set of setup pages than the Electron workbench. The density and hierarchy
observations are source-based hypotheses until equivalent current-build
screenshots are captured; they are not instructions to copy Electron's legacy
CSS literally.

The target is a Tauri-native workbench that keeps the Electron user's fast
project/task-to-execution mental model while retaining the decisions already
made for Tauri: one workspace (no tabs), native setup and folder selection,
free project prompts, explicit Local/Fleet ownership, tracker-authoritative
ticket state, and the lightweight Go companion. Do not copy Electron's
launcher process, settings dialog, unsupported task mutations, or web planning
surfaces into Tauri merely to look alike.

| Journey | Electron reference | Tauri today | Convergence direction |
| --- | --- | --- | --- |
| Find work | Project groups contain compact task/run rows, status and stage; orchestration is visible above projects. | Project groups reveal prompt, orchestration, recent conversations and run history; tracker tasks live in the board. | Keep one project/task rail; make tracker tasks and local conversations visually distinct, compact and easy to scan without duplicating a task in multiple navigation groups. |
| Start work | Task action or project console; engine/profile and folders are selected in the launch flow. | Free prompt is a first-class project surface with provider, model/mode and shared folder context; tracker task workflow has explicit next-step actions. | Preserve two clear starts: a ticket's next workflow step and an unrestricted project prompt. Show project, provider and actual folder context at the composer before submission. |
| Follow execution | Console/conversation or diff is the main surface; toolbar keeps directory, PR, next step and stop/review actions visible. | Execution list, conversation and workflow task are separate workspace views. | Keep the active run's identity, live/waiting state, folder, stop, question and review actions in one focused surface; avoid parallel status labels and repeated refresh controls. |
| Configure | Categorized settings dialog spans workstation and project controls, including provider, skills/MCP, execution and sandbox policy. | Native settings page separates engines, profile, trackers, projects and services; project configuration contains advanced workflow/folder settings. | Keep the native page and current capabilities. Make the active Local/Fleet owner explicit, put connection/provider and project essentials first, and collapse advanced controls by scope. |
| Read visual state | Dense rail and toolbar, consistent status colors, small context labels, expandable tool details and accessible hover/focus actions. | Shared palette is close; density and spacing vary across board, composer, workflow, settings and orchestration. | Use one semantic token system and a small spacing/type scale. Match hierarchy and density from evidence, not by copying stale component rules. |

## UX convergence plan for review

This is the ordered implementation plan. It deliberately separates behavioral
parity, information hierarchy and styling so a broad restyle cannot conceal a
broken workflow.

1. **Matched baseline and design-token map.** Build both applications from
   current source and use equivalent disposable project, tracker, task, run
   and conversation fixtures. Capture matched light and dark states at the same
   window sizes: project/task rail, board, prompt, active conversation with
   tools, waiting question, execution/diff and settings. Normalize language,
   connection mode and fixture state. Record font, line height, header/rail
   dimensions, control height, spacing and semantic color differences. This is
   an evidence-only lot: do not change layout or style until the comparison is
   reviewable. **Hard gate:** do not begin lot 2 until all required states have
   matched captures or a documented reason a surface cannot be matched.
2. **Workbench navigation and density.** Apply explicit launch rules: (a) on a
   returning launch, restore the last valid project and view; (b) if that
   project or conversation was removed or is inaccessible, fall back to the
   project board, then to Local setup if no project remains; (c) on a first
   launch with no projects, open native setup with the open-folder action
   immediately available; (d) after creating a folder project, land on its
   prompt. Do not silently select the first project's board on every launch.
   Keep the project rail resizable. Use exactly one navigation row for each
   remote ticket and each local conversation. Inline active/waiting run state
   on its owning ticket or conversation; put additional run history in the
   project's Executions view, not in a second copy of the same row. When a
   conversation snapshot is loaded, show its turn state; otherwise call a
   surviving provider process “Session active,” never “Working.” Preserve the
   command palette and one-click project prompt. Acceptance: all active/waiting
   work is reachable once from its owner; project selection and view are
   unambiguous; at 800 px and 1280 px, long labels truncate with their complete
   accessible name and the rail does not obscure primary actions. Verify first,
   returning, deleted-project and empty-workspace launch cases.
3. **Ticket board and workflow hierarchy.** Match Electron's task-first scan
   path using Tauri's tracker-authoritative board and current workflow model.
   Distinguish remote tickets from local workspace prompts, retain tracker
   status/stage filters only where meaningful, and put the ticket's true next
   action, current run and review result together. Acceptance: empty, stale,
   disconnected, waiting, failed and completed states say what the user can do;
   Local-only projects never imply a tracker is required.
4. **Prompt and execution surface.** Tighten first-prompt whitespace and give
   the composer predictable placement and width. Keep provider/profile/model/
   mode and the effective working directory visible; show attached folders as
   shared project context, with detailed paths progressively disclosed. Keep
   assistant Markdown readable, collapse tool groups, stream without scroll
   jumps, expose questions/approvals inline, and keep interrupt/review/diff
   controls close to the active run. Acceptance: first-run, resumed, active,
   waiting, interrupted, error and read-only conversations remain usable at
   narrow and normal widths and with keyboard navigation.
5. **Orchestration and execution supervision.** Align project-level
   orchestration settings, eligible task selection, shared capacity, launched
   and skipped reasons, run activity and stop controls into a predictable
   project scope. Use the same run-status vocabulary and compact row treatment
   as the project rail. Acceptance: an execution has one authoritative status,
   every stop targets the identified run/agent, and no UI implies an external
   tracker transition succeeded before Go confirms it.
6. **Native settings and advanced configuration.** Define topology in the
   language shown to users: Local means the Go backend/state and execution
   agent are on this device, while connected tracker data is still remote;
   Fleet means the selected backend owns profile, tracker and workflow state,
   while the paired local agent owns filesystem, CLI and process execution.
   Show the responsible owner beside the relevant profile, tracker, folder or
   execution controls rather than implying that one “Local” or “Fleet” label
   describes every resource in a project. Acceptance includes a Fleet-backed
   project with a local folder, a remote tracker and a local execution agent,
   plus connection loss and reconnect. Compare the Electron
   workstation/project settings inventory with native Local/Fleet sections.
   Keep one canonical location for each action; put core connection, profile,
   provider readiness and project folder choices first; group MCP, sandbox,
   permissions and workflow policy under clearly scoped advanced sections.
   Preserve OAuth, provider sign-in and native filesystem flows. Acceptance:
   labels identify which backend owns each setting and advanced details remain
   searchable and keyboard accessible. Verify that entering Settings,
   navigating its sections, reopening Settings, changing project/view while
   Settings is open, and returning after the prior project was removed each
   produce a truthful header action and restore a valid destination.
7. **Visual system and cleanup.** After the structural lots pass their
   acceptance checks, normalize typography, spacing, control sizing, borders,
   hover/focus/selected states and reduced-motion behavior against the matched
   captures. Consolidate shared color/spacing/type tokens; remove only CSS
   overrides proven unused by component and screenshot checks. Acceptance:
   both themes meet WCAG 2.2 AA text contrast (4.5:1 for normal text and 3:1
   for large text), focus indicators remain at least 2 px and visible, and no
   change weakens dense work surfaces. Record bundle size against the current
   committed baseline; size is tracked, not a visual acceptance criterion.

Every lot also passes these interaction checks: a keyboard-only user can
complete its primary journey; interactive controls have names and state that
screen readers can announce; status changes use an appropriate live region;
dialogs and the command palette expose correct dialog/label semantics; focus
lands in the opened surface, remains visible and returns to its invoker after
close/navigation. At the minimum supported window size, content either wraps or
truncates with a programmatically available full value; no required action is
reachable only through hover.

Before each implementation lot, review its screenshots and behavioral
acceptance against this plan. No tab UI, tracker write, provider message, mode
switch, or web planning change is included in the convergence work. The
independent design challenge accepted the direction after the launch rules,
Local/Fleet ownership model, navigation identity, accessibility checks and
matched-fixture gate were made explicit. Matched fixtures remain a required
baseline deliverable before lot 2; the review does not substitute for those
captures.

### Independent design challenge

The plan was reviewed by an independent UX sub-agent before implementation.
Its initial review found seven gaps: a missing exact launch rule; visual claims
based on unmatched states; an underspecified mixed Local/Fleet ownership model;
possible duplicate task/run rows; broad accessibility language; a loose gate
between fixture capture and navigation work; and untested settings-return
cases. The plan now defines first/returning launch behavior, labels ownership
per resource, assigns one navigation row per ticket/conversation, separates
turn status from process status, gives keyboard/screen-reader/contrast/focus
checks, makes matched screenshots a hard gate, and covers stale/reopened
Settings navigation. The reviewer found no reason to stop the UX work and
recommended completing matched fixture captures before implementing the
navigation lot.

## Concrete findings

| Finding | Evidence | Decision |
| --- | --- | --- |
| Connection state was repeated in the header, workspace footer and project sidebar. | `App.tsx` rendered `connection-pill`, `statusbar` and `sidebar-footer` from the same `connected` value. | Keep the persistent, labelled agent status in the sidebar, matching Electron's status placement. Remove the other two copies. |
| The workspace footer claimed “Données et exécution locales” regardless of which backend supplied tracker state. | The footer was static while the project board reads remote tracker tickets. | Remove the claim instead of presenting topology as business-data ownership. |
| A global refresh action only refreshed the project list, while the board had a separate refresh for ticket data. | Header called `refreshProjects`; the board toolbar called its own task refresh. | Remove the misleading global refresh. Keep project refresh in project/settings flows and task refresh beside the board it updates. |
| Electron's searchable command palette is missing from Tauri. | `desktop/src/main.js` indexes actions, projects and executions; Tauri had only direct sidebar/header controls. | Add a small React palette on Cmd/Ctrl+K with actions, projects and runs; do not add a UI dependency. |
| Styling is split between base rules and a second override sheet. | `App.css` owns foundational controls/layout while `workspace-polish.css` redefines tokens, radii, spacing and component density after it. | Consolidate tokens and component rules after the navigation behavior settles. Avoid broad cosmetic changes before checking light/dark screenshots. |
| Tauri and Electron have related but non-identical theme tokens. | `App.css` and Electron `desktop/src/style.css` share the dark palette and several light background/control values, but light `--sunken`/`--surface-sunken`, accent, selected and danger values differ; Electron also defines more semantic stage, diff and status tokens. `workspace-polish.css` currently derives only the workspace divider instead of redefining the base palette. | Treat `App.css` as the Tauri token source and compare semantic values against matched captures before aligning them. Do not claim palette parity based on similar colors or token names. |
| The Tauri sidebar was fixed width; Electron has a pointer and keyboard resizer. | Electron `desktop/src/main.js` wires `#sidebar-resizer`; Tauri originally rendered a fixed-width `.sidebar`. | Added a bounded, keyboard-accessible divider and persistent width. |
| Settings are a full native page in Tauri and a categorized dialog in Electron. | Tauri's `LocalSettings` has separate engine, profile, connection, project and service sections; Electron groups workstation/project settings in its dialog. | Keep native Tauri setup, but align section naming, compact spacing and progressive disclosure. Do not force the dialog layout into a native page. |
| The project sidebar and settings section navigation competed for attention on the native Settings page. | A same-size, light-theme current-source screenshot showed the Tauri project rail and settings rail taking about 418 CSS pixels before the settings content; Electron's settings view uses one rail. | Hide project navigation when Settings is opened from a workspace, retain the settings section rail, and return to the previous workspace from the header. Keep project navigation available for initial setup when there is no prior workspace. |
| Fleet configuration was labeled as Local in the settings header and service navigation. | `LocalSettings` used “VOTRE ESPACE LOCAL” and “Services locaux” for every mode, despite Fleet profile changes going to `/api/me` and the Fleet agent remaining remotely managed. | Identify Fleet in the settings heading, call the section “Connexion et services” in Fleet mode, and describe the profile according to its backend owner. |
| A local-only project shows tracker controls even though it has no remote tickets. | In the live Tauri window, `workspace` has a local path and zero tickets, while the board toolbar still says “Statuts du tracker · lecture seule” and offers tracker grouping/search. Go represents a local workspace with a synthetic `local` tracker row, but the desktop board projection omitted the provider, so the renderer could not distinguish it from a remote tracker. | Include tracker providers in the additive desktop board projection. Hide remote controls when a project has no trackers or only the synthetic local tracker; preserve remote controls for GitHub, GitLab and Jira. Keep “Nouveau prompt” as the single primary action. |
| An empty tracker-backed project with a local folder looked like an untracked local workspace. | The empty-board copy used the presence of `project.path` to say “Votre projet, prêt à démarrer,” even when an external tracker was connected and no remote tickets were returned. | Name the missing remote tickets, keep tracker search/grouping visible and point to the local prompt as a separate action. |
| A failed tracker refresh left previously read tickets visible without identifying them as stale. | `ProjectPane.refresh` preserves the last successful `tasks` on error, which is preferable to an empty board, but only displayed the transport error. | Keep the previous data visible and identify it as coming from the previous successful read until refresh succeeds. |
| Conversation turn readiness differed from the long-lived agent process status. | A provider process can remain running and ready for another turn after its current response completes; generic execution labels therefore said “En cours” while the conversation view said “Prêt”. | Show the loaded conversation turn state in project navigation. When the history is not loaded, label the process “Session active”; keep generic execution lifecycle and stop semantics intact. |
| The local-project empty state repeats the primary prompt action. | The project toolbar already has “Nouveau prompt”; the empty board adds “Commencer une conversation”. The live screenshot shows both at once. | Keep the global project action and remove the duplicate empty-state button; use the empty state to explain that the local project is ready for prompts. |
| The project toolbar offers navigation to the view already open. | The live board screenshot shows a “Tableau” button beside the current project; `App.tsx` always renders it, and its handler navigates to the same board view. | Hide the current-view action while the board is open. Keep “Tableau” available from prompts and execution views so navigation remains direct. |
| The prompt folder action obscured its project-wide scope. | `PromptFolders` listed the active directory and project attachments together; its “Ajouter un dossier” action did not explain that the selected folder is shared with every project conversation. | Call the action “Attacher au projet,” label the disclosure “Dossiers du projet,” and state that the attachment is shared across conversations. Keep detailed roles and paths in the disclosure. |
| The current live Electron/Tauri windows are not a valid populated-state visual comparison. | Both windows were captured at the same desktop size, but Electron was showing a long historical conversation and orchestration sidebar while Tauri showed an empty Local board. A follow-up read-only check opened the same project label in both apps: Electron showed a tracker-backed task list, while Tauri showed a Local project with no tracker mapping. The running Tauri release bundle also predates the current source fixes. | Use these captures only for shell/navigation comparison. A matching project name does not prove matching backend or tracker state. Before tuning typography, spacing or colors, capture equivalent data modes, themes and current-source builds; do not infer visual style changes from mismatched content or stale binaries. |

## Current source and runtime status

### Current-source visual baseline (source measurements only)

The token and layout comparison below records values from the current source;
it is not a replacement for the matched screenshots required by lot 1. The
Electron app's existing UI harness can return stable tracker/task fixtures, but
it deliberately launches hidden windows. Tauri routes the same views through a
managed Go companion and native Local/Fleet setup, so the Electron stand-in
server cannot be reused as its backend. The currently open production windows
also contain different projects and tracker modes. Therefore no populated
state has yet been captured from equivalent fixtures, and the visual gate
remains open.

| Measurement | Electron source | Tauri source | Review note |
| --- | --- | --- | --- |
| Base interface font | `14px system-ui` | `13px system-ui, -apple-system` | Tauri is one CSS pixel smaller. Compare rendered text at the same scale before changing it. |
| Header | `68px` | `54px` after `workspace-polish.css` (58px in base `App.css`) | The Electron native title-bar overlay is part of its 68px header. Compare the full native window chrome, not renderer content alone. |
| Project rail | `280px` base, no pointer resizing in the old shell | `268px` with keyboard/pointer resizing | Width alone does not describe available work-area width; compare at equal window sizes and rail widths. |
| Default button radius | `7px` | `8px` after polish (6px in base CSS) | Tauri's polish sheet overrides the base control radius. |
| Dark accent / selected | `#62d3be` / `#203033` | `#62d3be` / `#203033` | Core dark values match in source. |
| Light accent / selected | `#0f766e` / `#e3f2ef` | `#167d69` / `#e0eee9` | Values differ; rendered contrast and visual weight still need matched captures. |
| Light danger | `#b42318` danger, `#a8243d` danger text | `#a8243d` danger, `#fce9ed` danger background | Token names and roles differ, so compare semantic actions rather than matching variable names. |

Lot 1 should next provide a shared fixture contract for project, ticket,
conversation/tool events, waiting question, run/diff and settings. Keep any
fixture path test-only and isolated from Local/Fleet user data. Capture the
available matched states at 800px and 1280px in light and dark mode, then list
the exact states still blocked by native setup or backend-specific ownership.
Only close the gate when every required surface is captured or a specific
surface cannot be represented in both apps without changing product behavior.

The first isolated capture used the same `workspace` name and
`/private/tmp/sectile-ux-fixture/workspace` path in current Tauri and Electron
builds, at the same 2560 × 1600 desktop size in dark mode. Tauri reached its
first-class project composer with the folder context, engine, model and mode
visible. Electron reached its “Project prompt” dialog with the same folder and
Codex engine, then offered “Open console”; that path starts an interactive CLI
session rather than rendering an in-app prompt composer. This is a useful
pre-dispatch comparison, but it does not imply that the post-dispatch
conversation or tool views match. The Tauri Local board showed its trackerless
ready state; Electron had no local tasks and its central execution surface was
blank. Since task-board content is remote-tracker-owned in Tauri, those two
empty states are not equivalent ticket fixtures. The isolated app data,
workspace, mock agent, test bundle and port-52964 companion were removed; the
production companion on port 52963 remained running. No prompt was submitted,
provider or tracker was contacted, and no project was created outside the
disposable test data.

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
- A second isolated macOS visual check ran the latest Tauri source at commit
  `6a8caf8` with a uniquely named app bundle and a test-only Go companion on
  port `52964`. It verified that a trackerless Local project opens with the
  local-ready message and no remote ticket controls, Settings hides the project
  rail when entered from a workspace, and the header return action restores the
  prior workspace. The AI engines settings screen was also inspected. Electron
  still had Fleet/tracker-backed configuration, so this run verifies
  current-source Local behavior and navigation only; it does not close the
  matched visual-comparison item. The isolated profile, folder, app state,
  companion process, bundle and worktrees were removed. The production Local
  companion on port `52963` was not touched.
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
- Settings now hide the project rail when opened from a workspace and provide
  a direct return action; first-run setup without a prior workspace keeps
  project navigation available. A workspace UI regression covers the round trip.
- A failed ticket refresh now preserves the last successful board and labels it
  as previous data alongside the read error. A UI regression exercises a
  successful read followed by a failed refresh.
- An empty remote board now says that the tracker returned no tickets and
  offers the associated local folder as a separate prompt path. A UI regression
  verifies remote board controls remain visible for Jira.
- Conversation navigation uses the loaded conversation snapshot for its turn
  status and uses “Session active” when only process state is available. The
  command palette no longer assigns a potentially stale process status to a
  conversation. UI and domain regressions cover an idle conversation whose
  provider process remains running.
- A read-only live inspection confirmed that the currently installed Tauri
  release is older than the current UX source. Its Local project screen still
  shows tracker controls, while current source has a regression-tested guard.
  The open Electron and Tauri screens also use different backend/project
  mappings despite one matching project label; they are not a valid matched
  content comparison. Neither observation changes tracker data or runtime
  configuration.
- `workspace-polish.css` no longer duplicates Tauri's root palette; shared
  Tauri color tokens live in `App.css`. This is stylesheet ownership cleanup,
  not color parity: the light selection, accent, sunken surface and danger
  values still differ from Electron and require matched screenshot review.

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

## Remaining convergence work

The plan above follows the previous source audit and tested parity changes.
Matched current-source populated screenshots are still required before visual
normalization; current Electron and Tauri production windows do not contain
equivalent task/activity state. The work should be reviewed against the
acceptance criteria in sequence, preserving tracker-authoritative ticket state,
existing Go execution/MCP contracts, accessible focus behavior, the no-tabs
decision and the fast standalone Local path.
