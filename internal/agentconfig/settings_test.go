package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"tasks/internal/testhome"
	"testing"
)

func TestUserSettingsPreserveConnectionAndMigrate(t *testing.T) {
	testhome.Temp(t)
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
	testhome.Temp(t)
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
	testhome.Temp(t)
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
	testhome.Temp(t)
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

// Removing the last specifications folder must reach the file: an emptied map
// is omitted from the JSON, and the file's copy used to survive the write.
func TestSettingsSpecReposClearTheLastEntry(t *testing.T) {
	testhome.Temp(t)
	if err := WriteSettings(Overrides{SpecRepos: map[string]string{"p": "/specs"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSettings(t.TempDir()); err != nil || got.SpecRepos["p"] != "/specs" {
		t.Fatalf("the folder must be stored: %v %v", got.SpecRepos, err)
	}
	if err := WriteSettings(Overrides{SpecRepos: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSettings(t.TempDir()); err != nil || len(got.SpecRepos) != 0 {
		t.Fatalf("the folder must be removed: %v %v", got.SpecRepos, err)
	}
}

// The specification artefacts override (#487) is stored only once chosen, and
// clearing the last one leaves neither an empty map nor a null in the file.
func TestSettingsSpecArtifactsAddNoKeyUntilUsed(t *testing.T) {
	testhome.Temp(t)
	path, _ := SettingsPath()
	hasKey := func() bool {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		_, ok := fields["specArtifacts"]
		return ok
	}
	if err := WriteSettings(Overrides{Projects: map[string]string{"p": "/repo"}}); err != nil {
		t.Fatal(err)
	}
	if hasKey() {
		t.Fatal("a workstation that never overrode the setting must not gain the key")
	}
	if err := WriteSettings(Overrides{SpecArtifacts: map[string]string{"p": "drop"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadSettings(t.TempDir()); err != nil || got.SpecArtifacts["p"] != "drop" {
		t.Fatalf("the override must be stored: %v %v", got.SpecArtifacts, err)
	}
	if err := WriteSettings(Overrides{SpecArtifacts: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if hasKey() {
		t.Fatal("removing the last override must remove the key")
	}
}
