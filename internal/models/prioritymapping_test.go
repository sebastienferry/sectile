package models

import (
	"reflect"
	"strings"
	"testing"
)

// classifyByName knows "Highest" and "Low", and reads anything else by its
// rank, the way the Jira adapter does.
func classifyByName(name string, rank, n int) (Priority, bool) {
	switch name {
	case "Highest":
		return PriorityUrgent, true
	case "Low":
		return PriorityLow, true
	}
	return PriorityLevels[rank*len(PriorityLevels)/n], false
}

func TestMergePriorityMappingDiscoversAScheme(t *testing.T) {
	got := MergePriorityMapping(PriorityMapping{}, []PriorityOption{
		{ID: "1", Name: "Highest"}, {ID: "2", Name: "P2"}, {ID: "3", Name: "P3"}, {ID: "4", Name: "Low"},
	}, classifyByName)
	want := []PriorityMappingOption{
		{ID: "1", Name: "Highest", Level: PriorityUrgent},
		{ID: "2", Name: "P2", Level: PriorityHigh, Guessed: true},
		{ID: "3", Name: "P3", Level: PriorityMedium, Guessed: true},
		{ID: "4", Name: "Low", Level: PriorityLow},
	}
	if !reflect.DeepEqual(got.Options, want) {
		t.Fatalf("options = %+v, want %+v", got.Options, want)
	}
}

func TestMergePriorityMappingKeepsStoredLines(t *testing.T) {
	stored := PriorityMapping{
		Options: []PriorityMappingOption{
			{ID: "1", Name: "Highest", Level: PriorityUrgent},
			{ID: "2", Name: "P2", Level: PriorityMedium, Manual: true},
			{ID: "3", Name: "P3", Level: PriorityMedium, Guessed: true},
			{ID: "9", Name: "Gone", Level: PriorityLow, Manual: true},
		},
		Preferred: map[Priority]string{PriorityMedium: "2", PriorityLow: "9"},
	}
	got := MergePriorityMapping(stored, []PriorityOption{
		{ID: "1", Name: "Highest"}, {ID: "2", Name: "P2 renamed"}, {ID: "3", Name: "P3"}, {ID: "5", Name: "P5"}, {ID: "4", Name: "Low"},
	}, classifyByName)
	want := []PriorityMappingOption{
		{ID: "1", Name: "Highest", Level: PriorityUrgent},
		// A hand-set line keeps its level whatever the classifier says; only
		// its name follows the tracker.
		{ID: "2", Name: "P2 renamed", Level: PriorityMedium, Manual: true},
		// A stored guess stays a guess at its stored level.
		{ID: "3", Name: "P3", Level: PriorityMedium, Guessed: true},
		{ID: "5", Name: "P5", Level: PriorityMedium, Guessed: true},
		{ID: "4", Name: "Low", Level: PriorityLow},
	}
	if !reflect.DeepEqual(got.Options, want) {
		t.Fatalf("options = %+v, want %+v", got.Options, want)
	}
	// The preferred choice naming the option gone from the scheme goes with it.
	if !reflect.DeepEqual(got.Preferred, map[Priority]string{PriorityMedium: "2"}) {
		t.Fatalf("preferred = %v", got.Preferred)
	}
}

func TestMergePriorityMappingIgnoresDuplicateAndBlankIDs(t *testing.T) {
	got := MergePriorityMapping(PriorityMapping{}, []PriorityOption{
		{ID: "1", Name: "Highest"}, {ID: "", Name: "Nameless"}, {ID: "1", Name: "Highest again"},
	}, classifyByName)
	if len(got.Options) != 1 || got.Options[0].Name != "Highest" {
		t.Fatalf("options = %+v", got.Options)
	}
}

func TestPriorityMappingWritable(t *testing.T) {
	m := PriorityMapping{Options: []PriorityMappingOption{
		{ID: "1", Level: PriorityUrgent},
		{ID: "2", Level: PriorityHigh, Guessed: true},
		{ID: "3", Level: PriorityMedium},
		{ID: "4", Level: PriorityMedium, Manual: true},
	}}
	cases := map[Priority]bool{PriorityUrgent: true, PriorityHigh: false, PriorityMedium: true, PriorityLow: false}
	for level, want := range cases {
		if got := m.Writable(level); got != want {
			t.Errorf("Writable(%s) = %v, want %v", level, got, want)
		}
	}
	if got := m.WritableLevels(); !reflect.DeepEqual(got, []Priority{PriorityUrgent, PriorityMedium}) {
		t.Errorf("WritableLevels = %v", got)
	}
	if !(PriorityMapping{}).Empty() || m.Empty() {
		t.Error("Empty answers the wrong way")
	}
	if (PriorityMapping{}).WritableLevels() != nil {
		t.Error("an empty mapping has no writable level")
	}
}

func TestPriorityMappingOptionFor(t *testing.T) {
	m := PriorityMapping{Options: []PriorityMappingOption{
		{ID: "1", Level: PriorityHigh},
		{ID: "2", Level: PriorityHigh, Manual: true},
		{ID: "3", Level: PriorityHigh, Guessed: true},
		{ID: "4", Level: PriorityLow, Guessed: true},
	}}
	cases := []struct {
		name      string
		preferred map[Priority]string
		level     Priority
		offered   []string
		want      string
		ok        bool
	}{
		{"most urgent sure option by default", nil, PriorityHigh, nil, "1", true},
		{"preferred sure option", map[Priority]string{PriorityHigh: "2"}, PriorityHigh, nil, "2", true},
		{"preferred guess is ignored", map[Priority]string{PriorityHigh: "3"}, PriorityHigh, nil, "1", true},
		{"preferred option off the screen", map[Priority]string{PriorityHigh: "2"}, PriorityHigh, []string{"1", "3"}, "1", true},
		{"screen offering only the guess", nil, PriorityHigh, []string{"3"}, "", false},
		{"level with a guess only", nil, PriorityLow, nil, "", false},
		{"level with no line", nil, PriorityUrgent, nil, "", false},
	}
	for _, c := range cases {
		m.Preferred = c.preferred
		got, ok := m.OptionFor(c.level, c.offered)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: OptionFor = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestGuessedPriorityErrorNamesTheAcceptedLevels(t *testing.T) {
	err := &GuessedPriorityError{Level: PriorityHigh, Writable: []Priority{PriorityUrgent, PriorityMedium}}
	want := `Priority "high" is not mapped with certainty to a Jira priority of this project. Accepted priorities: urgent, medium. Confirm the mapping in the Tracker tab of the project settings.`
	if err.Error() != want {
		t.Fatalf("message = %q", err.Error())
	}
	none := &GuessedPriorityError{Level: PriorityLow}
	if !strings.Contains(none.Error(), "Accepted priorities: none.") {
		t.Fatalf("message = %q", none.Error())
	}
}
