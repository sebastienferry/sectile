package db

import (
	"context"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

func TestMacroMetadataQueuedWritesRespectRoadmapAndActor(t *testing.T) {
	for _, test := range []struct {
		name, key     string
		optedIn, bulk bool
		writes        int
	}{
		{"own", "PE-1", false, false, 4},
		{"foreign", "DATA-1", false, false, 0},
		{"opted in", "DATA-1", true, false, 2},
		{"bulk", "DATA-1", true, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := testDB(t)
			database.TrackerRegistry().Register("jira", &tracker.BaseTicketingSystem{TrackerName: "jira", Capabilities: []tracker.Capability{tracker.CapEpic, tracker.CapLabels, tracker.CapUpdate}})
			project, err := database.CreateProject(models.CreateProjectRequest{Name: "Roadmap", IssueTracker: "jira", JiraProject: "PE", RoadmapProjects: []string{"DATA"}, RoadmapAxisWrites: test.optedIn})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := database.SaveMacroMeta(project.ID, test.key, nil, nil, nil, nil); err != nil {
				t.Fatal(err)
			}
			ctx := tracker.WithActingUser(context.Background(), "default")
			if test.bulk {
				ctx = WithBulkMacroEdit(ctx)
			}
			horizon, priority, quarter, readiness := "next", "p1", "2026-Q4", "ready"
			saved, note, err := database.UpdateMacroMetadata(ctx, project.ID, test.key, MacroMetadata{Horizon: &horizon, Priority: &priority, Quarter: &quarter, Readiness: &readiness})
			if err != nil || saved.Priority != priority || saved.Readiness != readiness || note == "" {
				t.Fatalf("metadata: %+v %q %v", saved, note, err)
			}
			activities, err := database.GetProjectActivities(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, activity := range activities {
				if activity.SkillID != "tracker_op" {
					continue
				}
				count++
				if activity.UserID != "default" {
					t.Fatalf("lost queue actor: %+v", activity)
				}
			}
			if count != test.writes {
				t.Fatalf("queued %d writes, want %d", count, test.writes)
			}
			if test.key == "DATA-1" && (saved.LabelsWritable || saved.AxesWritable != (test.optedIn && !test.bulk)) {
				t.Fatalf("roadmap flags: %+v", saved)
			}
		})
	}
}

func TestMacroMetadataReturnsPendingFramingCopy(t *testing.T) {
	database, project, _ := jiraMirrorProject(t)
	previousDelay := todosMirrorDelay
	todosMirrorDelay = time.Hour
	t.Cleanup(func() { todosMirrorDelay = previousDelay })
	if _, err := database.SaveMacroMeta(project.ID, "PE-12", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	framing := "New framing"
	saved, _, err := database.UpdateMacroMetadata(tracker.WithActingUser(context.Background(), "default"), project.ID, "PE-12", MacroMetadata{FramingComment: &framing})
	if err != nil || saved.FramingMirror == nil || saved.FramingMirror.Kind != models.MacroTodosMirrorJiraComment || saved.FramingMirror.UpToDate || saved.FramingMirror.WrittenAt != nil {
		t.Fatalf("copy must be pending: %+v %v", saved, err)
	}
}

func TestMacroMetadataValidatesBeforeWriting(t *testing.T) {
	database := testDB(t)
	macro, err := database.CreateMacro(context.Background(), "default", "Original", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	changed, blank, invalid := "Changed", " ", "invalid"
	for _, edit := range []MacroMetadata{
		{}, {Title: &blank}, {Title: &changed, Horizon: &invalid}, {Title: &changed, Priority: &invalid}, {Title: &changed, Quarter: &invalid}, {Title: &changed, Readiness: &invalid},
	} {
		if _, _, err := database.UpdateMacroMetadata(context.Background(), "default", macro.Key, edit); err == nil {
			t.Fatalf("accepted %+v", edit)
		}
		saved, err := database.GetMacro("default", macro.Key)
		if err != nil || saved.Title != "Original" {
			t.Fatalf("invalid edit mutated title: %+v %v", saved, err)
		}
	}
	if _, _, err := database.UpdateMacroMetadata(context.Background(), "default", "M-404", MacroMetadata{Title: &changed}); err == nil {
		t.Fatal("created unknown macro")
	}
	if _, err := database.GetMacro("default", "M-404"); err == nil {
		t.Fatal("unknown macro was persisted")
	}
}

func TestMacroMetadataUsesTheActingPerson(t *testing.T) {
	fixture := newIsolationFixture(t)
	title := "Updated"
	before := fixture.github.count()
	saved, _, err := fixture.d.UpdateMacroMetadata(fixture.ada, fixture.project.ID, "M-3", MacroMetadata{Title: &title})
	if err != nil || saved.Title != title {
		t.Fatalf("metadata: %+v %v", saved, err)
	}
	wrote := false
	for _, request := range fixture.github.since(before) {
		if len(request) >= 6 && request[:6] == "PATCH " {
			signedOnlyBy(t, []string{request}, "ada-token")
			wrote = true
		}
	}
	if !wrote {
		t.Fatal("metadata did not reach the tracker")
	}
	saved, _, err = fixture.d.UpdateMacroMetadata(fixture.grace, fixture.project.ID, "M-3", MacroMetadata{Title: &title})
	if err == nil || saved == nil || saved.Title != title {
		t.Fatalf("lost partial success: %+v %v", saved, err)
	}
}

func TestMacroMetadataPreservesOmittedFieldsAndTodos(t *testing.T) {
	database := testDB(t)
	title, description, framing, horizon := "Title", "Description", "Framing", "later"
	closed := true
	todos := []models.MacroTodo{{ID: "linked", Text: "Story", StoryKey: "DEFAUL-1"}}
	if _, err := database.UpdateMacro(context.Background(), "default", "M-1", &title, &horizon, &description, &framing, &todos, &closed); err != nil {
		t.Fatal(err)
	}
	empty := ""
	closed = false
	saved, _, err := database.UpdateMacroMetadata(context.Background(), "default", "M-1", MacroMetadata{Description: &empty, Closed: &closed})
	if err != nil || saved.Title != title || saved.FramingComment != framing || saved.Horizon != horizon || saved.Description != "" || saved.Closed || len(saved.Todos) != 1 || saved.Todos[0].StoryKey != "DEFAUL-1" {
		t.Fatalf("partial update: %+v %v", saved, err)
	}
}
