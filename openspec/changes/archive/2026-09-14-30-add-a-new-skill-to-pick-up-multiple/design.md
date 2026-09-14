## Context

Voir `proposal.md` et `specs/batch-issue-pickup/spec.md`.
TaskFlow dispose déjà de composants de sélection par lot dans `CurationTable.tsx`, `TriageView.tsx` et `SprintTimelineView.tsx`, ainsi qu'un système de gestion de Git Worktrees et d'exécution d'agents via PTY/WebSocket.

## Goals / Non-Goals

**Goals:**
- Proposer un skill complet `pickup-issues` pour le traitement autonome séquentiel d'un lot de tickets.
- Enregistrer le template `pickup-issues` dans la base de données et le système de fichiers Go (`skilltemplates.go`).
- Ajouter dans la barre d'actions groupées de `CurationTable.tsx`, `TriageView.tsx` et `SprintTimelineView.tsx` un bouton permettant d'initialiser le lot, de basculer dans le worktree dédié et de lancer le skill agent.

**Non-Goals:**
- Fusionner automatiquement la PR générée dans le branch par défaut (la fusion reste réservée à l'utilisateur humain).
- Modifier la gestion unitaire des tickets isolés via `/pickup-issue`.

## Decisions

### 1. Structure du Skill `pickup-issues`
- **Fichier** : `.agents/skills/pickup-issues/SKILL.md` et template dans `internal/db/skilltemplates.go`.
- **Fonctionnement** :
  1. Récupération ordonnée des identifiants des tickets.
  2. Initialisation ou sélection du worktree batch (`batch-issues-KEY1-KEYN`).
  3. Boucle d'exécution séquentielle SDD (`clarify` -> `specify` -> `code` -> `test/build`).
  4. Création d'une PR combinée.

### 2. Intégration UX dans la barre d'action de sélection groupée
- **Emplacements** : `CurationTable.tsx` (Backlog), `TriageView.tsx` (Triage), `SprintTimelineView.tsx` (Sprint Timeline).
- **Composant UX** : Bouton dans la barre flottante de sélection multiple avec l'icône de robot/baguette et l'intitulé **"🚀 Lancer le lot (Git tree + Auto-pilot)"**.
- **Comportement au clic** :
  - Extraction de la liste des clés/IDs des tickets cochés.
  - Appel au handler backend d'ouverture/création du Worktree batch.
  - Lancement de la session terminal/runner avec `/pickup-issues KEY1 KEY2 ...`.

## Risks / Trade-offs

- **[Conflits entre tickets du lot]** → Atténué par le traitement séquentiel strict au sein du même branch avec ré-exécution des tests à chaque étape.
- **[Grand nombre de tickets sélectionnés]** → L'UX affiche une confirmation précisant le nombre de tickets inclus dans le lot avant le lancement.
