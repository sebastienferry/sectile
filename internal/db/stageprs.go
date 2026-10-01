package db

import (
	"fmt"
	"slices"
	"strings"

	"tasks/internal/agentprotocol"
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
	if len(given) == 0 && d.prDeferredBySpecArtifacts(task, actorID, skillID) {
		return stagePRSet{notice: prDeferredNotice}, nil
	}
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
	required := taskChangedRepositories(project, task)
	if primary != "" {
		required = append(required, primary)
	}
	own := projectRepositoryIdentities(project)
	chosen, fromGiven := map[string]string{}, map[string]bool{}
	for _, url := range given {
		link, ok := models.ParsePullRequestLink(url, "")
		if !ok || !slices.Contains(required, link.Identity()) {
			return stagePRSet{}, fmt.Errorf("pull request %s is not in a repository %s changed (%s); a repository becomes changed when prepare_repository_worktree is called for it", url, task.Key, strings.Join(required, ", "))
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
		if identity != primary && url == "" {
			if notice, unchanged := d.unchangedRepository(task, actorID, identity, branch); unchanged {
				notices = append(notices, notice)
				continue
			}
		}
		// Only the primary repository, when it is the project's own, is read
		// the way a single-repository ticket is: through the task checkout.
		// Every other repository is named to the agent, which answers from
		// that repository's own checkout, never from the primary worktree.
		target := repositoryTarget(identity)
		if identity == primary && slices.Contains(own, identity) {
			resolved, err := d.resolveStagePRTarget(project, url)
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
		// A repository the task prepared and left unchanged has no pull request
		// to check; the adjustment has no report to name it in.
		if url == "" {
			if _, unchanged := d.unchangedRepository(task, actorID, identity, branch); unchanged {
				continue
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

// unchangedRepository asks the agent whether the task branch carries no
// commit of its own in a secondary repository (#678). Only a verified "no"
// skips the repository; every other answer returns false and keeps the pull
// request required: an agent error, an agent too old to know the question, an
// answer for another repository, no checkout found, or commits ahead. The
// repository stays recorded as changed, so the question is asked again at the
// next check and a later commit there requires its pull request.
func (d *DB) unchangedRepository(task *models.Task, actorID, identity, branch string) (notice string, unchanged bool) {
	var answer struct {
		Repository    string `json:"repository"`
		Found         bool   `json:"found"`
		DefaultBranch string `json:"defaultBranch"`
		Exists        bool   `json:"exists"`
		Ahead         *int   `json:"ahead"`
	}
	op := agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "branch_changes", UserID: actorID, Repository: identity, Branch: branch}
	if err := d.callAgent(op, &answer); err != nil {
		return "", false
	}
	if answer.Repository != identity || !answer.Found || answer.DefaultBranch == "" || answer.DefaultBranch == branch || answer.Ahead == nil || *answer.Ahead != 0 {
		return "", false
	}
	_, request := evidenceTerms(repositoryTarget(identity).link.Forge)
	if !answer.Exists {
		return fmt.Sprintf("Prepared, unchanged: in %s, %s exists neither locally nor on origin, so no %s is expected there.", identity, branch, request), true
	}
	return fmt.Sprintf("Prepared, unchanged: %s has no commit of %s ahead of %s, so no %s is expected there.", identity, branch, answer.DefaultBranch, request), true
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

// noRepositoryChangeNotice ends the report of a stage recorded with the
// statement that the task changed no repository (#584), so the ticket says why
// it carries no pull request.
const noRepositoryChangeNotice = "No pull request: this task changed no repository."

// noRepositoryChangeEvidence accepts the statement that a task changed no
// repository in place of the pull request a stage requires (#584). The server
// cannot see the branch, so it refuses what it can see: a pull request recorded
// on the task's branch, or a repository recorded as changed through
// prepare_repository_worktree. Both say the task did change a repository. A
// stage that requires no pull request takes the statement as it is.
func (d *DB) noRepositoryChangeEvidence(task *models.Task, skillID, branch string) (stagePRSet, error) {
	if !d.stagePRRequired(task, skillID) {
		return stagePRSet{}, nil
	}
	for _, recorded := range task.PrLinks {
		if recorded.Branch == "" || recorded.Branch == branch {
			return stagePRSet{}, fmt.Errorf("%s records pull request %s on its branch, so it changed a repository: give that pull request instead of noRepositoryChange", task.Key, recorded.URL)
		}
	}
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return stagePRSet{}, fmt.Errorf("read project for the stage PR lookup: %w", err)
	}
	if changed := taskChangedRepositories(project, task); len(changed) > 0 {
		return stagePRSet{}, fmt.Errorf("%s changed %s through prepare_repository_worktree: give the pull request of each changed repository instead of noRepositoryChange", task.Key, strings.Join(changed, ", "))
	}
	return stagePRSet{notice: noRepositoryChangeNotice}, nil
}

// prDeferredNotice is added to the specified report of a project whose pull
// request would open at specification, when the workstation drops its
// specification artefacts and the branch therefore has nothing to show yet.
const prDeferredNotice = "Pull request deferred to the implemented stage: the specification artefacts are dropped on this workstation."

// prDeferredBySpecArtifacts says whether a clarified or specified transition
// may go without its pull request (#487, #580): the project opens it at one of
// those stages, and the agent of the reporting workstation says it drops the
// task's artefacts. Any other answer, an agent too old to know the question
// included, keeps the requirement as it was.
func (d *DB) prDeferredBySpecArtifacts(task *models.Task, actorID, skillID string) bool {
	skillID = models.NormalizeSkillID(skillID)
	if (skillID != "clarify" && skillID != "specify") || !d.stagePRRequired(task, skillID) {
		return false
	}
	var answer struct {
		Mode string `json:"mode"`
	}
	op := agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "spec_artifacts", UserID: actorID}
	if err := d.callAgent(op, &answer); err != nil {
		return false
	}
	return answer.Mode == models.SpecArtifactsDrop
}
