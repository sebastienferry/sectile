package db

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// The framing of a Jira epic is copied as a second comment Sectile owns (#636),
// through the same machinery as the todos copy of #663.

func framingOf(text string) *string { return &text }

func TestFramingMirrorEligibility(t *testing.T) {
	database, jira, _ := jiraMirrorProject(t)
	jira.RoadmapProjects = []string{"DS"}
	opened := *jira
	opened.RoadmapAxisWrites = true
	github := &models.Project{ID: "gh", IssueTracker: "github", GithubRepo: "acme/app"}
	gitlab := &models.Project{ID: "gl", IssueTracker: "gitlab", GithubRepo: "acme/app"}
	local := &models.Project{ID: "lo", IssueTracker: "local"}
	cases := []struct {
		proj   *models.Project
		key    string
		kind   string
		reason string
	}{
		{jira, "PE-12", models.MacroTodosMirrorJiraComment, ""},
		{jira, "DS-4", "", "autre projet Jira"},
		// A declared roadmap project's epic stays in Sectile whatever the
		// axis writes option says (ADR 0043).
		{&opened, "DS-4", "", "autre projet Jira"},
		{jira, "M-3", "", "sans épic Jira"},
		// A milestone takes no comment: no description block either (D1).
		{github, "M-3", "", "ne prend pas de commentaire"},
		{github, "#12", "", "ne prend pas de commentaire"},
		{gitlab, "M-1", "", "porter le cadrage"},
		{local, "M-1", "", "projet local"},
	}
	for _, c := range cases {
		kind, reason := database.todosMirrorScope(c.proj, nil).eligibilityOf(framingCopy, c.key)
		if kind != c.kind || !strings.Contains(reason, c.reason) || (c.kind == "" && reason == "") {
			t.Errorf("%s %s: kind %q reason %q, want %q containing %q", c.proj.IssueTracker, c.key, kind, reason, c.kind, c.reason)
		}
	}
	// The todos keep their own reasons.
	if _, reason := database.todosMirrorScope(gitlab, nil).eligibility("M-1"); !strings.Contains(reason, "porter la liste") {
		t.Errorf("todos reason on GitLab %q", reason)
	}
	if kind, _ := database.todosMirrorScope(github, nil).eligibility("M-3"); kind != models.MacroTodosMirrorGithubDescription {
		t.Errorf("todos of a milestone are still copied, got %q", kind)
	}
}

func TestRenderFramingMirror(t *testing.T) {
	framing := "Why this epic exists.\n\n```go\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |"
	body := renderFramingMirror("  " + framing + "\n\n")
	want := "### 🧭 [Sectile] Cadrage\n\n" + framing + "\n\n_Cadrage tenu dans Sectile : une modification faite ici est remplacée à la prochaine mise à jour._"
	if body != want {
		t.Fatalf("body:\n%s\nwant:\n%s", body, want)
	}
	if renderFramingMirror(framing) != body {
		t.Error("two equal framings render the same body")
	}
	if empty := renderFramingMirror(" \n"); !strings.Contains(empty, "Aucun cadrage") || !strings.HasPrefix(empty, "### 🧭 [Sectile] Cadrage") {
		t.Errorf("an empty framing says so: %q", empty)
	}
	// The fenced block and the table go through the Jira conversion whole.
	if adf := trackerapi.MarkdownToADF(body); adf == nil {
		t.Error("the body converts to ADF")
	}
}

func TestRenderFramingMirrorKeepsWithinTheBudget(t *testing.T) {
	line := strings.Repeat("é", 400) // two bytes each
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = line
	}
	for name, framing := range map[string]string{
		"many lines": strings.Join(lines, "\n"),
		"one line":   strings.Repeat("é", 20000),
	} {
		body := renderFramingMirror(framing)
		if len(body) > todosMirrorBudget {
			t.Fatalf("%s: %d bytes over the budget", name, len(body))
		}
		if !strings.Contains(body, "cadrage tronqué, texte complet dans Sectile") || !strings.HasSuffix(body, "prochaine mise à jour._") {
			t.Errorf("%s: the cut is said before the footer", name)
		}
		if !utf8.ValidString(body) {
			t.Errorf("%s: the cut split a character", name)
		}
	}
	cut := renderFramingMirror(strings.Join(lines, "\n"))
	if !strings.Contains(cut, line+"\n\n_… cadrage tronqué") {
		t.Errorf("a framing of many lines is cut on a line break")
	}
}

func TestFramingMirrorStatus(t *testing.T) {
	never := &models.MacroMeta{Key: "PE-1"}
	if s := macroCopyStatus(framingCopy, never, models.MacroTodosMirrorJiraComment, "", todosMirrorState{}); !s.UpToDate {
		t.Error("a framing never copied and empty waits for nothing")
	}
	framed := &models.MacroMeta{Key: "PE-1", FramingComment: "Why", ExternalURL: "https://site/browse/PE-1"}
	if s := macroCopyStatus(framingCopy, framed, models.MacroTodosMirrorJiraComment, "", todosMirrorState{}); s.UpToDate {
		t.Error("a framing never copied is waiting")
	}
	hash := todosMirrorHash(renderFramingMirror("Why"))
	s := macroCopyStatus(framingCopy, framed, models.MacroTodosMirrorJiraComment, "", todosMirrorState{ref: "7", hash: hash})
	if !s.UpToDate || s.URL != "https://site/browse/PE-1?focusedCommentId=7" {
		t.Errorf("status %+v", s)
	}
}

func TestFramingSavesSettleIntoOneWriteBesideTheTodos(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	todosMirrorDelay = 150 * time.Millisecond

	list := todosOf("a")
	if _, err := database.UpdateMacro(as("ada"), proj.ID, "PE-12", nil, nil, nil, nil, &list, nil); err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		user := "ada"
		if i == 9 {
			user = "bob"
		}
		text := strings.Repeat("v", i+1)
		if _, err := database.UpdateMacro(as(user), proj.ID, "PE-12", nil, nil, nil, framingOf(text), nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "both copies", func() bool { return len(fake.written()) >= 2 })
	time.Sleep(200 * time.Millisecond)
	writes := fake.written()
	if len(writes) != 2 {
		t.Fatalf("a todos save and ten framing saves make two writes, got %d", len(writes))
	}
	var framing, todos int
	for i, w := range writes {
		switch w.Marker {
		case framingMirrorMarker:
			framing++
			if !strings.Contains(w.Body, strings.Repeat("v", 10)) || strings.Contains(w.Body, "☐") {
				t.Errorf("the framing write renders the last framing only:\n%s", w.Body)
			}
			if fake.actors[i] != "bob" {
				t.Errorf("the framing write is signed by the last saver, got %q", fake.actors[i])
			}
		case todosMirrorMarker:
			todos++
			if strings.Contains(w.Body, "Cadrage") {
				t.Errorf("the todos comment carries no framing:\n%s", w.Body)
			}
		}
	}
	if framing != 1 || todos != 1 {
		t.Fatalf("framing %d todos %d", framing, todos)
	}
	waitFor(t, "the framing state", func() bool {
		state, _ := database.readMacroCopyState(framingCopy, proj.ID, "PE-12")
		return state.ref == "c-1" && state.hash != "" && state.at != nil
	})
	macro, err := database.GetMacro(proj.ID, "PE-12")
	if err != nil || macro.FramingMirror == nil || !macro.FramingMirror.UpToDate || macro.FramingMirror.Kind != models.MacroTodosMirrorJiraComment {
		t.Fatalf("macro %+v %v", macro, err)
	}
}

func TestFramingCopyIsScheduledOnlyByAFramingSave(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, framingOf("Why"), nil); err != nil {
		t.Fatal(err)
	}
	description, horizon := "Text", "now"
	if _, err := database.UpdateMacro(as("ada"), proj.ID, "PE-12", nil, &horizon, &description, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	// A bulk edit, a declared roadmap project's epic and a local key schedule
	// nothing either.
	if _, err := database.UpdateMacro(WithBulkMacroEdit(as("ada")), proj.ID, "PE-12", nil, nil, nil, framingOf("Bulk"), nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"DS-4", "M-2"} {
		if _, err := database.UpdateMacro(as("ada"), proj.ID, key, nil, nil, nil, framingOf("Why"), nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if n := len(fake.written()); n != 0 {
		t.Fatalf("no framing write was due, got %d", n)
	}
	if refusal := database.FramingMirrorRefusal(proj.ID, "DS-4"); !strings.Contains(refusal, "le cadrage de DS-4 reste dans Sectile") {
		t.Errorf("refusal %q", refusal)
	}
	if refusal := database.FramingMirrorRefusal(proj.ID, "PE-2"); refusal != "" {
		t.Errorf("an epic of the project is copied, got %q", refusal)
	}
}

func TestPushMacroFramingMirror(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	ctx := as("ada")

	// An empty framing never creates a comment.
	note, err := database.PushMacroFramingMirror(ctx, proj.ID, "PE-12", false)
	if err != nil || !strings.Contains(note, "aucun cadrage à recopier") || len(fake.written()) != 0 {
		t.Fatalf("nothing to copy: %q %v %d", note, err, len(fake.written()))
	}
	if note, err = database.PushMacroFramingMirror(ctx, proj.ID, "PE-12", true); err != nil || len(fake.written()) != 0 {
		t.Fatalf("even forced, an empty framing creates nothing: %q %v", note, err)
	}

	// The todos comment has its own id: the framing never names it.
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, framingOf("Why"), &list); err != nil {
		t.Fatal(err)
	}
	database.recordTodosMirrorSuccess(proj.ID, "PE-12", "todos-9", "h")
	if _, err := database.PushMacroFramingMirror(ctx, proj.ID, "PE-12", false); err != nil {
		t.Fatal(err)
	}
	note, err = database.PushMacroFramingMirror(ctx, proj.ID, "PE-12", false)
	if err != nil || !strings.Contains(note, "déjà à jour") || len(fake.written()) != 1 {
		t.Fatalf("an unchanged framing makes no call: %q %v, %d writes", note, err, len(fake.written()))
	}
	if _, err := database.PushMacroFramingMirror(ctx, proj.ID, "PE-12", true); err != nil {
		t.Fatal(err)
	}
	// Emptied: the comment is rewritten to say so, never deleted.
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, framingOf(""), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroFramingMirror(ctx, proj.ID, "PE-12", false); err != nil {
		t.Fatal(err)
	}
	writes := fake.written()
	if len(writes) != 3 {
		t.Fatalf("writes %+v", writes)
	}
	for i, w := range writes {
		if w.Marker != framingMirrorMarker || w.Value["macroKey"] != "PE-12" {
			t.Errorf("write %d marker %q value %v", i, w.Marker, w.Value)
		}
	}
	if writes[0].CommentID != "" || writes[1].CommentID != "c-1" || writes[2].CommentID != "c-1" {
		t.Errorf("the framing comment is created once, then rewritten: %+v", writes)
	}
	if !strings.Contains(writes[2].Body, "Aucun cadrage") {
		t.Errorf("the emptied framing says so:\n%s", writes[2].Body)
	}
	todos, _ := database.readTodosMirrorState(proj.ID, "PE-12")
	if todos.ref != "todos-9" {
		t.Errorf("the todos state is left alone, got %+v", todos)
	}
}

func TestPushMacroFramingMirrorFailures(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, framingOf("Why"), nil); err != nil {
		t.Fatal(err)
	}
	fake.failures = []error{&trackerapi.HTTPError{Status: http.StatusBadGateway}, &trackerapi.HTTPError{Status: http.StatusTooManyRequests}}
	if _, err := database.PushMacroFramingMirror(as("ada"), proj.ID, "PE-12", false); err != nil || len(fake.written()) != 3 {
		t.Fatalf("passing failures are retried: %v, %d attempts", err, len(fake.written()))
	}

	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, framingOf("Why not"), nil); err != nil {
		t.Fatal(err)
	}
	fake.failures = []error{&trackerapi.MissingPersonalCredentialError{Tracker: "jira"}}
	_, err := database.PushMacroFramingMirror(as("ada"), proj.ID, "PE-12", false)
	if err == nil || trackerapi.MissingCredentialTracker(err) != "jira" || !strings.Contains(err.Error(), "cadrage gardé dans Sectile") {
		t.Fatalf("the refusal keeps the credential chain: %v", err)
	}
	if n := len(fake.written()); n != 4 {
		t.Fatalf("a permanent failure is not retried, %d attempts", n)
	}
	macro, _ := database.GetMacro(proj.ID, "PE-12")
	if m := macro.FramingMirror; m.UpToDate || m.Error == "" || m.CredentialMissing != "jira" {
		t.Fatalf("status after the failure %+v", m)
	}
	if m := macro.TodosMirror; m.Error != "" {
		t.Errorf("the todos status keeps no framing failure %+v", m)
	}
	if _, err := database.PushMacroFramingMirror(as("ada"), proj.ID, "PE-12", false); err != nil {
		t.Fatal(err)
	}
	macro, _ = database.GetMacro(proj.ID, "PE-12")
	if m := macro.FramingMirror; !m.UpToDate || m.Error != "" || m.CredentialMissing != "" {
		t.Fatalf("a success clears the failure %+v", m)
	}
}

func TestFramingMirrorJobRunsThroughTheQueue(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, framingOf("Why"), nil); err != nil {
		t.Fatal(err)
	}
	act, err := database.EnqueueTrackerOp(as("ada"), TrackerOp{Kind: TrackerOpEpicFraming, ProjectID: proj.ID, TaskKey: "PE-12", EpicKey: "PE-12"})
	if err != nil || !strings.Contains(act.Action, "Cadrage de PE-12") {
		t.Fatalf("activity %+v %v", act, err)
	}
	waitFor(t, "the activity", func() bool {
		got, _ := database.GetActivityByID(act.ID)
		return got != nil && got.Status == string(models.ActivityStatusCompleted)
	})
	if len(fake.written()) != 1 || fake.actors[0] != "ada" || fake.written()[0].Marker != framingMirrorMarker {
		t.Fatalf("writes %+v actors %v", fake.written(), fake.actors)
	}
}

func TestFramingMirrorOnAGithubMilestone(t *testing.T) {
	database := mirrorDB(t)
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "App", Slug: "app", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveMacroMeta(proj.ID, "M-3", nil, nil, framingOf("Why"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroFramingMirror(context.Background(), proj.ID, "M-3", true); err == nil || !strings.Contains(err.Error(), "reste dans Sectile") {
		t.Fatalf("a milestone carries no framing: %v", err)
	}
	if refusal := database.FramingMirrorRefusal(proj.ID, "M-3"); !strings.Contains(refusal, "ne prend pas de commentaire") {
		t.Errorf("refusal %q", refusal)
	}
}
