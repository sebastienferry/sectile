package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"tasks/internal/testhome"
)

// The Any repository option and its clones folder (#737) are stored under
// their own keys, keep a section that holds nothing else, and leave the file
// once cleared.
func TestAnyRepositoryRoundTrip(t *testing.T) {
	testhome.Temp(t)
	on := true
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{"p": {AnyRepository: &on}, "q": {ClonesPath: "/clones"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || !got.AnyRepository("p") || got.AnyRepository("q") || got.ClonesPath("q", "/src/a") != "/clones" {
		t.Fatalf("not stored: %+v %v", got.ProjectSettings, err)
	}
	path, _ := SettingsPath()
	raw, _ := os.ReadFile(path)
	var file struct {
		ProjectSettings map[string]map[string]json.RawMessage `json:"projectSettings"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if _, ok := file.ProjectSettings["p"]["anyRepository"]; !ok {
		t.Fatalf("anyRepository missing from the file: %s", raw)
	}
	if _, ok := file.ProjectSettings["q"]["clonesPath"]; !ok {
		t.Fatalf("clonesPath missing from the file: %s", raw)
	}
	p, q := got.Project("p"), got.Project("q")
	p.AnyRepository, q.ClonesPath = nil, ""
	got.SetProject("p", p)
	got.SetProject("q", q)
	if err := WriteSettings(got); err != nil {
		t.Fatal(err)
	}
	again, err := ReadSettings(t.TempDir())
	if err != nil || len(again.ProjectSettings) != 0 {
		t.Fatalf("cleared settings kept their sections: %+v %v", again.ProjectSettings, err)
	}
}

func TestClonesPathDefaultsToTheParentOfTheCheckout(t *testing.T) {
	var settings Settings
	if got := settings.ClonesPath("p", filepath.Join("/src", "a")); got != "/src" {
		t.Errorf("default clones folder = %q", got)
	}
	if got := settings.ClonesPath("p", ""); got != "" {
		t.Errorf("no checkout, no default: %q", got)
	}
}

func TestOverlayStatesAnyRepositoryOff(t *testing.T) {
	on, off := true, false
	base := Settings{ProjectSettings: map[string]ProjectSettings{"p": {AnyRepository: &on, ClonesPath: "/clones"}}}
	top := Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/repo"}}}
	if out := overlay(base, top); !out.AnyRepository("p") || out.ClonesPath("p", "/repo") != "/clones" {
		t.Fatalf("overlay must fall back to the base: %+v", out.Project("p"))
	}
	top.ProjectSettings["p"] = ProjectSettings{AnyRepository: &off}
	if out := overlay(base, top); out.AnyRepository("p") {
		t.Fatalf("an option turned off on top stays off: %+v", out.Project("p"))
	}
}
