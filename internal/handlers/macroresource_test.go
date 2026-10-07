package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestMacroResourceHTTPReads(t *testing.T) {
	database, server, project := macroAxesFixture(t, "local")
	title := "Scoped macro"
	for _, key := range []string{"M-1", "Key/with%slash"} {
		if _, err := database.SaveMacroMeta(project.ID, key, nil, &title, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.SaveMacroMeta("default", "M-1", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"macros", "epics"} {
		for _, test := range []struct {
			suffix     string
			status     int
			collection bool
		}{
			{"", 200, true}, {"/M-1", 200, false}, {"/" + url.PathEscape("Key/with%slash"), 200, false},
			{"/missing", 404, false}, {"/M-1/unknown", 404, false}, {"/M-1/runs", 200, false},
		} {
			response, err := http.Get(server.URL + "/api/projects/" + project.ID + "/" + alias + test.suffix)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != test.status {
				t.Fatalf("%s%s: %d %s %v", alias, test.suffix, response.StatusCode, body, err)
			}
			if test.status != 200 || test.suffix == "/M-1/runs" {
				continue
			}
			if test.collection {
				var macros []any
				if err := json.Unmarshal(body, &macros); err != nil || len(macros) != 2 {
					t.Fatalf("collection changed: %s %v", body, err)
				}
			} else {
				var out struct {
					Macro struct {
						ProjectID   string `json:"projectId"`
						Description string `json:"description"`
					} `json:"macro"`
				}
				if err := json.Unmarshal(body, &out); err != nil || out.Macro.ProjectID != project.ID || out.Macro.Description != title {
					t.Fatalf("single resource: %s %v", body, err)
				}
			}
		}
	}
	response, err := http.Get(server.URL + "/api/projects/missing/macros/M-1")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatalf("unknown project: %d", response.StatusCode)
	}
}
