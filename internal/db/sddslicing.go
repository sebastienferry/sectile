package db

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"tasks/internal/models"
)

// La découpe d'une macro, produite depuis les artefacts SDD du dépôt.
//
// Deux sources, au choix, parce qu'elles ne disent pas la même chose. Les
// groupes de tasks.md sont ce qu'une spécification écrit pour être découpé : une
// story est un groupe, jamais une ligne, les lignes « - [ ] N.N » étant le
// détail d'exécution. Les exigences de spec.md sont plus proches du sens métier,
// et une exigence n'est pas toujours livrable seule.

// SlicingSource names where the slicing is read from.
type SlicingSource string

const (
	// SlicingFromTasks reads the group headings of tasks.md.
	SlicingFromTasks SlicingSource = models.MacroTodoFromTasks
	// SlicingFromSpec reads the requirements of spec.md, or the prioritised user
	// stories under Spec Kit.
	SlicingFromSpec SlicingSource = models.MacroTodoFromSpec
)

// NormalizeSlicingSource rend la source demandée, celle des tâches par défaut.
//
// Par défaut les tâches, et non les exigences : c'est ce qu'une spécification
// écrit en le groupant pour être découpé, donc celle qui demande le moins
// d'arbitrage à la relecture.
func NormalizeSlicingSource(raw string) SlicingSource {
	if strings.EqualFold(strings.TrimSpace(raw), string(SlicingFromSpec)) {
		return SlicingFromSpec
	}
	return SlicingFromTasks
}

// sddFileName rend le fichier à lire pour une source.
func sddFileName(source SlicingSource) string {
	if source == SlicingFromSpec {
		return "spec.md"
	}
	return "tasks.md"
}

// sddSearchRoots rend les répertoires où chercher le dossier d'une macro, selon
// le cadre SDD du projet.
//
// L'archive d'OpenSpec est écartée : un changement archivé est un chantier
// terminé, et en tirer une découpe reproduirait du travail déjà livré.
func sddSearchRoots(specFramework string) []string {
	if strings.EqualFold(strings.TrimSpace(specFramework), "openspec") {
		// Écrit avec une barre oblique à dessein : cette racine est aussi passée
		// à git sous la forme « <branche>:openspec/changes », et git ne fait
		// jamais correspondre un chemin à barre inverse. Les lectures sur disque
		// passent par filepath.Join, qui normalise le séparateur sous Windows.
		return []string{"openspec/changes"}
	}
	return []string{"specs"}
}

// macroSpecRepoPath returns the checkout to read a project's specifications
// from: its declared specifications repository, else its code repository.
//
// It is the single reader of that choice, so the slicing import, the macro
// worktree and the realignment all land in the same checkout. The agents'
// working directory stays RepoPath whatever this returns.
func macroSpecRepoPath(proj *models.Project) string {
	if proj == nil {
		return ""
	}
	if spec := strings.TrimSpace(proj.SpecRepoPath); spec != "" {
		return spec
	}
	return strings.TrimSpace(proj.RepoPath)
}

// FindMacroSpecDir cherche le dossier de spécification d'une macro dans un
// dépôt, par le préfixe de sa clé.
//
// La correspondance ignore la casse : le dossier d'un changement OpenSpec porte
// la clé en minuscules là où la branche la porte en majuscules. Le slug qui suit
// la clé n'est pas connu de Sectile, d'où la recherche par préfixe plutôt qu'un
// chemin construit.
//
// Deux dossiers pour la même clé, un abandonné et un repris, ne sont pas
// impossibles : le plus récemment modifié gagne, et l'appelant le dit.
func FindMacroSpecDir(repoPath string, specFramework string, macroKey string) (string, error) {
	repoPath = strings.TrimSpace(repoPath)
	key := strings.ToLower(strings.TrimSpace(macroKey))
	if repoPath == "" {
		return "", fmt.Errorf("aucun dépôt configuré pour ce projet : renseignez son chemin dans les options")
	}
	if key == "" {
		return "", fmt.Errorf("clé de macro manquante")
	}

	type candidate struct {
		path    string
		modTime int64
	}
	var found []candidate
	for _, root := range sddSearchRoots(specFramework) {
		entries, err := os.ReadDir(filepath.Join(repoPath, root))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if name != key && !strings.HasPrefix(name, key+"-") {
				continue
			}
			path := filepath.Join(repoPath, root, entry.Name())
			var mod int64
			if info, statErr := entry.Info(); statErr == nil {
				mod = info.ModTime().Unix()
			}
			found = append(found, candidate{path: path, modTime: mod})
		}
	}
	if len(found) == 0 {
		// Le dépôt est nommé, et pas seulement les racines cherchées : la
		// première cause de ce refus est de chercher dans le dépôt de code quand
		// les spécifications vivent ailleurs, et un message qui ne dit que
		// « dans specs » laisse croire à un problème de branche.
		return "", fmt.Errorf("aucun dossier de spécification pour %s dans %s (cherché sous %s) : soit la spécification est sur la branche de la macro, non fusionnée, soit ce dépôt n'est pas celui qui les porte (le « Dépôt des spécifications » se déclare dans les options du projet)",
			strings.ToUpper(macroKey), repoPath, strings.Join(sddSearchRoots(specFramework), ", "))
	}
	sort.Slice(found, func(i, j int) bool { return found[i].modTime > found[j].modTime })
	return found[0].path, nil
}

// readSDDFile lit le fichier d'une source, dans l'arbre de travail puis, à
// défaut, sur la branche de la macro.
//
// Le repli existe parce que la spécification d'une macro est écrite sur sa
// propre branche : produire la découpe depuis main doit marcher avant la fusion,
// sans quoi le geste ne servirait qu'une fois la branche fusionnée, c'est-à-dire
// trop tard pour découper.
func readSDDFile(repoPath string, specFramework string, macroKey string, source SlicingSource) (string, string, error) {
	name := sddFileName(source)

	dir, dirErr := FindMacroSpecDir(repoPath, specFramework, macroKey)
	if dirErr == nil {
		path := filepath.Join(dir, name)
		raw, readErr := os.ReadFile(path)
		if readErr == nil {
			return string(raw), path, nil
		}
		// Le dossier existe mais pas ce fichier : c'est la source choisie qui
		// manque, pas la spécification, et le message doit le distinguer.
		return "", "", fmt.Errorf("%s est absent de %s : cette source n'a rien à lire, essayez l'autre", name, dir)
	}

	// Repli sur la branche de la macro.
	if content, path, err := readSDDFileFromBranch(repoPath, specFramework, macroKey, name); err == nil {
		return content, path, nil
	}
	return "", "", dirErr
}

// readSDDFileFromBranch lit le fichier sur la branche de la macro, sans toucher
// à l'arbre de travail.
//
// La branche est cherchée par le préfixe de la clé, comme le dossier : c'est la
// convention que les skills de spécification appliquent.
func readSDDFileFromBranch(repoPath string, specFramework string, macroKey string, fileName string) (string, string, error) {
	branch, err := findMacroBranch(repoPath, macroKey)
	if err != nil {
		return "", "", err
	}

	key := strings.ToLower(strings.TrimSpace(macroKey))
	for _, root := range sddSearchRoots(specFramework) {
		// Le dossier est listé sur la branche, le slug n'étant pas connu.
		out, listErr := gitOutput(repoPath, "ls-tree", "--name-only", branch+":"+root)
		if listErr != nil {
			continue
		}
		for _, entry := range strings.Split(out, "\n") {
			name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(entry), "/"))
			if name == "" || (name != key && !strings.HasPrefix(name, key+"-")) {
				continue
			}
			path := root + "/" + strings.TrimSuffix(strings.TrimSpace(entry), "/") + "/" + fileName
			content, showErr := gitOutput(repoPath, "show", branch+":"+path)
			if showErr != nil {
				continue
			}
			return content, branch + ":" + path, nil
		}
	}
	return "", "", fmt.Errorf("rien à lire sur la branche %s", branch)
}

// findMacroBranch cherche la branche d'une macro par le préfixe de sa clé.
func findMacroBranch(repoPath string, macroKey string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(macroKey))
	if key == "" {
		return "", fmt.Errorf("clé de macro manquante")
	}
	out, err := gitOutput(repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		// The agent's macro worktree uses the same rule, so both sides name the
		// same branch.
		if models.MacroBranchMatches(ref, key) {
			return ref, nil
		}
	}
	return "", fmt.Errorf("aucune branche pour %s", strings.ToUpper(macroKey))
}

func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ExtractTaskGroups reads the group headings of a tasks.md, in file order.
//
// Un groupe est « ## 1. La liste sur le projet » : le numéro et le point sont
// retirés, le titre reste. Les lignes « - [ ] 1.1 … » ne sont jamais retenues :
// elles sont le détail d'exécution, et la story est le groupe.
func ExtractTaskGroups(tasks string) []string {
	var out []string
	for _, rawLine := range strings.Split(tasks, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		if title == "" {
			continue
		}
		out = append(out, stripLeadingNumber(title))
	}
	return out
}

// ExtractSpecRequirements reads what a spec.md offers as deliverable units.
//
// OpenSpec nomme ses exigences « ### Requirement: … ». Spec Kit écrit des user
// stories priorisées, sous un titre qui les annonce, d'où la seconde passe : les
// lignes d'une liste sous ce titre. Les deux formes sont reconnues sans que le
// projet ait à dire laquelle il emploie, le cadre étant déjà dans sa
// configuration mais le contenu d'un fichier pouvant en différer.
func ExtractSpecRequirements(spec string) []string {
	var out []string
	for _, rawLine := range strings.Split(spec, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "### ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "### "))
		for _, prefix := range []string{"Requirement:", "Exigence:", "Requirement :", "Exigence :"} {
			if strings.HasPrefix(title, prefix) {
				if kept := strings.TrimSpace(strings.TrimPrefix(title, prefix)); kept != "" {
					out = append(out, kept)
				}
				break
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	return extractUserStories(spec)
}

// extractUserStories reads the list items under a user stories heading, which is
// the shape Spec Kit writes.
//
// Une story peut tenir sur plusieurs lignes : les fichiers réels sont enroulés à
// quatre-vingts colonnes, et ne lire que la première ligne rend des énoncés
// coupés en plein milieu, finissant sur une virgule. La suite d'un élément est
// donc recollée, jusqu'à la ligne vide, l'élément suivant ou le titre suivant.
func extractUserStories(spec string) []string {
	var out []string
	inStories := false
	current := ""
	flush := func() {
		if trimmed := strings.TrimSpace(current); trimmed != "" {
			out = append(out, stripBoldMarkers(trimmed))
		}
		current = ""
	}
	for _, rawLine := range strings.Split(spec, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#") {
			flush()
			lower := strings.ToLower(line)
			inStories = strings.Contains(lower, "user stor")
			continue
		}
		if !inStories {
			continue
		}
		if line == "" {
			flush()
			continue
		}
		if item := listItemContent(line); item != "" {
			flush()
			current = item
			continue
		}
		// Ni élément de liste ni ligne vide : c'est la suite de l'élément en
		// cours, quand il y en a un. Hors élément, c'est de la prose entre le
		// titre et la liste, et elle est ignorée.
		if current != "" {
			current += " " + line
		}
	}
	flush()
	return out
}

// listItemContent rend le contenu d'une ligne de liste, ou une chaîne vide quand
// la ligne n'en est pas une.
//
// Les listes numérotées comptent au delà de un : ne reconnaître que « 1. »
// retiendrait la première story et perdrait toutes les suivantes.
func listItemContent(line string) string {
	for _, bullet := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, bullet) {
			return strings.TrimSpace(line[len(bullet):])
		}
	}
	idx := strings.IndexAny(line, ".)")
	if idx <= 0 || idx > 3 {
		return ""
	}
	if strings.Trim(line[:idx], "0123456789") != "" {
		return ""
	}
	return strings.TrimSpace(line[idx+1:])
}

// stripLeadingNumber removes the "1." or "1)" a heading or a list item carries.
func stripLeadingNumber(title string) string {
	trimmed := strings.TrimSpace(title)
	idx := strings.IndexAny(trimmed, ".)")
	if idx <= 0 || idx > 4 {
		return trimmed
	}
	if strings.Trim(trimmed[:idx], "0123456789") != "" {
		return trimmed
	}
	return strings.TrimSpace(trimmed[idx+1:])
}

// stripBoldMarkers removes the emphasis a Spec Kit story title carries.
//
// Les marqueurs sont retirés partout et non seulement aux extrémités : une story
// s'écrit « **Un PM veut X** pour Y », donc la paire fermante est au milieu de la
// ligne, et un simple élagage des bords laisserait les astérisques dans le texte
// de la découpe.
func stripBoldMarkers(text string) string {
	cleaned := strings.ReplaceAll(text, "**", "")
	cleaned = strings.ReplaceAll(cleaned, "__", "")
	return strings.TrimSpace(strings.Trim(cleaned, "*_ "))
}

// macroMetaByKey rend le cadrage enregistré d'une macro, ou une macro vide quand
// elle n'en a pas encore : produire une découpe sur une macro jamais cadrée est
// légitime, et c'est même le cas le plus courant juste après la spécification.
func (d *DB) macroMetaByKey(projectID string, macroKey string) (*models.MacroMeta, error) {
	metas, err := d.GetProjectMacros(projectID)
	if err != nil {
		return nil, err
	}
	for i := range metas {
		if strings.EqualFold(metas[i].Key, macroKey) {
			return &metas[i], nil
		}
	}
	return &models.MacroMeta{ProjectID: projectID, Key: macroKey, Todos: []models.MacroTodo{}}, nil
}

// TodosFromSDD produces the slicing of a macro from the SDD artefacts of the
// project's repository, and returns the updated macro plus what was read.
//
// Les lignes déjà présentes sont conservées : appariées sur leur texte
// normalisé, elles gardent leur identifiant, leur case et leur story, et une
// ligne sans correspondance est conservée. Rien n'est écrit dans le dépôt, rien
// dans le tracker, et aucune story n'est créée.
//
// L'origine réellement lue est rendue, chemin dans l'arbre de travail ou
// « branche:chemin » : la découpe pouvant venir de deux endroits, ne pas dire
// lequel laisserait l'utilisateur deviner pourquoi elle ne correspond pas à ce
// qu'il a sous les yeux.
func (d *DB) TodosFromSDD(projectID string, macroKey string, source SlicingSource) (*models.MacroMeta, string, error) {
	projectID = strings.TrimSpace(projectID)
	macroKey = strings.TrimSpace(macroKey)
	if projectID == "" || macroKey == "" {
		return nil, "", fmt.Errorf("projet et clé de macro obligatoires")
	}

	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, "", fmt.Errorf("projet non trouvé")
	}

	content, origin, err := readSDDFile(macroSpecRepoPath(proj), proj.SpecFramework, macroKey, source)
	if err != nil {
		return nil, "", err
	}

	var titles []string
	if source == SlicingFromSpec {
		titles = ExtractSpecRequirements(content)
	} else {
		titles = ExtractTaskGroups(content)
	}
	entries := SDDEntriesFrom(titles, entryKeyPrefixes(proj))
	if len(entries) == 0 {
		return nil, "", fmt.Errorf("%s ne porte aucune %s : la découpe est laissée telle quelle",
			origin, sourceUnitName(source))
	}

	meta, err := d.macroMetaByKey(projectID, macroKey)
	if err != nil {
		return nil, "", err
	}

	next := mergeSDDEntries(meta.Todos, entries, string(source))
	saved, err := d.SaveMacroMeta(projectID, macroKey, nil, nil, nil, &next)
	if err != nil {
		return nil, "", err
	}
	return saved, origin + describeAttached(entries), nil
}

// sourceUnitName nomme ce que la source cherchait, pour que le refus dise ce qui
// manque plutôt que « rien trouvé ».
func sourceUnitName(source SlicingSource) string {
	if source == SlicingFromSpec {
		return "exigence ni user story"
	}
	return "section de tâches"
}

// TodosFromMacroStories produces the slicing from the stories the macro already
// carries, and returns the updated macro plus what was read.
//
// L'inverse du bouton « Créer story » : celui-ci descend d'une ligne vers un
// ticket, celui-là remonte d'un ticket vers sa ligne. Une macro dont les stories
// ont été créées ailleurs, à la main ou par une synchro, avait une découpe vide
// alors que le travail était déjà découpé, et la retaper pour ensuite rattacher
// chaque ligne était le genre de corvée qui fait abandonner la découpe.
//
// Chaque ligne produite arrive déjà rattachée à sa story : c'est ce qui la
// distingue d'une ligne à faire, et ce qui fait qu'une création en lot la passe
// au lieu d'en produire un doublon.
//
// Une story qu'une ligne porte déjà n'en produit pas une seconde, quel que soit
// son énoncé : c'est la clé qui identifie, pas le texte, sans quoi une ligne
// renommée à la main verrait son ticket revenir en double à la prochaine reprise.
func (d *DB) TodosFromMacroStories(projectID string, macroKey string) (*models.MacroMeta, string, error) {
	projectID = strings.TrimSpace(projectID)
	macroKey = strings.TrimSpace(macroKey)
	if projectID == "" || macroKey == "" {
		return nil, "", fmt.Errorf("projet et clé de macro obligatoires")
	}

	meta, err := d.macroMetaByKey(projectID, macroKey)
	if err != nil {
		return nil, "", err
	}

	attached := map[string]bool{}
	for _, todo := range meta.Todos {
		if key := strings.ToUpper(strings.TrimSpace(todo.StoryKey)); key != "" {
			attached[key] = true
		}
	}

	stories, err := d.macroStories(projectID, macroKey, meta.Title)
	if err != nil {
		return nil, "", err
	}
	if len(stories) == 0 {
		return nil, "", fmt.Errorf("aucun ticket sous %s : il n'y a rien à reprendre", strings.ToUpper(macroKey))
	}

	next := append([]models.MacroTodo{}, meta.Todos...)
	added := 0
	for _, story := range stories {
		if attached[strings.ToUpper(story.Key)] {
			continue
		}
		text := strings.TrimSpace(story.Title)
		if text == "" {
			// Un ticket sans titre garde sa clé pour énoncé : mieux vaut une
			// ligne qui renvoie quelque part qu'une ligne vide, que la
			// sauvegarde écarterait de toute façon.
			text = story.Key
		}
		next = append(next, models.MacroTodo{
			Text:        text,
			StoryKey:    story.Key,
			SourceKind:  models.MacroTodoFromStories,
			SourceEntry: story.Key,
		})
		attached[strings.ToUpper(story.Key)] = true
		added++
	}

	if added == 0 {
		// Rien à faire n'est pas une erreur, mais le silence se lirait comme un
		// échec : le compte rendu dit que la découpe était déjà à jour.
		return meta, fmt.Sprintf("%d ticket(s) déjà repris : la découpe est à jour", len(stories)), nil
	}

	saved, err := d.SaveMacroMeta(projectID, macroKey, nil, nil, nil, &next)
	if err != nil {
		return nil, "", err
	}
	return saved, fmt.Sprintf("%d ligne(s) reprise(s) sur %d ticket(s)", added, len(stories)), nil
}

// macroStory is a child ticket of a macro, reduced to what a slicing line needs.
type macroStory struct {
	Key   string
	Title string
}

// macroStories lists the tickets attached to a macro, in key order.
//
// Le titre de la macro est interrogé en plus de sa clé parce que la synchro
// rattache par l'un ou par l'autre selon le tracker, et une reprise qui n'en
// verrait que la moitié serait pire que pas de reprise du tout.
func (d *DB) macroStories(projectID string, macroKey string, macroTitle string) ([]macroStory, error) {
	d.mu.RLock()
	rows, err := d.conn.Query(`
		SELECT key, title
		FROM tasks
		WHERE project_id = ? AND (parent_key = ? OR parent_title = ?)
		ORDER BY key ASC
	`, projectID, macroKey, macroTitle)
	d.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []macroStory{}
	for rows.Next() {
		var story macroStory
		if err := rows.Scan(&story.Key, &story.Title); err != nil {
			continue
		}
		if strings.TrimSpace(story.Key) == "" {
			continue
		}
		out = append(out, story)
	}
	return out, nil
}
