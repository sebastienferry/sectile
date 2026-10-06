package agentconfig

import (
	"fmt"
	"sort"
	"strings"
)

// RetiredProviders are the AI providers Sectile no longer runs (#614). A
// setting naming one is refused when it is saved and dropped when it is read.
var RetiredProviders = map[string]bool{"gemini": true, "cursor": true, "vibe": true}

// retiredProvider reports a provider in RetiredProviders, whatever its case.
func retiredProvider(provider string) bool {
	return RetiredProviders[strings.ToLower(strings.TrimSpace(provider))]
}

// RetiredDrop lists what dropRetiredProviders removed, for the start-up log.
type RetiredDrop struct {
	// Engines are the removed catalogue entries, as "Name [provider]".
	Engines []string
	// ModelLists and MCPConnections are the provider keys removed.
	ModelLists     []string
	MCPConnections []string
	// InitializationProvider is the provider cleared, if any.
	InitializationProvider string
}

// Empty reports a drop that removed nothing.
func (d RetiredDrop) Empty() bool {
	return len(d.Engines) == 0 && len(d.ModelLists) == 0 && len(d.MCPConnections) == 0 && d.InitializationProvider == ""
}

// String names what was dropped, for the agent's log.
func (d RetiredDrop) String() string {
	var parts []string
	if len(d.Engines) > 0 {
		parts = append(parts, "engines: "+strings.Join(d.Engines, ", "))
	}
	if len(d.ModelLists) > 0 {
		parts = append(parts, "model lists: "+strings.Join(d.ModelLists, ", "))
	}
	if len(d.MCPConnections) > 0 {
		parts = append(parts, "MCP connections: "+strings.Join(d.MCPConnections, ", "))
	}
	if d.InitializationProvider != "" {
		parts = append(parts, "initialization provider: "+d.InitializationProvider)
	}
	return strings.Join(parts, "; ")
}

// dropRetiredProviders removes, in memory, every setting naming a retired
// provider, as if the owner had removed it: an engine and the project and task
// choices pointing at it, a model list, an MCP connection choice and the
// initialization provider. A removed default engine gives way to the first
// remaining entry, or to the implicit engine when none remains. It runs after
// convertEngines, so an engine field of #305 naming a retired provider has
// already become a catalogue entry and goes the same way.
func dropRetiredProviders(s *Settings) RetiredDrop {
	var drop RetiredDrop
	kept := make([]Engine, 0, len(s.Engines.Catalogue))
	for _, engine := range s.Engines.Catalogue {
		if retiredProvider(engine.Provider) {
			drop.Engines = append(drop.Engines, fmt.Sprintf("%s [%s]", engine.Name, engine.provider()))
			continue
		}
		kept = append(kept, engine)
	}
	if len(drop.Engines) > 0 {
		s.Engines.Catalogue = kept
		s.pruneEngines()
		if _, ok := s.Engine(s.Seeded.DefaultEngine); !ok {
			s.Seeded.DefaultEngine = ""
		}
		if !s.hasDefaultEngine() {
			if len(s.Engines.Catalogue) > 0 {
				s.Engines.Default = s.Engines.Catalogue[0].ID
			} else {
				s.Engines.Default = s.addImplicitEngine()
			}
		}
	}
	drop.ModelLists = dropRetiredKeys(&s.Defaults.AIProviderModels)
	if w := s.Seeded.DefaultValues; w != nil {
		dropRetiredKeys(&w.AIProviderModels)
	}
	drop.MCPConnections = dropRetiredKeys(&s.MCPConnections)
	if retiredProvider(s.Defaults.InitializationProvider) {
		drop.InitializationProvider = strings.TrimSpace(s.Defaults.InitializationProvider)
		s.Defaults.InitializationProvider = ""
	}
	return drop
}

// dropRetiredKeys deletes the retired provider keys of a map, leaving it nil
// once emptied, and returns them sorted.
func dropRetiredKeys[V any](m *map[string]V) []string {
	var dropped []string
	for provider := range *m {
		if retiredProvider(provider) {
			delete(*m, provider)
			dropped = append(dropped, provider)
		}
	}
	if len(*m) == 0 {
		*m = nil
	}
	sort.Strings(dropped)
	return dropped
}
