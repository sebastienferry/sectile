# Electron and Tauri desktop UX audit

Date: 2026-10-29
Status: all seven scoped UX lots are implemented and revalidated against the
current source on 2026-10-29. Matched captures cover the shared board,
pre-dispatch prompt and settings surfaces. Owner-provided Electron conversation,
backlog and board screenshots on 2026-10-30 add explicit interaction references;
post-dispatch execution remains product-specific rather than a pixel-parity target.
This records hierarchy and behavior convergence, not pixel identity.

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
   window sizes for the project/task rail, board, pre-dispatch prompt and
   settings. Normalize language, connection mode and fixture state. Record
   font, line height, header/rail dimensions, control height, spacing and
   semantic color differences. This is an evidence-only lot: do not change
   layout or style until the comparison is reviewable. **Hard gate:** do not
   begin lot 2 until shared surfaces have matched captures and each
   product-specific surface has a documented reason it cannot be matched.
   Electron's interactive CLI console and Tauri's in-app conversation,
   transcript, questions and diff are product-specific after dispatch; do not
   manufacture parity by changing their execution model.
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

The findings below preserve the observations and decisions made during the
review. References to items being open or pending describe their state at that
point in the audit; the final verification and current disposition appear in
the convergence status section below.

| Finding | Evidence | Decision |
| --- | --- | --- |
| Connection state was repeated in the header, workspace footer and project sidebar. | `App.tsx` rendered `connection-pill`, `statusbar` and `sidebar-footer` from the same `connected` value. | Keep the persistent, labelled agent status in the sidebar, matching Electron's status placement. Remove the other two copies. |
| The workspace footer claimed “Données et exécution locales” regardless of which backend supplied tracker state. | The footer was static while the project board reads remote tracker tickets. | Remove the claim instead of presenting topology as business-data ownership. |
| Refresh actions can obscure which data they update. | The board-refresh control appeared in orchestration and execution views next to the supervisor's separate run refresh. | Show ticket refresh only on the board and execution refresh only in supervision, beside the data each action reloads. |
| Electron's searchable command palette is missing from Tauri. | `desktop/src/main.js` indexes actions, projects and executions; Tauri had only direct sidebar/header controls. | Add a small React palette on Cmd/Ctrl+K with actions, projects and runs; do not add a UI dependency. |
| Styling was split between base rules and a second refinement sheet. | `App.css` held foundational controls/layout while `workspace-polish.css` supplied presentation rules later in the cascade. | Combine both into `App.css` after behavior settled. This is now done without changing the existing declarations; keep `App.css` as the only Tauri stylesheet entry point. |
| Tauri and Electron have related but non-identical theme tokens. | `App.css` and Electron `desktop/src/style.css` share the dark palette and several light background/control values, but light `--sunken`/`--surface-sunken`, accent, selected and danger values differ; Electron also defines more semantic stage, diff and status tokens. | Treat `App.css` as the Tauri token source and compare semantic values against matched captures before aligning them. Do not claim palette parity based on similar colors or token names. |
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
| Prompt controls became uneven and the execution guidance wrapped into a narrow column on an 800 px workspace. | A current-source light-theme capture showed profile, engine, model and mode controls wrapping inconsistently, with the mode guidance squeezed beside the send action. | On narrow windows, place profile/engine and model/mode in two predictable rows and let the mode guidance use the full control width. Keep the existing wide-window layout. The retake at `/tmp/sectile-tauri-prompt-light-800.png` shows both rows and readable guidance. |
| The current live Electron/Tauri windows are not a valid populated-state visual comparison. | Both windows were captured at the same desktop size, but Electron was showing a long historical conversation and orchestration sidebar while Tauri showed an empty Local board. A follow-up read-only check opened the same project label in both apps: Electron showed a tracker-backed task list, while Tauri showed a Local project with no tracker mapping. The running Tauri release bundle also predates the current source fixes. | Use these captures only for shell/navigation comparison. A matching project name does not prove matching backend or tracker state. Before tuning typography, spacing or colors, capture equivalent data modes, themes and current-source builds; do not infer visual style changes from mismatched content or stale binaries. |

## Current source and runtime status

### Current-source visual baseline (source measurements only)

The token and layout comparison below records values from the current source;
it is not a replacement for matched screenshots. A disposable Electron copy
and a development-only Tauri fixture now provide the same project and three
Jira ticket records without reading or changing production app data. The
Electron copy uses a fake loopback agent and a separate user-data directory;
the Tauri fixture rejects unsupported commands and never contacts a tracker or
provider. The current production apps remain unsuitable for comparison because
their project and tracker modes differ.

| Measurement | Electron source | Tauri source | Review note |
| --- | --- | --- | --- |
| Base interface font | `14px system-ui` | `13px system-ui, -apple-system` | Tauri is one CSS pixel smaller. Compare rendered text at the same scale before changing it. |
| Header | `68px` | `54px` in the consolidated Tauri `App.css` | The Electron native title-bar overlay is part of its 68px header. Compare the full native window chrome, not renderer content alone. |
| Project rail | `280px` initial; pointer and keyboard resizing from 210px to 520px, further capped by window width | `268px` initial; pointer and keyboard resizing from 200px to 480px | Both shells support resizing. Their initial widths and bounds differ; compare at equal window sizes and rail widths. |
| Default button radius | `7px` | `8px` in the consolidated stylesheet (6px remains on selected base controls) | The refinement rules are consolidated in `App.css`; there is no separately imported polish sheet. |
| Dark accent / selected | `#62d3be` / `#203033` | `#62d3be` / `#203033` | Core dark values match in source. |
| Light accent / selected | `#0f766e` / `#e3f2ef` | `#167d69` / `#e0eee9` | Values differ; rendered contrast and visual weight still need matched captures. |
| Light danger | `#b42318` danger, `#a8243d` danger text | `#a8243d` danger, `#fce9ed` danger background | Token names and roles differ, so compare semantic actions rather than matching variable names. |

Contrast ratios computed from the current Tauri theme tokens meet AA for the
core text roles: primary text on a surface is 12.15:1 in dark mode and 18.62:1
in light mode; muted text is 6.15:1 and 6:1 respectively. Accent text on its
selected surface is 9.22:1 and 5.92:1, danger text on its danger surface is
9.02:1 and 6.04:1, and text on the accent button is 8.89:1 and 5.03:1. The
focus outline is 2px and its accent color contrasts with the surface at 8.91:1
and 5.03:1. Button transitions are limited to `prefers-reduced-motion:
no-preference`. The matched captures show no visual reason to change these
semantic tokens or enlarge the compact Tauri type scale; their small color
differences from Electron retain AA contrast and preserve the lighter native
settings hierarchy.

The board, pre-dispatch prompt and settings hierarchy have been inspected with
isolated data at 1280 × 900 CSS pixels and 800 × 820 CSS pixels, in both light
and dark themes.
The populated board uses the same three Jira tickets in both applications. The
isolated Electron copy used a fake loopback agent and separate user-data
directory; Tauri used the development-only fixture. The board captures are
`/tmp/sectile-electron-board-light-1280.png`,
`/tmp/sectile-electron-board-dark-1280.png`,
`/tmp/sectile-tauri-board-light-1280.png`,
`/tmp/sectile-tauri-board-dark-1280.png`,
`/tmp/sectile-electron-board-light-800.png`,
`/tmp/sectile-electron-board-dark-800.png`,
`/tmp/sectile-tauri-board-light-800.png` and
`/tmp/sectile-tauri-board-dark-800.png`. They are evidence artifacts, not
committed product assets.

At 800px, Electron allocates 280px to its project rail and has about 520px for
the work surface. Its task board groups by workflow stage and horizontally
scrolls through six possible stage columns; its toolbar wraps into multiple
lines. Tauri allocates 268px to its resizable rail and has about 532px for the
work surface. Its default tracker-authoritative board shows two of three remote
status columns at once and scrolls horizontally. Both retain their primary
board actions and task records. These are different semantics by design:
Electron groups by workflow stage, while Tauri defaults to the remote tracker’s
three statuses and retains workflow stage on each ticket. Do not force those
column semantics to match. The Tauri fixture is repeatable from
`tests/fixtures/ux-baseline.json` with `npm run dev:ux-fixture`.

The orchestration surface was reviewed at 800 × 820 with two live executions,
one waiting for user input, a saved backlog agent and server capacity data.
Each execution keeps its own view and stop actions when the rows wrap. The
summary now names activity on this workstation separately from the shared
server capacity, whose current occupancy may differ from local activity. The
project header shows its full short name, and the board refresh control is not
shown beside the separate execution-tracking refresh. The captured view is
`/tmp/sectile-tauri-orchestration-light-800-refined.png`.

The settings screens were captured at both sizes and in both themes. The
800px captures are
`/tmp/sectile-electron-settings-light-800.png`,
`/tmp/sectile-electron-settings-dark-800.png`,
`/tmp/sectile-tauri-settings-light-800.png` and
`/tmp/sectile-tauri-settings-dark-800.png`; the 1280px captures use the same
filenames with `-1280` in place of `-800`. The matched settings captures
confirm that both apps use a left-side settings index and a main
content region. Electron gives its index about 250px of the 800px window and
keeps workstation categories plus a collapsed project group visible; Tauri
uses a narrower native index and devotes more of the page to the selected
configuration. Their inventories and ownership are intentionally different:
Electron's workstation settings describe a paired server, whereas Tauri's
native page configures Local/Fleet-owned services and providers. The Electron
fixture does not contain provider installation/readiness evidence equivalent
to Tauri's synthetic ready profile. Compare navigation and hierarchy, not
readiness-card detail or copy. Tauri runtime strings remain French and
Electron's remain English; this is a product-localization difference, not a
reason to translate one app during a styling change.

The matched 800px captures also showed Tauri wrapping “Comptes et trackers”
inside its narrow settings index. The layout now gives the index 205px below
1100px and uses a 22px column gap. At an 800px window this leaves 517px for
the main settings pane. A fresh isolated `ux-fixture` preview at 800 × 820 in
light and dark themes showed all five section names on one line, with the
assistant settings card and its controls still visible. The preview used
mocked desktop commands; no provider or production companion was contacted.
The production build passed after the change.

The pre-dispatch project prompt was also captured from current builds at both
sizes and in both themes. Electron's “Project prompt” is an engine-selection
dialog that names the local repository and opens an interactive CLI console;
it has no prompt field. Tauri accepts the actual prompt in its workspace and
shows project-folder context, provider, model and Plan/execution mode before
dispatch. The first 800 × 820 capture put suggestions and the keyboard hint
below the visible area. A short-window layout now reduces intro and composer
spacing so the send action, provider/model/mode controls, prompt suggestions
and keyboard hint all fit without scrolling. At 1280 × 900 the composer and
suggestions retain the roomier layout. This is a matched pre-dispatch context
comparison, not a claim that these flows have identical behavior. The
screenshots are `/tmp/sectile-electron-prompt-light-800.png`,
`/tmp/sectile-electron-prompt-dark-800.png`,
`/tmp/sectile-tauri-prompt-light-800.png`,
`/tmp/sectile-tauri-prompt-dark-800.png`, and corresponding `-1280` captures.
Both applications used the same disposable `Project A` and
`/tmp/sectile-ux-fixture/workspace` folder. Electron used a fake loopback agent
and separate user data; Tauri used the development-only fixture. Nothing was
submitted, and no provider, tracker or production companion was contacted.
The refinement was checked in both themes at 800 × 820; the composer remains
within the viewport and the prompt suggestions and keyboard hint are visible
below it. `npm run check` passes.

Conversation, waiting-question and execution/diff states cannot be matched
between the current products without changing their behavior: Electron opens
CLI sessions in its console/terminal, while Tauri hosts the conversation and
tool transcript in its own renderer. Tauri fixtures capture those surfaces for
internal consistency, but they are not Electron parity targets. The matched
shared-surface gate is closed for the populated project board, pre-dispatch
prompt context and settings hierarchy at 1280 × 900 and 800 × 820 in both
themes. The captures are an evidence baseline, not pixel-approval for styling
changes.

A Tauri-only waiting-question fixture at 800 × 820 exposed a third, outer
scroll area that pushed the question action buttons below the visible window.
Conversation workspaces now clip to the available height, and narrow layouts
keep project-folder and MCP controls on compact rows while giving an active
question more of the response area. The retake shows the full question and its
answer/skip actions, with a 176px transcript viewport and no competing outer
scrollbar. The composer remains reachable by scrolling the response area after
answering; the active question is the primary action while it is pending.

The execution-history route had a separate style leak: the launch panel's
generic form selectors also affected the conversation composer rendered in an
execution detail. At the 800px fixture width the composer was split into 160px
and 220px columns; the launch rules are now scoped to that panel, and the
composer renders as one 390px column. The route still intentionally includes
execution identity, stop and review controls above the conversation.

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
- The desktop now restores its last valid project view after launch. A removed
  project falls back to the first available board, an unavailable saved
  conversation falls back to its project board, and an empty project list
  opens Local setup. Unit and UI regressions cover saved destinations,
  conversation availability and first-project fallback.
- Workflow executions no longer create a second project-rail row for a
  ticket already present on the board. Active task-run state is shown on that
  ticket; conversation rows remain independently navigable, and run history
  stays in the project's Executions view. A UI regression covers the active
  ticket state and absence of the duplicate row.
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
- On 2026-10-29, the isolated `Sectile Upgrade Qualification` app was opened
  after the workstation was unlocked. It was a fresh, unconfigured test
  identity, so its companion had not yet been requested to start; the native
  setup screen offered “Start in Local mode.” Starting Local created its
  isolated runtime on port `52964`. The authenticated runtime health check
  returned protocol 1, `ready: true` and `companion: true`. Saving a synthetic
  test profile changed the desktop status to “Agent connected” and created the
  default project. No tracker credentials, remote tracker, OAuth provider or
  AI provider were used. This distinguishes first-run setup from companion
  startup failure. The test bundle is an older qualification build and showed
  an obsolete Local/Fleet setup warning on its AI-engine screen after Local
  was connected; current source does not contain that warning. This is not
  evidence of a current-source settings defect. The production companion on
  port `52963` remained running and was not changed.
- Before consolidation, `workspace-polish.css` no longer duplicated Tauri's
  root palette; shared color tokens already lived in `App.css`. The light
  selection, accent, sunken surface and danger values still differ slightly
  from Electron. Matched captures and core-token contrast checks support
  retaining those values and Tauri's more compact native settings hierarchy.

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
- Consolidated the Tauri presentation refinements into `App.css`, removing the
  separately imported `workspace-polish.css` without changing existing style
  declarations. The focused narrow-prompt rule is scoped to the initial project
  composer so resumed conversations retain their established controls.

## Convergence status (verified)

Earlier closure was reopened because the style rules were split across two
imported stylesheets and the Electron board artifact was not a valid content
match for the Tauri board. Both issues have since been addressed: Tauri's
presentation refinements are consolidated in `App.css`, and the board baseline
uses equivalent disposable Jira tickets in both applications. A current-source
review also refreshed the short-window prompt and orchestration captures after
the final responsive layout changes. These checks support closing the seven
scoped lots; they do not claim pixel parity or map Electron's CLI console to
Tauri's provider conversation surface.

All seven lots are complete within the approved scope. The table below records
the implementation evidence and deliberate product differences. This work
preserves the single workspace, tracker-authoritative ticket state, native
project folders, Local and Fleet ownership, and existing Go execution/MCP
contracts.

| Lot | Outcome |
| --- | --- |
| Matched baseline | Board, pre-dispatch prompt and settings captured at normal and narrow sizes in light and dark themes. Product-specific post-dispatch surfaces have documented reasons they cannot be matched. |
| Workbench navigation | Restore and fallback rules, one navigation row per ticket/conversation, execution history, resizable rail and command palette are implemented and covered by regression evidence. |
| Board and workflow | Remote status remains tracker-owned; workflow stage, next action, current run and review state stay distinct and truthful. Local-only projects do not imply tracker setup. |
| Prompt and execution | Project context, folder attachments, provider/model/mode selection, Markdown, grouped tools, questions, streaming and stop/review controls remain available. Short-window prompt and question layouts keep their primary actions reachable. |
| Orchestration and supervision | Run rows retain identified view/stop actions; workstation activity and server-shared capacity are labeled separately; board refresh appears only on the board. |
| Native settings | Local/Fleet ownership, profile, tracker, project, provider and service responsibilities remain explicit. Advanced token, permission and execution controls stay available in their scoped sections. |
| Visual system | Matched theme captures, semantic color roles, AA text contrast, visible focus and reduced-motion behavior are recorded. No broad token change was justified by the captures. |

The subsequent conversation-status pass uses Go's `prLinks` contract to show
every linked PR/MR and its last known forge state in the conversation strip and
ticket workflow. The sidebar keeps a compact primary-link shortcut. When Go
reports a missing forge token or no refreshed state, the desktop says so rather
than treating the workflow stage as the PR/MR state. Domain and UI regressions
cover multiple links, state labels and the legacy `prUrl` fallback.
The isolated macOS app displayed the QA ticket's linked `MR !1 · état inconnu`
in the workflow. That fixture uses a local forge simulator and does not prove a
live forge-state refresh; the native capture is
`docs/ux-captures/workflow-pr-state-native.png`.
That capture also exposed an elongated blue activity indicator in the project
rail. The generic run-row flex rule matched the conversation indicator; it is
now scoped to non-conversation run rows. The native retake is recorded in
`docs/ux-captures/conversation-indicators-native.png`.
The same workflow capture showed that an unverified engine-preparation note
occupied a large card even though the launch path rechecks preparation and
remains available. This note is now a collapsed disclosure; a known MCP
configuration conflict still renders as an explicit error. The native retake
is `docs/ux-captures/workflow-preparation-compact-native.png`.
The ticket workflow also repeated board, prompt and project-settings actions in
its header despite having its own return action and the project sidebar. The
workflow now keeps the project identity and path in its header, then uses the
ticket-level return action; the native retake is
`docs/ux-captures/workflow-focused-header-native.png`.
The native Codex free-prompt conversation showed "Skill: None", "Ticket: free
prompt" and "Stage: no ticket" in the status row. Those values do not help a
user resume or supervise that discussion. Free prompts now show activity and
execution only; real skills and ticket-linked metadata remain visible. The
narrow-window native retake is `docs/ux-captures/free-conversation-status-native.png`.
At 800 CSS pixels, the retained 368-pixel project rail left only 432 pixels for
the conversation (`docs/ux-captures/free-conversation-status-before-native.png`).
Its visible width now caps at 32% of the window, with a 200 pixel floor; the
saved preference returns when the window widens. The divider's accessible value
and keyboard limits follow the visible width.

Current verification passes on Tauri commit `239d7c1`: `npm run check`, 14
domain tests, 101 UI tests, 35 Rust tests and `npm run build`. The short-window
prompt and orchestration surfaces were retaken from the current source at
800 × 820 in light and dark themes. The refreshed captures are
`/tmp/sectile-tauri-current-prompt-light-800.png`,
`/tmp/sectile-tauri-current-prompt-dark-800.png`,
`/tmp/sectile-tauri-current-orchestration-light-800.png` and
`/tmp/sectile-tauri-current-orchestration-dark-800.png`. Matched screenshots
are evidence for hierarchy and behavior, not pixel approval. Production
Electron and Tauri windows still use
different project and tracker modes, so they are not valid content-comparison
fixtures. Conversation transcripts, questions and execution/diff surfaces
remain intentionally Tauri-specific because Electron continues provider CLI
sessions in its console/terminal.

## Post-convergence model availability correction

A live tracked-task check on 2026-10-29 exposed an outdated Codex model
suggestion: `gpt-5-codex` was offered by the workstation catalogue but refused
by the signed-in CLI. The same static list is merged into Tauri's conversation
model selector even after Codex has returned its account-specific `model/list`.
That makes a known-unavailable suggestion appear beside verified live models.

For an active Codex conversation, use its live model list as the selectable
catalogue and retain the current model so an existing conversation remains
legible. Keep the workstation suggestions for the first prompt, where no live
conversation exists, and keep custom identifiers available. The first-prompt
list remains a suggestion rather than an availability guarantee; a future
provider-catalogue service may improve that path without delaying prompt entry.
Verify that live suggestions replace stale workstation suggestions, that the
current and pending selections survive a refresh, and that first-prompt and
custom-model flows remain intact.

The current board fixture also exposes a misleading action: each ticket card,
including a completed ticket, gives its workflow-navigation shortcut a play
icon and a “Launch” accessible name, while the click only opens the workflow
view. Keep the shortcut for fast task access, but use an open arrow and name the
destination consistently in its tooltip and accessible label. Verify the
completed card no longer suggests that a run will start on click.

Opening that completed card in the isolated UX fixture revealed a separate
loading race: React invalidated the first workflow read while a guard rejected
the replacement read. The view stayed on “Loading ticket” until manual refresh.
Allow a new navigation read to supersede an older one, let only the newest read
update the view, and suppress overlapping periodic polls. Verify the initial
workflow becomes usable under React's effect remount and that the completed
ticket still has no launch action.

The completed workflow also rendered disabled next-step, chain and custom-skill
controls plus a provider-preparation warning, despite Go refusing every launch
for a finished ticket. Show completion, linked PR and execution history without
those unavailable controls. Keep the preparation warning only on a loaded,
unfinished workflow so it does not appear while ticket metadata is loading.

Execution history on a ticket retained a disabled stop button for completed
runs. Keep the stop action on active runs only, and label an active run with a
pending stop request as such. Historical output and review remain readable.

The free-prompt component kept its draft state when the active project changed,
and an in-flight conversation creation could navigate back to the old project
after the user switched. Scope the composer instance to the selected project
and conversation; keep a submitted message running in Go, but only navigate to
its conversation while its originating composer remains mounted.

The Tauri package still used its starter icon while Electron shipped Sectile's
branded icon. Reuse Electron's existing MIT-licensed vector and native icon
assets for Tauri's bundle and preview, and generate the Tauri platform PNGs
from the same source. This changes the app identity in the dock and package
without altering its bundle identifier or user-data path.

The tracker setup screen previously said only that synchronization was
requested, then dropped the Go activity ID. A live Jira read on the existing
desktop accepted the request but left an empty board with no completion or
failure indication. Preserve the activity ID across the Tauri command boundary,
read its status until terminal, show the result beside its tracker and refresh
the board after completion. The same path works for Local and Fleet because
both use the selected Go backend's activity API. The observed desktop used an
older companion; this UI correction still needs a current-source native run.
The observed Jira failure was also a credential-policy mismatch: the Local
companion applied Fleet's server-credential sync rule despite its connected
installation account having a Jira OAuth grant. Go ADR 0068 now makes Local
sync read with that identity while preserving the Fleet server-credential
contract. A simulated Jira OAuth sync and a personal-token GitHub sync pass
without server credentials. A subsequent headless run of the current packaged
companion on the existing Local installation completed the real PE Jira sync:
the activity reported 1,988 synchronized issues, and a read-only database
check found 1,988 tickets attached to that tracker. The companion was stopped
and port 52963 released. A subsequent run from the current native macOS app
used its existing Local Jira account to synchronize the PE source. Settings
showed the in-progress state beneath PE, then “1988 tickets synchronisés.”
Opening the PE board showed 1,988 tickets grouped in Jira status columns.
The app and its package-owned companion were stopped afterward. This confirms
the native activity result and board refresh with a real Local Jira read;
fresh OAuth consent and Fleet remain separate checks.

The native credential and ticket-source forms used the same React provider and
site state. Switching one provider silently reset the other's site draft and
could submit a source under the wrong provider. Their drafts are now separate,
with a UI regression covering both forms. The Fleet service copy now describes
the backend-owned profile and project controls exposed on this page instead of
claiming that the page only changes the Local companion.
OAuth settings also used to hide a grant entirely when Go marked it
disconnected, leaving no explanation for a subsequent sync failure. The
provider row now identifies the disconnected account and gives a reconnect
instruction while retaining the existing sign-in action.

The live PE Jira sync returned 1,988 tickets. Tauri previously created a full
card for every ticket in a status column on first render; Electron's task rail
uses compact rows. Tauri now renders 40 tickets per column initially, with an
explicit count and continuation control. Search and grouping still operate on
the full result, and active workflow tickets are moved into the first visible
batch. The board grouping loop also appends to existing arrays instead of
copying a growing array for every ticket. A 1,988-ticket UI fixture verifies
the initial DOM bound, expansion and full-result search; a separate fixture
verifies an active ticket beyond the initial position stays visible.
A current-source isolated browser preview at 1280 × 720 in the dark theme
showed 40 cards in each of three columns containing 1,400, 400 and 188
tickets. Scrolling the first column exposed its continuation button; clicking
it displayed 80 cards while retaining the 1,400 total. Searching for the
last fixture key, `PE-4592`, returned that ticket alone. This preview uses
the development-only `?fixture=large-board` path and does not substitute for
a native packaged-window check or a live Jira board screenshot.

Matched dark-theme board captures at 1280 × 720 exposed another density gap:
Electron opened compact task rows, while Tauri's cards used a taller detailed
layout. Tauri now opens ticket cards in a compact layout that keeps the key,
title, stage, assignee and active workflow state visible. The board control
restores the detailed layout and saves the choice for the next launch. A
focused UI test checks the default and persisted choice; an isolated browser
preview of the 1,988-ticket fixture checked the compact layout at 1280 × 720.
At 800 × 820, the board had 530 px of viewport width for 942 px of columns,
but its scroll container had no keyboard focus target or accessible name. The
board is now a named, focusable region with a visible focus indicator so users
can move through columns with the keyboard as well as a pointer or trackpad.

The isolated bundle replacement check also exposed an inaccurate transcript
notice: an idle Codex conversation with a completed answer said “Conversation
paused for agent restart” after the companion was stopped. Go commit
`554374b2` records whether a turn was actually in progress at shutdown and
emits that notice only for an interrupted turn. Focused restart tests and the
full agent/companion package suites pass. The observed bundle predates this
fix. A fresh isolated native bundle with Go `554374b2` then completed a Codex
turn with `IDLE_FIRST_OK`. After quitting the desktop and stopping its
companion, reopening restored the project folder, completed answer,
`gpt-6-astra` model and Execution mode. The transcript remained Ready and
contained no false restart-pause notice.

A later settings review found two ownership and readiness gaps. Fleet Services
described every agent as managed by another installation, even when Tauri had
paired and started its own agent. The text now distinguishes paired and attached
agents. Jira OAuth guidance now asks Fleet members to have an administrator add
the Jira site, since shared tracker creation is admin-only. In provider setup,
one failed CLI status request previously discarded the other provider's result
and left both cards looking as if detection were still running. Claude Code and
Codex now report independent results and errors, and a later refresh clears a
recovered provider's error. The 124 UI tests, TypeScript check and production
build pass on 2026-10-29.

The same review found an OAuth cancellation race. A status poll already in
flight could complete after the user clicked Cancel and present a connected
outcome; closing the flow could also send a second cancellation. The desktop
now gives an in-progress cancellation precedence over polling and skips the
cleanup request after a confirmed cancellation. A focused UI test resolves a
late connected poll after cancellation and verifies the cancelled state and
single backend request.

Returning from Settings to a Local project exposed a transient board
misclassification: while its context or ticket read was pending, Tauri showed
remote tracker search and an “Aucun ticket distant” empty state. The board now
waits for project context before showing tracker controls or an empty state,
keeps the Local classification during ticket refresh, and reports a failed
context read with a retry action. The same review found that tickets and the
search filter from the previous project could remain visible during a project
switch; both are now scoped to the selected project. Focused UI tests hold the
context and ticket requests pending, reject/retry a context read, and check a
failed Local ticket refresh. The 29-test workspace suite and production build
pass. A separate macOS bundle built from this Tauri source and Go `554374b2`
used an isolated companion, data directory, and port. Native setup created a
profile and selected a temporary workspace with the system folder picker. Its
board showed “Votre projet local est prêt” and no remote ticket controls.
Switching to the folderless Default Project showed “Aucune source de tickets
configurée”; switching back restored the Local state and folder path. The
native retake covers this Local path. Remote cross-project ticket and search
isolation is covered by the UI suite, not a native remote-tracker retake.

The native retake also showed that the folderless Default Project board asked
for a local folder but offered no action there. Its empty state now opens the
existing native folder picker directly and says that no tracker is required.
A failed board refresh is shown only
on the board; it no longer follows the user into a new prompt. The focused
workspace UI suite covers both transitions.

At the compact prompt width, the toolbar repeated “Nouveau prompt” even while
the new-prompt composer was already open. Project settings similarly linked to
itself. The current-view shortcuts are now omitted, while an existing
conversation retains its new-prompt action. Workspace UI regressions cover the
three states.

Electron exposes its desktop agent log in Settings, while Tauri's Services
screen previously told users to inspect `companion.log` without opening it.
Services now reads the current Local companion or Fleet agent log directly,
including when startup failed. It shows the source path, empty/missing state
and a bounded 256 KiB tail; Fleet attachment explains that an externally
started agent may write to its original installation instead. The native
command rejects non-regular files, and Rust/UI regressions cover file bounds,
mode path selection, offline access and refresh.
An isolated unsigned macOS app built from Tauri `a2698ce` and a
test-port Go `554374b2` companion opened Services before profile setup. The
native panel resolved the private `companion.log` path and showed its empty
state. After a sentinel was appended only to that test log, “Actualiser le
journal” displayed the new line. The app and companion were stopped and the
test port was released; this check does not qualify a Fleet log from a native
bundle.
The first implementation hid diagnostics while the runtime mode was still
unconfigured, exactly when an initial Local or Fleet startup might fail. The
native command now accepts only the two fixed service kinds and Settings shows
both logs before mode selection or after setup status fails. A UI regression
checks that a failed first read opens Services and loads both diagnostics; the
earlier native test does not cover this follow-up source. A second unsigned
macOS bundle with a unique app identifier opened before mode selection. Its
Services page displayed separate `PRESTART_COMPANION_MARKER` and
`PRESTART_FLEET_MARKER` lines from the two isolated log paths, without starting
either Go service. The app, test logs and bundle were removed afterward. This
qualifies first-start visibility in a native window, not a real Fleet agent
failure or a signed release build.

An isolated Fleet failure retake used a unique unsigned macOS bundle, Go
`554374b2`, private synthetic pairing credentials and a closed loopback backend
at `127.0.0.1:65534`. Before the fix, Settings remained on its loading state
for over two minutes, offered first-start controls despite a selected Fleet
connection, and repeatedly restarted the agent while it retried the backend.
The revised app promptly displayed Fleet as selected, identified the agent as
disconnected, showed the backend error and the real `agent.log` connection
failure, and offered no Local activation or Fleet pairing form. A manual
**Actualiser** completed while the same agent PID remained alive and the
startup-log count stayed unchanged. The test app, agent and private data were
removed; the regular companion on port 52963 was untouched. This qualifies
native failure presentation and stable retry ownership for a closed loopback
backend. It does not prove recovery after a real Fleet outage or provider OAuth.

The follow-up source review found that a missing Fleet agent gateway still
caused `local_setup_status` to fail before returning the selected mode. Native
Settings now retains Fleet identity and diagnostics for gateway, startup and
backend failures. A UI recovery regression moves from an unavailable Fleet
gateway to a ready profile with one manual refresh and confirms the profile
fields are populated. The native gateway restart is covered below; live-backend
recovery still needs its own retake.

A unique unsigned app built from Tauri `9fbe40d` and Go `554374b2` then
reproduced the local gateway stop with synthetic Fleet credentials and a closed
loopback backend. After the isolated agent was terminated, the desktop's agent
request invalidated the stale connection and started one replacement agent.
Settings continued to identify Fleet, displayed the disconnected backend
state and its agent log, and a manual refresh completed without switching to
Local setup. The app and replacement agent were stopped and their private data
and bundle removed. This qualifies native gateway restart and retained mode;
the closed backend prevented a live profile restoration check.

An isolated current-source Go backend on `127.0.0.1:55460` supplied a synthetic
paired account and folder-only project to a separate unsigned Tauri bundle at
`2f2514c`. Native Settings initially showed the profile and project. After the
backend stopped, **Actualiser** showed Fleet disconnected and the backend error
while preserving the selected mode. The same Go database was reopened; the
agent reconnected without restarting, and a second native refresh restored
the profile fields and project list. The agent log had one startup entry across
the outage. This closes the native Fleet backend restart/recovery slice for
current source and synthetic data; real OAuth and tracker services remain
separate qualifications.

The Fleet provider native retake exposed a project-folder inconsistency:
the Fleet board offered **Choisir un dossier**, but the Rust picker and retry
command rejected any connection except Local before opening the native dialog.
The global **Ouvrir un dossier** path also created projects only through the
Local installation. These paths now use the selected Local or Fleet connection;
a paired Fleet desktop creates a folder-only project on its backend and maps
the chosen folder through its workstation agent. An isolated app against Go
`554374b2` on port 55461 changed an existing Fleet project's folder through
the native macOS picker, launched free prompts in the new folder, and created
a second Fleet-backed folder-only project from the global picker. Go API reads
confirmed the second backend project and its local agent mapping. Real signed-in
Claude Code and Codex CLIs each ran `pwd`; both native conversations showed the
tool call and the expected path in their answers. The backend, app, agent and
workspaces were disposable. This qualifies current-source Fleet folder and
provider response/tool projection on macOS, not external tracker workflows or
provider interruption in Fleet mode.

A separate unsigned native Fleet bundle from Tauri `f5ab0a9` and Go `554374b2`
tested **Interrompre** and same-conversation resume for signed-in Claude Code
and Codex. Both shell tools began a 90-second delayed write in a disposable
Git folder; the desktop returned each conversation to Ready, and neither
delayed file appeared after its deadline. A second message referred to the
file only by its previous-turn context; each engine checked the correct file
and returned the expected resume marker. The conversation history grew while
the original prompt and marker were not replayed, and model/mode remained
unchanged. This qualifies Fleet native execution controls for those CLI
versions on this host; signed distribution and tracker-backed execution remain
separate gates.

The isolated native Local GitLab setup test found that opening tracker settings
and clicking **Synchroniser** immediately could queue a second read while the
automatic read was already running. Settings now discovers queued and running
tracker sync activities, follows the existing activity, and disables its button
until that activity finishes. The click handler checks again just before
requesting a manual read, covering an activity that started after the last
settings poll. This is a desktop interaction fix; it does not serialize sync
requests from other clients at the backend.

The approved Local/Fleet contract starts fresh Local tracker access with a
personal provider token and keeps the previously configured OAuth application
flow for compatibility. Native Settings had inverted that priority by placing
OAuth first and hiding the token form as an advanced option. Local now shows
personal access directly and retains OAuth in a secondary disclosure; Fleet
continues to show backend-managed OAuth first and keeps token entry advanced.

Project setup also had a broken continuation: the folder-first card promised
that a tracker could be connected later, but the adjacent tracker form only
created another project. The native app now adds a registered tracker to the
existing project from either Settings or that project's configuration. The
current source UI test covers the direct project path; an isolated Go API test
covers folder retention and tracker selection across a server restart. A
packaged desktop interaction and live tracker import remain separate checks.

The first native Fleet retake of this project action at Tauri `5e44fde` and
Go `45940914` paired an isolated app with a disposable backend, attached one
GitLab fixture tracker to an existing folder-only project, and showed the
association in project settings. A backend read confirmed the original project
ID, one selected tracker, the new default and no duplicate project. The
screen capture exposed an unstyled native `fieldset` border and an empty
selector after the last available source was attached. The form now uses the
project settings layout and shows a completed state instead of empty controls.
A rebuilt isolated app at `2a1e59e` reopened the same Fleet project and
visually confirmed that completed state without the redundant form. The
isolated app, agent, backend and private data were removed after the check.

A separate native Local retake at Tauri `d0ecceb` opened a temporary folder as
a project, registered a fixture tracker in Settings, and attached it from that
project's configuration. The project, folder and tracker persisted across a
companion stop and desktop reopen, and a Codex free prompt returned its expected
answer with the same working directory. The test also exposed English agent
notice text in the French conversation surface. Go `5500840d` changes
Sectile-authored conversation notices to French while leaving provider-authored
responses untouched.

An isolated unsigned macOS app built from Tauri `317151c` and Go `5500840d`
(test-only companion port `55466`) then opened the same temporary folder as a
Local project. Real Codex and Claude Code conversations returned
`NOTICE_RETAKE_OK` and `CLAUDE_NOTICE_OK`; their initial notices read
“Conversation Codex” and “Conversation Claude Code,” and each completed turn
read “Prêt pour votre prochain message.” The provider replies were unchanged.
The app, companion, provider children and temporary data were removed after
the check. This qualifies the localized start/ready copy on the native Local
path; interrupt/error copy was covered by Go tests rather than this retake.

That native first-start path also exposed stale provider detection: the hidden
AI settings panel queried the agent before a mode was selected, then retained
those errors after Local profile setup. Settings now mounts provider detection
only once the agent is available and scopes it to the selected connection.
An onboarding UI regression verifies that detection appears after agent
readiness. TypeScript, 15 domain tests, 131 UI tests, the production build and
38 Rust tests pass; this follow-up has not had a separate native retake.

A workflow navigation review found that switching directly between tickets
reused the previous component state. Its title, draft instructions, selected
skill and errors could remain while the next ticket loaded. Workflow state is
now scoped to the project/ticket pair. A delayed-read UI regression verifies
that the old ticket and launch controls disappear during loading and that the
new ticket starts with empty instructions. TypeScript, 15 domain tests, 132 UI
tests, the production build and 38 Rust tests pass. This source fix still needs
a native navigation retake.

An unsigned native app from Tauri `d926f26` and Go `5500840d`, with only the
companion port changed to `55466`, verified first-start provider setup: the
unconfigured screen offered connection setup, and saving the Local profile
automatically detected both signed-in CLIs without stale mode-selection errors.
Two synthetic tracker tickets were seeded only in this disposable installation.
Opening the first workflow, entering instructions, returning through the board
and opening the second showed an empty draft. This covers native board-mediated
navigation; the direct prop-change/loading race remains covered by the UI test,
since the quick navigator does not expose ticket destinations.

The same native check found that chain dispatch was enabled on an unconfigured
project, although next-step dispatch was disabled. The shared launch guard now
requires project configuration for chain and custom skills too. A regression
test selects a custom skill on an unconfigured project and verifies that neither
action can dispatch. TypeScript, 15 domain tests, 133 UI tests, the production
build and 38 Rust tests pass. The final guard change was tested in source; the
observed native bundle predates it. No remote tracker write was made.

## Native tracker workflow qualification: connection identity blocker

On 2026-10-30, an isolated unsigned app from Tauri `0711eb8` and Go `5500840d`
(test-only port `55466`) created a GitHub-backed project and temporary Sectile
issue `#899` through its backend API. The native board displayed the issue,
and **Next step: Clarify** dispatched a real Codex conversation in a disposable
Git worktree. The transcript showed connected MCP transport and an approval
request for `sectile_companion.get_project_context`, but **Allow once** was
disabled. Native **Stop execution** stopped the run and showed a stopped skill.
This is a failed workflow qualification, not an end-to-end pass.

Source inspection identifies the missing link: free conversations receive
`X-Sectile-Connection-ID` at creation, but a tracker skill returns through the
server dispatch path without that desktop identity. The conversation endpoint
therefore treats it as an unbound read-only history; tracked runs cannot use
the free-history adoption path. Carry the initiating connection identity
through local launch/dispatch correlation, while preserving rejection of other
connections and legacy Electron behavior. Do not enable approval buttons by
dropping the connection check. Verify approvals and tool activity with the
real native fixture after fixing this path.

Go `fdffed9a` added race-safe correlation in either arrival order. Its first
native retake exposed a response-contract mistake: `activity.id` is the launch
activity, not the dispatched execution. Go `920e313f` adds an explicit `runId`
to the server response and uses only that field for correlation. A handler test
compares it with the real websocket dispatch and proves that the launch activity
has a different ID. Agent tests cover both arrival orders, mismatched project,
task or run, expiry, another desktop connection and legacy Electron access;
the targeted tests also pass with the race detector. The full agent suite passed
before the response-field correction; targeted agent and handler tests pass
after it. An older Fleet server without this response field does not qualify
for interactive Tauri tracker workflows.

The next native run on the same issue had a connection-bound conversation and
enabled **Allow once**. The test approved `get_project_context`, `get_task` and
`start_run` from the desktop, and each completed in the grouped tool transcript.
That initial slice did not yet prove the skill's final result and stage transition.

The native retake completed one real clarification workflow: the test approved
the ticket comment, waiting report and finish report, then sent the predefined
fixture confirmation from the conversation. The provider attempted to reuse
the already completed run and received a lifecycle refusal, but subsequently
completed `transition_stage`. Go returned `clarified`; a direct GitHub read
confirmed `#clarified`; the desktop showed **Result confirmed** and **Specify**
as the next step. The final `finish_run` was idempotent and retained its earlier
round summary. No repository files changed, and no push or PR occurred.
This qualifies native launch, tool consent, response, conversation follow-up
and one remote stage write. It does not qualify the full specification,
implementation and review chain. Follow-up lifecycle guidance should avoid
restarting a completed run after an owner-confirmation round.
The fixture issue was deleted and GitHub returned HTTP 410. The isolated app,
companion, worktrees and private data were removed after qualification.

Go `84e19b1b` corrects that later-round guidance in both the launch prompt and
the MCP `start_run` description: reuse the launcher's ID only while queued or
running; a later invocation after completion omits `runId` and reports with the
newly returned ID, even if the inherited `SECTILE_RUN_ID` still names the first
run. The terminal-run refusal remains in force. A database regression proves
that the suggested new invocation gets a distinct running ID while its earlier
canceled run stays canceled. Targeted agent command/conversation, database
lifecycle and MCP start/finish tests pass. This is a source guidance correction;
provider compliance in a new native continuation remains to be retaken.

## Standard native full-chain qualification (2026-10-30)

An unsigned Tauri `51e93f2` build with Go `84e19b1b` services (test-only port
`55466`) launched the standard Claude Code/Sonnet pickup chain from the native
ticket workflow. A fresh loopback GitLab test double supplied the tracker API,
a real Git HTTP origin and MR records; no real forge was used. Standard skills
were installed normally, without a project skill-editor override. The initial
run committed clarification, OpenSpec documents and the Python implementation,
then reported failure at PR creation. Retries preserved that accepted work.

Fixture repairs were required: glab configuration belongs to the linked
worktree as well as the parent; the local Claude hook requires an `ai:tech:*`
MR label; `glab auth status` needs a private host configuration; and the project
response must include `merge_requests_enabled: true` for the CLI to attempt a
create. The CLI also panicked in telemetry against incomplete synthetic data,
so telemetry was disabled only in its disposable config. The final companion
used private `GLAB_CONFIG_DIR`, an explicit dummy `GITLAB_TOKEN`, and the normal
Local runtime/MCP environment. Global provider/MCP configuration was not changed.
The CLI capability check is visible in its
[official create command source](https://gitlab.com/gitlab-org/cli/-/blob/v1.82.0/internal/commands/mr/create/mr_create.go).

Run `1041614c-9084-401a-be95-0571db8974d1` finished as completed on the server,
and both backend and tracker showed `#reviewed`. Independent checks confirmed
one passing Python unittest, OpenSpec/clarification artifacts, a clean worktree,
the preserved pre-retry commit ancestry, and feature HEAD
`edaa6e2cd8569314f4841b575e9768503ff3e16a` matching the pushed `feat/1` branch.
Exactly one MR was ready and still open; the issue was also still open. All
stage reports came through MCP, not database edits or fixture-side shortcuts.
Native Tauri showed **Result confirmed**, **Finalize** as the next step, the
PR action, and a seven-file review with 69 added lines, including committed
code, tests and specification documents.

This qualifies the standard native chain with recovery through a controlled
local forge. It is not an uninterrupted first-attempt pass, a real GitLab
service integration, a Codex full-chain run or signed-release qualification.
The chain action remained available after its configured Reviewed target had
been reached. The follow-up source now replaces that action with a reached-target
status, while preserving the next manual step. Its dispatch guard rechecks the
fresh task and policy, so a ticket advanced by another client does not trigger
an unnecessary chain. Domain tests cover both stop stages, mapping precedence
and unknown stages; UI tests cover the reached state and advancement between
rendering and clicking. This source change has not had a separate native retake.
The isolated app, companion, forge, Git repositories, worktrees and private
configuration were removed after the check; the regular companion on 52963
remained running.


### Native Fleet MCP isolation correction (2026-10-30)

Preparing a native Codex chain exposed a gap outside the previously qualified
Fleet setup slice: managed Fleet launch did not supply a process-scoped MCP
endpoint. Provider sessions could read the shared Electron registration, while
explicit skill/MCP preparation could bootstrap it. Tauri now passes its selected
origin through `SECTILE_DESKTOP_MCP_URL` and disables shared refresh. Go
`9b0b68f6` accepts HTTPS origins or HTTP loopback for this explicit opt-in,
preserves the older Local-only endpoint contract, suppresses shared refresh and
uses the daemon's paired credential privately. External attached agents retain
their own configuration.

`npm run test:fleet-mcp` passed for real Codex (`gpt-6-astra`) and Claude Code
(`sonnet`) against packaged server `84e19b1b` and a development build of the Go
fix on isolated port 64223. Each called `get_project_context` through the scoped
server, received the actual fixture project and used it in its response. Shared
Claude and Codex MCP tables remained unchanged. The first harness attempt
compared the entire Codex preferences file and failed after the tool/response
checks; the corrected harness parses all MCP tables independently of unrelated
preferences, using Python TOML parsing. No MCP fields are excluded. Fixtures
were removed; the regular companion on 52963 remained running.

Argument/environment, no-shared-write and endpoint-validation regressions pass;
Tauri TypeScript, 16 domain tests, 135 UI tests, production build and 39 Rust
tests pass. This is real provider/API evidence and native-launch source evidence,
not a native Fleet full-chain completion. The retained `4063b47`/`84e19b1b`
candidate predates this fix; a rebuilt native package must be tested before
claiming it includes the correction. Existing managed agents need a restart to
acquire the new configuration; external agents are never silently restarted.


### Native managed Fleet retake and approval wording (2026-10-30)

An isolated unsigned Tauri `8d8dd70` app with clean packaged Go `9b0b68f6`
services used a unique app identity and a disposable Fleet backend on port
49663. Native pairing started the managed workstation agent automatically.
The native folder picker created a folder-only project and opened its prompt.
Real Codex (`gpt-6-astra`, execution/workspace-write) requested project-context
MCP consent, accepted a one-time native approval and returned
`NATIVE_FLEET_MCP_OK native-project`. Its completed tool and connected MCP state
were visible. Closing the actual test app, reopening it and sending a follow-up
returned `REOPEN_OK native-project`; API checks confirmed the model, mode and
single original tool call. A separate native Claude Code prompt (`sonnet`,
execution/default permission mode) used ToolSearch and the scoped context tool,
then returned `NATIVE_FLEET_CLAUDE_OK native-project` with completed tools and
connected MCP state. No real tracker was modified. The regular Local companion
remained running on 52963.

The Codex approval history exposed a UI inconsistency: a completed decision
rendered as “Activity allow” instead of a readable French outcome. The transcript
now labels these entries as authorizations and translates the known decisions,
while preserving user/provider prose and answer details. A rendering regression
covers approval, refusal, remembered consent and answered questions. TypeScript,
16 domain tests, 136 UI tests, production build and 39 Rust tests pass. Native
confirmation of this wording and Claude bundle-replacement resume follows
separately; this retake does not establish tracker-chain execution or a signed
installation.


The follow-up native retake passed on the rebuilt `d25915f` test bundle: the
restored Codex transcript displayed the French authorization heading and
“Demande autorisée une fois.” Claude's native follow-up returned
`CLAUDE_REOPEN_OK native-project`; API comparison preserved the exact previous
history prefix, `sonnet`, default permission mode, folder and context, with no
additional tools. The Go suite also passed completely. The Mac locked only
after these UI observations. The test app, agent, backend and disposable state
were removed using their exact owned identities; the regular companion on
52963 remained running.
See the release checklist for the exact replacement boundary and limitations.


## Autonomous Codex chain and crash-safety correction (2026-10-30)

A controlled service-level check used real Codex CLI 0.165.0 / `gpt-6-astra`,
installed standard skills, the scoped Fleet MCP bridge, a loopback GitLab test
double and a real Git HTTP origin. No project skill override or real forge was
used. Initial setup required the Fleet server's synthetic read credential and
private glab configuration; personal write credentials alone do not authorize
Fleet server-wide synchronization.

The first run exposed a product defect: plain `codex exec` inherited read-only
permissions and could not answer MCP approvals, then exited zero at New. Go
`88154064` changes the built-in autonomous launch to `--approve-for-me --json`,
retaining Codex's workspace sandbox and automatic review, and normalizes Codex
JSONL into the existing local execution trace. Configured templates and
interactive conversations keep their own settings. Go `72119df5` additionally
preserves top-level Codex error messages before decoding Claude's different
message shape. See Go ADR 0069 and the official
[non-interactive documentation](https://learn.chatgpt.com/docs/non-interactive-mode).

The corrected implementation run `5e218b39-271f-40fd-a7ad-1e34a382b28d`
accepted clarification, specification and implementation. Review correctly
stopped on malformed fixture feedback responses. A continuation preserved those
commits and completed the review checklist, then refused to claim Reviewed when
the fixture ignored MR-description updates. After repairing only those fixture
HTTP contracts, run `fc4dcc3a-ea06-4476-93ad-ede44f171baf` completed Reviewed
through MCP. No stage was advanced through a direct database or fixture edit.

Independent checks confirmed three passing Python unittests, an additional
`greet("Ada") == "Hello, Ada!"` assertion, one ready open MR, an open issue,
`#reviewed` in both backend and tracker, a clean worktree and matching local,
pushed and MR HEAD `ab7668688248f7d60f51ad7e9563825ea86735de`. The implementation
commit remains an ancestor; the only extra commit completes the review checklist.
The default branch was unchanged. A real WebSocket subscriber received 33 live
trace frames, including 29 tool-progress lines and four stage-report lines.
Shared Claude/Codex MCP tables remained unchanged. This is a standard Codex
runtime chain with fixture-repair retries, not an uninterrupted first pass or a
native-button launch qualification. The owner deferred the locked-Mac visual
check; do not restart the unlock request.

A separate regression exposed autonomous process ownership: `StartDetached`
stopped explicit cancellations but allowed a delayed tool write after owner
SIGKILL. Go `85da2f24` applies the conversation lifetime guard to autonomous runs
and checkpoints their complete identity before provider startup. A fast Claude
run had also disappeared before the five-second checkpoint; a configured store
failing its initial write now refuses execution. Ordinary autonomous trace tails
remain periodic; this is not conversation-style per-event journaling.

Real custom autonomous tasks then ran a one-shot shell command with a ready file,
90-second sleep and delayed write. SIGKILL of the test agent prevented every
delayed-file variant after 95 seconds for Codex (`gpt-6-astra`, run
`91c51d3f-092f-486a-8615-49b351459357`) and Claude Code (`sonnet`, run
`5025ce23-df93-420f-acc6-0fc90bba17a7`). Restart restored each run as terminal,
with provider/model retained, unchanged ready-file timestamp, no tool replay and
the completed workflow history still present. Shared MCP tables remained intact.
These were development binaries of the committed fixes against packaged Go
`9b0b68f6` server services; the newly packaged source pairing requires its own
artifact verification. All Go packages passed, including startup-write ordering,
storage-failure refusal, owner-SIGKILL, explicit cancellation and trace routing.


## Refreshed unsigned candidate (2026-10-30)

Tauri `c4258210c209bc58be02cf7aaef69c382043931c` and clean Go
`85da2f243a9792bb76660b47e68b37a788e201d6` were packaged in an isolated detached
worktree, without replacing the regular app or companion. The arm64 app/DMG
verifier passed for embedded service hashes, architecture, fixed port 52963 and
52 license files; `hdiutil verify` passed. The DMG is 56,356,912 bytes with SHA-256
`b9d3bd3e1bef85e84adcecd357da94cbc1e54ac411ea8f9c3e12039cd629d0d0`.
The retained artifact directory is `src-tauri/target/candidates/2026-10-30-c425821/`
and contains the app, DMG, manifest, checksums and scope notes. It has no Developer
ID signature or notarization and is not release-eligible. No release/tag/version
bump was performed.

The exact packaged Go `85da2f24` companion also recovered the six source-prepared
durable migration boundaries, preserving installation identity, API credential,
encryption key, personal token, encrypted OAuth app/grants, projects, ticket,
snapshot/journal bytes, backup and the legacy DB fence. This remains packaged
checkpoint recovery, not a native installer, real grant refresh or arbitrary
process-kill timing inside migration.

The current/v0.7.6 compatibility harness passed again against source `85da2f24`:
27 operations, 21 MCP tools, both direct/STDIO directions, run lifecycle,
task-spec worktrees and fake tracker writes with personal credentials. It does
not qualify older servers for the connection-bound native tracker conversation
path requiring explicit `runId`, nor general Fleet OAuth/setup support.

Native inspection and install/service-replacement checks were explicitly
postponed by the owner. An isolated app with a distinct identity and the same Go
service hashes was prepared without launching it. Test backend/forge processes
were stopped; private fixture state is retained for that deferred check. The
temporary package worktree and stopped failed setup fixtures were removed. The
regular companion on port 52963 (PID 89986) remained untouched. Opening the new
UI while an older companion is running must not be counted as a service upgrade.


## Manual update lifecycle inconsistency (2026-10-30)

Source inspection found that quitting the desktop intentionally retains the
Local companion, while manual update instructions assumed replacing the app
also replaces its running Go service. The protocol-only startup probe can
reuse that older process. A new advanced Services action prepares a configured
Local installation: finish work, close other instances, stop the idle companion
and quit Tauri after transport closure and store ownership release. It does not
present a fake download/updater or stop Fleet. A failed preparation leaves a
startup fence; explicit retry and service restart remain available even after
Settings remounts. No status refresh follows an acknowledged stop.

The actual companion integration test verifies clean stop and retained project
and installation identity after SQLite reopen. UI tests cover double clicks,
post-acknowledgement refresh suppression, refusal recovery, remount recovery and
Fleet exclusion. Native rendering/exit and bundle replacement remain deferred;
the unpaired/bootstrap stop path and multi-instance update guarantee remain
release gates rather than inferred successes.


## Native update journey retake (2026-10-30)

The owner resumed native tests. The real Services action now passes stop and
quit with idle Claude/Codex conversations, bundle replacement, restored native
history/model/mode and an explicit follow-up recalling each provider's original
codeword without tools. A final native shutdown after those follow-ups passes
as well. The test found two lifecycle inconsistencies that source-only decoder
and empty-runtime tests had missed: open idle conversation exit channels and
Codex's persistent RPC stdin. Go `17a782df`/`f55e8768` fix the admission boundary
without treating active turns or queued input as idle.

The final qualification uses Tauri `d69c3ac`/Go `f55e8768`, an isolated app
identifier, port 52964 and test versions 0.1.3→0.1.4. Transcript prefixes,
provider session hashes, model/mode and marker files were independently
compared. Captures and redacted results are retained under
`src-tauri/target/qualifications/2026-10-30-native-update/`. The regular user
installation, Jira and Developer ID signing were left untouched. This closes
the requested native update/conversation slice, not every V1 release gate.

## Native tracker management retake (2026-10-30)

Tauri `1982df5` with Go `8d40b2aa` was packaged as a separate unsigned macOS
application using the existing QA identity and port 52964. The Go port and OAuth
origin were changed only in a disposable build worktree; the production Go
worktree and the regular port 52963 were unchanged. In the native Settings
window, an isolated GitHub tracker was created, renamed through **Modifier**,
and removed after the explicit **Supprimer** confirmation. The accessibility
tree showed “Tracker ajouté.”, “Tracker modifié.” and “Tracker supprimé.”, with
the renamed entry appearing and then disappearing. The retained linked GitLab
fixture was not modified. The test companion was stopped afterward while the
regular companion remained on port 52963.

This qualifies native tracker creation, rename and deletion in Local mode. Fleet
administrator behavior and the new bundle's MCP project-context fix still need
their own runtime checks; the UI and Rust tests alone do not establish them.

## Engine settings and scoped MCP retake (2026-10-30)

Tauri `be93c3c` was built as a separate unsigned macOS app with Go `8d40b2aa`.
Only the disposable build copies used port 52964; the regular companion stayed
on port 52963. Claude Code and Codex both appeared as connected with the same
brand marks used by Electron. Both cards fit in the 800 × 820 browser fixture,
and the native app showed both connected states. Captures are retained at
[`engine-settings-800.png`](ux-captures/engine-settings-800.png) and
[`engine-settings-native.png`](ux-captures/engine-settings-native.png).

Fresh native Codex and Claude Code conversations each called the companion's
`get_project_context` MCP tool with an empty argument object. Codex required
one-time consent and then returned the selected “Native Codex workflow”
project. Claude used ToolSearch followed by the same empty-argument MCP call
and returned that project. Neither trace contained a shell or tracker action.
This closes the previously open **Local macOS bundle** check for the Go
`8d40b2aa` project-context fix. It does not qualify remote Fleet or a signed
release bundle. The isolated app and its companion were stopped afterward.

## Owner-directed conversation and backlog retake (2026-10-30)

The owner supplied Electron captures of the composer, an airy conversation,
ticket list and board. Tauri now puts folder attachment, a native terminal
launcher and MCP verification directly in the composer. The folder beneath the
project title has a copy action. The attachment remains project-wide and becomes
available to an existing conversation at its next message; the control does
not imply a per-conversation private folder. Question forms scroll separately
so the composer remains visible at a 720 px high fixture viewport. The
transcript uses open spacing while tool calls remain grouped in disclosures.

Tracker projects now switch between board and backlog list. Selection persists
across those views and can prepare a confirmed Go `pickup_issues` batch. Cards
show priority and a stable color derived from the parent epic key, and their
native menu launches a configured skill after fresh task, project and execution
checks. The color is a visual grouping cue, not an imported Jira epic color:
the current desktop task contract carries the parent key but no epic color.
The desktop now uses Sectile's 12-color FNV palette and respects the project's
`epicColors` setting, so the same parent key has the same color in Electron,
Tauri and the web board. The card's left border keeps its width after the
generic card border rule; a disabled project setting leaves parent keys visible
without implying that colors are enabled.
Local-only projects still keep their prompt-ready empty state. The terminal
button opens the native terminal at the current working directory; it does not
send text to the agent's read-only execution console.

Fixture screenshots at 1280 × 720 show the [board](ux-captures/board-selection-1280.png),
[list](ux-captures/backlog-list-1280.png) and [pinned conversation
composer](ux-captures/conversation-composer-1280.png). UI tests cover view switching, selection, batch confirmation and
skill dispatch; Rust validates the batch request before it reaches Go.

An isolated unsigned macOS app built from Tauri `6a66e3a` (test worktree
`61550f5`) and Go `8d40b2aa` retained its existing conversation history and
368 px sidebar width after replacing the previous QA app. Its [native
conversation capture](ux-captures/conversation-native-workbench.png) shows the
working-directory copy feedback, airy transcript and composer actions inside
the window. The native Terminal action was invoked on an existing folder with
no error shown; the automation surface could not inspect Terminal's contents,
so its initial working directory is not yet qualified. The native MCP button
was accessible, but a manual click did not change its displayed unknown status
on this restored Codex history; the prior real tool-call qualification remains
the MCP evidence. The `Cmd+,` shortcut also did not navigate in automated
native input, while `Cmd+B` did; keyboard dispatch for this punctuation key
needed a native menu path. A later isolated unsigned macOS app with identity
`com.sebferry.sectile.settingsqa20261030` showed a native “Paramètres…” menu
item, and activating it left the first-run Settings screen available. A UI test
now verifies that this native menu event navigates from a workspace to Settings.
The fresh native profile opened in Settings already, so native board-to-Settings
navigation remains unqualified. Native tracker-backed list/board data was not
available in this isolated Local profile, so their visual/interaction evidence
comes from the browser fixture and automated tests. Remote Fleet, Jira OAuth
consent and signed distribution remain separate open gates.

The later `b0eb4bd` board color retake passes the complete TypeScript, UI,
Rust and production frontend checks. A separate unsigned macOS fixture bundle
with identity `com.sebferry.sectile.boardqa20261030` was built to inspect the
colored cards in a native WebView. Its window remained blank in two fixture
bootstrap variants, so it supplies no native board proof. This test-only bundle
did not start or change the regular port-52963 companion. A native board check
with tracker-backed data remains open; browser fixture captures and UI tests
must not be presented as a substitute for it.

The subsequent native retake paired Tauri `f9e4551` with the existing isolated
Go `8d40b2aa` companion at port 52964 under the QA app identity
`com.sebferry.sectile.nativeupdate20261030`. From a restored Codex conversation,
the native application menu's “Paramètres…” item opened Settings, and the
workspace return control restored that conversation. The Codex MCP check
explained that its disconnected session would verify MCP after the next message;
this is visible in the [native conversation capture](ux-captures/conversation-native-mcp-feedback.png).
The same project exposed one tracker-backed ticket on the native board. Its
checkbox changed the selection count, and the card menu listed the configured
workflow skills without launching one. The [native board capture](ux-captures/board-native-selection.png)
shows the selected card. That ticket had no parent epic, so the new epic-color
mapping still lacks a native colored-card capture. Automated `Cmd+,` key input
did not navigate even though selecting the native menu item did; the physical
key combination remains a manual check. This QA bundle reused its isolated
port-52964 service binaries and did not touch port 52963.

The `73deeba` board retake rebuilt that isolated native QA app with its existing
Go `8d40b2aa` companion binaries. A tracker-backed project opened with six
workflow-stage columns, their distinct tints, and its reviewed ticket in the
correct column. The [native stage-board capture](ux-captures/board-native-workflow-stages.png)
records the default layout. The native grouping selector switched to the
tracker's `opened` status column and back to workflow stages; the UI test
also covers persistence of the chosen grouping across React remounts. The
ticket has no parent epic, so a native epic-color proof remains open. This
unsigned QA app is not a distributed release or remote Fleet qualification.

The next native board retake removes the permanently disabled batch controls
from the idle toolbar. At the isolated QA window's 1280-point width, view
switching, search, grouping, density and ticket count fit on one row in the
[idle capture](ux-captures/board-native-compact-toolbar.png). Selecting the
tracker-backed ticket reveals selection count, clear and batch controls in a
separate row, as shown in the [selected capture](ux-captures/board-native-selection-toolbar.png).
Clearing selection hides those controls again. The one-ticket fixture leaves
batch preparation disabled, as intended; a multi-ticket native batch remains
separately qualified by earlier tests rather than this capture. The QA app
used port 52964 and did not change the regular port-52963 companion.

The project-header retake makes the free prompt the sole filled action on the
board. Configuration remains one click away through a labeled settings icon,
while refresh stays an icon. The [native header capture](ux-captures/board-native-compact-header.png)
shows the smaller header and longer visible folder path. Clicking the settings
icon opened the same project's folder, tracker and conversation settings; the
Tableau action returned to its ticket board. No project data was changed.

The later conversation retake removes board, new-prompt and project-settings
buttons from an open conversation. Its header keeps the title and response
state, with rename, run, stop and archive actions in a compact discussion menu.
The project name in the sidebar still opens the board and its project actions.
The [native conversation capture](ux-captures/conversation-context-menu-native.png)
shows the quieter header and the resize handle centered on its separator;
the [open-menu capture](ux-captures/conversation-actions-native.png) shows the
discussion actions. The isolated port-52964 companion retained its existing
Claude/Codex history. The native screenshot verifies handle alignment, while
automated pointer drag was interrupted by a foreground app change. Keyboard
resize retains its automated coverage; native pointer-drag retake remains open.

Ticket-linked conversations now show a PR/MR shortcut only when the tracker
provides a valid web link. UI tests cover the sidebar shortcut, status strip
and URL passed to the native link opener. The isolated native project has a
ticket with a linked MR, but its selected conversation is a free prompt; this
retake does not claim a native capture of the new ticket-conversation shortcut.
Remote Fleet and the signed release remain separate qualifications.
