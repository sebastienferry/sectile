package terminal

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Un pas du workflow doit vraiment tourner dans la session, et son code de
// sortie doit remonter : c'est lui qui décide si l'étape a réussi.
func TestRunCommandInSessionCapturesOutputAndExitCode(t *testing.T) {
	requirePosixShell(t)
	m := NewManager()
	defer func() { _ = m.CloseSession("test-run") }()

	res, err := m.RunCommandInSession(context.Background(), "test-run", t.TempDir(), nil,
		"printf 'ligne un\\nligne deux\\n'; (exit 3)", time.Minute)
	if err != nil {
		t.Fatalf("exécution impossible: %v", err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("code de sortie = %d, attendu 3", res.ExitCode)
	}
	if !strings.Contains(res.Output, "ligne un") || !strings.Contains(res.Output, "ligne deux") {
		t.Fatalf("sortie incomplète: %q", res.Output)
	}
	if strings.Contains(res.Output, "__SECTILE_") {
		t.Fatalf("les marqueurs ne doivent pas rester dans la sortie: %q", res.Output)
	}
}

// Le plafond est un plafond de silence : une commande longue mais bavarde ne
// doit pas être coupée, contrairement à l'ancien plafond de cinq minutes.
func TestRunCommandInSessionKeepsTalkativeCommandAlive(t *testing.T) {
	requirePosixShell(t)
	m := NewManager()
	defer func() { _ = m.CloseSession("test-talkative") }()

	res, err := m.RunCommandInSession(context.Background(), "test-talkative", t.TempDir(), nil,
		"for i in 1 2 3 4 5 6; do printf 'tick %s\\n' $i; sleep 1; done", 3*time.Second)
	if err != nil {
		t.Fatalf("exécution impossible: %v", err)
	}
	if res.IdleStopped {
		t.Fatal("une commande qui écrit toutes les secondes ne doit pas être arrêtée pour silence")
	}
	if res.ExitCode != 0 {
		t.Fatalf("code de sortie = %d, attendu 0", res.ExitCode)
	}
	if !strings.Contains(res.Output, "tick 6") {
		t.Fatalf("la commande n'est pas allée au bout: %q", res.Output)
	}
}

// Un agent bloqué sans rien écrire est arrêté sur le plafond de silence, et
// l'appelant doit pouvoir le distinguer d'une fin normale.
func TestRunCommandInSessionStopsOnSilence(t *testing.T) {
	requirePosixShell(t)
	m := NewManager()
	defer func() { _ = m.CloseSession("test-silent") }()

	res, err := m.RunCommandInSession(context.Background(), "test-silent", t.TempDir(), nil,
		"printf 'je commence\\n'; sleep 120", 2*time.Second)
	if err != nil {
		t.Fatalf("exécution impossible: %v", err)
	}
	if !res.IdleStopped {
		t.Fatal("le plafond de silence n'a pas joué")
	}
}

func TestStripTerminalNoise(t *testing.T) {
	raw := "\x1b[?2004h\x1b[32mvert\x1b[0m\r\nchargement 10%\rchargement 90%\rchargement fini\r\n\n\n\nfin\x07\n"
	got := StripTerminalNoise(raw)
	want := "vert\nchargement fini\n\nfin"
	if got != want {
		t.Fatalf("nettoyage inattendu:\n obtenu %q\n attendu %q", got, want)
	}
}

func TestRunCommandInSessionPropagatesEnvVars(t *testing.T) {
	requirePosixShell(t)
	m := NewManager()
	defer func() { _ = m.CloseSession("test-env") }()

	env := map[string]string{
		"SECTILE_TASK_ID":   "test-uuid-123",
		"SECTILE_TASK_KEY":  "#47",
		"TASKACAO_COMPAT":    "legacy-val",
	}

	cmd := `printf "ID=%s KEY=%s LEGACY=%s\n" "$SECTILE_TASK_ID" "$SECTILE_TASK_KEY" "$TASKACAO_COMPAT"`
	res, err := m.RunCommandInSession(context.Background(), "test-env", t.TempDir(), env, cmd, time.Minute)
	if err != nil {
		t.Fatalf("exécution impossible: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("code de sortie = %d, attendu 0", res.ExitCode)
	}
	if !strings.Contains(res.Output, "ID=test-uuid-123") || !strings.Contains(res.Output, "KEY=#47") || !strings.Contains(res.Output, "LEGACY=legacy-val") {
		t.Fatalf("variables d'environnement non propagées: %q", res.Output)
	}
	if strings.Contains(res.Output, "__SECTILE_") {
		t.Fatalf("les marqueurs __SECTILE_ ne doivent pas rester dans la sortie: %q", res.Output)
	}
}
