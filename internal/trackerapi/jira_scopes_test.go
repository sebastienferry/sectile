package trackerapi

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"tasks/internal/atlassian"
)

// jiraPathScopes is the table of FR16 (specs/654-jira-oauth-connect/spec.md):
// every Jira REST path this package calls, its variable segments written {},
// and the OAuth scopes Atlassian's reference requires for it. A call made
// through a person's grant to a path whose scope the consent did not ask for
// is refused by Atlassian, so a new path lands here, and its scope in
// atlassian.Scopes and on the registered app, in the same change.
var jiraPathScopes = map[string][]string{
	"/rest/api/3/myself":                            {"read:jira-user"},
	"/rest/api/3/search/jql":                        {"read:jira-work"},
	"/rest/api/3/field":                             {"read:jira-work"},
	"/rest/api/3/priority":                          {"read:jira-work"},
	"/rest/api/3/issue/createmeta/{}/issuetypes":    {"read:jira-work"},
	"/rest/api/3/issue/createmeta/{}/issuetypes/{}": {"read:jira-work"},
	"/rest/api/3/issue/{}/editmeta":                 {"read:jira-work"},
	"/rest/api/3/issue":                             {"write:jira-work"},
	"/rest/api/3/issue/{}":                          {"read:jira-work", "write:jira-work"},
	"/rest/api/3/issue/{}/transitions":              {"read:jira-work", "write:jira-work"},
	"/rest/api/3/issue/{}/comment":                  {"read:jira-work", "write:jira-work"},
	"/rest/api/3/issue/{}/comment/{}":               {"write:jira-work"},
	"/rest/api/3/issue/{}/assignee":                 {"write:jira-work"},
	"/rest/api/3/user/bulk":                         {"read:jira-user"},
	"/rest/api/3/user/assignable/search":            {"read:jira-user"},
	"/rest/api/3/jql/autocompletedata/suggestions":  {"read:jira-work"},
	"/rest/api/3/status":                            {"read:jira-work"},
	"/rest/api/3/project/{}/statuses":               {"read:jira-work"},
	"/rest/agile/1.0/board":                         {"read:board-scope:jira-software", "read:project:jira"},
	"/rest/agile/1.0/board/{}/configuration":        {"read:board-scope.admin:jira-software", "read:project:jira"},
	"/rest/agile/1.0/board/{}/sprint":               {"read:sprint:jira-software"},
	"/rest/agile/1.0/sprint":                        {"write:sprint:jira-software"},
	"/rest/agile/1.0/sprint/{}":                     {"write:sprint:jira-software", "delete:sprint:jira-software"},
	"/rest/agile/1.0/sprint/{}/issue":               {"write:sprint:jira-software"},
	"/rest/agile/1.0/backlog/issue":                 {"write:board-scope:jira-software"},
	// Requires manage:jira-configuration, an administration scope the consent
	// does not ask for: a client calling through a grant skips it and reads
	// the plain list above (readJiraPriorities).
	"/rest/api/3/priority/search": nil,
}

// jiraPathExpr matches a path literal and the escaped segments appended to
// it: "/rest/api/3/issue/" + url.PathEscape(key) + "/comment".
var jiraPathExpr = regexp.MustCompile(`"(/rest/(?:api/3|agile/1\.0)[^"]*)"((?:\s*\+\s*(?:url\.PathEscape\([^)]*\)|"[^"]*"))*)`)

var jiraSegmentExpr = regexp.MustCompile(`url\.PathEscape\([^)]*\)|"[^"]*"`)

// calledJiraPaths reads the package's own sources and answers every Jira path
// they call, the way jiraPathScopes writes them.
func calledJiraPaths(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, match := range jiraPathExpr.FindAllStringSubmatch(line, -1) {
				path := match[1]
				for _, segment := range jiraSegmentExpr.FindAllString(match[2], -1) {
					if strings.HasPrefix(segment, `"`) {
						path += strings.Trim(segment, `"`)
					} else {
						path += "{}"
					}
				}
				seen[path] = true
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// TestEveryJiraPathHasAScope is AC8: every Jira endpoint Sectile calls maps to
// a scope the consent asks for.
func TestEveryJiraPathHasAScope(t *testing.T) {
	paths := calledJiraPaths(t)
	if len(paths) < 20 {
		t.Fatalf("the source scan found only %d paths: %v", len(paths), paths)
	}
	for _, path := range paths {
		scopes, known := jiraPathScopes[path]
		if !known {
			t.Errorf("%s is called but has no scope in jiraPathScopes (spec FR16)", path)
			continue
		}
		for _, scope := range scopes {
			if !slices.Contains(atlassian.Scopes, scope) {
				t.Errorf("%s needs %s, which atlassian.Scopes does not ask for", path, scope)
			}
		}
	}
	// And the table names nothing the code no longer calls.
	for path := range jiraPathScopes {
		if !slices.Contains(paths, path) {
			t.Errorf("%s is in jiraPathScopes but no longer called", path)
		}
	}
}
