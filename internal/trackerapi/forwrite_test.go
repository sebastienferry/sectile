package trackerapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// ForWrite is the one rule every tracker write goes through (#482): a person
// writes with their own credential or not at all, unattended work writes with
// the server's, and a write naming neither is refused. It holds for the three
// providers alike.
func TestForWriteResolvesThePersonTheServerOrNothing(t *testing.T) {
	locked := errors.New("credential sealed and locked")
	for _, provider := range []string{"jira", "github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			c := &Client{
				GithubToken: "server-github", GitlabToken: "server-gitlab",
				JiraEmail: "service@example.com", JiraToken: "server-jira",
			}
			c.ResolveUser = func(userID, trackerName string) (string, string, string, error) {
				switch userID {
				case "u-ada":
					return "", "ada@example.com", "ada-" + trackerName, nil
				case "u-locked":
					return "", "", "", locked
				}
				return "", "", "", nil
			}
			token := func(client *Client) string {
				switch provider {
				case "jira":
					return client.JiraToken
				case "gitlab":
					return client.GitlabToken
				}
				return client.GithubToken
			}
			person := tracker.WithActingUser(context.Background(), "u-ada")

			client, err := c.ForWrite(person, provider, "p1")
			if err != nil || token(client) != "ada-"+provider {
				t.Fatalf("a person with a credential writes with it: %v %v", client, err)
			}

			_, err = c.ForWrite(tracker.WithActingUser(context.Background(), "u-grace"), provider, "p1")
			var missing *MissingPersonalCredentialError
			if !errors.As(err, &missing) || missing.Tracker != provider {
				t.Fatalf("a person without a credential is refused, never given the server's: %v", err)
			}
			if !strings.Contains(err.Error(), providerName(provider)) || !strings.Contains(err.Error(), "Profile → Tracker credentials") {
				t.Fatalf("the refusal names the provider and where to add a credential: %v", err)
			}

			if _, err = c.ForWrite(tracker.WithActingUser(context.Background(), "u-locked"), provider, "p1"); !errors.Is(err, locked) {
				t.Fatalf("a sealed credential nobody unlocked refuses the write: %v", err)
			}

			client, err = c.ForWrite(tracker.WithUnattended(context.Background()), provider, "p1")
			if err != nil || token(client) != "server-"+provider {
				t.Fatalf("unattended work writes with the server credential: %v %v", client, err)
			}

			if _, err = c.ForWrite(context.Background(), provider, "p1"); !errors.Is(err, ErrNoActingUser) {
				t.Fatalf("a write naming neither a person nor the marker is refused: %v", err)
			}
		})
	}
}

// Every write of the Jira and GitHub adapters goes through ForWrite: a person
// without a token of their own is refused before anything reaches the tracker,
// and so is a write that lost its author (#482).
func TestAdapterWritesAreRefusedWithoutAPersonalCredential(t *testing.T) {
	requests := 0
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer site.Close()

	client := &Client{HTTP: site.Client(), GithubURL: site.URL, GithubToken: "server-token",
		JiraURL: site.URL, JiraEmail: "service@example.com", JiraToken: "server-token"}
	client.ResolveUser = func(string, string) (string, string, string, error) { return "", "", "", nil }
	github, jira := NewGithubAdapter(client), NewJiraAdapter(client)
	ghProject := &models.Project{ID: "p1", GithubRepo: "acme/app"}
	jiraProject := jiraProject()

	writes := func(ctx context.Context) map[string]func() error {
		return map[string]func() error{
			"github CreateIssue": func() error {
				_, err := github.CreateIssue(ctx, tracker.CreateIssueRequest{Project: ghProject, Title: "T"})
				return err
			},
			"github UpdateIssue": func() error {
				return github.UpdateIssue(ctx, tracker.UpdateIssueRequest{Project: ghProject, Key: "#7"})
			},
			"github DeleteIssue": func() error {
				return github.DeleteIssue(ctx, tracker.DeleteIssueRequest{Project: ghProject, Key: "#7"})
			},
			"github AddComment": func() error {
				return github.AddComment(ctx, tracker.AddCommentRequest{Project: ghProject, Key: "#7", Body: "b"})
			},
			"github UpdateLabels": func() error { return github.UpdateLabels(ctx, "#7", []string{"a"}, nil) },
			"jira CreateIssue": func() error {
				_, err := jira.CreateIssue(ctx, tracker.CreateIssueRequest{Project: jiraProject, Title: "T"})
				return err
			},
			"jira UpdateIssue": func() error {
				return jira.UpdateIssue(ctx, tracker.UpdateIssueRequest{Project: jiraProject, Key: "PE-1"})
			},
			"jira DeleteIssue": func() error {
				return jira.DeleteIssue(ctx, tracker.DeleteIssueRequest{Project: jiraProject, Key: "PE-1"})
			},
			"jira AddComment": func() error {
				return jira.AddComment(ctx, tracker.AddCommentRequest{Project: jiraProject, Key: "PE-1", Body: "b"})
			},
			"jira Assign":       func() error { return jira.Assign(ctx, "PE-1", "acc") },
			"jira Transition":   func() error { return jira.Transition(ctx, "PE-1", "Done") },
			"jira SetSprint":    func() error { return jira.SetSprint(ctx, "12", []string{"PE-1"}) },
			"jira SetTeam":      func() error { return jira.SetTeam(ctx, "PE-1", "team") },
			"jira SetParent":    func() error { return jira.SetParent(ctx, "PE-1", "PE-2") },
			"jira UpdateLabels": func() error { return jira.UpdateLabels(ctx, "PE-1", []string{"a"}, nil) },
			"jira CreateSprint": func() error {
				_, err := jira.CreateSprint(ctx, tracker.SprintCreateRequest{Project: jiraProject, BoardID: "3", Name: "S"})
				return err
			},
			"jira UpdateSprint": func() error {
				name := "S2"
				_, err := jira.UpdateSprint(ctx, jiraProject, "12", models.SprintPatch{Name: &name})
				return err
			},
			"jira DeleteSprint": func() error { return jira.DeleteSprint(ctx, jiraProject, "12") },
		}
	}

	person := tracker.WithActingUser(context.Background(), "u-grace")
	for name, call := range writes(person) {
		var missing *MissingPersonalCredentialError
		if err := call(); !errors.As(err, &missing) {
			t.Errorf("%s: a person without a token must be refused, got %v", name, err)
		}
	}
	for name, call := range writes(context.Background()) {
		if err := call(); !errors.Is(err, ErrNoActingUser) {
			t.Errorf("%s: a write that names nobody must be refused, got %v", name, err)
		}
	}
	if requests != 0 {
		t.Fatalf("a refused write must reach nothing, the tracker received %d request(s)", requests)
	}
}

// Reads are out of #482: a GitHub person without a token still reads with the
// server credential, whatever the call.
func TestGithubReadsKeepTheServerTokenForAPersonWithoutOne(t *testing.T) {
	var seen []string
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer site.Close()

	client := &Client{HTTP: site.Client(), GithubURL: site.URL, GithubToken: "server-token"}
	client.ResolveUser = func(string, string) (string, string, string, error) { return "", "", "", nil }
	adapter := NewGithubAdapter(client)
	project := &models.Project{ID: "p1", GithubRepo: "acme/app"}
	ctx := tracker.WithActingUser(context.Background(), "u-grace")

	if _, err := adapter.SyncIssues(ctx, tracker.SyncRequest{Project: project}); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.GetComments(ctx, tracker.GetCommentsRequest{Project: project, Key: "#7"}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "Bearer server-token" || seen[1] != "Bearer server-token" {
		t.Fatalf("reads keep the server token for a person without one, got %v", seen)
	}
}
