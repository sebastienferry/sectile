# #360 — Technical Plan

Behavior and acceptance criteria live in [`spec.md`](./spec.md). This file records the implementation choices, terminology mapping, and verification strategy.

## Stack and Constraints

- Target file: `README.md` (repository root).
- Format: GitHub Flavored Markdown (GFM), UTF-8 encoded with Unix (`\n`) line endings.
- Language: English only, conforming to `AGENTS.md`.
- No modifications to source code (Go, TypeScript, CSS) or documentation files outside `README.md`.
- No database migrations, API changes, or dependency bumps.

---

## Shape of the Change

```
README.md
  │
  ├─ [A] Line 8: Introductory tagline
  │      Replace French tagline with English canonical definition.
  │
  ├─ [B] Lines 12–85: Features section (## ✨ Fonctionnalités implémentées)
  │      Translate heading and all 6 subsections to English:
  │      - Spec-Driven Design installable frameworks (Spec Kit & OpenSpec)
  │      - AI Copilot & Configurable AI Engine (agy, vibe, claude)
  │        * Fix /pick-issue to /pickup-issue
  │      - Complete sidebar & Workflow Stages
  │      - Profile & Personalized Ergonomics
  │      - Kanban Board & List View (Drag & Drop)
  │      - Quick Search (/) & Action Palette (Cmd+K)
  │        * Retire obsolete web PTY terminal bar mention
  │
  ├─ [C] Lines 231, 246, 256: Tracker connection UI references
  │      Replace *Connecter votre tracker* with *Connect your tracker*.
  │      Replace *Profil > Trackers* with *Profile > Tracker Credentials*.
  │
  ├─ [D] Lines 447–457: Technical Documentation (## 📚 Documentation Technique Complète)
  │      Translate heading and align bullet points with docs/README.md.
  │
  ├─ [E] Lines 459–468: Keyboard Shortcuts (## ⌨️ Raccourcis Clavier)
  │      Translate table headers and shortcut action descriptions.
  │
  └─ [F] Lines 648–651: Agent configuration guidance
         Remove obsolete mention of "Agents à configurer" checkboxes.
```

---

## Terminology Concordance Table

To ensure consistency across the project, translations are aligned with `web/src/locales/translations.ts` (`en`) and `docs/README.md`:

| French Term / Context | English Canonical Term | Source of Truth |
|---|---|---|
| `Outil moderne et agentique de gestion des tâches...` | `Modern, agentic task workflow manager for developers and engineering teams, built with **Go**, **React 19**, **Tailwind CSS v4**, and **SQLite**.` | Clarification §Round 1 |
| `Fonctionnalités implémentées` | `Implemented Features` | Standard heading |
| `Frameworks Spec-Driven Design installables` | `Installable Spec-Driven Design (SDD) Frameworks` | `translations.ts` (`profileModal.sdd.title`) |
| `Compétences IA & SDD` | `AI & SDD Skills` | `translations.ts` (`profileModal.tabs.sdd`) |
| `Agent Copilot & Moteur IA Configurable` | `Agent Copilot & Configurable AI Engines` | `translations.ts` (`profileModal.tabs.aiConfig`) |
| `Moteur IA` | `AI Engine` | `translations.ts` (`statusBar.aiProvider`) |
| `Clarify`, `Specify`, `Implement`, `Adjust`, `Auto-Pilot` | `Clarify`, `Specify`, `Implement`, `Adjust`, `Auto-Pilot` | `CAPABILITIES.md` |
| `/pick-issue` (obsolete) | `/pickup-issue` | Canonical skill command |
| `Sidebar complète & Workflow Stages` | `Comprehensive Sidebar & Workflow Stages` | `docs/UX_COMPONENTS.md` |
| `À clarifier` | `To Clarify` | `translations.ts` (`nav.toClarify`) |
| `Spécifié` | `Specified` | `translations.ts` (`nav.specified`) |
| `En cours` | `In Progress` | `translations.ts` (`nav.inProgress`) |
| `À valider` | `To Validate` | `translations.ts` (`nav.toValidate`) |
| `Terminé` | `Done` | `translations.ts` (`nav.done`) |
| `Tableau Kanban` | `Kanban Board` | `translations.ts` (`nav.board`) |
| `Vue Liste` | `List View` | `translations.ts` (`nav.list`) |
| `Mes tâches` | `My Tasks` | `translations.ts` (`nav.myTasks`) |
| `Priorité Haute` | `High Priority` | `translations.ts` (`nav.urgentHigh`) |
| `Étiquettes/Tags` | `Labels / Tags` | `translations.ts` (`nav.labels`) |
| `Profil & Ergonomie Personnalisée` | `Profile & Personalized Ergonomics` | `translations.ts` (`profileModal.title`) |
| `Couleur d'accent dynamique` | `Dynamic Accent Color` | `translations.ts` (`profileModal.accentColor`) |
| `Mode Sombre / Mode Clair` | `Dark Mode / Light Mode` | `translations.ts` (`profileModal.themes`) |
| `Taille d'affichage & Densité` | `Display Density & UI Scaling` | `translations.ts` (`profileModal.density`) |
| `Recherche Rapide (/) & Palette d'actions (Cmd+K)` | `Quick Search (/) & Action Palette (Cmd+K)` | `translations.ts` (`header.commandPalette`) |
| `Connecter votre tracker` | `Connect your tracker` | `translations.ts` (`trackerCredentials.setup.title`) |
| `Profil > Trackers` | `Profile > Tracker Credentials` | `translations.ts` (`profileModal.tabs.trackers`) |
| `Documentation Technique Complète` | `Comprehensive Technical Documentation` | `docs/README.md` |
| `Raccourcis Clavier` | `Keyboard Shortcuts` | Standard documentation |
| `Raccourci` / `Action` | `Shortcut` / `Action` | Table header |
| `Cibler la barre de recherche globale` | `Focus global search bar` | Action description |
| `Ouvrir la palette de commandes & skills` | `Open command & skill palette` | Action description |
| `Ouvrir la modale d'ajout rapide de tâche` | `Open quick task creation modal` | Action description |
| `Fermer la modale / vider la recherche` | `Close modal / clear search` | Action description |
| `Naviguer et valider dans la palette d'actions` | `Navigate and select in action palette` | Action description |

---

## Technical Decisions

### D1 — Target Strictly the Root `README.md` File

Other documentation files (`web/README.md`, `desktop/README.md`, `docs/README.md`, `docs/*.md`) are already written in English. Confining edits to `README.md` avoids unnecessary churn and prevents accidental regression on other surfaces.

### D2 — Prune Retired Features Rather Than Translating Dead Concepts

1. **Web PTY Bar (line 83)**: The former embedded web terminal bar was retired when interactive sessions moved to native agent-owned desktop consoles (`desktop/README.md`). Translating the web PTY bar instructions would mislead users. The section is updated to describe running skills from the action palette, card action menus, and desktop consoles.
2. **"Agents à configurer" Checkboxes (line 649)**: The multi-agent checkboxes in project settings were retired in commit `715e90b` in favor of direct active provider configuration and `sectile-agent init --provider <provider>`. The paragraph is rewritten to accurately describe current behavior.
3. **Fix `/pick-issue` to `/pickup-issue` (line 57)**: The skill name in Sectile is `/pickup-issue`. Correcting this prevents command confusion.

### D3 — Align Technical Documentation Summaries with `docs/README.md`

`docs/README.md` already provides canonical English summaries for all technical docs. Reusing them directly ensures consistency between the root `README.md` index and the `/docs` directory index.

### D4 — Markdown and Formatting Invariants

- Preserve existing emoji bullet markers (`📐`, `🤖`, `🗂`, `👤`, `🔀`, `🔍`, `🏛️`, `⚡`, `🎨`, `🔌`).
- Preserve HTML tags (`<kbd>Cmd+K</kbd>`).
- Preserve all CLI syntax, code fences, and endpoint signatures (`GET /api/spec-framework/status`, `POST /api/spec-framework/install`, `{mode:AUTONOMOUS|INTERACTIVE}`).
- Maintain GitHub Flavored Markdown table syntax.

---

## Verification Strategy

1. **Regex Pattern Scan**:
   Execute grep on `README.md` for French accents (`[éèàêçùôîïë]`) and common French stopwords (`pour`, `avec`, `dans`, `une`, `des`, `sont`, etc.) to ensure no French prose remains.
2. **Link and Anchor Validation**:
   Verify that all relative paths (`docs/ARCHITECTURE.md`, `desktop/README.md`, etc.) resolve and exist.
3. **Project Test Gate**:
   Run `make test` to ensure no workspace or repository build gates are affected.
