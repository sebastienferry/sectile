package agentconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitSettingsPathDoesNotTouchGlobalStore(t *testing.T) {
	globalRoot := t.TempDir()
	t.Setenv("SECTILE_DESKTOP_DATA_DIR", globalRoot)
	localRoot := t.TempDir()
	localPath := filepath.Join(localRoot, "settings.json")
	_, err := UpdateSettingsAt(localPath, localRoot, func(settings *Settings) error {
		settings.Repositories = map[string]string{"github.com/example/project": localRoot}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := ReadSettingsAt(localPath, localRoot)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Repositories["github.com/example/project"] != localRoot {
		t.Fatal("explicit settings path lost the repository mapping")
	}
	if _, err := os.Stat(filepath.Join(globalRoot, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("global settings were touched: %v", err)
	}
	claudePath, err := ClaudeSettingsPathAt(localPath, "project")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(claudePath) != filepath.Join(localRoot, "claude") {
		t.Fatalf("Claude settings escaped the local root: %s", claudePath)
	}
}
