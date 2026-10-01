package trackerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"tasks/internal/tracker"
)

func markedRequest(id string) tracker.UpsertMarkedCommentRequest {
	return tracker.UpsertMarkedCommentRequest{
		Project:   jiraProject(),
		Key:       "PE-7",
		CommentID: id,
		Marker:    "sectile.macroTodos",
		Value:     map[string]any{"macroKey": "PE-7"},
		Body:      "### Todos\n\n1. ☐ First",
	}
}

func TestJiraMarkedCommentIsCreatedWithItsProperty(t *testing.T) {
	site := newJiraSite(t)
	site.reply("GET", "/rest/api/3/issue/PE-7/comment", `{"total":1,"comments":[{"id":"5","properties":[]}]}`)
	var posted map[string]any
	site.on("POST", "/rest/api/3/issue/PE-7/comment", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&posted)
		fmt.Fprint(w, `{"id":"42"}`)
	})

	id, err := site.adapter().UpsertMarkedComment(unattended(), markedRequest(""))
	if err != nil || id != "42" {
		t.Fatalf("created %q, %v", id, err)
	}
	if body, _ := posted["body"].(map[string]any); body["type"] != "doc" {
		t.Fatalf("the body must be ADF: %#v", posted["body"])
	}
	props, _ := posted["properties"].([]any)
	if len(props) != 1 || props[0].(map[string]any)["key"] != "sectile.macroTodos" {
		t.Fatalf("the comment must carry the marker: %#v", posted["properties"])
	}
	if len(site.calls("PUT", "/rest/api/3/issue/PE-7/comment/5")) != 0 {
		t.Fatal("a comment without the marker must never be edited")
	}
}

func TestJiraMarkedCommentIsUpdatedByItsID(t *testing.T) {
	site := newJiraSite(t)
	site.on("PUT", "/rest/api/3/issue/PE-7/comment/42", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"id":"42"}`) })

	id, err := site.adapter().UpsertMarkedComment(unattended(), markedRequest("42"))
	if err != nil || id != "42" {
		t.Fatalf("updated %q, %v", id, err)
	}
	if len(site.calls("GET", "/rest/api/3/issue/PE-7/comment")) != 0 || len(site.calls("POST", "/rest/api/3/issue/PE-7/comment")) != 0 {
		t.Fatal("a remembered comment is rewritten in place, with no search and no creation")
	}
}

func TestJiraMarkedCommentIsFoundAgainByItsMarker(t *testing.T) {
	site := newJiraSite(t)
	site.on("PUT", "/rest/api/3/issue/PE-7/comment/42", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errorMessages":["gone"]}`)
	})
	site.reply("GET", "/rest/api/3/issue/PE-7/comment", `{"total":2,"comments":[
		{"id":"5","properties":[{"key":"other.marker"}]},
		{"id":"9","properties":[{"key":"sectile.macroTodos"}]}]}`)
	site.on("PUT", "/rest/api/3/issue/PE-7/comment/9", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"id":"9"}`) })

	id, err := site.adapter().UpsertMarkedComment(unattended(), markedRequest("42"))
	if err != nil || id != "9" {
		t.Fatalf("found %q, %v", id, err)
	}
	if calls := site.calls("GET", "/rest/api/3/issue/PE-7/comment"); len(calls) != 1 || !strings.Contains(calls[0].Query, "expand=properties") {
		t.Fatalf("the search must read the properties: %+v", calls)
	}
	if len(site.calls("PUT", "/rest/api/3/issue/PE-7/comment/5")) != 0 || len(site.calls("POST", "/rest/api/3/issue/PE-7/comment")) != 0 {
		t.Fatal("only the marked comment may be written")
	}
}

func TestJiraMarkedCommentKeepsOtherRefusals(t *testing.T) {
	site := newJiraSite(t)
	site.on("PUT", "/rest/api/3/issue/PE-7/comment/42", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	if _, err := site.adapter().UpsertMarkedComment(unattended(), markedRequest("42")); err == nil {
		t.Fatal("a refusal other than a missing comment must be reported, not routed around")
	}
	if len(site.calls("POST", "/rest/api/3/issue/PE-7/comment")) != 0 {
		t.Fatal("no comment may be created after a refusal")
	}
}

func TestGithubMilestoneDescriptionIsReadAndWrittenEmptyIncluded(t *testing.T) {
	var sent []map[string]any
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /repos/acme/app/milestones/3":
			fmt.Fprint(w, `{"number":3,"title":"Q4","description":"Notes","state":"open","html_url":"https://github.com/acme/app/milestone/3"}`)
		case "PATCH /repos/acme/app/milestones/3":
			raw, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(raw, &payload)
			sent = append(sent, payload)
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	m, err := c.GetGithubMilestone("acme/app", "", 3)
	if err != nil || m.Description != "Notes" || m.HTMLURL == "" {
		t.Fatalf("milestone %+v, %v", m, err)
	}
	if err := c.SetGithubMilestoneDescription("acme/app", "", 3, ""); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || len(sent[0]) != 1 {
		t.Fatalf("only the description travels: %+v", sent)
	}
	if desc, ok := sent[0]["description"]; !ok || desc != "" {
		t.Fatalf("an empty description must be sent: %+v", sent[0])
	}
}

func TestIsTransientTellsPassingFailures(t *testing.T) {
	for _, err := range []error{
		&HTTPError{Status: http.StatusTooManyRequests},
		fmt.Errorf("write: %w", &HTTPError{Status: http.StatusBadGateway}),
		fmt.Errorf("tracker request failed: %w", &net.OpError{Op: "dial", Err: errors.New("refused")}),
		context.DeadlineExceeded,
	} {
		if !IsTransient(err) {
			t.Errorf("%v should be transient", err)
		}
	}
	for _, err := range []error{nil, &HTTPError{Status: http.StatusNotFound}, &HTTPError{Status: http.StatusForbidden}, &MissingPersonalCredentialError{Tracker: "jira"}, errors.New("refused")} {
		if IsTransient(err) {
			t.Errorf("%v should not be transient", err)
		}
	}
}
