package db

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// spaceProject creates a project on the Jira space GODE with a label, through
// the project write path a member uses (#741).
func spaceProject(t *testing.T, d *DB, name, label string) *models.Project {
	t.Helper()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: name, IssueTracker: "jira", JiraProject: "GODE", Label: label})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// importTickets writes tickets into a tracker as a synchronisation does, one
// per key, each with its labels.
func importTickets(t *testing.T, d *DB, trackerID string, labels map[string][]string) {
	t.Helper()
	var tickets []models.Task
	for key, l := range labels {
		tickets = append(tickets, models.Task{
			Key:       key,
			Title:     key,
			Status:    models.StatusToClarify,
			Priority:  models.PriorityMedium,
			Labels:    l,
			Source:    "jira",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}
	if err := d.ImportOrUpdateTasks(trackerID, tickets); err != nil {
		t.Fatal(err)
	}
}

// listedKeys lists the keys a project shows, sorted.
func listedKeys(t *testing.T, d *DB, projectID string) []string {
	t.Helper()
	tasks, err := d.GetTasks("", "", "", "", projectID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, task := range tasks {
		keys = append(keys, task.Key)
	}
	sort.Strings(keys)
	return keys
}

func TestALabelledProjectListsOnlyTheTicketsCarryingItsLabel(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {"delivery-admin"}, "GODE-2": {"other"}})

	if got := listedKeys(t, d, delivery.ID); strings.Join(got, ",") != "GODE-1" {
		t.Fatalf("the labelled project lists %v, want GODE-1 only", got)
	}
}

func TestTheLabelMatchesWholeTokensRegardlessOfAsciiCase(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "Delivery-Admin")
	accented := spaceProject(t, d, "Equipe", "Équipe")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{
		"GODE-1": {"delivery-admin"},
		"GODE-2": {"DELIVERY-ADMIN", "ops"},
		"GODE-3": {"delivery-admin-v2"},
		"GODE-4": {"x-delivery-admin"},
		"GODE-5": {"Équipe"},
		"GODE-6": {"équipe"},
	})

	if got := listedKeys(t, d, delivery.ID); strings.Join(got, ",") != "GODE-1,GODE-2" {
		t.Fatalf("Delivery-Admin lists %v, want the whole token in any ASCII case", got)
	}
	if got := listedKeys(t, d, accented.ID); strings.Join(got, ",") != "GODE-5" {
		t.Fatalf("Équipe lists %v: only A-Z fold, an accented letter keeps its own case", got)
	}
}

func TestAnUnlabelledProjectListsEveryTicketOfItsTrackers(t *testing.T) {
	d := testDB(t)
	all := spaceProject(t, d, "Everything", "")
	labelled := spaceProject(t, d, "Delivery", "delivery-admin")
	importTickets(t, d, all.DefaultTrackerID, map[string][]string{"GODE-1": {"delivery-admin"}, "GODE-2": {}})

	if got := listedKeys(t, d, all.ID); strings.Join(got, ",") != "GODE-1,GODE-2" {
		t.Fatalf("the unlabelled project lists %v, want every ticket of GODE", got)
	}
	if got := listedKeys(t, d, labelled.ID); strings.Join(got, ",") != "GODE-1" {
		t.Fatalf("the labelled project lists %v", got)
	}
}

func TestATicketWithTwoProjectLabelsShowsInBothProjects(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	bidder := spaceProject(t, d, "Bidder", "bidderAdmin")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {"delivery-admin", "bidderadmin"}})

	for _, p := range []*models.Project{delivery, bidder} {
		tasks, err := d.GetTasks("", "", "", "", p.ID, "", "", "", "", nil, nil, false)
		if err != nil || len(tasks) != 1 {
			t.Fatalf("%s lists %d tickets (%v), want the shared one", p.Name, len(tasks), err)
		}
		if tasks[0].ProjectID != p.ID {
			t.Fatalf("listed in %s, the ticket shows the project %q", p.Name, tasks[0].ProjectID)
		}
		if ids := strings.Join(tasks[0].ProjectIDs, ","); ids != delivery.ID+","+bidder.ID {
			t.Fatalf("projectIds = %s, want both projects", ids)
		}
	}
	var rows int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE key = 'GODE-1'").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("%d records of GODE-1 (%v), want one", rows, err)
	}
}

func TestTheGoMembershipAgreesWithTheSqlMembership(t *testing.T) {
	d := testDB(t)
	projects := []*models.Project{
		spaceProject(t, d, "Delivery", "Delivery-Admin"),
		spaceProject(t, d, "Bidder", "bidderAdmin"),
		spaceProject(t, d, "Everything", ""),
		spaceProject(t, d, "Accents", "Équipe"),
		spaceProject(t, d, "Wildcards", "50%_off"),
	}
	importTickets(t, d, projects[0].DefaultTrackerID, map[string][]string{
		"GODE-1": {"delivery-admin"},
		"GODE-2": {"BIDDERADMIN", "delivery-admin"},
		"GODE-3": {"delivery-admin-v2"},
		"GODE-4": {"équipe"},
		"GODE-5": {"Équipe"},
		"GODE-6": {"50%_off"},
		"GODE-7": {"50x-off"},
		"GODE-8": {},
		// A ticket label is matched as it is stored, spaces included: only the
		// project label is trimmed, on save.
		"GODE-9": {" delivery-admin"},
	})
	all, err := d.GetTasks("", "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil || len(all) != 9 {
		t.Fatalf("%d tickets (%v), want 9", len(all), err)
	}
	for _, p := range projects {
		sql := map[string]bool{}
		for _, key := range listedKeys(t, d, p.ID) {
			sql[key] = true
		}
		for i := range all {
			members, err := d.MemberProjects(&all[i])
			if err != nil {
				t.Fatal(err)
			}
			inGo := false
			for _, m := range members {
				inGo = inGo || m.ID == p.ID
			}
			if inGo != sql[all[i].Key] {
				t.Errorf("%s in %s: Go says %v, SQL says %v", all[i].Key, p.Name, inGo, sql[all[i].Key])
			}
		}
	}
}

func TestAProjectsTaskCountFollowsItsMembership(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	everything := spaceProject(t, d, "Everything", "")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {"delivery-admin"}, "GODE-2": {}, "GODE-3": {}})

	counts := map[string]int{}
	projects, err := d.GetProjects()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		counts[p.ID] = p.TaskCount
	}
	if counts[delivery.ID] != 1 || counts[everything.ID] != 3 {
		t.Fatalf("counts = delivery %d, everything %d; want 1 and 3", counts[delivery.ID], counts[everything.ID])
	}
	one, err := d.GetProjectByID(delivery.ID)
	if err != nil || one.TaskCount != 1 {
		t.Fatalf("the project read alone counts %d (%v), want 1", one.TaskCount, err)
	}
	var sentinels int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE project_id = ?", trackerSentinel(delivery.DefaultTrackerID)).Scan(&sentinels); err != nil || sentinels != 3 {
		t.Fatalf("%d rows carry the tracker sentinel (%v), want the 3 imported", sentinels, err)
	}
}

// jiraSpace records a Jira tracker as an admin does.
func jiraSpace(t *testing.T, d *DB, key string) *models.Tracker {
	t.Helper()
	trk, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: key})
	if err != nil {
		t.Fatal(err)
	}
	return trk
}

func TestAProjectCanSelectTwoTrackersAndTheFirstIsTheDefault(t *testing.T) {
	d := testDB(t)
	gode, be := jiraSpace(t, d, "GODE"), jiraSpace(t, d, "BE")
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: " delivery-admin ",
		Trackers: []models.ProjectTracker{{TrackerID: be.ID}, {Identity: gode.Identity}, {TrackerID: be.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Trackers) != 2 || p.Trackers[0].TrackerID != be.ID || p.Trackers[1].TrackerID != gode.ID {
		t.Fatalf("trackers = %+v, want BE then GODE", p.Trackers)
	}
	if p.DefaultTrackerID != be.ID || p.Label != "delivery-admin" {
		t.Fatalf("default %q, label %q; want BE and the trimmed label", p.DefaultTrackerID, p.Label)
	}
	if p.IssueTracker != "jira" || p.JiraProject != "BE" {
		t.Fatalf("the legacy fields read %s/%s, want the default tracker's", p.IssueTracker, p.JiraProject)
	}
	if _, err := d.CreateProject(models.CreateProjectRequest{Name: "Other", Trackers: []models.ProjectTracker{{TrackerID: "nope"}}}); err == nil {
		t.Fatal("an unknown tracker was accepted")
	}
}

func TestRemovingTheDefaultTrackerFallsBackToTheFirstRemaining(t *testing.T) {
	d := testDB(t)
	gode, be, ops := jiraSpace(t, d, "GODE"), jiraSpace(t, d, "BE"), jiraSpace(t, d, "OPS")
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery",
		Trackers: []models.ProjectTracker{{TrackerID: gode.ID}, {TrackerID: be.ID}, {TrackerID: ops.ID}}, DefaultTrackerID: be.ID})
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultTrackerID != be.ID {
		t.Fatalf("default = %q, want the named BE", p.DefaultTrackerID)
	}
	remaining := []models.ProjectTracker{{TrackerID: ops.ID}, {TrackerID: gode.ID}}
	updated, err := d.UpdateProject(p.ID, models.UpdateProjectRequest{Trackers: &remaining})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DefaultTrackerID != ops.ID {
		t.Fatalf("default = %q, want OPS, the first remaining", updated.DefaultTrackerID)
	}
	// A legacy field write lands on the default tracker and leaves the
	// project's other trackers selected.
	board := "77"
	updated, err = d.UpdateProject(p.ID, models.UpdateProjectRequest{BoardID: &board})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Trackers) != 2 || updated.DefaultTrackerID != ops.ID || updated.BoardID != "77" {
		t.Fatalf("after a board write: trackers %+v, default %q, board %q", updated.Trackers, updated.DefaultTrackerID, updated.BoardID)
	}
}

func TestTwoGithubTrackersWithTheSameIssueNumberImportTwoRecords(t *testing.T) {
	d := testDB(t)
	api, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/api"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/web"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Acme", Trackers: []models.ProjectTracker{{TrackerID: api.ID}, {TrackerID: web.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, trk := range []*models.Tracker{api, web} {
		if err := d.ImportOrUpdateTasks(trk.ID, []models.Task{{
			ID:        "12",
			Key:       "#12",
			Title:     trk.Scope,
			Status:    models.StatusToClarify,
			Priority:  models.PriorityMedium,
			Source:    "github",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}}); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := d.GetTasks("", "", "", "", p.ID, "", "", "", "", nil, nil, false)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("%d tickets (%v), want one #12 per repository", len(tasks), err)
	}
	if tasks[0].ID == tasks[1].ID || tasks[0].TrackerID == tasks[1].TrackerID {
		t.Fatalf("the two #12 share an id or a tracker: %+v", tasks)
	}
}

func TestAnUnlinkedTrackerIsSynchronisedInFull(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{
		{Key: "LONE-1", Title: "One", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{Key: "LONE-2", Title: "Two", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	d, _ := jiraTestDB(t, fake)
	lone := jiraSpace(t, d, "LONE")
	activity := models.TaskActivity{ID: "sync-lone", TrackerID: lone.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := d.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := d.GetSettings()
	d.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, TrackerID: lone.ID}, settings)

	var rows int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE tracker_id = ? AND project_id = ?", lone.ID, trackerSentinel(lone.ID)).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("%d tickets of the unlinked tracker (%v), want both", rows, err)
	}
}

// A synchronisation filed for a tracker deleted since it was queued fails: it
// never falls back to the deployment's default source, which would import that
// source's tickets under no tracker.
func TestASyncOfADeletedTrackerFailsAndImportsNothing(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{Key: "PE-1", Title: "One", Status: models.StatusToClarify, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	d, _ := jiraTestDB(t, fake)
	activity := models.TaskActivity{ID: "sync-gone", TrackerID: "gone", SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := d.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := d.GetSettings()
	settings.JiraProject = "PE"
	d.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, TrackerID: "gone"}, settings)

	var rows int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("%d tickets imported (%v), want none", rows, err)
	}
	if fake.called("sync") {
		t.Fatal("the deleted tracker's job read a source")
	}
	act, err := d.GetActivityByID(activity.ID)
	if err != nil || act == nil || act.Status != string(models.ActivityStatusFailed) || !strings.Contains(act.Summary, "supprimé") {
		t.Fatalf("the job's activity = %+v (%v), want failed, saying the tracker was deleted", act, err)
	}
}
