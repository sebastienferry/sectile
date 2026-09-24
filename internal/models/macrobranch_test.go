package models

import "testing"

func TestMacroBranchMatches(t *testing.T) {
	cases := []struct {
		ref, key string
		want     bool
	}{
		{"M-7", "M-7", true},
		{"M-7-ux-improvements", "m-7", true},
		{"origin/M-7-ux", "M-7", true},
		{"feat/m-7-ux", "M-7", true},
		{"M-70-other", "M-7", false},
		{"feat/426", "M-7", false},
		{"M-7", "", false},
		{"", "M-7", false},
	}
	for _, c := range cases {
		if got := MacroBranchMatches(c.ref, c.key); got != c.want {
			t.Errorf("MacroBranchMatches(%q, %q) = %v, want %v", c.ref, c.key, got, c.want)
		}
	}
}

func TestMacroBranchName(t *testing.T) {
	cases := []struct {
		key, title, want string
	}{
		{"m-7", "Ux improvements and fixes", "M-7-ux-improvements-and-fixes"},
		{"M-7", "  Déploiement : phase 2 !", "M-7-d-ploiement-phase-2"},
		{"M-7", "A very long macro title that goes on and on", "M-7-a-very-long-macro-title-that-g"},
		{"M-7", "", "M-7"},
		{"M-7", "!!!", "M-7"},
	}
	for _, c := range cases {
		if got := MacroBranchName(c.key, c.title); got != c.want {
			t.Errorf("MacroBranchName(%q, %q) = %q, want %q", c.key, c.title, got, c.want)
		}
	}
}
