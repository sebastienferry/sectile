package db

import (
	"errors"
	"fmt"
	"testing"

	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// A queued write refused because the person who asked for it has no token of
// their own records which provider it was refused for, so the web app can
// offer to add that token (#645). Every other outcome records nothing.
func TestFailedTrackerWriteNamesTheMissingCredential(t *testing.T) {
	f := newIsolationFixture(t)
	task := f.remoteTask(t, "Missing token")

	_, act, err := f.d.TransitionTaskStageBy("usr_grace", task.ID, "clarified", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	done := f.finished(t, act.ID)
	if done.Status != string(models.ActivityStatusFailed) || done.CredentialMissing != "github" {
		t.Fatalf("a stage refused for want of a GitHub token must name GitHub: %+v", done)
	}
	listed, err := f.d.GetActivities(f.project.ID, "", "", task.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range listed {
		if a.ID == act.ID {
			found = true
			if a.CredentialMissing != "github" {
				t.Fatalf("the activity list must carry the provider: %+v", a)
			}
		}
	}
	if !found {
		t.Fatalf("activity %s missing from the list", act.ID)
	}

	// A board move goes through the transition operation.
	moved, err := f.d.EnqueueTrackerOp(f.grace, TrackerOp{
		Kind: TrackerOpTransition, ProjectID: f.project.ID, TaskID: task.ID, TaskKey: task.Key, TargetStatus: "closed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done := f.finished(t, moved.ID); done.Status != string(models.ActivityStatusFailed) || done.CredentialMissing != "github" {
		t.Fatalf("a board move refused for want of a GitHub token must fail and name GitHub: %+v", done)
	}

	// The same stage recorded by a person with a token completes and names nothing.
	_, act, err = f.d.TransitionTaskStageBy("usr_ada", task.ID, "specified", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if done := f.finished(t, act.ID); done.Status != string(models.ActivityStatusCompleted) || done.CredentialMissing != "" {
		t.Fatalf("a completed write names no missing credential: %+v", done)
	}

	// A write that lost its author fails, but there is no token to add.
	_, act, err = f.d.TransitionTaskStage(task.ID, "clarified", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if done := f.finished(t, act.ID); done.Status != string(models.ActivityStatusFailed) || done.CredentialMissing != "" {
		t.Fatalf("a write naming nobody is not a missing token: %+v", done)
	}
}

// A batch that wrote no ticket keeps the refusal in its error only when every
// ticket was refused for want of the token: a batch that also failed for
// another reason is not only a missing token.
func TestBatchFailureKeepsTheRefusalOnlyWhenEveryTicketMetIt(t *testing.T) {
	refused := &trackerapi.MissingPersonalCredentialError{Tracker: "gitlab"}
	all := refusalOrFailures("aucun ticket modifié", []string{"#1: refusé", "#2: refusé"}, refused, 2)
	if got := trackerapi.MissingCredentialTracker(all); got != "gitlab" {
		t.Fatalf("every ticket refused must name GitLab, got %q (%v)", got, all)
	}
	mixed := refusalOrFailures("aucun ticket modifié", []string{"#1: refusé", "#2: 500"}, refused, 1)
	if got := trackerapi.MissingCredentialTracker(mixed); got != "" {
		t.Fatalf("a mixed batch must name nothing, got %q", got)
	}
	none := refusalOrFailures("aucun ticket modifié", []string{"#1: 500"}, nil, 0)
	if errors.Is(none, refused) || none.Error() != fmt.Sprintf("aucun ticket modifié : %s", "#1: 500") {
		t.Fatalf("a batch with no refusal keeps its message, got %v", none)
	}
}
