# #360 — Implementation Checklist

Ordered implementation checklist for translating `README.md` to English and pruning obsolete references. References: [`spec.md`](./spec.md), [`plan.md`](./plan.md), [`docs/clarifications/360.md`](../../docs/clarifications/360.md).

---

## 1. Preparation & Baseline

- [ ] T1.1 Verify git status on branch `feat/360` and inspect lines 8, 12–85, 231, 246, 256, 447–468, and 648–651 in root `README.md`.
- [ ] T1.2 Verify canonical English terminology against `web/src/locales/translations.ts` (`en`) and `docs/README.md`.

---

## 2. Header & Implemented Features Translation (FR1, FR2, FR3, FR4, FR5, US1)

- [ ] T2.1 **Introductory Tagline (Line 8)**:
      Replace the French tagline with:
      `Modern, agentic task workflow manager for developers and engineering teams, built with **Go**, **React 19**, **Tailwind CSS v4**, and **SQLite**.`
- [ ] T2.2 **Features Section Heading (Line 12)**:
      Replace `## ✨ Fonctionnalités implémentées` with `## ✨ Implemented Features`.
- [ ] T2.3 **Spec-Driven Design Frameworks Subsection (Lines 21–28)**:
      Translate heading and bullets to English:
      - Toolchain installation from UI (project's *AI & SDD Skills* tab, or command palette <kbd>Cmd+K</kbd>).
      - GitHub Spec Kit: installs `specify` CLI via `uv` / `uvx` from `git+https://github.com/github/spec-kit.git`, executes `specify init --here`. Scaffolds `.specify/` and `specs/` (spec.md, plan.md, tasks.md) and agent `/speckit.*` commands.
      - OpenSpec: installs `openspec` CLI via `npm` / `npx` from `@fission-ai/openspec`, executes `openspec init`. Scaffolds `openspec/` (proposals, spec deltas `ADDED` / `MODIFIED` / `REMOVED`, checklists).
      - Endpoints `GET /api/spec-framework/status` and `POST /api/spec-framework/install`.
      - Activity tracking in *Activities* view.
      - Scaffolding of `/specify-issue` skill and agent prompt guidance.
- [ ] T2.4 **Agent Copilot & Configurable AI Engine Subsection (Lines 30–58)**:
      Translate heading and bullets to English:
      - AI Engine choices (`claude`, `agy`, `vibe`, `codex`) and execution modes (interactive vs autonomous). Attested command templates and autonomous mode refusal rules.
      - Per-launch model selection via profile *AI Engine* settings, task card `...` menu, and card badges.
      - Custom command templates and `{mode:AUTONOMOUS|INTERACTIVE}` placeholder behavior.
      - Prompt customization for skills:
        1. 🔍 **Clarify** (`/clarify-issue`): Ambiguity analysis and framing questions.
        2. 📝 **Specify** (`/specify-issue`): Specification drafting (Spec Kit or OpenSpec) and Git branch initialization.
        3. 💻 **Implement** (`/code-issue`): Code planning, file edits, unit tests.
        4. **Adjust** (`/adjust-issue`): Full branch review, PR feedback resolution, checks, PR update.
        5. ⚡ **Auto-Pilot** (`/pickup-issue`): Intelligent router sequencing next optimal workflow stage (correcting `/pick-issue` to `/pickup-issue`).
      - CLI status panel: real-time checks for `git`, `gh`, `agy`, `claude`, `codex`, `uv`, `specify`, and `openspec`.
- [ ] T2.5 **Sidebar & Workflow Stages Subsection (Lines 60–65)**:
      Translate heading and bullets to English:
      - Workflow stages: `Backlog` ➔ `To Clarify` ➔ `Specified` ➔ `In Progress` ➔ `To Validate` ➔ `Done` with real-time counters.
      - View toggle (`Kanban Board` / `List View`).
      - Quick filters (`My Tasks`, `High Priority`, `Labels / Tags`) and source filter (`GitHub`, `Jira`, `Local`).
      - Smooth sidebar expand / collapse.
- [ ] T2.6 **Profile & Personalized Ergonomics Subsection (Lines 67–75)**:
      Translate heading and bullets to English:
      - Dynamic accent color (*Indigo, Violet, Emerald, Amber, Rose, Cyan, Blue, Orange*).
      - Theme (Dark Mode / Light Mode).
      - Multi-language (French FR / English EN with instant switching).
      - Display density and UI scaling (*Compact*, *Standard*, *Comfortable*).
- [ ] T2.7 **Kanban Board & List View Subsection (Lines 76–79)**:
      Translate heading and bullets to English:
      - Kanban Board View: drag-and-drop between columns with automatic tracker sync.
      - List View: status grouping, multi-column sorting, inline editing.
- [ ] T2.8 **Quick Search & Action Palette Subsection (Lines 80–84)**:
      Translate heading and bullets to English:
      - Keyboard shortcut `/` to focus global search.
      - Action palette (<kbd>Cmd+K</kbd>) with fuzzy search and direct keyboard skill execution.
      - Prune obsolete web PTY terminal bar mention (line 83); clarify that skills are launched from the action palette, card action menus, or the agent-owned desktop console.

---

## 3. Documentation, Shortcuts, Credentials & Agent Updates (FR6, FR7, US2, US3)

- [ ] T3.1 **Tracker Connection UI References (Lines 231, 246, 256)**:
      - Replace `*Connecter votre tracker*` with `*Connect your tracker*`.
      - Replace `*Profil > Trackers*` with `*Profile > Tracker Credentials*`.
- [ ] T3.2 **Technical Documentation Section (Lines 447–457)**:
      - Replace `## 📚 Documentation Technique Complète` with `## 📚 Comprehensive Technical Documentation`.
      - Translate introductory sentence: `A comprehensive documentation suite for developers and LLMs is available in the [`/docs`](./docs) folder:`.
      - Align all bullet points with canonical English summaries from `docs/README.md`.
- [ ] T3.3 **Keyboard Shortcuts Table (Lines 459–468)**:
      - Replace `## ⌨️ Raccourcis Clavier` with `## ⌨️ Keyboard Shortcuts`.
      - Replace header `| Raccourci | Action |` with `| Shortcut | Action |`.
      - Translate table rows:
        - `/` ➔ `Focus global search bar`
        - `Cmd+K` or `Ctrl+K` ➔ `Open command & skill palette`
        - `N` or `C` ➔ `Open quick task creation modal`
        - `Esc` ➔ `Close modal / clear search`
        - `↑` / `↓` + `Enter` ➔ `Navigate and select in action palette`
- [ ] T3.4 **Agent Configuration Guidance (Lines 648–651)**:
      - Remove obsolete mention of `**Agents à configurer** checkboxes in the project's AI tab`.
      - Update explanation to state that agent configuration is managed per active provider and initialized via `sectile-agent init --provider <provider>`.

---

## 4. Verification & Quality Gates (NFR1, NFR2, NFR3, US4)

- [ ] T4.1 Run regex scan on `README.md` for French accented characters and common French stopwords to verify zero remaining French prose.
- [ ] T4.2 Verify all relative Markdown links and anchor links in `README.md`.
- [ ] T4.3 Run `git diff` to review all changes, ensuring no unexpected formatting or whitespace churn.
- [ ] T4.4 Run repository build and test gates (`make test`) to ensure clean execution.
