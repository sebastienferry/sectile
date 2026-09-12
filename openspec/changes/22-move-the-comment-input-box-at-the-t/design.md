# Design: Repositionnement de la zone de saisie des commentaires en haut de la modale

## Technical Decisions

### 1. Réordonnancement du composant `TaskComments.tsx`
Dans la structure JSX du composant `TaskComments` :
- **Rang 1** : En-tête de section (Titre "Commentaires", compteur et bouton "Actualiser").
- **Rang 2** : Zone de saisie d'un nouveau commentaire (`MarkdownEditor`, champ de rédaction et bouton "Publier").
- **Rang 3** : Zone d'affichage et de défilement des commentaires existants (`comments.map(...)`).

### 2. Ergonomie et flux d'interaction
- Placer la zone de saisie en haut permet à l'utilisateur d'interagir immédiatement dès l'ouverture de la section commentaires sans devoir parcourir l'historique.
- L'affichage antéchronologique (commentaire le plus récent en premier) assure que la publication d'un nouveau commentaire insère le message directement sous la zone de saisie.

## Rejected Alternatives

- **Alternative 1 : Zone de saisie fixe ("sticky") en bas de fenêtre**.
  - *Raison du rejet* : Masque une partie de la liste déroulante et réduit inutilement la hauteur disponible pour la lecture des commentaires.
- **Alternative 2 : Duplication du champ de saisie en haut et en bas**.
  - *Raison du rejet* : Surcharge inutilement l'interface UI avec des éléments redondants.
