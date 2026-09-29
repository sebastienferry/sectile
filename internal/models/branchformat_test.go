package models

import (
	"strings"
	"testing"
)

func TestTaskBranchName(t *testing.T) {
	const title = "Allow the user to choose the format"
	cases := []struct {
		format, key, title, want string
	}{
		// The default renders what the agent rendered before formats existed.
		{"", "#621", title, "feat/621"},
		{"", "AUC-1234", title, "feat/auc-1234"},
		{"   ", "AUC-1234", title, "feat/auc-1234"},
		{"", "GL#12", "", "feat/gl-12"},
		{"", "a_b.c", "", "feat/a-b-c"},
		{DefaultBranchNameFormat, "AUC-1234", title, "feat/auc-1234"},
		{"{key}", "AUC-1234", title, "AUC-1234"},
		{"{key}", "#621", title, "621"},
		{"{key_lower}", "AUC-1234", title, "auc-1234"},
		{"feat/{key}-{title}", "AUC-1234", title, "feat/AUC-1234-allow-the-user-to-choose-the-f"},
		{"feat/{key}-{title}", "AUC-1234", "Déploiement : phase 2 !", "feat/AUC-1234-d-ploiement-phase-2"},
		// An empty title drops the separator left dangling next to it.
		{"feat/{key}-{title}", "AUC-1234", "", "feat/AUC-1234"},
		{"feat/{key}-{title}", "AUC-1234", "!!!", "feat/AUC-1234"},
		{"{title}-{key}", "AUC-1234", "", "AUC-1234"},
		{"feat/{title}/{key}", "AUC-1234", "", "feat/AUC-1234"},
		{"{key}/{title}", "AUC-1234", "", "AUC-1234"},
		{"{key}/{title}", "AUC-1234", "Fix", "AUC-1234/fix"},
	}
	for _, c := range cases {
		got, err := TaskBranchName(c.format, c.key, c.title)
		if err != nil || got != c.want {
			t.Errorf("TaskBranchName(%q, %q, %q) = %q, %v, want %q", c.format, c.key, c.title, got, err, c.want)
		}
	}
}

func TestTaskBranchNameRefusesWhatCannotRender(t *testing.T) {
	for _, c := range []struct{ format, key string }{
		{"", "#"},
		{"", "---"},
		{"feat/{id}", "AUC-1"},
		{"feat/{key", "AUC-1"},
		{"feat/key}", "AUC-1"},
		{"feat/{{key}}", "AUC-1"},
	} {
		if got, err := TaskBranchName(c.format, c.key, "Title"); err == nil {
			t.Errorf("TaskBranchName(%q, %q) = %q, want an error", c.format, c.key, got)
		}
	}
}

func TestValidateBranchNameFormat(t *testing.T) {
	for _, format := range []string{"", "   ", DefaultBranchNameFormat, "{key}", "feat/{key}-{title}", "users/me/{key_lower}_{title}"} {
		if err := ValidateBranchNameFormat(format); err != nil {
			t.Errorf("ValidateBranchNameFormat(%q) = %v, want nil", format, err)
		}
	}
	cases := []struct{ format, message string }{
		{"feat/{title}", "{key} ou {key_lower}"},
		{"main", "{key} ou {key_lower}"},
		{"feat/{id}-{key}", "placeholder inconnu {id}"},
		{"feat/{key", "accolade non fermée"},
		{"feat/{key}}", "accolade fermante"},
		{"feat//{key}", "pas un nom de branche Git"},
		{"{key}.lock", "pas un nom de branche Git"},
		{"feat {key}", "pas un nom de branche Git"},
		{"-{key}", "pas un nom de branche Git"},
		{"feat/{key}/", "pas un nom de branche Git"},
		{"feat/.{key}", "pas un nom de branche Git"},
		{"feat/{key}~1", "pas un nom de branche Git"},
	}
	for _, c := range cases {
		err := ValidateBranchNameFormat(c.format)
		if err == nil || !strings.Contains(err.Error(), c.message) {
			t.Errorf("ValidateBranchNameFormat(%q) = %v, want an error containing %q", c.format, err, c.message)
		}
	}
}

func TestValidBranchName(t *testing.T) {
	for _, name := range []string{"feat/621", "AUC-1234", "users/me/a_b.c", "v1.2"} {
		if !ValidBranchName(name) {
			t.Errorf("ValidBranchName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "@", "HEAD", "-x", "/x", "x/", "x.", "a..b", "a@{b", "a//b", "a b", "a~b", "a^b", "a:b", "a?b", "a*b", "a[b", "a\\b", "a\tb", ".a", "a/.b", "a.lock", "a/b.lock/c"} {
		if ValidBranchName(name) {
			t.Errorf("ValidBranchName(%q) = true, want false", name)
		}
	}
}
