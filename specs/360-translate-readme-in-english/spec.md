# #360 — Translate README.md in English

Ticket: https://github.com/sebastienferry/sectile/issues/360  
Type: Documentation / Technical Debt  
Branch: `feat/360`  
Clarification: [`docs/clarifications/360.md`](../../docs/clarifications/360.md)  
Framework: Spec Kit SDD  

## Context

The Sectile project documentation policy (`AGENTS.md`) mandates that all project documentation (`README.md`, `CHANGELOG.md`, `/docs`, ADRs), code comments, commit messages, and PR descriptions must be authored strictly in **English**.

While approximately 85% of the repository root `README.md` is already in English (due to previous feature additions like tracker credentials, desktop companion, and MCP endpoints), several legacy sections remain in French. Additionally, a few statements describe obsolete mechanisms that have since been retired:
1. **Introductory tagline (line 8)**: Written in French.
2. **Features section (lines 12–85)**: `## ✨ Fonctionnalités implémentées` and its subsections (Spec-Driven Design frameworks, AI Copilot & configurable engines, sidebar & workflow stages, profile & personalized ergonomics, Kanban & list views, quick search & action palette) are written in French.
3. **Tracker connection parameters (lines 231, 246–247, 256–257)**: French UI strings (`*Connecter votre tracker*`, `*Profil > Trackers*`) appear embedded in English paragraphs.
4. **Technical documentation index (lines 447–457)**: `## 📚 Documentation Technique Complète` and its description bullets are written in French, despite canonical English descriptions already existing in `docs/README.md`.
5. **Keyboard shortcuts table (lines 459–468)**: `## ⌨️ Raccourcis Clavier` header and action descriptions are written in French.
6. **Obsolete operational references**:
   - Line 57 references `/pick-issue` instead of the canonical `/pickup-issue`.
   - Line 83 mentions selecting skills in the web PTY terminal bar, which was retired in favor of agent-owned desktop consoles (`desktop/README.md`).
   - Line 649 mentions `**Agents à configurer** checkboxes in the project's AI tab`, which was removed in commit `715e90b` in favor of direct active provider configuration and `sectile-agent init --provider <provider>`.

Out of scope:
- Other README files (`web/README.md`, `desktop/README.md`, `docs/README.md`), which are already authored in English.
- Runtime application strings, error logs, and UI locale bundles (`web/src/locales/translations.ts`).
- Authoring brand-new architectural documentation beyond updating the stale references in `README.md`.

This specification covers behavior, user stories, and acceptance criteria only. Implementation choices and technical details are recorded in [`plan.md`](./plan.md); the ordered checklist is in [`tasks.md`](./tasks.md).

---

## Decisions being specified

The clarification process recorded in `docs/clarifications/360.md` resolved all questions with the ticket owner in Round 2:

1. **Target strictly root `README.md`**: No other READMEs or application sources are modified.
2. **Clean translation with obsolete reference pruning**: Rather than literally translating dead features, obsolete statements (the retired web PTY bar at line 83 and the removed `Agents à configurer` checkboxes at line 649) are updated or pruned to reflect the current system architecture.
3. **Canonical English UI terminology**: References to UI controls, tabs, and workflow stages use the English strings from `web/src/locales/translations.ts` (`en`), such as `Connect your tracker`, `Profile > Tracker Credentials`, and `Backlog ➔ To Clarify ➔ Specified ➔ In Progress ➔ To Validate ➔ Done`.
4. **Harmonized documentation index**: Descriptions in the technical documentation section match the canonical descriptions in `docs/README.md`.
5. **Skill naming correction**: Fix `/pick-issue` to canonical `/pickup-issue`.

---

## User stories

### US1 — Developer reads the project overview and implemented features in English (P1)

As a software engineer evaluating or contributing to Sectile,  
I want the root `README.md` overview, introduction, and feature breakdown to be in English,  
So that I can understand the product capabilities, supported AI engines, and SDD toolchains without encountering untranslated text.

**Acceptance Criteria**

- **Given** the root `README.md` file,  
  **When** an engineer reads line 8,  
  **Then** the introductory tagline is in English and describes Sectile as an agentic task workflow manager built with Go, React 19, Tailwind CSS v4, and SQLite.
- **Given** the root `README.md` file,  
  **When** an engineer navigates to the features section (starting at line 12),  
  **Then** the heading reads `## ✨ Implemented Features` (or `## ✨ Features`) in English.
- **Given** the subsections under implemented features,  
  **When** an engineer reads each subsection (Spec-Driven Design frameworks, AI Copilot & configurable engines, sidebar & workflow stages, profile & personalized ergonomics, Kanban & list views, quick search & action palette),  
  **Then** all prose, list items, and descriptions are in grammatically correct, technical English.
- **Given** the AI skill descriptions in the features subsection,  
  **When** an engineer inspects the skill commands list,  
  **Then** the auto-pilot skill is documented as `/pickup-issue` rather than `/pick-issue`.
- **Given** the quick search and action palette subsection,  
  **When** an engineer reads the execution instructions,  
  **Then** there is no reference to a web PTY terminal bar, and skill execution is accurately described as accessible via the action palette, task cards, or agent-owned desktop console.

### US2 — Developer navigates technical documentation and keyboard shortcuts in English (P1)

As a contributor or LLM browsing the repository,  
I want the technical documentation index and keyboard shortcuts table to be written in English,  
So that I can quickly navigate to the appropriate subsystem documentation and learn keybindings.

**Acceptance Criteria**

- **Given** the root `README.md` file,  
  **When** a user reaches the technical documentation section (around line 447),  
  **Then** the heading reads `## 📚 Comprehensive Technical Documentation` in English.
- **Given** the bullet points under the technical documentation section,  
  **When** a user reviews the linked documentation files (`ARCHITECTURE.md`, `CAPABILITIES.md`, `UX_COMPONENTS.md`, `API_AND_DATA_SPEC.md`, `REIMPLEMENTATION_GUIDE.md`),  
  **Then** their titles and summaries match the English descriptions in `docs/README.md`.
- **Given** the keyboard shortcuts section (around line 459),  
  **When** a user reviews the shortcuts table,  
  **Then** the table header reads `| Shortcut | Action |` and all action descriptions (`Focus global search bar`, `Open command & skill palette`, `Open quick task creation modal`, `Close modal / clear search`, `Navigate and select in action palette`) are in English.

### US3 — Operator configures tracker credentials and agent settings using consistent English terminology (P2)

As an operator reading the tracker and agent configuration guides in `README.md`,  
I want references to UI buttons, tabs, and setup steps to match the application's English interface,  
So that I can follow the instructions without confusing French UI terms with the English UI.

**Acceptance Criteria**

- **Given** the tracker connection parameters section (around line 229),  
  **When** an operator follows the setup steps,  
  **Then** UI references read `*Connect your tracker*` (instead of `*Connecter votre tracker*`) and `*Profile > Tracker Credentials*` (instead of `*Profil > Trackers*`).
- **Given** the agent configuration guide (around line 648),  
  **When** an operator configures CLI agents,  
  **Then** there is no reference to obsolete `**Agents à configurer** checkboxes`, and the text states that agent setup is managed per active provider and initialized via `sectile-agent init --provider <provider>`.

### US4 — Repository adheres to English-only documentation policy with clean link integrity (P2)

As a repository maintainer,  
I want `README.md` to be verified against the English-only documentation rule,  
So that no French sentences remain and all existing Markdown anchors, links, and formatting remain valid.

**Acceptance Criteria**

- **Given** the modified `README.md`,  
  **When** scanned for remaining French prose or accented stopwords (`pour`, `avec`, `dans`, `une`, `des`, `sont`, etc.),  
  **Then** zero occurrences of French descriptive text are found in `README.md`.
- **Given** all relative links and anchor links in `README.md`,  
  **When** verified against the file tree and document headers,  
  **Then** all links resolve without broken paths.

---

## Functional Requirements

- **FR1 (Tagline)**: Replace line 8 with:
  `Modern, agentic task workflow manager for developers and engineering teams, built with **Go**, **React 19**, **Tailwind CSS v4**, and **SQLite**.`
- **FR2 (Features Section)**: Translate `## ✨ Fonctionnalités implémentées` and its entire subsection body into natural, idiomatic technical English.
- **FR3 (SDD Toolchain)**: Accurately translate Spec Kit and OpenSpec installation instructions, endpoints (`GET /api/spec-framework/status`, `POST /api/spec-framework/install`), and skill scaffolding details.
- **FR4 (AI Providers & Commands)**: Translate AI provider options (`claude`, `agy`, `vibe`, `codex`), model resolution rules, custom command syntax (`{mode:AUTONOMOUS|INTERACTIVE}`), and skill prompt customization. Update `/pick-issue` to `/pickup-issue`.
- **FR5 (Sidebar, Views, Ergonomics)**: Translate sidebar workflow stages (`Backlog ➔ To Clarify ➔ Specified ➔ In Progress ➔ To Validate ➔ Done`), view options, theme/accent options, and density options.
- **FR6 (Documentation & Shortcuts)**: Translate documentation index and shortcuts table matching `docs/README.md` and standard keyboard nomenclature.
- **FR7 (Obsolete Mentions)**:
  - Remove line 83 PTY bar text; replace with guidance on desktop console and action palette execution.
  - Remove line 649 `Agents à configurer` checkboxes reference; align with active provider setup and `sectile-agent init`.
  - Replace French UI labels in lines 231, 246, 256 with English counterparts.

---

## Non-Functional Requirements

- **NFR1 (Policy Compliance)**: Complete compliance with `AGENTS.md` Language Policy (English-only production).
- **NFR2 (Format & Structural Integrity)**: Preserve Markdown formatting, tables, `<kbd>` tags, code fences, and line break spacing.
- **NFR3 (Zero Code Regressions)**: Strictly non-code changes; no impact on Go packages, web components, or tests.
