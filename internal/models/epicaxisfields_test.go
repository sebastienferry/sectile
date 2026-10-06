package models

import (
	"reflect"
	"testing"
)

// The fields and options below are synthetic: no test names a field or an
// option of a real site (#680).

func TestDeduceEpicAxisOptionsPriority(t *testing.T) {
	candidate := EpicFieldCandidate{ID: "cf-epic-priority", Kind: EpicFieldSelect, Options: []EpicFieldOption{
		{ID: "o1", Value: "P0"},
		{ID: "o2", Value: " p1 "},
		{ID: "o3", Value: "P2 - Medium"},
		{ID: "o4", Value: "Low"},
		{ID: "o5", Value: "P1"},
	}}
	got := DeduceEpicAxisOptions(EpicAxisPriority, candidate)
	want := map[string]string{"p0": "o1", "p1": "o2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deduced = %v, want %v", got, want)
	}
}

func TestDeduceEpicAxisOptionsFlatQuarter(t *testing.T) {
	candidate := EpicFieldCandidate{ID: "cf-epic-quarter", Kind: EpicFieldSelect, Options: []EpicFieldOption{
		{ID: "a", Value: "2026 - Q4"},
		{ID: "b", Value: "2027-q1"},
		{ID: "c", Value: "2027 Q2"},
		{ID: "d", Value: "Q3"},
		{ID: "e", Value: "Someday"},
		{ID: "f", Value: "2027/Q3"},
	}}
	got := DeduceEpicAxisOptions(EpicAxisQuarter, candidate)
	want := map[string]string{"2026-Q4": "a", "2027-Q1": "b", "2027-Q2": "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deduced = %v, want %v", got, want)
	}
}

func TestDeduceEpicAxisOptionsCascadeQuarter(t *testing.T) {
	candidate := EpicFieldCandidate{ID: "cf-epic-quarter", Kind: EpicFieldCascade, Options: []EpicFieldOption{
		{ID: "y26", Value: "2026", Children: []EpicFieldOption{
			{ID: "q3", Value: "Q3"},
			{ID: "q4", Value: "2026 - Q4"},
			{ID: "bad", Value: "2025 - Q1"},
		}},
		{ID: "later", Value: "Later", Children: []EpicFieldOption{{ID: "lq1", Value: "Q1"}}},
	}}
	got := DeduceEpicAxisOptions(EpicAxisQuarter, candidate)
	want := map[string]string{"2026-Q3": "y26/q3", "2026-Q4": "y26/q4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deduced = %v, want %v", got, want)
	}
}

func TestDeduceEpicAxisOptionsCascadePriorityReadsLeaves(t *testing.T) {
	candidate := EpicFieldCandidate{ID: "cf-epic-priority", Kind: EpicFieldCascade, Options: []EpicFieldOption{
		{ID: "g", Value: "Group", Children: []EpicFieldOption{{ID: "c1", Value: "P1"}}},
	}}
	got := DeduceEpicAxisOptions(EpicAxisPriority, candidate)
	if want := map[string]string{"p1": "g/c1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("deduced = %v, want %v", got, want)
	}
}

func TestFillEpicAxisOptionsFillsHolesOnly(t *testing.T) {
	stored := EpicAxisField{
		ID:      "cf-epic-priority",
		Options: map[string]string{"p0": "kept"},
		Manual:  []string{"p2"},
	}
	got, added := FillEpicAxisOptions(stored, map[string]string{"p0": "other", "p1": "o1", "p2": "o2"})
	if !added {
		t.Fatal("added = false, want true")
	}
	want := map[string]string{"p0": "kept", "p1": "o1"}
	if !reflect.DeepEqual(got.Options, want) {
		t.Fatalf("options = %v, want %v", got.Options, want)
	}
	if !reflect.DeepEqual(stored.Options, map[string]string{"p0": "kept"}) {
		t.Fatalf("the stored map was changed in place: %v", stored.Options)
	}
	if _, again := FillEpicAxisOptions(got, map[string]string{"p1": "o9"}); again {
		t.Fatal("a second fill over a filled line reported an addition")
	}
}

func TestEpicAxisFieldReverseLookup(t *testing.T) {
	field := &EpicAxisField{Options: map[string]string{"2026-Q4": "y/q4", "2027-Q1": "y/q4"}}
	if got := field.ValueOf("y/q4"); got != "2026-Q4" {
		t.Fatalf("ValueOf = %q, want the first value in sorted order", got)
	}
	if got := field.ValueOf("unknown"); got != "" {
		t.Fatalf("ValueOf(unknown) = %q, want empty", got)
	}
	var none *EpicAxisField
	if none.OptionFor("p1") != "" || none.ValueOf("x") != "" || none.IsManual("p1") {
		t.Fatal("a nil field answered something")
	}
}

func TestEpicAxisFieldsIDs(t *testing.T) {
	fields := EpicAxisFields{
		Priority: &EpicAxisField{ID: "cf-a"},
		Quarter:  &EpicAxisField{ID: "cf-a"},
	}
	if got := fields.IDs(); !reflect.DeepEqual(got, []string{"cf-a"}) {
		t.Fatalf("IDs = %v", got)
	}
	if got := (EpicAxisFields{}).IDs(); got != nil {
		t.Fatalf("IDs of no field = %v, want nil", got)
	}
}
