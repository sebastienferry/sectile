package agentconfig

import (
	"reflect"
	"tasks/internal/testhome"
	"testing"
)

func TestOrchestrationAgentsMigrateAndPreserveExplicitEmptyLists(t *testing.T) {
	testhome.Temp(t)
	legacy := ProjectSettings{OrchestrationFile: "docs/legacy.md"}
	agents := legacy.OrchestrationAgentDefinitions()
	if len(agents) != 1 || agents[0].ID != "default" || agents[0].File != "docs/legacy.md" {
		t.Fatalf("legacy policy lost: %+v", agents)
	}
	if got := (ProjectSettings{}).OrchestrationAgentDefinitions(); len(got) != 1 || got[0].File != "AGENTS.md" {
		t.Fatal(got)
	}
	definitions := []OrchestrationAgent{{ID: "docs", Name: "Docs", File: "docs.md"}, {ID: "tests", Name: "Tests", File: "tests.md"}}
	settings := Settings{}
	settings.SetProject("p", ProjectSettings{OrchestrationAgents: definitions})
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	read, err := ReadSettings(t.TempDir())
	if err != nil || !reflect.DeepEqual(read.Project("p").OrchestrationAgentDefinitions(), definitions) {
		t.Fatalf("named policies lost: %+v %v", read.Project("p"), err)
	}
	settings.SetProject("p", ProjectSettings{OrchestrationAgents: []OrchestrationAgent{}})
	if err := WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	read, err = ReadSettings(t.TempDir())
	if err != nil || read.Project("p").OrchestrationAgents == nil || len(read.Project("p").OrchestrationAgentDefinitions()) != 0 {
		t.Fatal("explicit empty list restored a legacy agent")
	}
	merged := overlay(Settings{ProjectSettings: map[string]ProjectSettings{"p": {OrchestrationAgents: definitions}}}, read)
	if len(merged.Project("p").OrchestrationAgentDefinitions()) != 0 {
		t.Fatal("overlay restored deleted agents")
	}
}

func TestOrchestrationAgentCatalogueValidation(t *testing.T) {
	for _, agents := range [][]OrchestrationAgent{
		{{ID: "same", Name: "One"}, {ID: "same", Name: "Two"}},
		{{ID: "a", Name: "Docs"}, {ID: "b", Name: " docs "}},
		{{ID: "bad id", Name: "Agent"}}, {{ID: "a", Name: " "}},
		make([]OrchestrationAgent, MaxOrchestrationAgents+1),
	} {
		if _, err := NormalizeOrchestrationAgents(agents); err == nil {
			t.Errorf("accepted invalid catalogue: %+v", agents)
		}
	}
	got, err := NormalizeOrchestrationAgents([]OrchestrationAgent{{ID: "docs", Name: " Docs "}})
	if err != nil || got[0].Name != "Docs" || got[0].File != "AGENTS.md" {
		t.Fatalf("normalization: %+v %v", got, err)
	}
}
