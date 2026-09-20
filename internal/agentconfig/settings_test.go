package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUserSettingsPreserveConnectionAndMigrate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".taskflow"), 0700)
	os.WriteFile(filepath.Join(root, ".taskflow", "agent.json"), []byte("{\"projects\":{\"p\":\"/repo\"},\"parallelism\":{\"p\":3}}"), 0600)
	settings, err := ReadSettings(root)
	if err != nil || settings.Projects["p"] != "/repo" {
		t.Fatal(settings, err)
	}
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("{\"server\":\"https://example.test\",\"secret\":\"encrypted\"}"), 0600)
	settings, err = ReadSettings(root)
	if err != nil || settings.Parallelism["p"] != 3 {
		t.Fatal(settings, err)
	}
	settings.Parallelism = nil
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["secret"] != "encrypted" {
		t.Fatal("connection secret lost")
	}
	settings, err = ReadSettings(root)
	if err != nil || len(settings.Parallelism) != 0 {
		t.Fatal("cleared override restored", err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("settings permissions")
	}
}

func TestDisconnectionIsWorkstationOnlyAndSurvivesLegacyFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".taskflow"), 0700)
	os.WriteFile(filepath.Join(root, ".taskflow", "agent.json"), []byte(`{"projects":{"p":"/legacy"},"worktrees":{"p":true},"disconnectedProjects":{"other":true}}`), 0600)
	settings, err := ReadSettings(root)
	if err != nil || settings.Projects["p"] != "/legacy" || len(settings.DisconnectedProjects) != 0 {
		t.Fatal(settings, err)
	}
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"server":"https://example.test","secret":"preserved","custom":42}`), 0600)
	settings.Projects = nil
	settings.Worktrees = nil
	settings.DisconnectedProjects = map[string]bool{"p": true}
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadSettings(root)
	if err != nil || !settings.DisconnectedProjects["p"] || len(settings.Projects) != 0 || len(settings.Worktrees) != 0 {
		t.Fatal(settings, err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["secret"] != "preserved" || fields["custom"] != float64(42) {
		t.Fatal("unrelated settings lost")
	}
	delete(settings.DisconnectedProjects, "p")
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadSettings(root)
	if err != nil || len(settings.DisconnectedProjects) != 0 {
		t.Fatal("stale disconnection", settings, err)
	}
}

func TestSettingsAIProvidersAndModelsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"server":"https://example.test","secret":"saved"}`), 0600)

	settings := Overrides{
		Projects:    map[string]string{"p1": "/path/to/p1"},
		AIProviders: map[string]string{"p1": "claude"},
		AIModels:    map[string]string{"p1": "claude-opus-5"},
	}
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AIProviders["p1"] != "claude" {
		t.Fatalf("expected AIProviders[p1] to be 'claude', got %q", loaded.AIProviders["p1"])
	}
	if loaded.AIModels["p1"] != "claude-opus-5" {
		t.Fatalf("expected AIModels[p1] to be 'claude-opus-5', got %q", loaded.AIModels["p1"])
	}

	// Verify connection fields preserved
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["secret"] != "saved" {
		t.Fatal("connection secret was lost")
	}

	// Now delete the overrides (reset to server default)
	delete(loaded.AIProviders, "p1")
	delete(loaded.AIModels, "p1")
	if err := WriteSettings(loaded); err != nil {
		t.Fatal(err)
	}

	reloaded, err := ReadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.AIProviders) != 0 {
		t.Fatalf("expected AIProviders to be empty, got %v", reloaded.AIProviders)
	}
	if len(reloaded.AIModels) != 0 {
		t.Fatalf("expected AIModels to be empty, got %v", reloaded.AIModels)
	}
}

func TestSettingsTerminalAndTerminalsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	path, _ := SettingsPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"server":"https://example.test","secret":"saved"}`), 0600)

	settings := Overrides{
		Terminal:  "ghostty",
		Terminals: map[string]string{"p1": "iterm", "p2": "terminal"},
	}
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Terminal != "ghostty" {
		t.Fatalf("expected Terminal 'ghostty', got %q", loaded.Terminal)
	}
	if loaded.Terminals["p1"] != "iterm" || loaded.Terminals["p2"] != "terminal" {
		t.Fatalf("expected Terminals round-trip, got %v", loaded.Terminals)
	}

	// Verify connection fields preserved
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["secret"] != "saved" {
		t.Fatal("connection secret was lost")
	}

	// Now delete project overrides
	delete(loaded.Terminals, "p1")
	delete(loaded.Terminals, "p2")
	if err := WriteSettings(loaded); err != nil {
		t.Fatal(err)
	}

	reloaded, err := ReadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Terminals) != 0 {
		t.Fatalf("expected Terminals to be empty, got %v", reloaded.Terminals)
	}
	if reloaded.Terminal != "ghostty" {
		t.Fatalf("expected Terminal 'ghostty', got %q", reloaded.Terminal)
	}
}
