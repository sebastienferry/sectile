package db

import (
	"fmt"
	"slices"
	"strings"

	"tasks/internal/models"
)

// stagePRSet is what validateStagePRs accepted: the pull request of each
// repository the ticket changed, the primary repository's last, so that it is
// the ticket's current pull request once they are appended in order.
type stagePRSet struct {
	urls   []string
	notice string
}

// primary is the current pull request of the set, "" when it is empty.
func (s stagePRSet) primary() string {
	if len(s.urls) == 0 {
		return ""
	}
	return s.urls[len(s.urls)-1]
}

// multiRepoTask says whether a ticket's pull requests are checked per
// repository: it has changed a secondary repository, or it is pinned to a
// repository other than its project's own. Every other ticket keeps the single
// pull request of #392, unchanged.
func multiRepoTask(project *models.Project, task *models.Task) bool {
	if len(task.ChangedRepositories) > 0 {
		return true
	}
	pinned := strings.TrimSpace(task.Repository)
	return pinned != "" && !slices.Contains(projectRepositoryIdentities(project), pinned)
}

// validateStagePRs checks one pull request per repository the ticket changed
// (#456): its primary repository and every secondary one it has a worktree in.
// Each is looked up and checked with the rules of #392, on the head of its own
// checkout. The given links are matched to their repository; a repository
// without one is looked up by branch, and the recorded links of the ticket are
// tried before that. A link naming a repository the ticket did not change is
// refused, as is a changed repository without a pull request.
func (d *DB) validateStagePRs(task *models.Task, actorID, skillID, repoPath, branch string, given []string) (stagePRSet, error) {
	given = cleanURLs(given)
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return stagePRSet{}, fmt.Errorf("read project for the stage PR lookup: %w", err)
	}
	if project == nil || !multiRepoTask(project, task) {
		if len(given) > 1 {
			return stagePRSet{}, fmt.Errorf("%d pull requests given, but %s changed a single repository", len(given), task.Key)
		}
		url := ""
		if len(given) == 1 {
			url = given[0]
		}
		verified, notice, err := d.validateStagePR(task, actorID, skillID, repoPath, branch, url)
		if err != nil {
			return stagePRSet{}, err
		}
		set := stagePRSet{notice: notice}
		if verified != "" {
			set.urls = []string{verified}
		}
		return set, nil
	}
	if !d.stagePRRequired(task, skillID) {
		return stagePRSet{urls: given}, nil
	}

	primary := TaskPrimaryRepository(project, task)
	required := []string{}
	for _, identity := range append(slices.Clone(task.ChangedRepositories), primary) {
		if identity != "" && !slices.Contains(required, identity) {
			required = append(required, identity)
		}
	}
	chosen, fromGiven := map[string]string{}, map[string]bool{}
	for _, url := range given {
		link, ok := models.ParsePullRequestLink(url, "")
		if !ok || !slices.Contains(required, link.Identity()) {
			return stagePRSet{}, fmt.Errorf("pull request %s is not in a repository %s changed (%s)", url, task.Key, strings.Join(required, ", "))
		}
		if previous, twice := chosen[link.Identity()]; twice && previous != url {
			return stagePRSet{}, fmt.Errorf("two pull requests given for %s: %s and %s", link.Identity(), previous, url)
		}
		chosen[link.Identity()], fromGiven[link.Identity()] = url, true
	}
	for _, recorded := range task.PrLinks {
		link, ok := models.ParsePullRequestLink(recorded.URL, "")
		if !ok || !slices.Contains(required, link.Identity()) || (recorded.Branch != "" && recorded.Branch != branch) {
			continue
		}
		if !fromGiven[link.Identity()] {
			chosen[link.Identity()] = recorded.URL
		}
	}

	set := stagePRSet{}
	var notices []string
	for _, identity := range required {
		url := chosen[identity]
		target, err := d.resolveStagePRTarget(project, url)
		if err != nil {
			return stagePRSet{}, err
		}
		if url == "" && !slices.Contains(projectRepositoryIdentities(project), identity) {
			target = repositoryTarget(identity)
		}
		verified, notice, err := d.validateStagePRAt(task, actorID, skillID, repoPath, branch, url, target)
		if err != nil {
			return stagePRSet{}, fmt.Errorf("%s: %w", identity, err)
		}
		set.urls = append(set.urls, verified)
		if notice != "" {
			notices = append(notices, notice)
		}
	}
	set.notice = strings.Join(notices, "\n")
	return set, nil
}

// repositoryTarget reads a repository's pull request by branch when no link
// names it: GitHub for a host naming it, GitLab otherwise, the two forges
// stage evidence knows.
func repositoryTarget(identity string) stagePRTarget {
	host, path, _ := strings.Cut(identity, "/")
	forge := "gitlab"
	if strings.Contains(host, "github") {
		forge = "github"
	}
	return stagePRTarget{link: models.PullRequestLink{Forge: forge, Host: host, Repository: path}, foreign: true}
}

func cleanURLs(urls []string) []string {
	var out []string
	for _, url := range urls {
		if url = strings.TrimSpace(url); url != "" && !slices.Contains(out, url) {
			out = append(out, url)
		}
	}
	return out
}

// checkSecondaryPRs checks the pull request of every secondary repository a
// ticket changed as an adjustment requires: ready, on the ticket branch, on a
// clean checkout whose head it contains.
func (d *DB) checkSecondaryPRs(task *models.Task, actorID, branch string) error {
	for _, identity := range task.ChangedRepositories {
		url := ""
		for _, recorded := range task.PrLinks {
			if link, ok := models.ParsePullRequestLink(recorded.URL, ""); ok && link.Identity() == identity {
				url = recorded.URL
			}
		}
		target := repositoryTarget(identity)
		if url != "" {
			link, _ := models.ParsePullRequestLink(url, "")
			target = stagePRTarget{link: link, url: url, foreign: true}
		}
		if _, _, err := d.validateStagePRAt(task, actorID, "adjust", d.adjustmentCheckout(task), branch, url, target); err != nil {
			return fmt.Errorf("%s: %w", identity, err)
		}
	}
	return nil
}
