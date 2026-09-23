package db

import (
	"path/filepath"
	"reflect"
	"testing"

	"tasks/internal/models"
)

// TestNormalizeEnabledViews pins the two properties the sidebar relies on: an
// unknown view never reaches it, and two projects that enabled the same set
// store the same value whatever order the interface sent them in.
func TestNormalizeEnabledViews(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"nil means none", nil, []string{}},
		{"empty means none", []string{}, []string{}},
		{"unknown views are dropped", []string{"triage", "gantt", ""}, []string{"triage"}},
		{"case and spacing are forgiven", []string{" Roadmap ", "TIMELINE"}, []string{"roadmap", "timeline"}},
		{"duplicates collapse", []string{"triage", "triage"}, []string{"triage"}},
		{"order is canonical", []string{"timeline", "triage", "roadmap"}, []string{"triage", "roadmap", "timeline"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := models.NormalizeEnabledViews(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeEnabledViews(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestProjectEnabledViewsRoundTrip is the guard on the setting that decides
// whether Triage, Roadmap and Timeline appear at all. A project created without
// asking for them must come back with none: that default is what keeps the
// three views out of every existing project's sidebar.
func TestProjectEnabledViewsRoundTrip(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Views"})
	if err != nil {
		t.Fatal(err)
	}
	if len(project.EnabledViews) != 0 {
		t.Fatalf("a new project shows %q, want no optional view", project.EnabledViews)
	}

	enabled := []string{"timeline", "triage"}
	updated, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{EnabledViews: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"triage", "timeline"}; !reflect.DeepEqual(updated.EnabledViews, want) {
		t.Fatalf("after enabling, project shows %q, want %q", updated.EnabledViews, want)
	}

	// The list must survive a re-read, not just the write path's own return.
	reread, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"triage", "timeline"}; !reflect.DeepEqual(reread.EnabledViews, want) {
		t.Fatalf("re-read project shows %q, want %q", reread.EnabledViews, want)
	}

	// Turning every view back off is what an empty list means, and it has to be
	// distinguishable from "the caller did not touch the setting".
	none := []string{}
	cleared, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{EnabledViews: &none})
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.EnabledViews) != 0 {
		t.Fatalf("after clearing, project shows %q, want no optional view", cleared.EnabledViews)
	}
}
