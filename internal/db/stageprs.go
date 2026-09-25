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
	if len(taskChangedRepositories(project, task)) > 0 {
		return true
	}
	pinned := taskPin(project, task)
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
	// prUrl, the first link, names the primary repository's pull request:
	// it goes last so that it stays the ticket's current one.
	if len(given) > 1 {
		given = append(slices.Clone(given[1:]), given[0])
	}
	given = cleanURLs(given)
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return stagePRSet{}, fmt.Errorf("read project for the stage PR lookup: %w", err)
	}
	// A ticket launched from a view with a repository (#429) and no secondary
	// worktree changed that one repository, whatever its pin says.
	if project == nil || !multiRepoTask(project, task) || (taskViewRepository(project, task) != "" && len(taskChangedRepositories(project, task)) == 0) {
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

	primary := evidencePrimary(project, task)
	required := taskChangedRepositories(project, task)
	if primary != "" && !slices.Contains(required, primary) {
		required = append(required, primary)
	}
	own := projectRepositoryIdentities(project)
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
		// Only the primary repository, when it is the project's own, is read
		// the way a single-repository ticket is: through the task checkout.
		// Every other repository is named to the agent, which answers from
		// that repository's own checkout, never from the primary worktree.
		target := repositoryTarget(identity)
		if identity == primary && slices.Contains(own, identity) {
			resolved, err := d.resolveStagePRTarget(project, task, url)
			if err != nil {
				return stagePRSet{}, err
			}
			target = resolved
		} else if url != "" {
			link, _ := models.ParsePullRequestLink(url, "")
			target = stagePRTarget{link: link, url: url, foreign: true}
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
func (d *DB) checkSecondaryPRs(project *models.Project, task *models.Task, actorID, branch string) error {
	for _, identity := range taskChangedRepositories(project, task) {
		url := ""
		for _, recorded := range task.PrLinks {
			if recorded.Branch != "" && recorded.Branch != branch {
				continue
			}
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

// pullRequestLinkLast moves the link to url to the end of the set, which makes
// it the ticket's current pull request. Appending a link already recorded
// leaves it where it was, so on a ticket that changed several repositories
// the primary repository's could otherwise end up before another's.
func pullRequestLinkLast(links []models.TaskPullRequest, url string) []models.TaskPullRequest {
	for i, link := range links {
		if link.URL == url && i != len(links)-1 {
			moved := append(slices.Clone(links[:i]), links[i+1:]...)
			return append(moved, link)
		}
	}
	return links
}
