# Proposal: Déplacer la zone de saisie des commentaires en haut de la modale de tâche

## Problem
Dans l'interface de la modale de détail de tâche (`TaskDetailModal.tsx` / `TaskComments.tsx`), le bloc de saisie d'un nouveau commentaire (`MarkdownEditor` et le bouton "Publier") est positionné en bas de la section des commentaires, sous la liste complète des échanges existants. Lorsque la discussion comporte de nombreux commentaires, l'utilisateur doit défiler jusqu'en bas pour poster un nouveau message, ce qui nuit à l'ergonomie.

## Proposed Solution
Reconditionner le composant `TaskComments.tsx` pour placer la zone de saisie des commentaires en haut :
1. **Positionnement en haut** : Placer l'éditeur `MarkdownEditor` et le bouton de publication directement sous le titre de la section "Commentaires" et au-dessus de la liste déroulante des commentaires existants.
2. **Affichage antéchronologique conservé** : Conserver le tri des commentaires du plus récent au plus ancien, afin que tout nouveau commentaire publié apparaisse immédiatement juste sous la zone de saisie sans nécessiter de défilement.
3. **Maintien des raccourcis** : Conserver le raccourci `Cmd/Ctrl + Entrée` pour publier rapidement.

## In Scope
- Réorganisation de la disposition JSX dans `web/src/components/TaskComments.tsx`.
- Ajustement des espacements et bordures visuelles entre la zone de saisie supérieure et la liste des commentaires.
- Validation OpenSpec SDD via `openspec validate 22-move-the-comment-input-box-at-the-t --strict`.

## Out of Scope
- Modifications backend des handlers de commentaires (`/api/tasks/{id}/comments`).
- Modification de l'éditeur Markdown lui-même.
