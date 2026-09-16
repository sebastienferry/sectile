package tracker

import (
	"context"
	"errors"
	"strings"
	"tasks/internal/models"
	"testing"
)

func TestCapabilitiesDifferPerTracker(t *testing.T) {
	cases := []struct {
		name   string
		writer Writer
		has    []Capability
		lacks  []Capability
	}{
		{
			name:   "github",
			writer: NewGithubWriter(),
			has:    []Capability{CapLabels, CapAssign},
			// Sprint et équipe sont exactement ce que ce tracker n'a pas :
			// c'est la raison d'être de cette interface.
			lacks: []Capability{CapSprint, CapTeam, CapEpic, CapTransition},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, c := range tc.has {
				if !tc.writer.Supports(c) {
					t.Errorf("%s devrait prendre en charge %q", tc.name, c)
				}
			}
			for _, c := range tc.lacks {
				if tc.writer.Supports(c) {
					t.Errorf("%s ne devrait pas prendre en charge %q", tc.name, c)
				}
			}
		})
	}
}

func TestUnsupportedNamesWhatIsMissing(t *testing.T) {
	err := NewGithubWriter().SetSprint(context.Background(), "42", []string{"PROJ-1"})
	if err == nil {
		t.Fatal("un tracker sans sprint doit refuser")
	}
	if !IsUnsupported(err) {
		t.Errorf("le refus doit être reconnaissable comme une capacité absente, pas comme une panne")
	}
	// Le message doit nommer l'opération : « ça n'a pas marché » n'apprend rien à
	// qui vient de cliquer.
	if !strings.Contains(err.Error(), "sprint") {
		t.Errorf("message = %q, il devrait nommer le sprint", err.Error())
	}
}

func TestIsUnsupportedIgnoresOtherErrors(t *testing.T) {
	if IsUnsupported(errors.New("le réseau a coupé")) {
		t.Error("une panne réseau n'est pas une capacité absente")
	}
	if IsUnsupported(nil) {
		t.Error("nil n'est pas un refus")
	}
}

func TestBaseTicketingSystemDefaultRefusals(t *testing.T) {
	base := &BaseTicketingSystem{TrackerName: "dummy"}
	ctx := context.Background()

	if _, err := base.CreateIssue(ctx, CreateIssueRequest{}); !IsUnsupported(err) {
		t.Errorf("CreateIssue should return Unsupported, got %v", err)
	}
	if _, err := base.GetIssue(ctx, GetIssueRequest{}); !IsUnsupported(err) {
		t.Errorf("GetIssue should return Unsupported, got %v", err)
	}
	if err := base.UpdateIssue(ctx, UpdateIssueRequest{}); !IsUnsupported(err) {
		t.Errorf("UpdateIssue should return Unsupported, got %v", err)
	}
	if err := base.DeleteIssue(ctx, DeleteIssueRequest{}); !IsUnsupported(err) {
		t.Errorf("DeleteIssue should return Unsupported, got %v", err)
	}
	if _, err := base.SyncIssues(ctx, SyncRequest{}); !IsUnsupported(err) {
		t.Errorf("SyncIssues should return Unsupported, got %v", err)
	}
	if err := base.AddComment(ctx, AddCommentRequest{}); !IsUnsupported(err) {
		t.Errorf("AddComment should return Unsupported, got %v", err)
	}
	if _, err := base.GetComments(ctx, GetCommentsRequest{}); !IsUnsupported(err) {
		t.Errorf("GetComments should return Unsupported, got %v", err)
	}
}

func TestRegistryRegistrationAndResolution(t *testing.T) {
	reg := NewRegistry()
	local := NewLocalAdapter()
	reg.Register("local", local)

	dummy := &BaseTicketingSystem{TrackerName: "github", Capabilities: []Capability{CapCreate, CapSync}}
	reg.Register("github", dummy)

	// Get by name
	got, ok := reg.Get("github")
	if !ok || got.Name() != "github" {
		t.Errorf("failed to get github tracker: %v, %v", got, ok)
	}

	// Case-insensitive lookup
	if _, ok := reg.Get("GITHUB"); !ok {
		t.Error("registry should be case-insensitive")
	}

	// ForProject resolution
	projGH := &models.Project{IssueTracker: "github"}
	ts, err := reg.ForProject(projGH)
	if err != nil || ts.Name() != "github" {
		t.Errorf("ForProject github failed: %v, %v", ts, err)
	}

	projLocal := &models.Project{IssueTracker: "local"}
	ts, err = reg.ForProject(projLocal)
	if err != nil || ts.Name() != "local" {
		t.Errorf("ForProject local failed: %v, %v", ts, err)
	}

	projUnknown := &models.Project{IssueTracker: "unknown_tracker"}
	if _, err := reg.ForProject(projUnknown); err == nil {
		t.Error("ForProject should fail for unregistered tracker")
	}
}

func TestBaseTicketingSystemFormatTaskID(t *testing.T) {
	base := &BaseTicketingSystem{TrackerName: "dummy"}
	if id := base.FormatTaskID("proj", "KEY-1", "custom-raw-id"); id != "custom-raw-id" {
		t.Errorf("expected custom-raw-id, got %s", id)
	}
	if id := base.FormatTaskID("proj", "KEY-1", ""); id != "KEY-1" {
		t.Errorf("expected KEY-1, got %s", id)
	}
}
