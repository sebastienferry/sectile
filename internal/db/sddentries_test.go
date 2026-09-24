package db

import (
	"testing"

	"tasks/internal/models"
)

func TestSDDEntriesFromStripsGroupPrefixAndAttachesKey(t *testing.T) {
	titles := []string{
		"Group 1 - PE-2021 - Alerts are defined and target Platform (spec US2)",
		"Groups 7 to 12 - One service reaches the ten-column bar (spec US3, US4)",
		"User Story 3 - A service meets the full ten-column bar (Priority: P2)",
	}
	entries := SDDEntriesFrom(titles, []string{"PE"})
	if len(entries) != 3 {
		t.Fatalf("3 entrées attendues, %d obtenues", len(entries))
	}
	if entries[0].Text != "Alerts are defined and target Platform" {
		t.Fatalf("titre nettoyé attendu, obtenu %q", entries[0].Text)
	}
	if entries[0].StoryKey != "PE-2021" {
		t.Fatalf("PE-2021 attendu, obtenu %q", entries[0].StoryKey)
	}
	if entries[1].Text != "One service reaches the ten-column bar" || entries[1].StoryKey != "" {
		t.Fatalf("entrée sans clé attendue, obtenu %+v", entries[1])
	}
	if entries[2].Text != "A service meets the full ten-column bar" {
		t.Fatalf("marqueur de priorité non retiré : %q", entries[2].Text)
	}
	// Le titre brut est conservé tel que le fichier l'écrit : c'est par lui
	// qu'on retrouvera l'entrée, le texte nettoyé ne la désignant plus.
	if entries[0].Raw != titles[0] {
		t.Fatalf("titre brut attendu %q, obtenu %q", titles[0], entries[0].Raw)
	}
}

func TestSDDEntriesFromKeepsUnknownKeyInText(t *testing.T) {
	entries := SDDEntriesFrom([]string{"Group 1 - ZZZ-9 - Something happens"}, []string{"PE"})
	if len(entries) != 1 {
		t.Fatalf("une entrée attendue, %d obtenues", len(entries))
	}
	if entries[0].StoryKey != "" {
		t.Fatalf("aucune clé ne doit être rattachée, obtenu %q", entries[0].StoryKey)
	}
	if entries[0].Text != "ZZZ-9 - Something happens" {
		t.Fatalf("la clé inconnue reste dans le texte, obtenu %q", entries[0].Text)
	}
}

func TestSDDEntriesFromKeepsTitleReducedToNothing(t *testing.T) {
	entries := SDDEntriesFrom([]string{"Group 4 - PE-77"}, []string{"PE"})
	if len(entries) != 1 || entries[0].Text != "Group 4 - PE-77" {
		t.Fatalf("titre d'origine conservé attendu, obtenu %+v", entries)
	}
	if entries[0].StoryKey != "PE-77" {
		t.Fatalf("PE-77 attendu, obtenu %q", entries[0].StoryKey)
	}
}

// Sectile ne connaît qu'un préfixe par projet, celui de son projet Jira. Une clé
// d'un autre projet reste dans le texte, faute de savoir à quoi la rattacher.
func TestEntryKeyPrefixesUsesTheJiraProjectKey(t *testing.T) {
	proj := &models.Project{JiraProject: "pe"}
	entries := SDDEntriesFrom([]string{"Group 2 - PE-14 - Feed the warehouse"}, entryKeyPrefixes(proj))
	if entries[0].StoryKey != "PE-14" {
		t.Fatalf("PE-14 attendu, obtenu %q", entries[0].StoryKey)
	}

	sansCle := SDDEntriesFrom([]string{"Group 2 - DS-14 - Feed the warehouse"}, entryKeyPrefixes(proj))
	if sansCle[0].StoryKey != "" {
		t.Fatalf("une clé d'un autre projet ne doit pas être rattachée, obtenu %q", sansCle[0].StoryKey)
	}

	if got := entryKeyPrefixes(&models.Project{}); len(got) != 0 {
		t.Fatalf("un projet sans clé Jira n'en rattache aucune, obtenu %v", got)
	}
}

func TestMergeSDDEntriesNeverStealsAnExistingStory(t *testing.T) {
	current := []models.MacroTodo{{ID: "a", Text: "Alerts are defined", StoryKey: "PE-9999", Done: true}}
	entries := []SDDEntry{{Text: "Alerts are defined", StoryKey: "PE-2021"}}
	merged := mergeSDDEntries(current, entries, string(SlicingFromTasks))
	if len(merged) != 1 {
		t.Fatalf("une ligne attendue, %d obtenues", len(merged))
	}
	if merged[0].StoryKey != "PE-9999" {
		t.Fatalf("la story déjà portée fait foi, obtenu %q", merged[0].StoryKey)
	}
	if merged[0].ID != "a" || !merged[0].Done {
		t.Fatalf("identité et coche conservées attendues, obtenu %+v", merged[0])
	}
}

func TestMergeSDDEntriesAttachesKeyToNewLine(t *testing.T) {
	merged := mergeSDDEntries(nil, []SDDEntry{
		{Text: "Alerts are defined", StoryKey: "PE-2021", Raw: "Group 1 - PE-2021 - Alerts are defined"},
		{Text: "Nothing named here", Raw: "Group 2 - Nothing named here"},
	}, string(SlicingFromTasks))
	if merged[0].StoryKey != "PE-2021" {
		t.Fatalf("PE-2021 attendu, obtenu %q", merged[0].StoryKey)
	}
	if merged[1].StoryKey != "" {
		t.Fatalf("ligne sans clé attendue, obtenu %q", merged[1].StoryKey)
	}
	// L'origine est posée sur toute ligne que la lecture reprend.
	for i, todo := range merged {
		if todo.SourceKind != models.MacroTodoFromTasks || todo.SourceEntry == "" {
			t.Fatalf("ligne %d sans origine : %+v", i, todo)
		}
	}
}

// Un énoncé qui revient deux fois n'est apparié qu'une fois : la seconde
// occurrence produit une ligne neuve plutôt que de voler l'identifiant.
func TestMergeTodoLinesDoesNotStealAnIdentityTwice(t *testing.T) {
	current := []models.MacroTodo{{ID: "a", Text: "La même ligne"}}
	merged := mergeTodoLines(current, []string{"La même ligne", "La même ligne"})
	if len(merged) != 2 {
		t.Fatalf("deux lignes attendues, %d obtenues", len(merged))
	}
	if merged[0].ID != "a" {
		t.Fatalf("la première occurrence garde l'identité, obtenu %q", merged[0].ID)
	}
	if merged[1].ID != "" {
		t.Fatalf("la seconde occurrence est une ligne neuve, obtenu %q", merged[1].ID)
	}
}

// La casse et les espaces ne distinguent pas deux énoncés : un titre réindenté
// dans le fichier reste la même ligne de découpe.
func TestMergeTodoLinesMatchesDespiteCaseAndSpacing(t *testing.T) {
	current := []models.MacroTodo{{ID: "a", Text: "Porter   la Colonne"}}
	merged := mergeTodoLines(current, []string{"porter la colonne"})
	if len(merged) != 1 || merged[0].ID != "a" {
		t.Fatalf("la ligne devait être appariée, obtenu %+v", merged)
	}
	// Le texte de la source fait foi sur l'énoncé.
	if merged[0].Text != "porter la colonne" {
		t.Fatalf("texte de la source attendu, obtenu %q", merged[0].Text)
	}
}

func TestDescribeAttachedCountsOnlyAttached(t *testing.T) {
	if got := describeAttached([]SDDEntry{{Text: "a"}}); got != "" {
		t.Fatalf("silence attendu quand rien n'est rattaché, obtenu %q", got)
	}
	if got := describeAttached([]SDDEntry{{Text: "a", StoryKey: "PE-1"}}); got != " · 1 ligne déjà rattachée à sa story" {
		t.Fatalf("mention au singulier attendue, obtenu %q", got)
	}
	if got := describeAttached([]SDDEntry{{Text: "a", StoryKey: "PE-1"}, {Text: "b", StoryKey: "PE-2"}}); got == "" {
		t.Fatalf("mention au pluriel attendue")
	}
}

// Sectile n'a qu'un chemin de dépôt par projet, celui des options du projet.
func TestMacroSpecRepoPathUsesTheProjectRepository(t *testing.T) {
	if got := macroSpecRepoPath(&models.Project{RepoPath: " /code "}); got != "/code" {
		t.Fatalf("/code attendu, obtenu %q", got)
	}
	if got := macroSpecRepoPath(nil); got != "" {
		t.Fatalf("chaîne vide attendue pour un projet absent, obtenu %q", got)
	}
}
