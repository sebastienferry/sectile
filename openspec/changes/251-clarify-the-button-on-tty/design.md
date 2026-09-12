# Design: Clarification des contrôles et de la terminologie du Terminal TTY

## Technical Decisions

### 1. Séparation stricte entre masquage du panneau et terminaison de session
- **Bouton `X` de l'en-tête du volet (`WorkspaceTerminalPanel.tsx`)** : Exécute uniquement `setIsTerminalPanelOpen(false)` avec l'info-bulle explicite `"Masquer le panneau"`. Ce bouton ne détruit aucun processus PTY.
- **Action de terminaison de session (`resetTerminalSession`)** : Déplacée exclusivement dans le sélecteur de sessions `<select>` ou sous forme d'action explicite `"Terminer la session"` associée au menu des sessions actives.

### 2. Renommage et clarification des actions de la console (`InteractiveTerminal.tsx`)
- **Action `Clear`** : Consolidée avec l'info-bulle `"Effacer le texte à l'écran"`.
- **Action `Reset` ➔ `Relancer le shell`** : Le bouton `<RotateCcw /> Reset` est renommé en `<RotateCcw /> Relancer le shell` avec l'info-bulle `"Redémarrer une nouvelle session shell vierge"`.
- **Action `Ctrl+C`** : Conservée pour l'interruption du processus interactif en cours (envoi de `\x03`).

### 3. Consolidation de la hiérarchie des barres d'outils
- **`WorkspaceTerminalPanel.tsx` (Niveau Volet / Workspace)** :
  - Sélecteur de session active (`<select>`).
  - Action d'arrêt de la session sélectionnée.
  - Commutateur de position d'ancrage (Gauche `PanelLeft`, Bas `PanelBottom`, Droite `PanelRight`).
  - Bascule mode Plein Écran (`Maximize2` / `Minimize2`).
  - Bouton de fermeture du volet `X` ("Masquer le panneau").
- **`InteractiveTerminal.tsx` (Niveau Session Active)** :
  - Témoin d'état de la connexion PTY (`Connecté (Live PTY)`, `Connexion...`, `Déconnecté`).
  - Actions rapides de session : `Ctrl+C`, `Clear`, `Relancer le shell`.
  - Barre d'actions agent : `Lancer agy` / `agy en cours`, raccourcis de skills (`/specify-issue`, `/code-issue`, etc.), `git status`, éditeur.

## Rejected Alternatives

- **Alternative 1 : Afficher une modale de confirmation à chaque clic sur `X`**.
  - *Raison du rejet* : `X` est une action UI standard de fermeture de panneau rétractable. Ajouter une modale créerait de la friction inutile lors de la simple navigation UI.
- **Alternative 2 : Regrouper `Clear` et `Reset` dans un sous-menu déroulant**.
  - *Raison du rejet* : `Clear` est une action fréquente à clic unique dans l'usage quotidien du terminal. La masquer dans un sous-menu dégraderait l'expérience développeur.
