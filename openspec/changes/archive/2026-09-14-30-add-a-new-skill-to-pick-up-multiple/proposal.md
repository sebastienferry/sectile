# Proposal: Traitement par lot de tickets (Skill pickup-issues & Action UX multi-sélection)

## Why

Dans TaskFlow, le développement piloté par les agents (SDD) permet d'exécuter l'ensemble du cycle de vie d'un ticket (`clarify`, `specify`, `code`, `create-pr`, `pickup-issue`). Cependant, lorsqu'un utilisateur sélectionne plusieurs tâches dans les différentes vues de l'application (Triage, Backlog/Curation, Sprint Timeline), l'UX actuelle ne propose la sélection multiple que pour les actions de métadonnées (affectation de sprint, d'équipe ou de macro).

Il est nécessaire de permettre aux utilisateurs d'exécuter un lot de plusieurs tickets en autonomie au sein d'un **unique Git Worktree dédié** produisant **une seule Pull Request combinée**, aussi bien via la commande skill `/pickup-issues` que directement depuis la barre d'actions de sélection multiple dans les vues UI.

## What Changes

1. **Nouveau Skill Agent `pickup-issues`** :
   - Création et intégration du skill `.agents/skills/pickup-issues/SKILL.md`.
   - Enchaînement séquentiel autonome (`clarify` -> `specify` -> `code` -> `test/build` -> `create-pr`) pour une liste de tickets dans un seul branch/worktree dédié (ex. `batch-issues-<KEYS>`).
   - Enregistrement du modèle de skill `pickup-issues` dans le backend Go.

2. **Évolution UX de sélection multiple (Batch Actions)** :
   - Ajout d'un bouton d'action groupée dans la barre de sélection multiple de `CurationTable.tsx`, `TriageView.tsx` et `SprintTimelineView.tsx` : **"🚀 Lancer le lot (Git tree + Auto-pilot)"**.
   - Ouverture/création automatique du Git Worktree dédié aux tâches sélectionnées lors du clic.
   - Lancement automatique de l'exécution du runner autonome `/pickup-issues` en lui transmettant la liste des clés/IDs des tâches cochées.

## Capabilities

### New Capabilities

- `batch-issue-pickup`: Prise en charge du traitement groupé de tâches via le skill `pickup-issues` et déclenchement d'un lot d'auto-pilot avec Git Worktree depuis les barres de sélection des vues UI.

### Modified Capabilities

- Aucune.

## Impact

- **Frontend (`web/src`)** : `CurationTable.tsx`, `TriageView.tsx`, `SprintTimelineView.tsx`, `TaskCard.tsx`, et le contexte de lancement d'agents.
- **Backend Go (`internal`)** : `internal/db/skilltemplates.go`, `internal/handlers/handlers.go`, `internal/runner/runner.go`.
- **Skills (`.agents/skills`)** : `.agents/skills/pickup-issues/SKILL.md`.
