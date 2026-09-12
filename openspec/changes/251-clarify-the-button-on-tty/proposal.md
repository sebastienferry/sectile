# Proposal: Clarifier les boutons et la terminologie du Terminal TTY

## Problem
Dans l'interface TaskFlow, l'interface du terminal interactif (`InteractiveTerminal.tsx` et `WorkspaceTerminalPanel.tsx`) comporte plusieurs boutons aux intitulés et comportements superposés et ambigus ("Close TTY", "Close screen", "Reset", "Disconnect"). Les utilisateurs confondent fréquemment le masque de la vue (fermeture du volet latéral via l'icône `X`) et l'extinction/destruction du processus PTY en arrière-plan (`resetTerminalSession`). De plus, la différence entre effacer le texte à l'écran (`Clear`) et réinitialiser le processus shell (`Reset`) n'est pas claire, et certains boutons de contrôle sont dupliqués entre l'en-tête du volet et l'en-tête de la console.

## Proposed Solution
Clarifier la terminologie et la hiérarchie des boutons de contrôle du terminal TTY :
1. **Isolation de la fermeture du volet** : Réserver l'icône `X` de l'en-tête du volet `WorkspaceTerminalPanel` exclusivement au masquage du panneau UI (`setIsTerminalPanelOpen(false)`).
2. **Terminaison explicite des sessions PTY** : Placer l'action d'arrêt/destruction de session PTY ("Terminer la session") de manière explicite dans le menu/sélecteur de sessions, évitant ainsi toute destruction accidentelle de session lors de la fermeture du volet.
3. **Terminologie explicite des actions terminal** : Conserver le bouton `Clear` (avec l'info-bulle "Effacer le texte à l'écran") et renommer le bouton `Reset` en **"Relancer le shell"** (avec l'info-bulle "Redémarrer une nouvelle session shell vierge").
4. **Hiérarchie consolidée** : Regrouper les contrôles de structure (sélection de session, ancrage du volet, plein écran, masquage du volet) dans l'en-tête de `WorkspaceTerminalPanel`, et conserver dans `InteractiveTerminal` uniquement les raccourcis d'action sur la session active (`Ctrl+C`, `Clear`, `Relancer le shell`, `Lancer agy`/skills).

## In Scope
- Réorganisation et clarification des boutons et info-bulles dans `web/src/components/WorkspaceTerminalPanel.tsx`.
- Réorganisation et renommage des boutons dans `web/src/components/InteractiveTerminal.tsx`.
- Consolidation des actions de terminaison de session dans le sélecteur de sessions.
- Validation OpenSpec SDD via `openspec validate 251-clarify-the-button-on-tty --strict`.

## Out of Scope
- Modifications du backend Go PTY (`internal/pty`) ou des handlers WebSocket.
- Modification du comportement de rendu xterm.js.
