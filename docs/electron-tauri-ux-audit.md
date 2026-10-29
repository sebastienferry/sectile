# Electron and Tauri desktop UX audit

Date: 2026-10-29
Status: all seven scoped UX lots are implemented and revalidated against the
current source on 2026-10-29. Matched captures cover the shared board,
pre-dispatch prompt and settings surfaces; post-dispatch conversation and
execution surfaces remain product-specific rather than visual-parity targets.
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
and port 52963 released. This confirms the Go credential path and activity
outcome; the current-source native Settings screen has not been visually
rechecked with this result.

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
