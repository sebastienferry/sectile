## 1. Skill Backend & Infrastructure

- [x] 1.1 Créer et documenter le fichier de skill `.agents/skills/pickup-issues/SKILL.md` pour l'enchaînement séquentiel autonome d'un lot et vérifier la présence du fichier.
- [x] 1.2 Enregistrer le modèle de skill `pickup-issues` dans `internal/db/skilltemplates.go` et vérifier la compilation backend Go avec `go build ./cmd/server`.
- [x] 1.3 Mettre à jour le runner agent dans `internal/runner/runner.go` pour supporter la commande `/pickup-issues` et vérifier avec `go test ./internal/runner/...`.

## 2. Intégration UX de Sélection Multiple (UI)

- [x] 2.1 Ajouter le bouton **"🚀 Lancer le lot (Git tree + Auto-pilot)"** dans la barre d'action groupée de `CurationTable.tsx`.
- [x] 2.2 Ajouter le bouton **"🚀 Lancer le lot (Git tree + Auto-pilot)"** dans la barre d'action groupée de `TriageView.tsx`.
- [x] 2.3 Ajouter le bouton **"🚀 Lancer le lot (Git tree + Auto-pilot)"** dans la barre d'action groupée de `SprintTimelineView.tsx`.
- [x] 2.4 Raccorder le déclenchement du lot à la création du Git Worktree et à l'exécution du runner pour la liste des clés sélectionnées, et vérifier la compilation frontend via le build Vite (`cd web && npm run build`).

## 3. Validation globale & Qualification

- [x] 3.1 Exécuter la suite complète de tests unitaires et d'intégration Go via `go test ./...` et vérifier l'absence d'erreurs.
- [x] 3.2 Valider la spécification OpenSpec avec `openspec validate 30-add-a-new-skill-to-pick-up-multiple --strict`.
