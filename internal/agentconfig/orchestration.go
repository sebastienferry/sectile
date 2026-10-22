package agentconfig

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const MaxOrchestrationAgents = 20

// OrchestrationAgent selects one local policy for a project's backlog runs.
// IDs remain stable when the name or file changes.
type OrchestrationAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
}

// OrchestrationAgentDefinitions preserves the former single-file setup until
// the project explicitly saves a list. An empty list means no agents.
func (p ProjectSettings) OrchestrationAgentDefinitions() []OrchestrationAgent {
	if p.OrchestrationAgents != nil {
		return slices.Clone(p.OrchestrationAgents)
	}
	file := p.OrchestrationFile
	if file == "" {
		file = "AGENTS.md"
	}
	return []OrchestrationAgent{{ID: "default", Name: "Default agent", File: file}}
}

var orchestrationAgentID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

// NormalizeOrchestrationAgents bounds the local catalogue and makes each
// agent unambiguous. The agent validates files against the mapped checkout.
func NormalizeOrchestrationAgents(agents []OrchestrationAgent) ([]OrchestrationAgent, error) {
	if len(agents) > MaxOrchestrationAgents {
		return nil, fmt.Errorf("a project supports at most %d orchestration agents", MaxOrchestrationAgents)
	}
	result := make([]OrchestrationAgent, 0, len(agents))
	ids, names := map[string]bool{}, map[string]bool{}
	for _, agent := range agents {
		agent.ID, agent.Name, agent.File = strings.TrimSpace(agent.ID), strings.TrimSpace(agent.Name), strings.TrimSpace(agent.File)
		if !orchestrationAgentID.MatchString(agent.ID) || agent.Name == "" || len(agent.Name) > 80 {
			return nil, fmt.Errorf("each agent needs a stable ID and a name of 1 to 80 bytes")
		}
		name := strings.ToLower(agent.Name)
		if ids[agent.ID] || names[name] {
			return nil, fmt.Errorf("agent IDs and names must be unique within a project")
		}
		if agent.File == "" {
			agent.File = "AGENTS.md"
		}
		ids[agent.ID], names[name] = true, true
		result = append(result, agent)
	}
	return result, nil
}

func firstOrchestrationAgents(top, base []OrchestrationAgent) []OrchestrationAgent {
	if top != nil {
		return slices.Clone(top)
	}
	return slices.Clone(base)
}
