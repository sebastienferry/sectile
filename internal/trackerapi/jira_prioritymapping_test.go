package trackerapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// numberedSchemeJSON is a project scheme whose names mean nothing to the
// aliases: every line of its mapping starts as a guess.
const numberedSchemeJSON = `[{"id":"20","name":"P1"},{"id":"21","name":"P2"},{"id":"22","name":"P3"},{"id":"23","name":"P4"}]`

func jiraSiteWithNumberedScheme(t *testing.T) *jiraSite {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/priority/search", `{"isLast":true,"values":`+numberedSchemeJSON+`}`)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes", `{"total":1,"issueTypes":[{"id":"3","name":"Task"}]}`)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", `{"total":1,"fields":[{"fieldId":"priority","name":"Priority","allowedValues":`+numberedSchemeJSON+`}]}`)
	site.reply("GET", "/rest/api/3/issue/PE-7/editmeta", `{"fields":{"priority":{"allowedValues":`+numberedSchemeJSON+`}}}`)
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","priority":{"name":"P3"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}}`)
	return site
}

// mappedProject is the test project with P1 confirmed urgent, P3 set by hand
// to medium, and P2 and P4 still guesses.
func mappedProject() *models.Project {
	p := jiraProject()
	p.PriorityMapping = models.PriorityMapping{Options: []models.PriorityMappingOption{
		{ID: "20", Name: "P1", Level: models.PriorityUrgent, Manual: true},
		{ID: "21", Name: "P2", Level: models.PriorityHigh, Guessed: true},
		{ID: "22", Name: "P3", Level: models.PriorityMedium, Manual: true},
		{ID: "23", Name: "P4", Level: models.PriorityLow, Guessed: true},
	}}
	return p
}

func TestJiraClassifiesKnownSchemesAsSureAndNumberedOnesAsGuessed(t *testing.T) {
	adapter := &JiraAdapter{}
	for _, c := range []struct {
		name   string
		scheme jiraPriorityScheme
		sure   bool
	}{
		{"Atlassian default", jiraCloudScheme, true},
		{"Server", jiraServerScheme, true},
		{"French", jiraFrenchScheme, true},
		{"numbered", jiraNumberScheme, false},
	} {
		for rank, option := range c.scheme {
			level, sure := adapter.ClassifyPriority(option.Name, rank, len(c.scheme))
			if sure != c.sure {
				t.Errorf("%s: %q sure = %v, want %v", c.name, option.Name, sure, c.sure)
			}
			// The level is the one a write without a mapping sends, so the
			// table starts from what the board already shows.
			if want := jiraPriority(option.Name, c.scheme); level != want {
				t.Errorf("%s: %q classified %q, read as %q", c.name, option.Name, level, want)
			}
		}
	}
}

func TestJiraPrioritySchemeReadsTheProjectsCreationScreen(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	got, err := site.adapter().PriorityScheme(unattended(), jiraProject(), false)
	if err != nil {
		t.Fatal(err)
	}
	want := []models.PriorityOption{{ID: "20", Name: "P1"}, {ID: "21", Name: "P2"}, {ID: "22", Name: "P3"}, {ID: "23", Name: "P4"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scheme = %+v", got)
	}
	// A second read comes from the cache; a fresh one asks the site again.
	if _, err := site.adapter().PriorityScheme(unattended(), jiraProject(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := site.adapter().PriorityScheme(unattended(), jiraProject(), true); err != nil {
		t.Fatal(err)
	}
	if calls := site.calls("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3"); len(calls) != 2 {
		t.Fatalf("the creation screen was read %d times, want 2", len(calls))
	}
}

func TestJiraPrioritySchemeFallsBackOnTheSiteList(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", `{"total":1,"fields":[{"fieldId":"summary","name":"Summary"}]}`)
	got, err := site.adapter().PriorityScheme(unattended(), jiraProject(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Name != "P1" {
		t.Fatalf("scheme = %+v", got)
	}
}

func TestJiraWritesTheMappedOption(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	var updated map[string]any
	site.on("PUT", "/rest/api/3/issue/PE-7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&updated)
		w.WriteHeader(http.StatusNoContent)
	})
	// Medium is set by hand on P3, where the rank alone would also say P3;
	// moving it to P2 by hand shows the mapping decides, not the rank.
	project := mappedProject()
	project.PriorityMapping.Options[1] = models.PriorityMappingOption{ID: "21", Name: "P2", Level: models.PriorityMedium, Manual: true}
	project.PriorityMapping.Preferred = map[models.Priority]string{models.PriorityMedium: "21"}
	medium := models.PriorityMedium
	if err := site.adapter().UpdateIssue(unattended(), tracker.UpdateIssueRequest{Project: project, Key: "PE-7", Priority: &medium}); err != nil {
		t.Fatal(err)
	}
	if got := updated["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "21"}) {
		t.Fatalf("update sent %#v, want the preferred option", got)
	}
}

// FR-8: a queued write that meets a mapping no longer sure of its level fails
// rather than sending a guess.
func TestJiraRefusesToUpdateAGuessedPriority(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	high := models.PriorityHigh
	err := site.adapter().UpdateIssue(unattended(), tracker.UpdateIssueRequest{Project: mappedProject(), Key: "PE-7", Priority: &high})
	var guessed *models.GuessedPriorityError
	if !errors.As(err, &guessed) {
		t.Fatalf("err = %v, want a guessed priority refusal", err)
	}
	if !reflect.DeepEqual(guessed.Writable, []models.Priority{models.PriorityUrgent, models.PriorityMedium}) {
		t.Fatalf("writable = %v", guessed.Writable)
	}
	if puts := site.calls("PUT", "/rest/api/3/issue/PE-7"); len(puts) != 0 {
		t.Fatalf("a refused update reached the site %d times", len(puts))
	}
}

// FR-7: a creation whose level is only guessed goes out without a priority,
// and says so.
func TestJiraCreatesWithoutAGuessedPriority(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	var created map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	task, err := site.adapter().CreateIssue(unattended(), tracker.CreateIssueRequest{Project: mappedProject(), Title: "New", Priority: models.PriorityHigh})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := created["fields"].(map[string]any)["priority"]; ok {
		t.Fatalf("a guessed priority was sent: %#v", created["fields"])
	}
	if !strings.HasPrefix(task.PriorityNotice, "The ticket was created without a priority.") || !strings.Contains(task.PriorityNotice, "urgent, medium") {
		t.Fatalf("notice = %q", task.PriorityNotice)
	}
	// No follow-up write tries to put the guess on afterwards.
	if puts := site.calls("PUT", "/rest/api/3/issue/PE-42"); len(puts) != 0 {
		t.Fatalf("the guess was put on afterwards %d times", len(puts))
	}
}

// US4.2: a sure level is created as before, with no notice.
func TestJiraCreatesWithASurePriority(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	var created map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	task, err := site.adapter().CreateIssue(unattended(), tracker.CreateIssueRequest{Project: mappedProject(), Title: "New", Priority: models.PriorityUrgent})
	if err != nil {
		t.Fatal(err)
	}
	if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "20"}) {
		t.Fatalf("creation sent %#v", got)
	}
	if task.PriorityNotice != "" {
		t.Fatalf("notice = %q", task.PriorityNotice)
	}
}

// A creation screen without the field puts the level on afterwards; a guessed
// level is not put on, and the notice says so.
func TestJiraDoesNotPutAGuessedPriorityOnAfterCreation(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", `{"total":1,"fields":[{"fieldId":"summary","name":"Summary","required":true}]}`)
	site.reply("GET", "/rest/api/3/issue/PE-42/editmeta", `{"fields":{"priority":{"allowedValues":`+numberedSchemeJSON+`}}}`)
	site.reply("POST", "/rest/api/3/issue", `{"id":"1","key":"PE-42"}`)
	task, err := site.adapter().CreateIssue(unattended(), tracker.CreateIssueRequest{Project: mappedProject(), Title: "New", Priority: models.PriorityLow})
	if err != nil {
		t.Fatal(err)
	}
	if puts := site.calls("PUT", "/rest/api/3/issue/PE-42"); len(puts) != 0 {
		t.Fatalf("the guess was put on afterwards %d times", len(puts))
	}
	if task.PriorityNotice == "" {
		t.Fatal("the creation must say its priority was not written")
	}
}

// FR-10: an empty mapping keeps the rank guess.
func TestJiraWithoutMappingStillWritesByRank(t *testing.T) {
	site := jiraSiteWithNumberedScheme(t)
	var updated map[string]any
	site.on("PUT", "/rest/api/3/issue/PE-7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&updated)
		w.WriteHeader(http.StatusNoContent)
	})
	high := models.PriorityHigh
	if err := site.adapter().UpdateIssue(unattended(), tracker.UpdateIssueRequest{Project: jiraProject(), Key: "PE-7", Priority: &high}); err != nil {
		t.Fatal(err)
	}
	if got := updated["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "21"}) {
		t.Fatalf("update sent %#v", got)
	}
}
