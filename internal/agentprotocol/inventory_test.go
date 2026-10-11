package agentprotocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectInventoryValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"schemaVersion":1}`, `{"schemaVersion":1,"projectIds":null}`, `{"schemaVersion":2,"projectIds":[]}`, `{"schemaVersion":1,"projectIds":[""]}`, `{"schemaVersion":1,"projectIds":[42]}`, `bad`, strings.Repeat(" ", InventoryMaxBytes+1)} {
		if _, err := ParseProjectInventory([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid snapshot of %d bytes", len(raw))
		}
	}
	p, err := ParseProjectInventory([]byte(`{"schemaVersion":1,"projectIds":["b","a","b"]}`))
	if err != nil || len(p.ProjectIDs) != 2 || p.ProjectIDs[0] != "a" {
		t.Fatalf("canonical snapshot = %+v, %v", p, err)
	}
	p, err = ParseProjectInventory([]byte(`{"schemaVersion":1,"projectIds":[]}`))
	if err != nil || p.ProjectIDs == nil || len(p.ProjectIDs) != 0 {
		t.Fatalf("empty withdrawal = %+v, %v", p, err)
	}
	ids := make([]string, InventoryMaxProjects+1)
	for i := range ids {
		ids[i] = "p"
	}
	raw, _ := json.Marshal(ProjectInventory{SchemaVersion: 1, ProjectIDs: ids})
	if _, err := ParseProjectInventory(raw); err == nil {
		t.Fatal("accepted oversized list")
	}
}
