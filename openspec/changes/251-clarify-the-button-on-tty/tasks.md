# Tasks: Clarifier les boutons et la terminologie du Terminal TTY

- [ ] 1. **Refactoriser les contrôles de l'en-tête du volet (`WorkspaceTerminalPanel.tsx`)**
  - [ ] 1.1 Mettre à jour l'info-bulle et l'intitulé du bouton de fermeture `X` vers `"Masquer le panneau"`.
  - [ ] 1.2 Consolider l'action de terminaison de session dans le sélecteur de sessions avec un bouton/icône explicite `"Terminer la session"`.
  - [ ] 1.3 Vérifier que le masquage du volet ne détruit pas la session PTY en arrière-plan.

- [ ] 2. **Refactoriser la barre d'actions de `InteractiveTerminal.tsx`**
  - [ ] 2.1 Renommer le bouton `Reset` en `"Relancer le shell"` avec l'info-bulle `"Redémarrer une nouvelle session shell vierge"`.
  - [ ] 2.2 S'assurer que le bouton `Clear` dispose de l'info-bulle `"Effacer le texte à l'écran"`.
  - [ ] 2.3 Éliminer les doublons de boutons de fermeture/redimensionnement lorsque `InteractiveTerminal` est encapsulé dans `WorkspaceTerminalPanel`.

- [ ] 3. **Validation & Conformité OpenSpec**
  - [ ] 3.1 Vérifier la compilation et le build frontend sans erreur (`npm run build`).
  - [ ] 3.2 Valider la conformité du changement avec `openspec validate 251-clarify-the-button-on-tty --strict`.
