package models

import (
	"regexp"
	"slices"
	"strings"
)

// EpicAxisFields maps the epic priority and the epic quarter each to one
// custom field of the tracker's epics (#680). A nil axis is written and read
// as labels only, which is every project's behaviour until a person picks a
// field.
//
// Nothing here names a field of a real site: the field, its kind and its
// option map are what a person picked from an epic's edit screen, stored on
// the project.
type EpicAxisFields struct {
	Priority *EpicAxisField `json:"priority,omitempty"`
	Quarter  *EpicAxisField `json:"quarter,omitempty"`
}

// Epic axis field kinds: a single select, or a two-level cascading select.
const (
	EpicFieldSelect  = "select"
	EpicFieldCascade = "cascade"
)

// Epic axes a custom field can carry.
const (
	EpicAxisPriority = "priority"
	EpicAxisQuarter  = "quarter"
)

// EpicAxisField is one custom field mapped to an epic axis.
type EpicAxisField struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Options maps each axis value ("p1", "2026-Q4") to the option it is
	// written as: its id for a select, "parentId/childId" for a cascade.
	Options map[string]string `json:"options,omitempty"`
	// Manual lists the axis values a person set, changed or cleared by hand.
	// No deduction fills or changes them again.
	Manual []string `json:"manual,omitempty"`
}

// EpicFieldCandidate is a closed-list custom field of an epic's edit screen,
// one a person may map an axis to.
type EpicFieldCandidate struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Kind    string            `json:"kind"`
	Options []EpicFieldOption `json:"options"`
}

// EpicFieldOption is one option of a candidate field; a cascade's parent
// options carry their children.
type EpicFieldOption struct {
	ID       string            `json:"id"`
	Value    string            `json:"value"`
	Children []EpicFieldOption `json:"children,omitempty"`
}

// Field returns the field mapped to an axis, nil when none is.
func (f EpicAxisFields) Field(axis string) *EpicAxisField {
	switch axis {
	case EpicAxisPriority:
		return f.Priority
	case EpicAxisQuarter:
		return f.Quarter
	}
	return nil
}

// IDs lists the ids of the mapped fields, the priority's first.
func (f EpicAxisFields) IDs() []string {
	var out []string
	for _, field := range []*EpicAxisField{f.Priority, f.Quarter} {
		if field != nil && field.ID != "" && !slices.Contains(out, field.ID) {
			out = append(out, field.ID)
		}
	}
	return out
}

// OptionFor is the option path an axis value is written as, "" when the map
// has none.
func (f *EpicAxisField) OptionFor(value string) string {
	if f == nil || value == "" {
		return ""
	}
	return f.Options[value]
}

// ValueOf is the axis value an option path stands for, "" when no line of
// the map carries it. Two values sharing an option read as the first in
// sorted order, so the answer does not depend on map iteration.
func (f *EpicAxisField) ValueOf(path string) string {
	if f == nil || path == "" {
		return ""
	}
	found := ""
	for value, option := range f.Options {
		if option == path && (found == "" || value < found) {
			found = value
		}
	}
	return found
}

// IsManual reports whether a person set, changed or cleared the line of a
// value by hand.
func (f *EpicAxisField) IsManual(value string) bool {
	return f != nil && slices.Contains(f.Manual, value)
}

var (
	// epicFieldPriorityPattern is an option labelled exactly a level.
	epicFieldPriorityPattern = regexp.MustCompile(`^p([0-3])$`)
	// epicFieldQuarterPattern reads "2026 - q4", "2026-q4" and "2026 q4".
	epicFieldQuarterPattern = regexp.MustCompile(`^(\d{4})(?:\s*-\s*|\s+)q([1-4])$`)
	// epicFieldBareQuarterPattern is a cascade child naming the quarter only.
	epicFieldBareQuarterPattern = regexp.MustCompile(`^q([1-4])$`)
	// epicFieldYearPattern is a cascade parent naming a year.
	epicFieldYearPattern = regexp.MustCompile(`^\d{4}$`)
)

func foldOptionLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// DeduceEpicAxisOptions matches the options of a candidate field with the
// values of an axis, by generic rules on their labels only: an option
// labelled exactly the level for the priority ("P0" for "p0"), a quarter in
// a recognised format for the quarter ("2026 - Q4", "2026-Q4", "2026 Q4", or
// a year parent with a "Q4" child). When two options match one value, the
// first in the field's order wins. Anything else is left for a person.
func DeduceEpicAxisOptions(axis string, candidate EpicFieldCandidate) map[string]string {
	out := map[string]string{}
	keep := func(value, path string) {
		if _, taken := out[value]; !taken && value != "" {
			out[value] = path
		}
	}
	for _, option := range candidate.Options {
		if candidate.Kind != EpicFieldCascade {
			keep(deduceLeaf(axis, "", option.Value), option.ID)
			continue
		}
		for _, child := range option.Children {
			keep(deduceLeaf(axis, option.Value, child.Value), option.ID+"/"+child.ID)
		}
	}
	return out
}

// deduceLeaf reads one option label as an axis value, "" when it is none.
// parent is the label of a cascade's parent option, "" for a flat field.
func deduceLeaf(axis, parent, label string) string {
	clean := foldOptionLabel(label)
	switch axis {
	case EpicAxisPriority:
		if m := epicFieldPriorityPattern.FindStringSubmatch(clean); m != nil {
			return "p" + m[1]
		}
	case EpicAxisQuarter:
		year := foldOptionLabel(parent)
		if parent != "" && !epicFieldYearPattern.MatchString(year) {
			return ""
		}
		if m := epicFieldQuarterPattern.FindStringSubmatch(clean); m != nil {
			if parent != "" && m[1] != year {
				return ""
			}
			return m[1] + "-Q" + m[2]
		}
		if m := epicFieldBareQuarterPattern.FindStringSubmatch(clean); m != nil && parent != "" {
			return year + "-Q" + m[1]
		}
	}
	return ""
}

// FillEpicAxisOptions adds the deduced lines to a field's map where it has
// none and no person set or cleared the value by hand. It never changes an
// existing line, and says whether it added anything.
func FillEpicAxisOptions(field EpicAxisField, deduced map[string]string) (EpicAxisField, bool) {
	added := false
	for value, path := range deduced {
		if path == "" || field.OptionFor(value) != "" || field.IsManual(value) {
			continue
		}
		if field.Options == nil {
			field.Options = map[string]string{}
		} else if !added {
			field.Options = cloneOptions(field.Options)
		}
		field.Options[value] = path
		added = true
	}
	return field, added
}

func cloneOptions(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
