package db

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// schemeTracker is a Jira fake that also lists a priority scheme, the way
// the Jira adapter does (#679).
type schemeTracker struct {
	*fakeTracker
	schemeMu  sync.Mutex
	scheme    []models.PriorityOption
	schemeErr error
	reads     []bool // the fresh flag of every read
}

func (s *schemeTracker) PriorityScheme(ctx context.Context, project *models.Project, fresh bool) ([]models.PriorityOption, error) {
	s.schemeMu.Lock()
	defer s.schemeMu.Unlock()
	s.reads = append(s.reads, fresh)
	return s.scheme, s.schemeErr
}

// ClassifyPriority knows the names of Atlassian's default scheme and reads
// anything else by rank.
func (s *schemeTracker) ClassifyPriority(name string, rank, n int) (models.Priority, bool) {
	switch strings.ToLower(name) {
	case "highest":
		return models.PriorityUrgent, true
	case "high":
		return models.PriorityHigh, true
	case "medium":
		return models.PriorityMedium, true
	case "low":
		return models.PriorityLow, true
	}
	return models.PriorityLevels[rank*len(models.PriorityLevels)/n], false
}

func (s *schemeTracker) readCount() int {
	s.schemeMu.Lock()
	defer s.schemeMu.Unlock()
	return len(s.reads)
}

// numberedScheme is a scheme Sectile cannot name: every line is a guess.
var numberedScheme = []models.PriorityOption{{ID: "1", Name: "P1"}, {ID: "2", Name: "P2"}, {ID: "3", Name: "P3"}, {ID: "4", Name: "P4"}}

func priorityMappingDB(t *testing.T, scheme []models.PriorityOption) (*DB, *schemeTracker, *models.Project) {
	t.Helper()
	fake := &schemeTracker{fakeTracker: newFakeTracker(), scheme: scheme}
	database, project := jiraTestDB(t, fake.fakeTracker)
	database.TrackerRegistry().Register("jira", fake)
	return database, fake, project
}

func reloadProject(t *testing.T, database *DB, id string) *models.Project {
	t.Helper()
	proj, err := database.GetProjectByID(id)
	if err != nil || proj == nil {
		t.Fatalf("project %s: %v", id, err)
	}
	return proj
}

func TestRefreshPriorityMappingStoresTheScheme(t *testing.T) {
	database, fake, project := priorityMappingDB(t, []models.PriorityOption{
		{ID: "1", Name: "Highest"}, {ID: "2", Name: "P2"}, {ID: "3", Name: "Medium"}, {ID: "4", Name: "Low"},
	})
	note, err := database.RefreshPriorityMapping(context.Background(), project.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if note != "4 priorities, 1 guessed" {
		t.Fatalf("note = %q", note)
	}
	got := reloadProject(t, database, project.ID).PriorityMapping.Options
	want := []models.PriorityMappingOption{
		{ID: "1", Name: "Highest", Level: models.PriorityUrgent},
		{ID: "2", Name: "P2", Level: models.PriorityHigh, Guessed: true},
		{ID: "3", Name: "Medium", Level: models.PriorityMedium},
		{ID: "4", Name: "Low", Level: models.PriorityLow},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(fake.reads, []bool{true}) {
		t.Fatalf("reads = %v, want one fresh read", fake.reads)
	}
}

func TestRefreshPriorityMappingNeverChangesAStoredLine(t *testing.T) {
	database, fake, project := priorityMappingDB(t, numberedScheme)
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err != nil {
		t.Fatal(err)
	}
	edit := models.PriorityMapping{Options: []models.PriorityMappingOption{{ID: "1", Level: models.PriorityHigh}}}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{PriorityMapping: &edit}); err != nil {
		t.Fatal(err)
	}
	// P4 leaves the scheme, P5 joins it.
	fake.scheme = []models.PriorityOption{{ID: "1", Name: "P1"}, {ID: "2", Name: "P2"}, {ID: "3", Name: "P3"}, {ID: "5", Name: "P5"}}
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err != nil {
		t.Fatal(err)
	}
	got := reloadProject(t, database, project.ID).PriorityMapping.Options
	want := []models.PriorityMappingOption{
		{ID: "1", Name: "P1", Level: models.PriorityHigh, Manual: true},
		{ID: "2", Name: "P2", Level: models.PriorityHigh, Guessed: true},
		{ID: "3", Name: "P3", Level: models.PriorityMedium, Guessed: true},
		{ID: "5", Name: "P5", Level: models.PriorityLow, Guessed: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
}

func TestRefreshPriorityMappingKeepsTheMappingOnAFailedRead(t *testing.T) {
	database, fake, project := priorityMappingDB(t, numberedScheme)
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err != nil {
		t.Fatal(err)
	}
	fake.schemeErr = errors.New("503 site down")
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err == nil {
		t.Fatal("a failed read must be reported")
	}
	if n := len(reloadProject(t, database, project.ID).PriorityMapping.Options); n != 4 {
		t.Fatalf("mapping lost its %d lines", 4-n)
	}
}

func TestRefreshPriorityMappingSkipsATrackerWithoutScheme(t *testing.T) {
	database, project := jiraTestDB(t, newFakeTracker())
	note, err := database.RefreshPriorityMapping(context.Background(), project.ID, true)
	if err != nil || note != "" {
		t.Fatalf("note = %q, err = %v", note, err)
	}
	if !reloadProject(t, database, project.ID).PriorityMapping.Empty() {
		t.Fatal("a tracker without scheme stores no mapping")
	}
}

// US1.6: the sync rediscovers the scheme when it describes the project, and
// not on an incremental background poll.
func TestSyncRefreshesThePriorityMappingOnAFullSyncOnly(t *testing.T) {
	database, fake, project := priorityMappingDB(t, numberedScheme)
	steps := database.afterTrackerSync(context.Background(), project, fake, nil, SyncOptions{Background: true, WindowMin: 10})
	if fake.readCount() != 0 {
		t.Fatalf("a background poll read the scheme: %v", steps)
	}
	steps = database.afterTrackerSync(context.Background(), project, fake, nil, SyncOptions{})
	if fake.readCount() != 1 || fake.reads[0] {
		t.Fatalf("reads = %v, want one cached read", fake.reads)
	}
	if !containsStep(steps, "7. Priorities: 4 priorities, 4 guessed") {
		t.Fatalf("steps = %v", steps)
	}
}

func containsStep(steps []string, want string) bool {
	for _, s := range steps {
		if s == want {
			return true
		}
	}
	return false
}

func TestEditPriorityMapping(t *testing.T) {
	stored := models.PriorityMapping{Options: []models.PriorityMappingOption{
		{ID: "1", Name: "P1", Level: models.PriorityUrgent, Guessed: true},
		{ID: "2", Name: "P2", Level: models.PriorityHigh, Guessed: true},
		{ID: "3", Name: "P3", Level: models.PriorityMedium, Guessed: true},
	}}
	got, err := editPriorityMapping(stored, models.PriorityMapping{
		Options: []models.PriorityMappingOption{
			// Confirmed as guessed: becomes sure at the same level.
			{ID: "1", Name: "renamed by the client", Level: models.PriorityUrgent},
			// Moved: becomes sure at the new level.
			{ID: "2", Level: models.PriorityMedium, Guessed: true},
			// Untouched: stays a guess.
			{ID: "3", Level: models.PriorityMedium, Guessed: true},
		},
		Preferred: map[models.Priority]string{models.PriorityMedium: "2", models.PriorityLow: ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []models.PriorityMappingOption{
		{ID: "1", Name: "P1", Level: models.PriorityUrgent, Manual: true},
		{ID: "2", Name: "P2", Level: models.PriorityMedium, Manual: true},
		{ID: "3", Name: "P3", Level: models.PriorityMedium, Guessed: true},
	}
	if !reflect.DeepEqual(got.Options, want) {
		t.Fatalf("options = %+v, want %+v", got.Options, want)
	}
	if !reflect.DeepEqual(got.Preferred, map[models.Priority]string{models.PriorityMedium: "2"}) {
		t.Fatalf("preferred = %v", got.Preferred)
	}

	for name, sent := range map[string]models.PriorityMapping{
		"unknown option":    {Options: []models.PriorityMappingOption{{ID: "9", Level: models.PriorityLow}}},
		"bad level":         {Options: []models.PriorityMappingOption{{ID: "1", Level: "critical"}}},
		"unknown preferred": {Preferred: map[models.Priority]string{models.PriorityLow: "9"}},
		"preferred level":   {Preferred: map[models.Priority]string{"critical": "1"}},
	} {
		if _, err := editPriorityMapping(stored, sent); !errors.Is(err, ErrInvalidPriorityMapping) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestUpdateProjectRefusesAnInvalidPriorityMapping(t *testing.T) {
	database, _, project := priorityMappingDB(t, numberedScheme)
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err != nil {
		t.Fatal(err)
	}
	bad := models.PriorityMapping{Options: []models.PriorityMappingOption{{ID: "nope", Level: models.PriorityLow}}}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{PriorityMapping: &bad}); !errors.Is(err, ErrInvalidPriorityMapping) {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckPriorityWritable(t *testing.T) {
	mapped := &models.Project{IssueTracker: "jira", PriorityMapping: models.PriorityMapping{Options: []models.PriorityMappingOption{
		{ID: "1", Level: models.PriorityUrgent},
		{ID: "2", Level: models.PriorityHigh, Guessed: true},
	}}}
	if err := CheckPriorityWritable(mapped, models.PriorityUrgent); err != nil {
		t.Fatalf("a sure level is refused: %v", err)
	}
	var guessed *models.GuessedPriorityError
	if err := CheckPriorityWritable(mapped, models.PriorityHigh); !errors.As(err, &guessed) || !reflect.DeepEqual(guessed.Writable, []models.Priority{models.PriorityUrgent}) {
		t.Fatalf("err = %v", err)
	}
	// US3.7 and US3.8: no mapping yet, or another tracker, writes as before.
	for _, proj := range []*models.Project{
		nil,
		{IssueTracker: "jira"},
		{IssueTracker: "github", PriorityMapping: mapped.PriorityMapping},
	} {
		if err := CheckPriorityWritable(proj, models.PriorityHigh); err != nil {
			t.Errorf("%+v: %v", proj, err)
		}
	}
}

// US3: an update to a guessed level is refused before anything is written.
func TestUpdateTaskRefusesAGuessedPriorityBeforeWriting(t *testing.T) {
	database, _, project := priorityMappingDB(t, numberedScheme)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Guarded", Priority: models.PriorityMedium, Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.RefreshPriorityMapping(context.Background(), project.ID, false); err != nil {
		t.Fatal(err)
	}
	before, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}

	title := "Renamed"
	high := models.PriorityHigh
	_, err = database.UpdateTaskBy(Actor{}, task.ID, models.UpdateTaskRequest{Title: &title, Priority: &high})
	var guessed *models.GuessedPriorityError
	if !errors.As(err, &guessed) {
		t.Fatalf("err = %v, want a guessed priority refusal", err)
	}
	after, _ := database.GetTaskByID(task.ID)
	if after.Priority != models.PriorityMedium || after.Title != "Guarded" {
		t.Fatalf("the refused update wrote the card: %+v", after)
	}
	activities, _ := database.GetTaskActivities(task.ID)
	if len(activities) != len(before) {
		t.Fatalf("the refused update queued %d activities", len(activities)-len(before))
	}

	// US3.5: sending the priority the card already has is not a change.
	medium := models.PriorityMedium
	if _, err := database.UpdateTaskBy(Actor{}, task.ID, models.UpdateTaskRequest{Title: &title, Priority: &medium}); err != nil {
		t.Fatalf("an unchanged priority was refused: %v", err)
	}

	// US3.6: once confirmed, the level is accepted.
	edit := models.PriorityMapping{Options: []models.PriorityMappingOption{{ID: "2", Level: models.PriorityHigh}}}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{PriorityMapping: &edit}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.UpdateTaskBy(Actor{}, task.ID, models.UpdateTaskRequest{Priority: &high}); err != nil {
		t.Fatalf("a confirmed level was refused: %v", err)
	}
}

// US3.7: before the first discovery, priority updates behave as before.
func TestUpdateTaskWithoutMappingWritesAsBefore(t *testing.T) {
	database, _, project := priorityMappingDB(t, numberedScheme)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Free", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	high := models.PriorityHigh
	if _, err := database.UpdateTaskBy(Actor{}, task.ID, models.UpdateTaskRequest{Priority: &high}); err != nil {
		t.Fatal(err)
	}
}

// noticeTracker answers a creation the way the Jira adapter does when the
// project's mapping only guessed the level (#679).
type noticeTracker struct {
	*schemeTracker
}

func (c *noticeTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	return &models.Task{Key: "PE-9", Priority: req.Priority, PriorityNotice: "The ticket was created without a priority."}, nil
}

// US4.1: the adapter's notice reaches whoever created the ticket.
func TestCreateTaskCarriesThePriorityNotice(t *testing.T) {
	database, fake, project := priorityMappingDB(t, numberedScheme)
	fake.Capabilities = append(fake.Capabilities, tracker.CapCreate)
	database.TrackerRegistry().Register("jira", &noticeTracker{fake})
	task, err := database.CreateTaskAs(context.Background(), models.CreateTaskRequest{ProjectID: project.ID, Title: "New", Priority: models.PriorityHigh})
	if err != nil {
		t.Fatal(err)
	}
	if task.Key != "PE-9" || task.PriorityNotice != "The ticket was created without a priority." {
		t.Fatalf("created task = %+v", task)
	}
}
