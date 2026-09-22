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
	jiraServerScheme  = jiraPriorityScheme{{ID: "1", Name: "Blocker"}, {ID: "2", Name: "Major"}, {ID: "3", Name: "Minor"}}
	jiraEquativScheme = jiraPriorityScheme{{ID: "1", Name: "Blocker"}, {ID: "2", Name: "Critical"}, {ID: "3", Name: "Major"}, {ID: "4", Name: "Minor"}, {ID: "5", Name: "Trivial"}}
	jiraFrenchScheme  = jiraPriorityScheme{{ID: "10", Name: "La plus élevée"}, {ID: "11", Name: "Élevée"}, {ID: "12", Name: "Moyenne"}, {ID: "13", Name: "Basse"}}
	jiraNumberScheme  = jiraPriorityScheme{{ID: "20", Name: "P1"}, {ID: "21", Name: "P2"}, {ID: "22", Name: "P3"}, {ID: "23", Name: "P4"}, {ID: "24", Name: "P5"}}
	jiraCloudScheme   = jiraPriorityScheme{{ID: "1", Name: "Highest"}, {ID: "2", Name: "High"}, {ID: "3", Name: "Medium"}, {ID: "4", Name: "Low"}, {ID: "5", Name: "Lowest"}}
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
		// The Server scheme is read as the sites running it use it: Blocker
		// alone is urgent, Critical is the level under it.
		{"Critique", jiraFrenchScheme, models.PriorityHigh},
		{"La plus élevée", jiraFrenchScheme, models.PriorityUrgent},
		{"Critical", jiraEquativScheme, models.PriorityHigh},
		// Major is where most work items of such a site sit: the ordinary
		// level, not an elevated one.
		{"Major", jiraEquativScheme, models.PriorityMedium},
		{"Major", jiraServerScheme, models.PriorityMedium},
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
		{"P1 - Critical", jiraNumberScheme, models.PriorityHigh},
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
		// The scheme project PE runs: the four Sectile levels each find a
		// home of their own, and Trivial is the one level with no writer.
		{models.PriorityUrgent, jiraEquativScheme, "1"},
		{models.PriorityHigh, jiraEquativScheme, "2"},
		{models.PriorityMedium, jiraEquativScheme, "3"},
		{models.PriorityLow, jiraEquativScheme, "4"},
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
	for _, scheme := range []jiraPriorityScheme{jiraNumberScheme, jiraEquativScheme, jiraCloudScheme, jiraFrenchScheme} {
		for _, p := range []models.Priority{models.PriorityUrgent, models.PriorityHigh, models.PriorityMedium, models.PriorityLow} {
			option, _ := scheme.option(p)
			if got := jiraPriority(option.Name, scheme); got != p {
				t.Errorf("writing %q wrote %q, which reads back as %q", p, option.Name, got)
			}
		}
	}
}

// The bug, as the reporting site has it. equativ.atlassian.net carries ten
// priorities — Jira Server's Blocker…Trivial and Atlassian's Highest…Lowest —
// but project PE's scheme holds only the first five. So "Highest" is a name
// the *site* has and the *project* refuses, and every write Sectile made was
// answered with "priority: The priority selected is invalid". The screen that
// will receive the write is what names the options it may use.
const jiraPEScheme = `[
	{"id":"1","name":"Blocker"},{"id":"2","name":"Critical"},{"id":"3","name":"Major"},
	{"id":"4","name":"Minor"},{"id":"5","name":"Trivial"}
]`

// jiraTenPriorities is the site's own list: every option of every scheme, the
// five the project refuses included. Reading a write off this is what still
// sent "Medium" to a project that has no such option.
const jiraTenPriorities = `{"isLast":true,"values":[
	{"id":"2","name":"Critical"},{"id":"1","name":"Blocker"},{"id":"10000","name":"Highest"},
	{"id":"3","name":"Major"},{"id":"10001","name":"High"},{"id":"10002","name":"Medium"},
	{"id":"4","name":"Minor"},{"id":"5","name":"Trivial"},{"id":"10004","name":"Lowest"},
	{"id":"10003","name":"Low"}
]}`

func jiraSiteWithScreens(t *testing.T) *jiraSite {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/priority/search", jiraTenPriorities)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes", `{"total":1,"issueTypes":[{"id":"3","name":"Task"}]}`)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", `{"total":1,"fields":[{"fieldId":"priority","name":"Priority","allowedValues":`+jiraPEScheme+`}]}`)
	site.reply("GET", "/rest/api/3/issue/PE-7/editmeta", `{"fields":{"priority":{"allowedValues":`+jiraPEScheme+`}}}`)
	site.reply("GET", "/rest/api/3/issue/PE-42", `{"key":"PE-42","fields":{"summary":"New","priority":{"name":"Blocker"},"status":{"name":"To Do","statusCategory":{"key":"new"}},"labels":[]}}`)
	return site
}

func TestJiraWritesAPriorityTheProjectsSchemeHas(t *testing.T) {
	site := jiraSiteWithScreens(t)
	var created, updated map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	site.on("PUT", "/rest/api/3/issue/PE-7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&updated)
		w.WriteHeader(http.StatusNoContent)
	})

	task, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent})
	if err != nil {
		t.Fatal(err)
	}
	// Blocker, the project's own top option — not "Highest", which the site
	// has and the project refuses.
	if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "1"}) {
		t.Fatalf("creation must use the project's scheme: %#v", got)
	}
	if task.Priority != models.PriorityUrgent {
		t.Fatalf("read back as %q", task.Priority)
	}

	// An update is judged by the work item's own edit screen, which is the
	// only place a key alone can name its project and type.
	low := models.PriorityLow
	if err := site.adapter().UpdateIssue(context.Background(), tracker.UpdateIssueRequest{Project: jiraProject(), Key: "PE-7", Priority: &low}); err != nil {
		t.Fatal(err)
	}
	if got := updated["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "4"}) {
		t.Fatalf("update must use the project's scheme: %#v", got)
	}
	// The creation screen is asked once per project and type, not once per
	// write; the edit screen belongs to one work item and is not remembered.
	if calls := site.calls("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3"); len(calls) != 1 {
		t.Fatalf("the creation screen must be read once, not %d times", len(calls))
	}
}

// Project PE carries no priority on its creation screens at all, while its
// edit screen does. Sending the field on creation is refused ("Field
// 'priority' cannot be set"), which would fail the whole creation; leaving it
// out and stopping there would drop the level the caller asked for. So the
// creation goes out without it and the level is put on afterwards.
func TestJiraPutsThePriorityOnAfterACreationScreenRefusesIt(t *testing.T) {
	site := jiraSiteWithScreens(t)
	site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", `{"total":1,"fields":[{"fieldId":"summary","name":"Summary","required":true}]}`)
	site.reply("GET", "/rest/api/3/issue/PE-42/editmeta", `{"fields":{"priority":{"allowedValues":`+jiraPEScheme+`}}}`)
	var created, put map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})
	site.on("PUT", "/rest/api/3/issue/PE-42", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&put)
		w.WriteHeader(http.StatusNoContent)
	})

	if _, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent}); err != nil {
		t.Fatal(err)
	}
	if _, ok := created["fields"].(map[string]any)["priority"]; ok {
		t.Fatalf("a screen without the field must receive no priority: %#v", created["fields"])
	}
	if got := put["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "1"}) {
		t.Fatalf("the level must be put on afterwards, from the edit screen: %#v", put)
	}
	if _, ok := put["fields"].(map[string]any)["summary"]; ok {
		t.Fatalf("nothing but the priority must travel: %#v", put)
	}
}

// Neither screen takes the field, or the site refuses the follow-up: the work
// item exists, so the creation stands. The caller is answered with the
// priority the site actually holds rather than the one it asked for.
func TestJiraKeepsACreationThatCouldNotTakeItsPriority(t *testing.T) {
	for _, c := range []struct {
		name     string
		editmeta http.HandlerFunc
		put      http.HandlerFunc
		wantPuts int
	}{
		{"the edit screen has no priority either", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"fields":{"summary":{}}}`)
		}, nil, 0},
		{"the site refuses the follow-up", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"fields":{"priority":{"allowedValues":`+jiraPEScheme+`}}}`)
		}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"errors":{"priority":"The priority selected is invalid."}}`)
		}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			site := jiraSiteWithScreens(t)
			site.reply("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", `{"total":1,"fields":[{"fieldId":"summary","name":"Summary","required":true}]}`)
			site.on("GET", "/rest/api/3/issue/PE-42/editmeta", c.editmeta)
			site.reply("POST", "/rest/api/3/issue", `{"id":"1","key":"PE-42"}`)
			if c.put != nil {
				site.on("PUT", "/rest/api/3/issue/PE-42", c.put)
			}

			task, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent})
			if err != nil {
				t.Fatalf("the work item exists; its creation must stand: %v", err)
			}
			// PE-42 comes back carrying Blocker in this fixture, which is what
			// the site holds — the answer is never the level that did not stick.
			if task.Key != "PE-42" || task.Priority != models.PriorityUrgent {
				t.Fatalf("created task: %+v", task)
			}
			if puts := site.calls("PUT", "/rest/api/3/issue/PE-42"); len(puts) != c.wantPuts {
				t.Fatalf("got %d follow-up writes, want %d", len(puts), c.wantPuts)
			}
		})
	}
}

// A screen that cannot be read falls back on the site's list, which is a
// better guess than the default names.
func TestJiraFallsBackOnTheSiteListWhenTheScreenCannotBeRead(t *testing.T) {
	site := jiraSiteWithScreens(t)
	site.on("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"errorMessages":["no permission"]}`)
	})
	var created map[string]any
	site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&created)
		fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
	})

	if _, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent}); err != nil {
		t.Fatal(err)
	}
	// Blocker: the first option the site list names urgent. It happens to be
	// one the project has too, which is luck rather than design — the site
	// list knows nothing of the project's scheme.
	if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, map[string]any{"id": "1"}) {
		t.Fatalf("an unreadable screen must fall back on the site list: %#v", got)
	}
}

// An older site serves the bare list instead of the paginated search, and a
// site that answers neither still gets its write, under the default name.
func TestJiraFallsBackThroughTheBareListToTheDefaultNames(t *testing.T) {
	refuse := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"errorMessages":["no permission"]}`)
	}
	for _, c := range []struct {
		name     string
		bareList http.HandlerFunc
		want     map[string]any
	}{
		{"the bare list answers", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"id":"1","name":"Blocker"},{"id":"3","name":"Major"},{"id":"4","name":"Minor"}]`)
		}, map[string]any{"id": "1"}},
		{"nothing answers", refuse, map[string]any{"name": "Highest"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			site := jiraSiteWithScreens(t)
			site.on("GET", "/rest/api/3/issue/createmeta/PE/issuetypes/3", refuse)
			site.on("GET", "/rest/api/3/priority/search", refuse)
			site.on("GET", "/rest/api/3/priority", c.bareList)
			var created map[string]any
			site.on("POST", "/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&created)
				fmt.Fprint(w, `{"id":"1","key":"PE-42"}`)
			})

			if _, err := site.adapter().CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: jiraProject(), Title: "New", Priority: models.PriorityUrgent}); err != nil {
				t.Fatal(err)
			}
			if got := created["fields"].(map[string]any)["priority"]; !sameJSON(got, c.want) {
				t.Fatalf("got %#v, want %#v", got, c.want)
			}
		})
	}
}

// A synchronisation reads the site's list once for the whole page, not once
// per work item: a read has no screen to ask and no project scheme to narrow.
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
