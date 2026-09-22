package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"testing"
)

// The scheme a site runs decides what a priority is called. These are the ones
// met in the wild: Atlassian's default, the one Jira Server shipped, a site
// served in French, and a numbered one whose names mean nothing at all.
var (
	jiraServerScheme = jiraPriorityScheme{{ID: "1", Name: "Blocker"}, {ID: "2", Name: "Major"}, {ID: "3", Name: "Minor"}}
	jiraFrenchScheme = jiraPriorityScheme{{ID: "10", Name: "La plus élevée"}, {ID: "11", Name: "Élevée"}, {ID: "12", Name: "Moyenne"}, {ID: "13", Name: "Basse"}}
	jiraNumberScheme = jiraPriorityScheme{{ID: "20", Name: "P1"}, {ID: "21", Name: "P2"}, {ID: "22", Name: "P3"}, {ID: "23", Name: "P4"}, {ID: "24", Name: "P5"}}
	jiraCloudScheme  = jiraPriorityScheme{{ID: "1", Name: "Highest"}, {ID: "2", Name: "High"}, {ID: "3", Name: "Medium"}, {ID: "4", Name: "Low"}, {ID: "5", Name: "Lowest"}}
)

func TestJiraPriorityIsReadByNameThenByRank(t *testing.T) {
	for _, c := range []struct {
		name   string
		scheme jiraPriorityScheme
		want   models.Priority
	}{
		// The name decides whenever it means something, whatever the scheme.
		{"Highest", jiraCloudScheme, models.PriorityUrgent},
		{"Blocker", jiraServerScheme, models.PriorityUrgent},
		{"Critique", jiraFrenchScheme, models.PriorityUrgent},
		{"La plus élevée", jiraFrenchScheme, models.PriorityUrgent},
		{"Major", jiraServerScheme, models.PriorityHigh},
		{"Élevée", jiraFrenchScheme, models.PriorityHigh},
		// Accents are folded, so a site writing it flat says the same thing.
		{"elevee", jiraFrenchScheme, models.PriorityHigh},
		{"Normal", jiraCloudScheme, models.PriorityMedium},
		{"Moyenne", jiraFrenchScheme, models.PriorityMedium},
		{"Minor", jiraServerScheme, models.PriorityLow},
		{"Lowest", jiraCloudScheme, models.PriorityLow},
		{"La plus basse", jiraFrenchScheme, models.PriorityLow},
		// A scheme spelling a level twice is read on either word, and the whole
		// name is tried first: "la plus élevée" is not the "élevée" it carries.
		{"P1 - Critical", jiraNumberScheme, models.PriorityUrgent},
		// A name that means nothing is placed by its rank in the site's own
		// scheme. Reading every one of them as medium, which the name table
		// alone did, put a whole board on one level.
		{"P1", jiraNumberScheme, models.PriorityUrgent},
		{"P2", jiraNumberScheme, models.PriorityHigh},
		{"P3", jiraNumberScheme, models.PriorityMedium},
		{"P5", jiraNumberScheme, models.PriorityLow},
		// A name in neither the table nor the scheme: a priority removed from
		// the scheme since the work item was given it.
		{"Retired", jiraNumberScheme, models.PriorityMedium},
		{"", jiraCloudScheme, models.PriorityMedium},
		// No scheme to rank against is not a reason to refuse a read.
		{"P2", nil, models.PriorityMedium},
	} {
		if got := jiraPriority(c.name, c.scheme); got != c.want {
			t.Errorf("%q in a %d-option scheme: got %q, want %q", c.name, len(c.scheme), got, c.want)
		}
	}
}

func TestJiraPriorityIsWrittenAsAnOptionTheSiteHas(t *testing.T) {
	for _, c := range []struct {
		priority models.Priority
		scheme   jiraPriorityScheme
		wantID   string
	}{
		// The name wins over the rank: Jira's default has five options for the
		// four Sectile levels, and "low" belongs on Low, not on Lowest.
		{models.PriorityUrgent, jiraCloudScheme, "1"},
		{models.PriorityHigh, jiraCloudScheme, "2"},
		{models.PriorityMedium, jiraCloudScheme, "3"},
		{models.PriorityLow, jiraCloudScheme, "4"},
		{models.PriorityUrgent, jiraFrenchScheme, "10"},
		{models.PriorityHigh, jiraFrenchScheme, "11"},
		{models.PriorityMedium, jiraFrenchScheme, "12"},
		{models.PriorityLow, jiraFrenchScheme, "13"},
		// Three options, and no name for the middle one: the rank answers.
		{models.PriorityUrgent, jiraServerScheme, "1"},
		{models.PriorityMedium, jiraServerScheme, "2"},
		{models.PriorityLow, jiraServerScheme, "3"},
		// A scheme this adapter cannot name at all is still spread over.
		{models.PriorityUrgent, jiraNumberScheme, "20"},
		{models.PriorityHigh, jiraNumberScheme, "21"},
		{models.PriorityMedium, jiraNumberScheme, "22"},
		{models.PriorityLow, jiraNumberScheme, "24"},
	} {
		option, ok := c.scheme.option(c.priority)
		if !ok || option.ID != c.wantID {
			t.Errorf("%q in %v: got %+v (%v), want id %s", c.priority, c.scheme, option, ok, c.wantID)
		}
	}
	// An empty scheme names nothing, which is what sends the write back to the
	// default names rather than to an id the site never gave.
	if _, ok := (jiraPriorityScheme{}).option(models.PriorityHigh); ok {
		t.Error("an empty scheme must not answer an option")
	}
	// What a write puts in a scheme it cannot read back is what the read side
	// makes of it: a level written then synchronised must not move.
	for _, p := range []models.Priority{models.PriorityUrgent, models.PriorityHigh, models.PriorityMedium, models.PriorityLow} {
		option, _ := jiraNumberScheme.option(p)
		if got := jiraPriority(option.Name, jiraNumberScheme); got != p {
			t.Errorf("writing %q wrote %q, which reads back as %q", p, option.Name, got)
		}
	}
}

// The bug: the adapter wrote "Highest", a name a site whose scheme is not
// Atlassian's default does not have, and every write came back as
// "priority: The priority selected is invalid".
func TestJiraWritesThePriorityIDOfTheSitesOwnScheme(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/priority/search", `{"isLast":true,"values":[
		{"id":"10300","name":"P1"},{"id":"10301","name":"P2"},{"id":"10302","name":"P3"},{"id":"10303","name":"P4"}
	]}`)
	var created, updated map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","priority":{"name":"P2"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}}`)
	site.on("PUT", "/rest/api/3/issue/PE-7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&updated)
		w.WriteHeader(http.StatusNoContent)
	})

	task, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityHigh})
	if err != nil {
		t.Fatal(err)
	}
	if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "10301"}) {
		t.Fatalf("creation must name an option the site has: %#v", got)
	}
	// And the work item comes back on the level it was created with.
	if task.Priority != models.PriorityHigh {
		t.Fatalf("read back as %q", task.Priority)
	}

	low := models.PriorityLow
	if err := site.adapter().UpdateIssue(context.Background(), tracker.UpdateIssueRequest{Project: jiraProject(), Key: "PE-7", Priority: &low}); err != nil {
		t.Fatal(err)
	}
	if got := updated["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "10303"}) {
		t.Fatalf("update must name an option the site has: %#v", got)
	}
	// One discovery for both writes: the scheme belongs to the site.
	if calls := site.calls("GET", "/rest/api/3/priority/search"); len(calls) != 1 {
		t.Fatalf("the scheme must be discovered once, not %d times", len(calls))
	}
}

// An older site serves the bare list instead of the paginated search.
func TestJiraReadsTheBarePriorityListWhenTheSearchIsGone(t *testing.T) {
	site := newJiraSite(t)
	site.on("GET", "/rest/api/3/priority/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errorMessages":["not found"]}`)
	})
	site.reply("GET", "/rest/api/3/priority", `[{"id":"1","name":"Blocker"},{"id":"2","name":"Major"},{"id":"3","name":"Minor"}]`)
	var created map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","priority":{"name":"Blocker"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}}`)

	task, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent})
	if err != nil {
		t.Fatal(err)
	}
	if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "1"}) {
		t.Fatalf("creation must name an option the site has: %#v", got)
	}
	if task.Priority != models.PriorityUrgent {
		t.Fatalf("read back as %q", task.Priority)
	}
}

// A site that will not say which priorities it has is no reason to refuse the
// write: it goes out under the default name, as it always did, and the site
// decides.
func TestJiraFallsBackOnThePriorityNameWhenTheSchemeCannotBeRead(t *testing.T) {
	site := newJiraSite(t)
	for _, path := range []string{"/rest/api/3/priority/search", "/rest/api/3/priority"} {
		site.on("GET", path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"errorMessages":["no permission"]}`)
		})
	}
	var created map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}}`)

	if _, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent}); err != nil {
		t.Fatal(err)
	}
	if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"name": "Highest"}) {
		t.Fatalf("an unreadable scheme must not stop the write: %#v", got)
	}
}

// A synchronisation reads the scheme once for the whole page, not once per
// work item.
func TestJiraSyncPlacesUnnamedPrioritiesByRank(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/priority/search", `{"isLast":true,"values":[
		{"id":"1","name":"P1"},{"id":"2","name":"P2"},{"id":"3","name":"P3"},{"id":"4","name":"P4"}
	]}`)
	site.reply("GET", "/rest/api/3/search/jql", `{"isLast":true,"issues":[
		{"key":"PE-1","fields":{"summary":"One","priority":{"name":"P1"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}},
		{"key":"PE-2","fields":{"summary":"Two","priority":{"name":"P4"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}}
	]}`)

	tasks, err := site.adapter().SyncIssues(context.Background(), tracker.SyncRequest{Project: jiraProject()})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Priority != models.PriorityUrgent || tasks[1].Priority != models.PriorityLow {
		t.Fatalf("priorities: %+v", tasks)
	}
	if calls := site.calls("GET", "/rest/api/3/priority/search"); len(calls) != 1 {
		t.Fatalf("the scheme must be read once per page, not %d times", len(calls))
	}
}

func sameJSON(got any, want map[string]any) bool {
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	return string(a) == string(b)
}
