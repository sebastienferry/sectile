package trackerapi

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// The field ids, names and options below are synthetic, and deliberately
// not of the form a real site uses (#680).

const epicEditMeta = `{"fields":{
	"summary":{"name":"Summary","schema":{"type":"string","system":"summary"}},
	"priority":{"name":"Priority","schema":{"type":"priority","system":"priority"},"allowedValues":[{"id":"1","name":"High"}]},
	"cf-epic-rank":{"name":"Epic rank","schema":{"type":"option","custom":"com.example:select"},"allowedValues":[
		{"id":"o0","value":"P0"},{"id":"o1","value":"P1"},{"id":"old","value":"P9","disabled":true}]},
	"cf-epic-period":{"name":"Epic period","schema":{"type":"option-with-child","custom":"com.example:cascadingselect"},"allowedValues":[
		{"id":"y26","value":"2026","children":[{"id":"q4","value":"Q4"}]}]},
	"cf-free-text":{"name":"Notes","schema":{"type":"string","custom":"com.example:textfield"}},
	"cf-multi":{"name":"Tags","schema":{"type":"array","custom":"com.example:multiselect"},"allowedValues":[{"id":"t","value":"T"}]}
}}`

func TestJiraEpicAxisFieldCandidatesKeepClosedListCustomFields(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/issue/PE-4/editmeta", epicEditMeta)
	got, err := site.adapter().EpicAxisFieldCandidates(unattended(), jiraTracker(), "PE-4")
	if err != nil {
		t.Fatal(err)
	}
	want := []models.EpicFieldCandidate{
		{ID: "cf-epic-period", Name: "Epic period", Kind: models.EpicFieldCascade, Options: []models.EpicFieldOption{
			{ID: "y26", Value: "2026", Children: []models.EpicFieldOption{{ID: "q4", Value: "Q4"}}},
		}},
		{ID: "cf-epic-rank", Name: "Epic rank", Kind: models.EpicFieldSelect, Options: []models.EpicFieldOption{
			{ID: "o0", Value: "P0"}, {ID: "o1", Value: "P1"},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestJiraSetEpicAxisFieldSendsTheOptionOnly(t *testing.T) {
	site := newJiraSite(t)
	site.reply("PUT", "/rest/api/3/issue/PE-4", ``)
	adapter := site.adapter()
	ctx := unattended()
	selectField := models.EpicAxisField{ID: "cf-epic-rank", Kind: models.EpicFieldSelect}
	cascadeField := models.EpicAxisField{ID: "cf-epic-period", Kind: models.EpicFieldCascade}
	for _, write := range []struct {
		field models.EpicAxisField
		path  string
	}{{selectField, "o1"}, {cascadeField, "y26/q4"}, {selectField, ""}} {
		if err := adapter.SetEpicAxisField(ctx, jiraTracker(), "PE-4", write.field, write.path); err != nil {
			t.Fatal(err)
		}
	}
	calls := site.calls("PUT", "/rest/api/3/issue/PE-4")
	if len(calls) != 3 {
		t.Fatalf("%d writes", len(calls))
	}
	want := []string{
		`{"fields":{"cf-epic-rank":{"id":"o1"}}}`,
		`{"fields":{"cf-epic-period":{"child":{"id":"q4"},"id":"y26"}}}`,
		`{"fields":{"cf-epic-rank":null}}`,
	}
	for i, call := range calls {
		var compact any
		_ = json.Unmarshal([]byte(call.Body), &compact)
		body, _ := json.Marshal(compact)
		if string(body) != want[i] {
			t.Errorf("write %d = %s, want %s", i, body, want[i])
		}
	}
	if err := adapter.SetEpicAxisField(ctx, jiraTracker(), "PE-4", cascadeField, "y26"); err == nil {
		t.Error("a cascade path without a child was sent")
	}
}

func epicSearchFields(t *testing.T, site *jiraSite) []string {
	t.Helper()
	calls := site.calls("GET", "/rest/api/3/search/jql")
	if len(calls) != 1 {
		t.Fatalf("%d searches", len(calls))
	}
	query, _ := url.ParseQuery(calls[0].Query)
	return strings.Split(query.Get("fields"), ",")
}

func TestJiraListEpicsReadsTheMappedFields(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/search/jql", `{"issues":[
		{"key":"PE-1","fields":{"summary":"One","status":{"name":"To Do","statusCategory":{"key":"new"}},
			"cf-epic-rank":{"id":"o1","value":"P1"},"cf-epic-period":{"id":"y26","value":"2026","child":{"id":"q4","value":"Q4"}}}},
		{"key":"PE-2","fields":{"summary":"Two","status":{"name":"To Do","statusCategory":{"key":"new"}},"cf-epic-rank":null}}
	],"isLast":true}`)
	// The mapped fields stay on the project and travel beside its tracker (#741).
	mapped := models.EpicAxisFields{
		Priority: &models.EpicAxisField{ID: "cf-epic-rank", Kind: models.EpicFieldSelect},
		Quarter:  &models.EpicAxisField{ID: "cf-epic-period", Kind: models.EpicFieldCascade},
	}
	epics, err := site.adapter().ListEpics(unattended(), tracker.ProjectRequest{Tracker: jiraTracker(), EpicAxisFields: mapped})
	if err != nil {
		t.Fatal(err)
	}
	fields := epicSearchFields(t, site)
	if !containsAll(fields, "labels", "cf-epic-rank", "cf-epic-period") {
		t.Fatalf("fields asked = %v", fields)
	}
	if want := map[string]string{"cf-epic-rank": "o1", "cf-epic-period": "y26/q4"}; !reflect.DeepEqual(epics[0].AxisFieldValues, want) {
		t.Errorf("PE-1 values = %v", epics[0].AxisFieldValues)
	}
	if len(epics[1].AxisFieldValues) != 0 {
		t.Errorf("PE-2 values = %v, want none for an empty field", epics[1].AxisFieldValues)
	}
}

func TestJiraListEpicsWithoutMappedFieldsAsksTheSameFields(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/search/jql", `{"issues":[],"isLast":true}`)
	if _, err := site.adapter().ListEpics(unattended(), tracker.ProjectRequest{Tracker: jiraTracker()}); err != nil {
		t.Fatal(err)
	}
	if got := epicSearchFields(t, site); !reflect.DeepEqual(got, jiraBaseFields) {
		t.Fatalf("fields asked = %v, want the base fields only", got)
	}
}

func containsAll(list []string, want ...string) bool {
	seen := map[string]bool{}
	for _, v := range list {
		seen[v] = true
	}
	for _, w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}
