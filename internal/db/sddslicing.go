package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/agentprotocol"
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

// TodosFromSDD produces the slicing of a macro from its SDD artefacts, and
// returns the updated macro plus what was read.
//
// The file is read by the requesting user's local agent, in the
// specifications folder of their workstation: the server holds no
// specifications path, and its own disk carries none. Only the read moves;
// the parsing and the merge stay here.
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
func (d *DB) TodosFromSDD(ctx context.Context, userID, projectID, macroKey string, source SlicingSource) (*models.MacroMeta, string, error) {
	projectID = strings.TrimSpace(projectID)
	macroKey = strings.TrimSpace(macroKey)
	if projectID == "" || macroKey == "" {
		return nil, "", fmt.Errorf("projet et clé de macro obligatoires")
	}

	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, "", fmt.Errorf("projet non trouvé")
	}

	var file agentprotocol.MacroSpecFile
	err = d.callAgentContext(ctx, agentprotocol.Operation{UserID: strings.TrimSpace(userID), ProjectID: proj.ID,
		Action: "macro_spec_file", MacroKey: macroKey, Framework: proj.SpecFramework, SpecFile: sddFileName(source)}, &file)
	if err != nil {
		return nil, "", err
	}
	content, origin := file.Content, file.Origin

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
