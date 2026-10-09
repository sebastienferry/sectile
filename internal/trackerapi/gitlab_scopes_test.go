package trackerapi

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"tasks/internal/forgeoauth"
)

// gitlabPathScopes is every GitLab REST path this package calls, its variable
// segments written {} and its query left out, and the OAuth scope GitLab
// requires for it. read_api would serve the reads alone, but the writes need
// api, which includes it, and a consent asks once for both: every path is
// written with api. A new path lands here in the same change as its scope in
// forgeoauth.GitLab.Scopes and on the registered app.
var gitlabPathScopes = map[string][]string{
	"/projects/{}":                                  {"api"},
	"/projects/{}/issues":                           {"api"},
	"/projects/{}/issues/{}":                        {"api"},
	"/projects/{}/issues/{}/notes":                  {"api"},
	"/projects/{}/issues/{}/related_merge_requests": {"api"},
	"/projects/{}/members/all":                      {"api"},
	"/projects/{}/labels":                           {"api"},
	"/projects/{}/boards":                           {"api"},
	"/projects/{}/milestones":                       {"api"},
	"/projects/{}/milestones/{}":                    {"api"},
	"/projects/{}/merge_requests":                   {"api"},
	"/user":                                         {"api"},
}

// gitlabGraphQLScopes is every GraphQL operation this package sends to GitLab,
// written the way the source spells it, and the scope it needs. The
// iteration mutations cannot run under read_api.
var gitlabGraphQLScopes = map[string][]string{
	"iterationCadences(": {"api"},
	"iterations(":        {"api"},
	"iteration(id:":      {"api"},
	"issueSetIteration(": {"api"},
	"updateIteration(":   {"api"},
	"iterationDelete(":   {"api"},
}

// gitlabPathExpr matches a GitLab path literal and the segments appended to
// it: "/projects/" + gitlabProjectSegment(path) + "/issues", and
// fmt.Sprintf("/projects/%s/milestones/%d").
var gitlabPathExpr = regexp.MustCompile(`"(/projects/[^"]*)"((?:\s*\+\s*` + forgePathSegment + `)*)`)

// gitlabIssuePathExpr matches the helper building an issue's path, and the
// literals appended to it: gitlabIssuePath(path, iid) + "/notes".
var gitlabIssuePathExpr = regexp.MustCompile(`gitlabIssuePath\([^)]*\)((?:\s*\+\s*"[^"]*")*)`)

// gitlabAccountExpr matches the account read of CheckGitlab.
var gitlabAccountExpr = regexp.MustCompile(`checkAccount\([^"]*DefaultGitlabURL\),\s*"([^"]*)"`)

var gitlabGraphQLCallExpr = regexp.MustCompile(`\bc\.gitlabGraphQL\(`)

// calledGitlabPaths reads the package's own sources and answers every GitLab
// REST path they call, the way gitlabPathScopes writes them.
func calledGitlabPaths(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, line := range sourceLines(t) {
		for _, match := range gitlabPathExpr.FindAllStringSubmatch(line, -1) {
			seen[forgePath(match[1], match[2])] = true
		}
		for _, match := range gitlabIssuePathExpr.FindAllStringSubmatch(line, -1) {
			seen[forgePath("/projects/{}/issues/{}", match[1])] = true
		}
		for _, match := range gitlabAccountExpr.FindAllStringSubmatch(line, -1) {
			seen[match[1]] = true
		}
	}
	return sortedKeys(seen)
}

// TestEveryGitlabPathHasAScope: every GitLab REST endpoint Sectile calls maps
// to a scope the consent asks for.
func TestEveryGitlabPathHasAScope(t *testing.T) {
	paths := calledGitlabPaths(t)
	if len(paths) < 10 {
		t.Fatalf("the source scan found only %d paths: %v", len(paths), paths)
	}
	for _, path := range paths {
		scopes, known := gitlabPathScopes[path]
		if !known {
			t.Errorf("%s is called but has no scope in gitlabPathScopes", path)
			continue
		}
		for _, scope := range scopes {
			if !slices.Contains(forgeoauth.GitLab.Scopes, scope) {
				t.Errorf("%s needs %s, which forgeoauth.GitLab.Scopes does not ask for", path, scope)
			}
		}
	}
	// And the table names nothing the code no longer calls.
	for path := range gitlabPathScopes {
		if !slices.Contains(paths, path) {
			t.Errorf("%s is in gitlabPathScopes but no longer called", path)
		}
	}
}

// TestEveryGitlabGraphQLOperationHasAScope: every operation of the table is
// still sent, and the code sends no more operations than the table lists.
func TestEveryGitlabGraphQLOperationHasAScope(t *testing.T) {
	source := strings.Join(sourceLines(t), "\n")
	for operation := range gitlabGraphQLScopes {
		if !strings.Contains(source, operation) {
			t.Errorf("%s is in gitlabGraphQLScopes but no longer sent", operation)
		}
	}
	// One operation per call site: a new call lands in the table with its
	// scope.
	if calls := len(gitlabGraphQLCallExpr.FindAllString(source, -1)); calls != len(gitlabGraphQLScopes) {
		t.Errorf("%d GitLab GraphQL call sites for %d operations in gitlabGraphQLScopes", calls, len(gitlabGraphQLScopes))
	}
}

// TestGitlabConsentAsksForExactlyTheNeededScopes: the consent asks for every
// scope a call needs, and for nothing else.
func TestGitlabConsentAsksForExactlyTheNeededScopes(t *testing.T) {
	needed := map[string]bool{}
	for _, table := range []map[string][]string{gitlabPathScopes, gitlabGraphQLScopes} {
		for _, scopes := range table {
			for _, scope := range scopes {
				needed[scope] = true
			}
		}
	}
	asked := slices.Clone(forgeoauth.GitLab.Scopes)
	sort.Strings(asked)
	if got := sortedKeys(needed); !slices.Equal(got, asked) {
		t.Errorf("the calls need %v, the consent asks for %v", got, asked)
	}
}
