package db

import (
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// A tracker's synchronisation names no project and no ticket: it is shown by
// every project selecting the tracker, a labelled one included, and by no
// other (#741).
func TestATrackerSyncActivityIsListedByTheProjectsSelectingItsTracker(t *testing.T) {
	d := testDB(t)
	// A tracker that cannot sync fails the job at once, without the network.
	d.TrackerRegistry().Register("jira", &tracker.BaseTicketingSystem{TrackerName: "jira"})
	gode := jiraSpace(t, d, "GODE")
	other := jiraSpace(t, d, "OPS")
	labelled, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery", Trackers: []models.ProjectTracker{{TrackerID: gode.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := d.CreateProject(models.CreateProjectRequest{Name: "Ops", Trackers: []models.ProjectTracker{{TrackerID: other.ID}}})
	if err != nil {
		t.Fatal(err)
	}

	act, err := d.EnqueueTrackerSyncWith("admin", gode.ID, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}

	listed := func(projectRef string) bool {
		t.Helper()
		activities, err := d.GetActivities(projectRef, "", "", "", "", 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range activities {
			if a.ID == act.ID {
				return true
			}
		}
		return false
	}
	if !listed(labelled.ID) {
		t.Fatal("the sync of a tracker the project selects is missing from its activities")
	}
	if !listed(labelled.Slug) {
		t.Fatal("the sync is missing when the project is named by its slug")
	}
	if listed(elsewhere.ID) {
		t.Fatal("a project selecting another tracker lists the sync")
	}
}
