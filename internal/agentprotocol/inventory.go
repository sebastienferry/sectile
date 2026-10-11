package agentprotocol

import (
	"encoding/json"
	"errors"
	"slices"
)

const ProjectInventoryType = "project_inventory"
const InventoryVersion = 1
const InventoryMaxProjects = 4096
const InventoryMaxBytes = 1 << 20

// ProjectInventory is a complete snapshot. An empty array withdraws availability.
type ProjectInventory struct {
	SchemaVersion int      `json:"schemaVersion"`
	ProjectIDs    []string `json:"projectIds"`
}

// ParseProjectInventory rejects invalid snapshots without revealing their contents.
func ParseProjectInventory(raw []byte) (ProjectInventory, error) {
	var p ProjectInventory
	if len(raw) > InventoryMaxBytes || json.Unmarshal(raw, &p) != nil || p.SchemaVersion != InventoryVersion || p.ProjectIDs == nil || len(p.ProjectIDs) > InventoryMaxProjects {
		return p, errors.New("invalid project inventory")
	}
	for _, id := range p.ProjectIDs {
		if id == "" {
			return p, errors.New("invalid project inventory")
		}
	}
	slices.Sort(p.ProjectIDs)
	p.ProjectIDs = slices.Compact(p.ProjectIDs)
	return p, nil
}
