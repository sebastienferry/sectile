package db

import (
	"fmt"
	"regexp"
	"strings"

	"tasks/internal/models"
)

// Ce qu'un titre de spécification porte en plus de son énoncé.
//
// Les titres réellement écrits par les équipes ne sont pas des énoncés nus. Un
// tasks.md écrit « ## Group 1 - PE-2021 - Alerts are defined and target Platform
// (spec US2) », et un spec.md « ### User Story 3 - A service meets the full
// ten-column bar (Priority: P2) ». Importés tels quels, ces titres donnent des
// lignes de découpe illisibles, et surtout : la clé qu'ils nomment est celle de
// la story que la ligne est déjà devenue. Créer les tickets depuis une telle
// découpe produirait un doublon par ligne.
//
// D'où les deux gestes faits ici, et nulle part ailleurs : on retire le bruit du
// titre, et on rattache la ligne à la story que son titre nomme.

// SDDEntry is one entry read from a specification: what it delivers, and the
// story key its title already named, when it named one.
type SDDEntry struct {
	Text     string
	StoryKey string
	// Raw est le titre tel que le fichier l'écrit, avant nettoyage. Il est
	// conservé sur la ligne produite : c'est par lui qu'on retrouvera l'entrée à
	// réécrire, Text ayant perdu de quoi la désigner.
	Raw string
}

// entryKeyPattern reconnaît une clé de ticket, « PE-2021 ». Le préfixe n'est pas
// gravé ici : il vient du projet, et c'est ce qui empêche d'inventer une clé.
var entryKeyPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9_]+)-([0-9]+)\b`)

// entryGroupPrefix couvre les formes que Spec Kit et les équipes écrivent devant
// un titre : « Group 1 - », « Groups 7 to 12 - », « User Story 3 - ».
var entryGroupPrefix = regexp.MustCompile(`^(?i:groupe?s?|user stor(?:y|ies)|us)\s+[0-9]+(?:\s+(?:to|à|-)\s*[0-9]+)?\s*[-:]\s*`)

// entryTrailingMarker couvre le renvoi que le titre porte en fin de ligne :
// « (spec US2) », « (Priority: P2) », « (spec FR-001, FR-004) ». C'est une
// traçabilité interne au fichier, sans valeur sur un ticket.
var entryTrailingMarker = regexp.MustCompile(`\s*\((?i:spec|priority|priorité|priorite)\b[^)]*\)\s*$`)

// entryPriorityPrefix couvre le cran que Spec Kit met devant une user story,
// « P0. » ou « P1 - ». C'est la priorité de la story, portée par le ticket et
// non par son titre.
var entryPriorityPrefix = regexp.MustCompile(`^P[0-9]+\s*[.\-:)]\s*`)

// SDDEntriesFrom turns raw titles into entries, stripping the noise and
// attaching the story key each title named.
//
// Les préfixes reconnus sont ceux du projet. Une clé hors de cette liste reste
// dans le texte : une clé qu'on ne sait pas rattacher est un mot du titre et non
// une référence.
func SDDEntriesFrom(titles []string, keyPrefixes []string) []SDDEntry {
	allowed := map[string]bool{}
	for _, prefix := range keyPrefixes {
		if clean := strings.ToUpper(strings.TrimSpace(prefix)); clean != "" {
			allowed[clean] = true
		}
	}
	out := make([]SDDEntry, 0, len(titles))
	for _, title := range titles {
		text, key := splitEntryTitle(title, allowed)
		if text == "" {
			// Un titre qui ne se réduit qu'à une clé garde son titre d'origine :
			// mieux vaut une ligne bavarde qu'une ligne vide.
			text = strings.TrimSpace(title)
		}
		if text == "" {
			continue
		}
		out = append(out, SDDEntry{Text: text, StoryKey: key, Raw: strings.TrimSpace(title)})
	}
	return out
}

// splitEntryTitle rend l'énoncé nettoyé et la clé reconnue.
func splitEntryTitle(title string, allowed map[string]bool) (string, string) {
	text := strings.TrimSpace(title)
	text = entryGroupPrefix.ReplaceAllString(text, "")
	text = entryPriorityPrefix.ReplaceAllString(text, "")
	text = entryTrailingMarker.ReplaceAllString(text, "")

	storyKey := ""
	// Seule la première clé reconnue rattache la ligne : un titre qui en cite
	// plusieurs parle de plusieurs tickets, et choisir la première est aussi
	// arbitraire que n'importe quel autre choix, mais au moins c'est stable.
	text = entryKeyPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := entryKeyPattern.FindStringSubmatch(match)
		if !allowed[strings.ToUpper(parts[1])] {
			return match
		}
		if storyKey == "" {
			storyKey = strings.ToUpper(match)
		}
		return ""
	})

	return trimEntrySeparators(text), storyKey
}

// trimEntrySeparators nettoie ce que le retrait de la clé laisse derrière lui :
// un tiret orphelin, une double espace, une ponctuation en tête.
func trimEntrySeparators(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimLeft(text, " -:·|")
	text = strings.TrimRight(text, " -:·|")
	return strings.TrimSpace(text)
}

// entryKeyPrefixes rend les préfixes de clé qu'un projet sait rattacher.
//
// Sectile n'en connaît qu'un, celui du projet Jira. Taskativ y ajoute les
// projets distants que sa roadmap lit ; ce réglage n'existe pas ici, et une clé
// d'un autre projet restera donc dans le texte de la ligne, ce qui est le refus
// prudent : une clé qu'on ne sait pas rattacher n'est pas une story.
func entryKeyPrefixes(proj *models.Project) []string {
	if proj == nil {
		return nil
	}
	own := strings.ToUpper(strings.TrimSpace(proj.JiraProject))
	if own == "" {
		return nil
	}
	return []string{own}
}

// normalizeTodoText rend la forme sur laquelle deux lignes sont dites égales.
//
// La casse et les espaces ne distinguent pas deux énoncés : un titre réindenté
// ou recapitalisé dans le fichier reste la même ligne de découpe.
func normalizeTodoText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// mergeTodoLines apparie les énoncés lus avec les lignes déjà là.
//
// L'appariement se fait sur le texte normalisé, jamais sur un numéro d'ordre :
// le numéro d'un groupe de tâches se renumérote dès qu'une entrée est insérée au
// milieu, et apparier là-dessus décalerait toutes les stories déjà rattachées.
func mergeTodoLines(current []models.MacroTodo, entries []string) []models.MacroTodo {
	// Un énoncé qui revient deux fois n'est apparié qu'une fois : la seconde
	// occurrence produit une ligne neuve plutôt que de voler l'identifiant.
	existing := map[string][]models.MacroTodo{}
	for _, todo := range current {
		norm := normalizeTodoText(todo.Text)
		existing[norm] = append(existing[norm], todo)
	}

	next := make([]models.MacroTodo, 0, len(entries)+len(current))
	matched := map[string]int{}
	for _, statement := range entries {
		norm := normalizeTodoText(statement)
		used := matched[norm]
		if candidates := existing[norm]; used < len(candidates) {
			matched[norm] = used + 1
			kept := candidates[used]
			// Le texte est repris de la source : c'est elle qui fait foi sur
			// l'énoncé, seule l'identité de la ligne est conservée.
			kept.Text = statement
			next = append(next, kept)
			continue
		}
		next = append(next, models.MacroTodo{Text: statement})
	}

	// Ce que la production n'a pas repris est conservé à la suite, dans l'ordre
	// d'origine : une ligne saisie à la main n'est pas du bruit à balayer, et
	// c'est aussi ce qui rend les deux sources cumulables plutôt qu'exclusives.
	for _, todo := range current {
		norm := normalizeTodoText(todo.Text)
		if matched[norm] > 0 {
			matched[norm]--
			continue
		}
		next = append(next, todo)
	}
	return next
}

// mergeSDDEntries apparie les entrées lues avec les lignes déjà là, puis
// rattache celles dont le titre nommait une story.
//
// Le rattachement ne remplace jamais une story déjà portée par la ligne : une
// ligne dont on a créé le ticket depuis Sectile fait foi sur ce ticket, et le
// fichier de spécification peut nommer un ancien numéro.
func mergeSDDEntries(current []models.MacroTodo, entries []SDDEntry, kind string) []models.MacroTodo {
	texts := make([]string, 0, len(entries))
	for _, entry := range entries {
		texts = append(texts, entry.Text)
	}
	merged := mergeTodoLines(current, texts)

	keyByText := map[string]string{}
	rawByText := map[string]string{}
	for _, entry := range entries {
		if _, already := rawByText[entry.Text]; !already {
			rawByText[entry.Text] = entry.Raw
		}
		if entry.StoryKey == "" {
			continue
		}
		if _, already := keyByText[entry.Text]; !already {
			keyByText[entry.Text] = entry.StoryKey
		}
	}
	for i := range merged {
		// L'origine est posée sur toute ligne que cette lecture reprend, neuve
		// ou appariée : la source fait foi sur l'énoncé, elle fait donc foi sur
		// d'où il vient. Une ligne que la lecture n'a pas reprise garde la
		// sienne, qui peut être vide, et se lit alors comme saisie à la main.
		if raw, imported := rawByText[merged[i].Text]; imported {
			merged[i].SourceKind = kind
			merged[i].SourceEntry = raw
		}
		if merged[i].StoryKey != "" {
			continue
		}
		if key := keyByText[merged[i].Text]; key != "" {
			merged[i].StoryKey = key
		}
	}
	return merged
}

// attachedEntryCount compte les lignes que la lecture vient de rattacher, pour
// que le compte rendu le dise : une ligne arrivée déjà rattachée est une ligne
// dont le ticket existe, et l'utilisateur doit le savoir avant de cocher.
func attachedEntryCount(entries []SDDEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.StoryKey != "" {
			count++
		}
	}
	return count
}

// describeAttached rend le membre de phrase à ajouter à l'origine lue, ou une
// chaîne vide quand rien n'a été rattaché.
func describeAttached(entries []SDDEntry) string {
	count := attachedEntryCount(entries)
	if count == 0 {
		return ""
	}
	if count == 1 {
		return " · 1 ligne déjà rattachée à sa story"
	}
	return fmt.Sprintf(" · %d lignes déjà rattachées à leur story", count)
}
