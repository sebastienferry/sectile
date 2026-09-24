package db

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// Un tasks.md tel qu'une spécification l'écrit : des groupes numérotés, et sous
// chacun le détail d'exécution.
const tasksFile = `## 1. La liste sur le projet

- [x] 1.1 Ajouter le champ au modèle.
- [ ] 1.2 Porter la colonne.

## 2. L'origine d'une macro

- [ ] 2.1 Écrire la règle.

## 3. Les portes du projet

- [ ] 3.1 go build, go test.
`

const specFile = `## ADDED Requirements

### Requirement: Un projet déclare ses projets distants

Le projet SHALL porter une liste.

#### Scenario: Trois clés sont retenues

- **WHEN** on saisit trois clés
- **THEN** elles sont retenues

### Requirement: Rien n'est écrit sur une macro distante

Une écriture SHALL être refusée.
`

func TestExtractionDesGroupesDeTaches(t *testing.T) {
	got := ExtractTaskGroups(tasksFile)
	if len(got) != 3 {
		t.Fatalf("trois groupes attendus, obtenu %d : %v", len(got), got)
	}
	if got[0] != "La liste sur le projet" {
		t.Fatalf("le numéro devait être retiré, obtenu %q", got[0])
	}
	// Une ligne d'exécution n'est jamais une story.
	for _, entry := range got {
		if strings.Contains(entry, "Ajouter le champ") {
			t.Fatalf("une ligne de détail a été retenue : %q", entry)
		}
	}
}

func TestExtractionDesExigences(t *testing.T) {
	got := ExtractSpecRequirements(specFile)
	if len(got) != 2 {
		t.Fatalf("deux exigences attendues, obtenu %d : %v", len(got), got)
	}
	if got[0] != "Un projet déclare ses projets distants" {
		t.Fatalf("énoncé inattendu : %q", got[0])
	}
	// Un scénario n'est pas une exigence.
	for _, entry := range got {
		if strings.Contains(entry, "Trois clés") {
			t.Fatalf("un scénario a été retenu : %q", entry)
		}
	}
}

// Spec Kit n'écrit pas d'exigences mais des user stories priorisées : la seconde
// passe les reconnaît, sans que le projet ait à dire laquelle des deux formes son
// fichier emploie.
func TestExtractionDesUserStoriesDeSpecKit(t *testing.T) {
	speckit := `# Spécification

## Prioritised user stories

1. **Un PM veut lire la découpe** pour ne plus la retaper.
2. **Un développeur veut une story par ligne** pour la livrer seule.

## Out of scope

- Le reste
`
	got := ExtractSpecRequirements(speckit)
	if len(got) != 2 {
		t.Fatalf("deux user stories attendues, obtenu %d : %v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "Un PM veut lire") {
		t.Fatalf("énoncé inattendu : %q", got[0])
	}
	// Ce qui suit un autre titre n'est pas une story.
	for _, entry := range got {
		if entry == "Le reste" {
			t.Fatal("une ligne hors de la section a été retenue")
		}
	}
}

func TestNormalisationDeLaSource(t *testing.T) {
	if NormalizeSlicingSource("spec") != SlicingFromSpec {
		t.Fatal("« spec » attendu comme source des exigences")
	}
	// Par défaut les tâches : c'est ce qu'une spécification écrit pour être
	// découpé.
	if NormalizeSlicingSource("") != SlicingFromTasks {
		t.Fatal("les tâches sont la source par défaut")
	}
	if NormalizeSlicingSource("nimportequoi") != SlicingFromTasks {
		t.Fatal("une source inconnue retombe sur les tâches")
	}
}

// Le dossier est cherché par le préfixe de la clé, la casse étant ignorée : un
// changement OpenSpec porte la clé en minuscules là où la branche la porte en
// majuscules.
func TestRechercheDuDossierParPrefixe(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "openspec", "changes", "pe-69-slicing-from-sdd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("préparation : %v", err)
	}

	got, err := FindMacroSpecDir(repo, "openspec", "PE-69")
	if err != nil {
		t.Fatalf("dossier non trouvé : %v", err)
	}
	if got != dir {
		t.Fatalf("attendu %q, obtenu %q", dir, got)
	}
}

// Sous Spec Kit la racine est « specs » et non « openspec/changes » : le cadre
// du projet décide où chercher.
func TestRacineSelonLeCadreSDD(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "specs", "pe-70-speckit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("préparation : %v", err)
	}

	if _, err := FindMacroSpecDir(repo, "speckit", "PE-70"); err != nil {
		t.Fatalf("dossier non trouvé sous specs : %v", err)
	}
	if _, err := FindMacroSpecDir(repo, "openspec", "PE-70"); err == nil {
		t.Fatal("un projet OpenSpec ne doit pas lire la racine de Spec Kit")
	}
}

func TestDossierAbsentNommeLaCause(t *testing.T) {
	repo := t.TempDir()
	_, err := FindMacroSpecDir(repo, "openspec", "PE-69")
	if err == nil {
		t.Fatal("un dossier absent doit être dit")
	}
	if !strings.Contains(err.Error(), "PE-69") || !strings.Contains(err.Error(), "branche") {
		t.Fatalf("le refus doit nommer la macro et suggérer la branche : %v", err)
	}
}

func TestSansDepotConfigure(t *testing.T) {
	_, err := FindMacroSpecDir("", "openspec", "PE-69")
	if err == nil {
		t.Fatal("un projet sans dépôt doit être dit")
	}
	if !strings.Contains(err.Error(), "dépôt") {
		t.Fatalf("le refus doit nommer le dépôt manquant : %v", err)
	}
}

// Deux dossiers pour la même clé : le plus récemment modifié gagne.
func TestDeuxDossiersLePlusRecentGagne(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "openspec", "changes")
	ancien := filepath.Join(root, "pe-69-abandonne")
	recent := filepath.Join(root, "pe-69-repris")
	for _, dir := range []string{ancien, recent} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("préparation : %v", err)
		}
	}
	if err := os.Chtimes(ancien, sddTime0(), sddTime0()); err != nil {
		t.Fatalf("préparation : %v", err)
	}

	got, err := FindMacroSpecDir(repo, "openspec", "pe-69")
	if err != nil {
		t.Fatalf("dossier non trouvé : %v", err)
	}
	if got != recent {
		t.Fatalf("le dossier repris était attendu, obtenu %q", got)
	}
}

func sddProject(t *testing.T, framework string) (*DB, *models.Project, string) {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("base de test : %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	repo := t.TempDir()
	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Platform",
		Slug:         "platform",
		IssueTracker: "local",
		RepoPath:     repo,
		JiraProject:  "PE",
	})
	if err != nil || proj == nil {
		t.Fatalf("projet de test : %v", err)
	}
	updated, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{SpecFramework: &framework})
	if err != nil || updated == nil {
		t.Fatalf("cadre SDD : %v", err)
	}
	return database, updated, repo
}

// seedMacro fait connaître la macro au projet, comme la synchro le fait : le
// bouton n'est offert que sur une macro que la roadmap affiche, donc déjà
// enregistrée.
func seedMacro(t *testing.T, database *DB, projectID, key string) {
	t.Helper()
	title := key
	if _, err := database.saveMacroMetaFull(projectID, key, nil, nil, nil, nil, &title, nil, nil); err != nil {
		t.Fatalf("préparation de la macro : %v", err)
	}
}

func writeSpecDir(t *testing.T, repo, root, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(repo, filepath.FromSlash(root), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("préparation : %v", err)
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatalf("préparation : %v", err)
		}
	}
}

func TestDecoupeDepuisLesGroupesDeTaches(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-440-cloudprober", map[string]string{"tasks.md": tasksFile})
	seedMacro(t, database, proj.ID, "PE-440")

	meta, origin, err := database.TodosFromSDD(proj.ID, "PE-440", SlicingFromTasks)
	if err != nil {
		t.Fatalf("production : %v", err)
	}
	if len(meta.Todos) != 3 {
		t.Fatalf("trois lignes attendues, obtenu %d : %v", len(meta.Todos), meta.Todos)
	}
	if !strings.Contains(origin, "tasks.md") {
		t.Fatalf("l'origine doit nommer le fichier lu, obtenu %q", origin)
	}
	// Rien n'est créé sur le tracker : la découpe reste locale et réversible.
	for _, todo := range meta.Todos {
		if todo.StoryKey != "" {
			t.Fatalf("aucune story ne doit être créée, obtenu %q", todo.StoryKey)
		}
	}
	// Chaque ligne sait d'où elle vient, et par quelle entrée du fichier.
	for _, todo := range meta.Todos {
		if todo.SourceKind != models.MacroTodoFromTasks {
			t.Fatalf("origine attendue %q, obtenu %q", models.MacroTodoFromTasks, todo.SourceKind)
		}
		if todo.SourceEntry == "" {
			t.Fatalf("l'entrée brute devait être conservée sur %q", todo.Text)
		}
	}
}

func TestDecoupeDepuisLesExigences(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-441-roadmap", map[string]string{"spec.md": specFile})
	seedMacro(t, database, proj.ID, "PE-441")

	meta, _, err := database.TodosFromSDD(proj.ID, "PE-441", SlicingFromSpec)
	if err != nil {
		t.Fatalf("production : %v", err)
	}
	if len(meta.Todos) != 2 {
		t.Fatalf("deux lignes attendues, obtenu %d : %v", len(meta.Todos), meta.Todos)
	}
	if meta.Todos[0].SourceKind != models.MacroTodoFromSpec {
		t.Fatalf("origine attendue %q, obtenu %q", models.MacroTodoFromSpec, meta.Todos[0].SourceKind)
	}
}

// La source choisie manque alors que la spécification est là : le message doit
// distinguer ce cas de l'absence de spécification, et proposer l'autre source.
func TestSourceAbsenteDuDossier(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-442-sans-taches", map[string]string{"spec.md": specFile})

	_, _, err := database.TodosFromSDD(proj.ID, "PE-442", SlicingFromTasks)
	if err == nil {
		t.Fatal("l'absence du fichier de la source doit être dite")
	}
	if !strings.Contains(err.Error(), "tasks.md") || !strings.Contains(err.Error(), "l'autre") {
		t.Fatalf("le refus doit nommer le fichier et proposer l'autre source : %v", err)
	}
}

func TestFichierSansRienDeReconnaissable(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-443-vide", map[string]string{"tasks.md": "Du texte sans aucun titre de groupe.\n"})

	_, _, err := database.TodosFromSDD(proj.ID, "PE-443", SlicingFromTasks)
	if err == nil {
		t.Fatal("un fichier sans section doit être dit")
	}
	if !strings.Contains(err.Error(), "section de tâches") {
		t.Fatalf("le refus doit nommer ce qui manque : %v", err)
	}
}

// Le coeur de la promesse : relancer ne défait pas ce qui a été validé, et une
// ligne saisie à la main survit.
func TestLignesValideesConserveesEtLigneManuelleGardee(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-444-merge", map[string]string{"tasks.md": tasksFile})
	seedMacro(t, database, proj.ID, "PE-444")

	first, _, err := database.TodosFromSDD(proj.ID, "PE-444", SlicingFromTasks)
	if err != nil {
		t.Fatalf("première production : %v", err)
	}

	todos := append([]models.MacroTodo{}, first.Todos...)
	todos[1].Done = true
	todos[1].StoryKey = "PE-999"
	keptID := todos[1].ID
	todos = append(todos, models.MacroTodo{Text: "Purger le cache avant la bascule"})
	if _, err := database.SaveMacroMeta(proj.ID, "PE-444", nil, nil, nil, &todos); err != nil {
		t.Fatalf("validation : %v", err)
	}

	second, _, err := database.TodosFromSDD(proj.ID, "PE-444", SlicingFromTasks)
	if err != nil {
		t.Fatalf("seconde production : %v", err)
	}
	if second.Todos[1].ID != keptID || !second.Todos[1].Done || second.Todos[1].StoryKey != "PE-999" {
		t.Fatalf("la ligne validée devait survivre, obtenu %+v", second.Todos[1])
	}
	found := false
	for _, todo := range second.Todos {
		if todo.Text == "Purger le cache avant la bascule" {
			found = true
			// Une ligne que la lecture n'a pas reprise reste saisie à la main.
			if todo.SourceKind != "" {
				t.Fatalf("la ligne manuelle ne doit pas recevoir d'origine, obtenu %q", todo.SourceKind)
			}
		}
	}
	if !found {
		t.Fatal("la ligne saisie à la main a disparu")
	}
}

// L'appariement se fait sur le texte normalisé et non sur l'ordre : insérer un
// groupe en tête ne doit pas décaler les stories déjà rattachées.
func TestInsertionEnTeteNeDecalePasLesStories(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-447-ordre", map[string]string{"tasks.md": tasksFile})
	seedMacro(t, database, proj.ID, "PE-447")

	first, _, err := database.TodosFromSDD(proj.ID, "PE-447", SlicingFromTasks)
	if err != nil {
		t.Fatalf("première production : %v", err)
	}
	todos := append([]models.MacroTodo{}, first.Todos...)
	todos[2].StoryKey = "PE-500"
	cibleTexte := todos[2].Text
	cibleID := todos[2].ID
	if _, err := database.SaveMacroMeta(proj.ID, "PE-447", nil, nil, nil, &todos); err != nil {
		t.Fatalf("validation : %v", err)
	}

	// Un groupe est inséré en tête, et tous les numéros glissent.
	renumbered := "## 1. Un nouveau groupe en tête\n\n- [ ] 1.1 Rien.\n\n" +
		strings.Replace(strings.Replace(strings.Replace(tasksFile,
			"## 1. ", "## 2. ", 1), "## 2. ", "## 3. ", 1), "## 3. ", "## 4. ", 1)
	writeSpecDir(t, repo, "openspec/changes", "pe-447-ordre", map[string]string{"tasks.md": renumbered})

	second, _, err := database.TodosFromSDD(proj.ID, "PE-447", SlicingFromTasks)
	if err != nil {
		t.Fatalf("seconde production : %v", err)
	}
	for _, todo := range second.Todos {
		if todo.Text != cibleTexte {
			continue
		}
		if todo.ID != cibleID || todo.StoryKey != "PE-500" {
			t.Fatalf("la ligne devait garder son identité malgré la renumérotation, obtenu %+v", todo)
		}
		return
	}
	t.Fatalf("la ligne rattachée a disparu après renumérotation : %v", second.Todos)
}

// Les deux sources se cumulent : produire depuis les exigences après les tâches
// ajoute sans écraser, ce qui est ce qui rend le choix de la source utilisable.
func TestLesDeuxSourcesSeCumulent(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	writeSpecDir(t, repo, "openspec/changes", "pe-445-deux", map[string]string{
		"tasks.md": tasksFile,
		"spec.md":  specFile,
	})
	seedMacro(t, database, proj.ID, "PE-445")

	if _, _, err := database.TodosFromSDD(proj.ID, "PE-445", SlicingFromTasks); err != nil {
		t.Fatalf("production depuis les tâches : %v", err)
	}
	meta, _, err := database.TodosFromSDD(proj.ID, "PE-445", SlicingFromSpec)
	if err != nil {
		t.Fatalf("production depuis les exigences : %v", err)
	}
	if len(meta.Todos) != 5 {
		t.Fatalf("trois groupes plus deux exigences attendus, obtenu %d : %v", len(meta.Todos), meta.Todos)
	}
}

// Le repli sur la branche : la spécification n'est pas dans l'arbre de travail,
// mais elle existe sur la branche de la macro. Produire depuis main doit marcher,
// sans quoi le geste ne servirait qu'après la fusion, trop tard pour découper.
func TestRepliSurLaBrancheDeLaMacro(t *testing.T) {
	database, proj, repo := sddProject(t, "openspec")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git absent")
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v : %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("essai\n"), 0o644); err != nil {
		t.Fatalf("préparation : %v", err)
	}
	run("add", "README.md")
	run("commit", "-qm", "premier")

	run("checkout", "-q", "-b", "PE-446-sur-la-branche")
	writeSpecDir(t, repo, "openspec/changes", "pe-446-sur-la-branche", map[string]string{"tasks.md": tasksFile})
	seedMacro(t, database, proj.ID, "PE-446")
	run("add", ".")
	run("commit", "-qm", "spec")
	run("checkout", "-q", "main")

	// L'arbre de travail est revenu sur main : le dossier n'y est plus.
	if _, err := os.Stat(filepath.Join(repo, "openspec", "changes", "pe-446-sur-la-branche")); !os.IsNotExist(err) {
		t.Fatalf("le dossier ne devait plus être dans l'arbre de travail : %v", err)
	}

	meta, origin, err := database.TodosFromSDD(proj.ID, "PE-446", SlicingFromTasks)
	if err != nil {
		t.Fatalf("le repli sur la branche devait marcher : %v", err)
	}
	if len(meta.Todos) != 3 {
		t.Fatalf("trois lignes attendues, obtenu %d", len(meta.Todos))
	}
	if !strings.Contains(origin, "PE-446-sur-la-branche") {
		t.Fatalf("l'origine doit nommer la branche lue, obtenu %q", origin)
	}
}

// sddTime0 rend une date ancienne, pour vieillir un dossier dans un test.
func sddTime0() time.Time {
	return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
}

// seedStory attache un ticket à une macro, comme la synchro le fait.
func seedStory(t *testing.T, database *DB, projectID, macroKey, key, title string) {
	t.Helper()
	task, err := database.CreateTask(models.CreateTaskRequest{
		Title:     title,
		ProjectID: projectID,
		ParentKey: macroKey,
	})
	if err != nil || task == nil {
		t.Fatalf("préparation du ticket : %v", err)
	}
	database.mu.Lock()
	_, err = database.conn.Exec("UPDATE tasks SET key = ? WHERE id = ?", key, task.ID)
	database.mu.Unlock()
	if err != nil {
		t.Fatalf("préparation de la clé : %v", err)
	}
}

// La reprise remonte d'un ticket vers sa ligne, là où « Créer story » descend
// d'une ligne vers un ticket. Chaque ligne produite arrive déjà rattachée.
func TestRepriseDesStoriesExistantes(t *testing.T) {
	database, proj, _ := sddProject(t, "openspec")
	seedMacro(t, database, proj.ID, "PE-500")
	seedStory(t, database, proj.ID, "PE-500", "PE-501", "Poser le champ sur le modèle")
	seedStory(t, database, proj.ID, "PE-500", "PE-502", "Porter la colonne")

	meta, origin, err := database.TodosFromMacroStories(proj.ID, "PE-500")
	if err != nil {
		t.Fatalf("reprise : %v", err)
	}
	if len(meta.Todos) != 2 {
		t.Fatalf("deux lignes attendues, obtenu %d : %v", len(meta.Todos), meta.Todos)
	}
	for _, todo := range meta.Todos {
		if todo.StoryKey == "" {
			t.Errorf("une ligne reprise arrive rattachée, obtenu %+v", todo)
		}
		if todo.SourceKind != models.MacroTodoFromStories {
			t.Errorf("origine attendue %q, obtenu %q", models.MacroTodoFromStories, todo.SourceKind)
		}
	}
	if !strings.Contains(origin, "2") {
		t.Errorf("le compte rendu doit dire combien de lignes, obtenu %q", origin)
	}
}

// La clé identifie, pas le texte : une ligne renommée à la main ne doit pas
// voir son ticket revenir en double à la reprise suivante.
func TestRepriseNeDupliquePasUneStoryDejaRattachee(t *testing.T) {
	database, proj, _ := sddProject(t, "openspec")
	seedMacro(t, database, proj.ID, "PE-510")
	seedStory(t, database, proj.ID, "PE-510", "PE-511", "Titre d'origine")

	first, _, err := database.TodosFromMacroStories(proj.ID, "PE-510")
	if err != nil {
		t.Fatalf("première reprise : %v", err)
	}
	renamed := append([]models.MacroTodo{}, first.Todos...)
	renamed[0].Text = "Énoncé retravaillé à la main"
	if _, err := database.SaveMacroMeta(proj.ID, "PE-510", nil, nil, nil, &renamed); err != nil {
		t.Fatalf("renommage : %v", err)
	}

	second, origin, err := database.TodosFromMacroStories(proj.ID, "PE-510")
	if err != nil {
		t.Fatalf("seconde reprise : %v", err)
	}
	if len(second.Todos) != 1 {
		t.Fatalf("une seule ligne attendue, obtenu %d : %v", len(second.Todos), second.Todos)
	}
	if second.Todos[0].Text != "Énoncé retravaillé à la main" {
		t.Errorf("le texte retravaillé devait survivre, obtenu %q", second.Todos[0].Text)
	}
	// Rien à faire n'est pas une erreur, mais le silence se lirait comme un échec.
	if !strings.Contains(origin, "à jour") {
		t.Errorf("le compte rendu doit dire que la découpe est à jour, obtenu %q", origin)
	}
}

// Les lignes déjà là, saisies ou importées, ne sont pas touchées par la reprise.
func TestRepriseConserveLesLignesExistantes(t *testing.T) {
	database, proj, _ := sddProject(t, "openspec")
	seedMacro(t, database, proj.ID, "PE-520")
	manual := []models.MacroTodo{{Text: "Prévenir l'équipe réseau"}}
	if _, err := database.SaveMacroMeta(proj.ID, "PE-520", nil, nil, nil, &manual); err != nil {
		t.Fatalf("préparation : %v", err)
	}
	seedStory(t, database, proj.ID, "PE-520", "PE-521", "Livrer la sonde")

	meta, _, err := database.TodosFromMacroStories(proj.ID, "PE-520")
	if err != nil {
		t.Fatalf("reprise : %v", err)
	}
	if len(meta.Todos) != 2 {
		t.Fatalf("deux lignes attendues, obtenu %d : %v", len(meta.Todos), meta.Todos)
	}
	if meta.Todos[0].Text != "Prévenir l'équipe réseau" || meta.Todos[0].SourceKind != "" {
		t.Errorf("la ligne manuelle devait rester intacte et en tête, obtenu %+v", meta.Todos[0])
	}
}

func TestRepriseSansTicketLeDit(t *testing.T) {
	database, proj, _ := sddProject(t, "openspec")
	seedMacro(t, database, proj.ID, "PE-530")

	_, _, err := database.TodosFromMacroStories(proj.ID, "PE-530")
	if err == nil {
		t.Fatal("une macro sans ticket doit être dite")
	}
	if !strings.Contains(err.Error(), "PE-530") || !strings.Contains(err.Error(), "reprendre") {
		t.Errorf("le refus doit nommer la macro et ce qu'il n'a pas pu faire : %v", err)
	}
}
