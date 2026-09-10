package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

// Le lancement en session doit résoudre le moteur du projet, pas un moteur par
// défaut : c'est ce qui produisait « binaire agy introuvable » sur une machine
// où seul claude est installé.
func TestInteractiveAgentLaunchUnknownEngineExplainsWhereItLooked(t *testing.T) {
	_, err := InteractiveAgentLaunch(&models.Settings{AIProvider: "agy-qui-nexiste-pas"})
	if err == nil {
		t.Fatal("un moteur inconnu doit être refusé")
	}
	if !strings.Contains(err.Error(), "n'a pas de mode interactif") {
		t.Fatalf("message peu actionnable: %v", err)
	}
}

func TestInteractiveAgentLaunchMissingBinaryNamesTheSearchPath(t *testing.T) {
	_, err := InteractiveAgentLaunch(&models.Settings{AIProvider: "vibe"})
	if err == nil {
		t.Skip("vibe est installé sur cette machine, rien à vérifier ici")
	}
	for _, want := range []string{"PATH", ".local/bin", "homebrew"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("le message doit dire où il a cherché (%s manquant): %v", want, err)
		}
	}
}

// Un moteur personnalisé n'a que son modèle de commande : le binaire est son
// premier mot.
func TestInteractiveAgentLaunchCustomUsesTemplateBinary(t *testing.T) {
	line, err := InteractiveAgentLaunch(&models.Settings{
		AIProvider:        "custom",
		AICommandTemplate: `sh -c "echo {prompt}"`,
	})
	if err != nil {
		t.Fatalf("le premier mot du modèle doit être retenu: %v", err)
	}
	if !strings.Contains(line, "sh") {
		t.Fatalf("binaire attendu sh, obtenu %q", line)
	}
}

func TestSkillCallLineFollowsProjectCommand(t *testing.T) {
	task := &models.Task{Key: "PROJ-238", Title: "Titre\navec saut", Source: "jira"}
	got := SkillCallLineWithCommand("clarify-workitem", task, "jira")
	want := "/clarify-workitem PROJ-238 (Titre avec saut) suivi dans jira"
	if got != want {
		t.Fatalf("ligne inattendue:\n obtenu %q\n attendu %q", got, want)
	}
}

func TestInteractiveAgentLaunchCodex(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "codex")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	line, err := InteractiveAgentLaunch(&models.Settings{AIProvider: " Codex "})
	if err != nil {
		t.Fatal(err)
	}
	if line != shellQuote(bin) {
		t.Fatalf("got %q, want %q", line, shellQuote(bin))
	}
}

func TestSkillCallLineForProvider(t *testing.T) {
	task := &models.Task{Key: "#38", Title: "Codex\r\ncase PTY"}
	for _, tc := range []struct{ provider, command, want string }{
		{"codex", "/clarify-issue", "clarify-issue"},
		{" Codex ", " /clarify-workitem ", "clarify-workitem"},
		{"codex", "clarify-workitem", "clarify-workitem"},
		{"claude", "/clarify-issue", "/clarify-issue"},
		{"gemini", "clarify-workitem", "/clarify-workitem"},
		{"", "clarify-issue", "/clarify-issue"},
	} {
		t.Run(tc.provider+tc.command, func(t *testing.T) {
			want := tc.want + " #38 (Codex case PTY) suivi dans github"
			if got := SkillCallLineForProvider(tc.provider, tc.command, task, "github"); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			if got := SkillCallLineForProvider(tc.provider, tc.command, nil, ""); got != tc.want {
				t.Fatalf("nil task: got %q", got)
			}
		})
	}
	if got := SkillCallLineForProvider("codex", "/clarify-issue", &models.Task{Key: "LOCAL-1"}, "local"); got != "clarify-issue LOCAL-1" {
		t.Fatalf("local context: %q", got)
	}
}
