package models

import (
	"errors"
	"testing"
)

func TestNormalizeProjectLabel(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty means no membership filter", in: "", want: ""},
		{name: "blank is empty", in: "   ", want: ""},
		{name: "plain value is kept verbatim", in: "team-alpha", want: "team-alpha"},
		{name: "case is kept, the tracker is case-sensitive", in: "Team-Alpha", want: "Team-Alpha"},
		{name: "surrounding whitespace is trimmed", in: "  team-alpha\n", want: "team-alpha"},
		{name: "inner space is refused", in: "team alpha", wantErr: true},
		{name: "inner tab is refused", in: "team\talpha", wantErr: true},
		{name: "inner space survives trimming and is refused", in: "  team alpha  ", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeProjectLabel(tc.in)
			if tc.wantErr {
				if !errors.Is(err, ErrProjectLabelWhitespace) {
					t.Fatalf("expected ErrProjectLabelWhitespace, got %v", err)
				}
				if got != "" {
					t.Fatalf("expected no value on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeProjectLabelRefusesWorkflowSpellings(t *testing.T) {
	for _, in := range []string{"#team-alpha", "new", "Implemented", "  finished ", "#reviewed", "closed"} {
		got, err := NormalizeProjectLabel(in)
		if !errors.Is(err, ErrProjectLabelReserved) {
			t.Errorf("%q: expected ErrProjectLabelReserved, got %v", in, err)
		}
		if got != "" {
			t.Errorf("%q: expected no value on error, got %q", in, got)
		}
	}
	// A stage name inside a longer label is an ordinary label.
	if got, err := NormalizeProjectLabel("new-team"); err != nil || got != "new-team" {
		t.Errorf("expected new-team to be accepted, got %q, %v", got, err)
	}
}
