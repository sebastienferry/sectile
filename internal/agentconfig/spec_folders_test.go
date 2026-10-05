package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"tasks/internal/testhome"
)

// A workstation that set the single specifications folder before #736 reads it
// back as the Macro folder, and has no Issue folder.
func TestExistingSpecPathReadsAsTheMacroFolder(t *testing.T) {
	testhome.Temp(t)
	path, _ := SettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw := `{"layout": ` + jsonInt(SettingsLayout) + `, "projectSettings": {"p": {"path": "/repo", "specPath": "/specs"}}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.MacroSpecPath("p") != "/specs" || got.IssueSpecPath("p") != "" {
		t.Fatalf("macro %q issue %q", got.MacroSpecPath("p"), got.IssueSpecPath("p"))
	}
}

// The Issue folder is stored under its own key, keeps a section that holds
// nothing else, and leaves the file once emptied.
func TestIssueSpecPathRoundTrip(t *testing.T) {
	testhome.Temp(t)
	if err := WriteSettings(Settings{ProjectSettings: map[string]ProjectSettings{"p": {IssueSpecPath: "/issues"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettings(t.TempDir())
	if err != nil || got.IssueSpecPath("p") != "/issues" || got.MacroSpecPath("p") != "" {
		t.Fatalf("not stored: %+v %v", got.Project("p"), err)
	}
	path, _ := SettingsPath()
	raw, _ := os.ReadFile(path)
	var file struct {
		ProjectSettings map[string]map[string]json.RawMessage `json:"projectSettings"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if _, ok := file.ProjectSettings["p"]["issueSpecPath"]; !ok {
		t.Fatalf("issueSpecPath missing from the file: %s", raw)
	}
	project := got.Project("p")
	project.IssueSpecPath = ""
	got.SetProject("p", project)
	if err := WriteSettings(got); err != nil {
		t.Fatal(err)
	}
	again, err := ReadSettings(t.TempDir())
	if err != nil || len(again.ProjectSettings) != 0 {
		t.Fatalf("an emptied Issue folder kept its section: %+v %v", again.ProjectSettings, err)
	}
}

func TestOverlayMergesBothSpecificationsFolders(t *testing.T) {
	base := Settings{ProjectSettings: map[string]ProjectSettings{"p": {MacroSpecPath: "/macro", IssueSpecPath: "/issue"}}}
	top := Settings{ProjectSettings: map[string]ProjectSettings{"p": {Path: "/repo"}}}
	out := overlay(base, top)
	if out.MacroSpecPath("p") != "/macro" || out.IssueSpecPath("p") != "/issue" {
		t.Fatalf("overlay must fall back to the base: %+v", out.Project("p"))
	}
	top.ProjectSettings["p"] = ProjectSettings{IssueSpecPath: "/other"}
	if out := overlay(base, top); out.IssueSpecPath("p") != "/other" || out.MacroSpecPath("p") != "/macro" {
		t.Fatalf("the top level wins for the Issue folder only: %+v", out.Project("p"))
	}
}

func jsonInt(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}
