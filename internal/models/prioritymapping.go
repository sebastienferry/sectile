package models

import (
	"fmt"
	"strings"
)

// PriorityMapping is how the options of a tracker's ticket priority scheme map
// to Sectile's four levels (#679). It is empty until the first discovery, and
// an empty mapping leaves every priority write as it was before it existed.
type PriorityMapping struct {
	// Options are the scheme's options in its own order, most urgent first.
	Options []PriorityMappingOption `json:"options,omitempty"`
	// Preferred names, per level, the option a write sends when several sure
	// options carry that level. Absent means the most urgent of them.
	Preferred map[Priority]string `json:"preferred,omitempty"`
}

// PriorityMappingOption is one option of the scheme and the level it maps to.
// A guessed line was matched by its rank only; a manual one was set by a
// person, and discovery never changes it.
type PriorityMappingOption struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Level   Priority `json:"level"`
	Guessed bool     `json:"guessed,omitempty"`
	Manual  bool     `json:"manual,omitempty"`
}

// PriorityOption is one option of a tracker's priority scheme, as the tracker
// lists it.
type PriorityOption struct {
	ID   string
	Name string
}

// PriorityLevels are Sectile's priority levels, most urgent first.
var PriorityLevels = []Priority{PriorityUrgent, PriorityHigh, PriorityMedium, PriorityLow}

// IsPriorityLevel reports whether p is one of Sectile's four levels.
func IsPriorityLevel(p Priority) bool {
	for _, level := range PriorityLevels {
		if p == level {
			return true
		}
	}
	return false
}

// Empty reports whether no discovery has filled the mapping yet.
func (m PriorityMapping) Empty() bool { return len(m.Options) == 0 }

// Writable reports whether a sure line carries the level, which is what lets
// a write send it without guessing.
func (m PriorityMapping) Writable(level Priority) bool {
	_, ok := m.OptionFor(level, nil)
	return ok
}

// WritableLevels lists the levels a write can send, most urgent first.
func (m PriorityMapping) WritableLevels() []Priority {
	var levels []Priority
	for _, level := range PriorityLevels {
		if m.Writable(level) {
			levels = append(levels, level)
		}
	}
	return levels
}

// OptionFor picks the option a write of the level sends: the preferred one
// when it is sure and offered, else the most urgent sure offered option of the
// level. offered restricts the choice to the option ids a screen accepts; nil
// means every option.
func (m PriorityMapping) OptionFor(level Priority, offered []string) (string, bool) {
	accepts := func(id string) bool {
		if offered == nil {
			return true
		}
		for _, o := range offered {
			if o == id {
				return true
			}
		}
		return false
	}
	sure := func(o PriorityMappingOption) bool {
		return o.Level == level && !o.Guessed && accepts(o.ID)
	}
	if preferred := m.Preferred[level]; preferred != "" {
		for _, o := range m.Options {
			if o.ID == preferred && sure(o) {
				return o.ID, true
			}
		}
	}
	for _, o := range m.Options {
		if sure(o) {
			return o.ID, true
		}
	}
	return "", false
}

// MergePriorityMapping folds a freshly read scheme into the stored mapping.
// A line already stored keeps its level and its confidence, so neither a
// person's choice nor a confirmed guess is ever undone; only its name follows
// the tracker. An option the mapping does not carry yet is classified, sure
// when classify knows its name. An option gone from the scheme loses its line,
// and a preferred choice naming it is dropped. The result follows the scheme's
// order.
func MergePriorityMapping(stored PriorityMapping, scheme []PriorityOption, classify func(name string, rank, n int) (Priority, bool)) PriorityMapping {
	known := make(map[string]PriorityMappingOption, len(stored.Options))
	for _, o := range stored.Options {
		known[o.ID] = o
	}
	merged := PriorityMapping{}
	present := make(map[string]bool, len(scheme))
	for rank, option := range scheme {
		if option.ID == "" || present[option.ID] {
			continue
		}
		present[option.ID] = true
		if line, ok := known[option.ID]; ok {
			line.Name = option.Name
			merged.Options = append(merged.Options, line)
			continue
		}
		level, sure := classify(option.Name, rank, len(scheme))
		merged.Options = append(merged.Options, PriorityMappingOption{
			ID:      option.ID,
			Name:    option.Name,
			Level:   level,
			Guessed: !sure,
		})
	}
	for level, id := range stored.Preferred {
		if present[id] {
			if merged.Preferred == nil {
				merged.Preferred = map[Priority]string{}
			}
			merged.Preferred[level] = id
		}
	}
	return merged
}

// GuessedPriorityError refuses a priority write whose level no sure line of
// the project's mapping carries (#679): sending it would mean sending a guess.
type GuessedPriorityError struct {
	Level    Priority
	Writable []Priority
}

func (e *GuessedPriorityError) Error() string {
	accepted := "none"
	if len(e.Writable) > 0 {
		names := make([]string, len(e.Writable))
		for i, level := range e.Writable {
			names[i] = string(level)
		}
		accepted = strings.Join(names, ", ")
	}
	return fmt.Sprintf("Priority %q is not mapped with certainty to a Jira priority of this project. Accepted priorities: %s. Confirm the mapping in the Tracker tab of the project settings.", e.Level, accepted)
}
