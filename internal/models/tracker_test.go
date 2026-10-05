package models

import (
	"reflect"
	"testing"
)

func TestTrackerIdentityFoldsTheSiteAndTheScopeLikeARepositoryIdentity(t *testing.T) {
	for _, c := range []struct {
		provider, site, scope string
		same                  [3]string
	}{
		{"jira", "https://Acme.Atlassian.net/", "pe", [3]string{"JIRA", "acme.atlassian.net", "PE"}},
		{"github", "https://API.github.com", " Owner/Repo.git ", [3]string{"github", "https://api.github.com/", "owner/repo"}},
		{"gitlab", "https://gitlab.example.org/api/v4", "/Group/App/", [3]string{"GitLab", "gitlab.example.org/api/v4/", "group/app"}},
	} {
		got, want := TrackerIdentity(c.provider, c.site, c.scope), TrackerIdentity(c.same[0], c.same[1], c.same[2])
		if got != want {
			t.Errorf("TrackerIdentity(%q, %q, %q) = %q, want %q", c.provider, c.site, c.scope, got, want)
		}
	}
	if got := TrackerIdentity("jira", "https://acme.atlassian.net", "PE"); got != "jira|acme.atlassian.net|PE" {
		t.Errorf("identity = %q", got)
	}
	if got := TrackerIdentity("local", "", "Proj-1"); got != "local||Proj-1" {
		t.Errorf("a local scope is the project id as it is: %q", got)
	}
}

func TestTrackerIdentityTellsTwoJiraSpacesOfOneSiteApart(t *testing.T) {
	gode, be := TrackerIdentity("jira", "https://acme.atlassian.net", "GODE"), TrackerIdentity("jira", "https://acme.atlassian.net", "BE")
	if gode == be {
		t.Fatalf("two spaces of one site share the identity %q", gode)
	}
	if other := TrackerIdentity("jira", "https://other.atlassian.net", "GODE"); other == gode {
		t.Fatalf("one key on two sites shares the identity %q", gode)
	}
}

func TestNormalizeProjectTrackersKeepsTheFirstOfTwoSameIdentities(t *testing.T) {
	got := NormalizeProjectTrackers([]ProjectTracker{
		{TrackerID: "t1", Identity: "jira|acme.atlassian.net|GODE"},
		{TrackerID: " ", Identity: ""},
		{TrackerID: "t2", Identity: "jira|acme.atlassian.net|BE"},
		{TrackerID: "t3", Identity: "jira|acme.atlassian.net|GODE"},
		{TrackerID: "t2", Identity: "jira|acme.atlassian.net|BE"},
	})
	want := []ProjectTracker{
		{TrackerID: "t1", Identity: "jira|acme.atlassian.net|GODE"},
		{TrackerID: "t2", Identity: "jira|acme.atlassian.net|BE"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeProjectTrackers = %+v, want %+v", got, want)
	}
}
