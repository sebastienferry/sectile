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
