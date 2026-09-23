# batch-issue-pickup Specification

## Purpose
TBD - created by archiving change 30-add-a-new-skill-to-pick-up-multiple. Update Purpose after archive.
## Requirements
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


#### Scenario: Selecting cards on the Kanban board
- **WHEN** the user ticks cards on the board, with their checkbox or with Ctrl/Cmd+click, and clicks "Lancer le lot (Git tree + Auto-pilot)" in the selection bar
- **THEN** only cards whose workflow stage is `new` or `clarified` can be selected, and `pickup-issues` receives them in board order: columns from left to right, then cards from top to bottom

#### Scenario: Launching selected tasks from the Backlog
- **WHEN** the user selects visible Backlog tasks from one project, all at the `new` or `clarified` stage
- **THEN** the selection bar enables the batch launch action and passes the tasks in displayed group and row order
- **AND** an accepted launch clears the submitted selection, while a refused launch keeps it for retry
- **AND** the action is disabled during launch or for an ineligible selection, with an explanation for ineligible selections

#### Scenario: Preparing the execution order and worktree before launch
- **WHEN** the user chooses the batch action in any supported view
- **THEN** a shared dialog opens with the selected tasks in their initial view order and an editable worktree name
- **AND** the user can move each task up or down before confirming the launch
- **AND** no execution request is sent until confirmation
- **AND** confirmation sends the chosen order and worktree name to the `pickup-issues` skill
- **AND** cancelling preserves the caller's selection; a failed launch keeps the dialog, task order and worktree name available for retry
- **AND** worktree names contain 1–80 letters, digits, hyphens or underscores, starting with a letter or digit, and cannot contain paths
