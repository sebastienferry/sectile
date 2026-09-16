# Proposal: Retirer des tickets d'un sprint dans la vue Sprint

## Problem
Dans l'interface Sectile, lorsqu'un utilisateur consulte la vue Sprint (`SprintTimelineView`), il ne dispose pas d'un moyen simple et universel pour désattribuer un ticket de son sprint (renvoyer la tâche au backlog en réinitialisant son `sprintId`/`sprint`). Actuellement, le retrait n'est accessible qu'en mode planification avec le panneau Backlog ouvert via le glisser-déposer ou un bouton discret 'X' sur les cartes en mode édition. En mode compact (cartes ou chips) et depuis le menu contextuel universel des cartes (`TaskCard`), aucune action "Retirer du sprint" / "Déplacer vers le backlog" n'est disponible. De plus, la sélection multiple de cartes dans un sprint ne permet pas de retirer un lot de tickets d'un sprint vers le backlog en une seule action.

## Proposed Solution
Ajouter des points d'action clairs et réactifs pour retirer un ou plusieurs tickets d'un sprint directement depuis la vue Sprint et depuis les cartes de tickets :
1. **Action individuelle dans le menu contextuel `TaskCard`** : Ajouter une entrée "Retirer du sprint" (ou "Déplacer vers le backlog") dans le menu contextuel de la carte lorsqu'un ticket est associé à un sprint.
2. **Actions individuelles en vue Sprint (modes compact et planification)** : Permettre le retrait individuel immédiat d'un ticket en mode compact (cartes & chips) ainsi qu'en mode planification.
3. **Action groupée (batch action)** : Proposer une action "Retirer du sprint" / "Renvoyer au backlog" dans la barre de sélection multiple lorsque plusieurs tickets d'un sprint sont cochés.
4. **Mise à jour réactive & persistance API** : Exécuter `setTaskSprint(taskId, "", "")` ou `setTasksSprint(projectId, taskIds, "", "")` pour mettre à jour instantanément l'état local UI (disparition du ticket du sprint et réapparition dans le backlog), persister la suppression en base SQLite via `/api/tasks/{id}/sprint` ou `/api/projects/{id}/sprint-move`, et empiler l'opération de synchronisation tracker dans la file d'activités.

## In Scope
- Ajout du bouton/action "Retirer du sprint" dans `TaskCard.tsx` (menu contextuel `...`).
- Ajout des actions de retrait individuel dans `SprintTimelineView.tsx` pour l'ensemble des modes d'affichage (cartes compactes, chips compactes, cartes de planification, chips de planification).
- Prise en charge des actions par lots (batch remove) pour les tickets sélectionnés au sein des sprints dans `SprintTimelineView.tsx`.
- Validation de l'intégration avec le state `AppContext` (`setTaskSprint`, `setTasksSprint`) et les endpoints API backend (`POST /api/tasks/{id}/sprint` et `POST /api/projects/{id}/sprint-move`).
- Validation OpenSpec SDD via `openspec validate 27-on-ne-peut-pas-enlever-des-ticket-d --strict`.

## Out of Scope
- Modification du calcul automatique des dates des cycles de sprints.
- Refonte visuelle complète de la timeline des sprints.
- Implémentation de nouvelles fonctionnalités de gestion des milestones GitHub non liées au champ `sprint`.
