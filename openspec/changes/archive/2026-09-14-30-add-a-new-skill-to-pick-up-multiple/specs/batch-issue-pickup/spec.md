## Purpose

Permet de regrouper et d'exécuter un lot de tâches en autonomie au sein d'un Git Worktree dédié avec un skill agent et de fournir une action UX directe de sélection par lot dans les différentes vues de TaskFlow.

## ADDED Requirements

### Requirement: Traitement par lot CLI via skill pickup-issues
Le système SHALL fournir un skill agent `pickup-issues` capable de prendre en charge une liste ordonnée d'identifiants de tâches, d'initialiser un Git Worktree unique pour le lot, de faire progresser séquentiellement chaque tâche (clarification, spécification, implémentation, tests) et de publier une Pull Request combinée.

#### Scenario: Lancement du traitement par lot via commande skill
- **WHEN** l'agent ou l'utilisateur exécute la commande `/pickup-issues` avec plusieurs identifiants de tâches
- **THEN** le système crée ou bascule sur un Git Worktree dédié au lot et exécute séquentiellement le cycle SDD pour chaque tâche avant de soumettre une PR globale

### Requirement: Bouton d'action groupée UX dans la barre de sélection multiple
Les vues Kanban/Curation, Triage et Sprint Timeline SHALL intégrer un bouton d'action groupée **"Lancer le lot (Git tree + Auto-pilot)"** dans la barre d'action affichée lorsque plusieurs tâches sont sélectionnées.

#### Scenario: Clic sur le bouton de lancement de lot depuis une vue UI
- **WHEN** l'utilisateur coche plusieurs tâches dans une vue et clique sur le bouton "Lancer le lot (Git tree + Auto-pilot)"
- **THEN** l'application web initie la création du Git Worktree pour le lot sélectionné et démarre la session d'exécution autonome du skill `pickup-issues`
