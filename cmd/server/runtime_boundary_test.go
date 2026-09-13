package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRuntimeDependencyBoundaries(t *testing.T) {
	cases := []struct {
		command   string
		forbidden []string
	}{
		{"../agent", []string{"tasks/internal/db", "tasks/internal/handlers", "tasks/internal/webui", "modernc.org/sqlite"}},
		{"../server", []string{"tasks/internal/runner", "tasks/internal/workspace", "tasks/internal/terminal"}},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			out, err := exec.Command("go", "list", "-deps", tc.command).CombinedOutput()
			if err != nil {
				t.Fatalf("dependency graph: %s (%v)", out, err)
			}
			packages := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, pkg := range packages {
				for _, forbidden := range tc.forbidden {
					if pkg == forbidden {
						t.Errorf("%s imports %s", tc.command, pkg)
					}
				}
			}
		})
	}
}
