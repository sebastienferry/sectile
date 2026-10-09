package trackerapi

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"tasks/internal/forgeoauth"
)

// githubPathScopes is every GitHub REST path this package calls, its variable
// segments written {} and its query left out, and the classic OAuth scopes
// GitHub requires for it. A call made through a person's grant to a path whose
// scope the consent did not ask for is refused, or sees no private
// repository, so a new path lands here, and its scope in
// forgeoauth.GitHub.Scopes and on the registered app, in the same change.
var githubPathScopes = map[string][]string{
	"repos/{}":                    {"repo"},
	"repos/{}/issues":             {"repo"},
	"repos/{}/issues/{}":          {"repo"},
	"repos/{}/issues/{}/comments": {"repo"},
	"repos/{}/milestones":         {"repo"},
	"repos/{}/milestones/{}":      {"repo"},
	"repos/{}/pulls":              {"repo"},
	// Reading the login of the token's own account needs no scope.
	"user": nil,
}

// githubGraphQLScopes is every GraphQL operation this package sends to GitHub,
// written the way the source spells it, and the scopes it needs. The scan
// cannot read a query built in pieces, so this table is kept by hand, and
// TestEveryGithubGraphQLOperationHasAScope counts the call sites to catch a
// new one.
var githubGraphQLScopes = map[string][]string{
	"transferIssue(":                  {"repo"},
	"closedByPullRequestsReferences(": {"repo"},
	// The board status columns of GithubStatusQuery; without the scope the
	// read fails, and the board falls back to the open/closed state.
	"projectsV2(":         {"read:project"},
	"pullRequest(number:": {"repo"},
}

// forgePathSegment is one concatenated segment of a path expression: a
// literal, or a value (a variable, a call, an indexed field) written {}.
const forgePathSegment = `(?:"[^"]*"|[A-Za-z_][\w.]*(?:\[[^\]]*\])?(?:\([^)]*\))?)`

// githubPathExpr matches a GitHub path literal and the segments appended to
// it: "repos/" + repo + "/issues?state=all", and fmt.Sprintf("repos/%s/issues/%d").
var githubPathExpr = regexp.MustCompile(`"(repos/[^"]*)"((?:\s*\+\s*` + forgePathSegment + `)*)`)

// githubAccountExpr matches the account read of CheckGithub.
var githubAccountExpr = regexp.MustCompile(`checkAccount\([^"]*DefaultGithubURL\),\s*"/?([^"]*)"`)

var forgeSegmentExpr = regexp.MustCompile(`\+\s*(` + forgePathSegment + `)`)

var forgeFormatVerbExpr = regexp.MustCompile(`%[sdv]`)

var githubGraphQLCallExpr = regexp.MustCompile(`\bc\.graphql(?:Partial)?\(`)

// sourceLines answers every line of the package's non-test sources that is
// not a comment.
func sourceLines(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// forgePath normalises a matched path the way the scope tables write it: the
// appended values and the format verbs become {}, and the query is dropped.
func forgePath(literal, appended string) string {
	path := literal
	for _, match := range forgeSegmentExpr.FindAllStringSubmatch(appended, -1) {
		if strings.HasPrefix(match[1], `"`) {
			path += strings.Trim(match[1], `"`)
		} else {
			path += "{}"
		}
	}
	path = forgeFormatVerbExpr.ReplaceAllString(path, "{}")
	path, _, _ = strings.Cut(path, "?")
	return strings.TrimRight(path, "/")
}

func sortedKeys(seen map[string]bool) []string {
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// calledGithubPaths reads the package's own sources and answers every GitHub
// REST path they call, the way githubPathScopes writes them.
func calledGithubPaths(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, line := range sourceLines(t) {
		for _, match := range githubPathExpr.FindAllStringSubmatch(line, -1) {
			seen[forgePath(match[1], match[2])] = true
		}
		for _, match := range githubAccountExpr.FindAllStringSubmatch(line, -1) {
			seen[match[1]] = true
		}
	}
	return sortedKeys(seen)
}

// TestEveryGithubPathHasAScope: every GitHub REST endpoint Sectile calls maps
// to a scope the consent asks for.
func TestEveryGithubPathHasAScope(t *testing.T) {
	paths := calledGithubPaths(t)
	if len(paths) < 7 {
		t.Fatalf("the source scan found only %d paths: %v", len(paths), paths)
	}
	for _, path := range paths {
		scopes, known := githubPathScopes[path]
		if !known {
			t.Errorf("%s is called but has no scope in githubPathScopes", path)
			continue
		}
		for _, scope := range scopes {
			if !slices.Contains(forgeoauth.GitHub.Scopes, scope) {
				t.Errorf("%s needs %s, which forgeoauth.GitHub.Scopes does not ask for", path, scope)
			}
		}
	}
	// And the table names nothing the code no longer calls.
	for path := range githubPathScopes {
		if !slices.Contains(paths, path) {
			t.Errorf("%s is in githubPathScopes but no longer called", path)
		}
	}
}

// TestEveryGithubGraphQLOperationHasAScope: every operation of the table is
// still sent, and the code sends no more operations than the table lists.
func TestEveryGithubGraphQLOperationHasAScope(t *testing.T) {
	source := strings.Join(sourceLines(t), "\n")
	for operation := range githubGraphQLScopes {
		if !strings.Contains(source, operation) {
			t.Errorf("%s is in githubGraphQLScopes but no longer sent", operation)
		}
	}
	// One operation per call site: a new call lands in the table with its
	// scope.
	if calls := len(githubGraphQLCallExpr.FindAllString(source, -1)); calls != len(githubGraphQLScopes) {
		t.Errorf("%d GitHub GraphQL call sites for %d operations in githubGraphQLScopes", calls, len(githubGraphQLScopes))
	}
}

// TestGithubConsentAsksForExactlyTheNeededScopes: the consent asks for every
// scope a call needs, and for nothing else.
func TestGithubConsentAsksForExactlyTheNeededScopes(t *testing.T) {
	needed := map[string]bool{}
	for _, table := range []map[string][]string{githubPathScopes, githubGraphQLScopes} {
		for _, scopes := range table {
			for _, scope := range scopes {
				needed[scope] = true
			}
		}
	}
	asked := slices.Clone(forgeoauth.GitHub.Scopes)
	sort.Strings(asked)
	if got := sortedKeys(needed); !slices.Equal(got, asked) {
		t.Errorf("the calls need %v, the consent asks for %v", got, asked)
	}
}
