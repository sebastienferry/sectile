# TaskFlow (React + Go + SQLite)

Outil moderne et agentique de gestion des tâches pour développeurs et équipes techniques, construit avec **Go**, **React 19**, **Tailwind CSS v4**, et **SQLite**.

---

## ✨ Fonctionnalités implémentées

- 🔄 **Support Multi-Trackers Hybride (Linear, GitHub CLI, Jira & Local)** :
  - **Chargement & Synchronisation complète** :
    - `POST /api/sync/all` : Synchronise Linear, GitHub et Jira en un seul clic.
    - `POST /api/sync/linear` : Synchronise les tickets d'équipe Linear.
    - `POST /api/sync/github` : Synchronise les issues GitHub du repository configuré.
    - `POST /api/sync/jira` : Synchronise les tickets du projet Jira via la CLI Atlassian (`acli jira workitem list`).
  - **Création d'Issues avec routage CLI** :
    - Choix de la destination lors de l'ajout rapide (<kbd>N</kbd> ou `+`) : 🟣 **Linear**, 🐙 **GitHub**, 🔷 **Jira**, ou 📁 **Local SQLite**.
    - Exécution transparente de `linear issue create`, `gh issue create` ou `acli jira workitem create` en arrière-plan avec récupération automatique des identifiants et URLs.
  - **Mise à jour d'état bidirectionnelle** :
    - Déplacer une carte dans le Kanban ou la Liste met à jour automatiquement l'état sur Linear (`linear issue update --state`), sur GitHub (`gh issue close` / `reopen`) et sur Jira (`acli jira workitem transition --state`).
    - Les rapports d'exécution des skills sont postés en commentaire sur le ticket distant (`acli jira workitem comment` pour Jira).
  - **Configuration Jira par projet** :
    - Champ *Projet Jira* (clé passée à `acli --project`) et *URL Jira* pour construire les liens `/browse/<KEY>`.
    - Détection des statuts réels du workflow Jira via `acli jira workitem search`, avec repli sur *To Do / In Progress / In Review / Done*.
  - **Filtres par source & Badges d'origine** :
    - Filtrez en 1 clic dans la barre latérale : *Toutes les sources*, *Linear*, *GitHub*, *Jira*, *Local*, avec compteurs en temps réel.
    - Badges d'origine avec lien direct vers le ticket dans le navigateur.

- 📐 **Frameworks Spec-Driven Design installables (Spec Kit & OpenSpec)** :
  - **Installation réelle de la chaîne d'outils depuis l'interface** (onglet *Compétences IA & SDD* d'un projet, ou palette <kbd>Cmd+K</kbd>) :
    - **GitHub Spec Kit** : installe la CLI `specify` via `uv` / `uvx` depuis `git+https://github.com/github/spec-kit.git`, puis exécute `specify init --here`. Scaffolde `.specify/` et `specs/` (spec.md, plan.md, tasks.md) ainsi que les commandes `/speckit.*` de l'agent.
    - **OpenSpec** : installe la CLI `openspec` via `npm` / `npx` depuis `@fission-ai/openspec`, puis exécute `openspec init`. Scaffolde `openspec/` (propositions de changement, deltas de specs `ADDED` / `MODIFIED` / `REMOVED`, checklists).
  - `GET /api/spec-framework/status` : indique, par framework, si la CLI est trouvée dans le PATH et si le répertoire de travail est déjà initialisé.
  - `POST /api/spec-framework/install` : lance l'installation et renvoie **chaque commande exécutée** avec sa sortie, de sorte qu'un échec soit diagnosticable et rejouable à la main.
  - Chaque installation est tracée comme une activité dans la vue *Activités*.
  - Le framework choisi pilote le contenu de la skill `/specify-issue` scaffoldée dans le projet et le prompt envoyé à l'agent IA.

- 🤖 **Agent Copilot & Moteur IA Configurable (`agy`, `vibe`, `claude`)** :
  - **Choix du moteur d'IA** :
    - `agy` : Antigravity CLI (`agy -p "{prompt}" --dangerously-skip-permissions`).
    - `vibe` : Mistral Vibe CLI (`vibe -p "{prompt}" --auto-approve`).
    - `claude` : Claude Code CLI (`claude -p "{prompt}"`).
    - `custom` : Template de commande shell entièrement personnalisable avec variables d'injection.
  - **Personnalisation des Prompts par Skill** :
    1. 🔍 **Clarify** (`/clarify-issue`) : Analyse les ambiguïtés et génère les questions de cadrage.
    2. 📝 **Specify** (`/specify-issue`) : Rédige la spec (Spec Kit ou OpenSpec, selon le framework du projet) et initialise la branche Git.
    3. 💻 **Implement** (`/code-issue`) : Plan de code, modification des fichiers et tests unitaires.
    4. 🚀 **Create PR** (`/create-pr`) : Commit sémantique et description Markdown complète de la PR.
    5. ⚡ **Auto-Pilot** (`/pick-issue`) : Routeur intelligent qui enchaîne automatiquement l'étape optimale.
  - **Panneau de statut des CLI** : Vérification en temps réel de l'installation et de l'authentification de `git`, `gh`, `linear`, `acli`, `agy`, `vibe`, `claude`, `gemini`, `codex`, ainsi que des outils SDD `uv`, `specify` et `openspec`.

- 🗂 **Sidebar complète & Workflow Stages** :
  - `Backlog` ➔ `À clarifier` ➔ `Spécifié` ➔ `En cours` ➔ `À valider` ➔ `Terminé` avec compteurs en temps réel.
  - Bascule des vues (`Tableau Kanban` / `Vue Liste`).
  - Filtres rapides (`Mes tâches`, `Priorité Haute`, `Étiquettes/Tags`) et filtre par source (`Linear`, `GitHub`, `Jira`, `Local`).
  - Repli / Dépli fluide de la barre latérale.

- 👤 **Profil & Ergonomie Personnalisée** :
  - **Couleur d'accent dynamique** : *Indigo, Violet, Émeraude, Ambre, Rose, Cyan, Bleu, Orange*.
  - **Thème** : Mode Sombre (Dark) / Mode Clair (Light).
  - **Multi-langues** : Français (FR) / English (EN) avec bascule instantanée.
  - **Taille d'affichage & Densité** :
    - *Compact* (13px, espacements réduits, idéal écrans denses).
    - *Standard* (14px, vue équilibrée).
    - *Confortable* (15px, espacements aérés).

- 🔀 **Tableau Kanban & Vue Liste (Drag & Drop)** :
  - **Vue Tableau Kanban** : Glisser-déposer fluide entre colonnes avec mise à jour automatique Linear/GitHub.
  - **Vue Liste** : Regroupement par statut, tri multi-colonnes et édition inline.

- 🔍 **Recherche Rapide (`/`) & Palette d'actions (`Cmd+K`)** :
  - Raccourci clavier `/` pour cibler immédiatement la recherche.
  - Palette d'actions avec recherche floue et exécution directe des skills au clavier.
  - Dans la barre du terminal PTY, démarrer l'agent puis sélectionner un skill : Codex reçoit son nom sans `/`, les autres moteurs conservent le slash. Les noms personnalisés du projet sont respectés.

---

## 🚀 Démarrage rapide

### 1. Mode Production (Serveur unique Go servant le frontend React & l'API)
```bash
make run
```
L'application est disponible sur **http://localhost:8080**.

---

### 2. Mode Développement (Hot-Reloading React + Go API)
Dans deux terminaux séparés :
```bash
# Terminal 1: Go Backend API sur le port 8080
make dev-server

# Terminal 2: React Vite Dev Server sur le port 5173 (avec proxy API automatique)
make dev-web
```
Puis ouvrez **http://localhost:5173**.

---

## 📚 Documentation Technique Complète

Une suite documentaire complète pour développeurs et LLMs est disponible dans le dossier [`/docs`](./docs) :

- 🏛️ [**Architecture & Conception Générale** (`docs/ARCHITECTURE.md`)](./docs/ARCHITECTURE.md) : Modèle de concurrence, persistance SQLite, isolation Git Worktrees, PTY ZSH & WebSockets.
- ⚡ [**Capacités & Workflows Agentiques** (`docs/CAPABILITIES.md`)](./docs/CAPABILITIES.md) : Multi-projets, pipeline de 5 skills, Auto-Pilot, synchronisation Linear / GitHub.
- 🎨 [**Composants UX & Design Frontend** (`docs/UX_COMPONENTS.md`)](./docs/UX_COMPONENTS.md) : Kanban drag-and-drop, vue liste, terminal interactif Xterm.js, inspecteur de Diff Git.
- 🔌 [**Spécification API & Schéma de Données** (`docs/API_AND_DATA_SPEC.md`)](./docs/API_AND_DATA_SPEC.md) : Schéma SQLite complet, endpoints REST et protocole WebSocket `/ws/terminal`.
- 🤖 [**Guide de Ré-implémentation pour LLMs** (`docs/REIMPLEMENTATION_GUIDE.md`)](./docs/REIMPLEMENTATION_GUIDE.md) : Blueprint étape par étape pour reconstruire TaskFlow de zéro.

---

## ⌨️ Raccourcis Clavier

| Raccourci | Action |
|---|---|
| `/` | Cibler la barre de recherche globale |
| `Cmd+K` ou `Ctrl+K` | Ouvrir la palette de commandes & skills |
| `N` ou `C` | Ouvrir la modale d'ajout rapide de tâche |
| `Esc` | Fermer la modale / vider la recherche |
| `↑` / `↓` + `Entrée` | Naviguer et valider dans la palette d'actions |
