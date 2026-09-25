package db

import (
	"fmt"
	"slices"
	"strings"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

func (d *DB) prCreationOwner(task *models.Task) string {
	p, _ := d.GetProjectByID(task.ProjectID)
	if p != nil && p.PRCreationStage == "specified" {
		return "specify"
	}
	return "implement"
}

func (d *DB) adjustmentPrerequisite(task *models.Task, actorID string, ready bool) (trackerapi.PullRequest, error) {
	origin, _ := adjustmentOverrideOrigin(d.projectSkillOverrides(task.ProjectID))
	settings, _ := d.GetSettings()
	project, _ := d.GetProjectByID(task.ProjectID)
	if origin != "adjust" && project != nil && strings.TrimSpace(project.SkillOverrides["adjust"]) == "" && (strings.TrimSpace(project.SkillOverrides["review"]) != "") {
		return trackerapi.PullRequest{}, fmt.Errorf("legacy command override requires reconciliation in Skills")
	}
	if origin == "review" || (origin != "adjust" && settings != nil && strings.TrimSpace(settings.PromptCreatePR) != "") {
		return trackerapi.PullRequest{}, fmt.Errorf("legacy adjustment customization requires reconciliation in Skills: review and save under Adjust or reset")
	}
	branch := ""
	if task.BranchName != nil {
		branch = *task.BranchName
	}
	// The PR to adjust is the task's current one: when it lives in another
	// repository, that is where the branch is looked up.
	// A mono-repo project never reads another repository; its prerequisite finds
	// the PR by branch, as it always did, even when the recorded link names an
	// older name of the repository.
	target, err := d.resolveStagePRTarget(project, models.CurrentPullRequest(task.PrLinks))
	if err != nil {
		target = stagePRTarget{}
	}
	if project != nil && multiRepoTask(project, task) {
		// A ticket pinned elsewhere reads its pull request in its own
		// repository, and every secondary repository it changed must have a
		// ready pull request on a clean checkout before it is adjusted (#456).
		if primary := TaskPrimaryRepository(project, task); !target.foreign && !slices.Contains(projectRepositoryIdentities(project), primary) {
			target = repositoryTarget(primary)
		}
		if err := d.checkSecondaryPRs(task, actorID, branch); err != nil {
			return trackerapi.PullRequest{}, fmt.Errorf("adjustment prerequisite: %w", err)
		}
	}
	pr, err := d.lookupStagePR(task, actorID, d.adjustmentCheckout(task), branch, target)
	if err != nil {
		return pr, fmt.Errorf("adjustment prerequisite: %w (creation owner: %s)", err, d.prCreationOwner(task))
	}
	// A task holds several PRs over its life: the one the forge reports for the
	// branch is a follow-up as long as it shares a branch with a recorded link.
	if err = models.AcceptPullRequest(task.PrLinks, pr.URL, pr.Branch); err != nil {
		return pr, err
	}
	if ready && pr.Draft {
		_, request := evidenceTerms(pr.Forge)
		return pr, fmt.Errorf("adjustment requires a ready %s", request)
	}
	if len(models.AppendPullRequestLink(task.PrLinks, pr.URL, pr.Branch)) == len(task.PrLinks) {
		return pr, nil
	}
	// The link is appended to the set on the locked row, not to the snapshot
	// the forge lookup started from, so a link attached meanwhile survives.
	var links []models.TaskPullRequest
	d.mu.Lock()
	err = d.conn.WithTx(func(tx *sqlTx) error {
		locked, err := d.lockTaskUnsafe(tx, task.ID)
		if err != nil || locked == nil {
			return err
		}
		links = models.AppendPullRequestLink(locked.PrLinks, pr.URL, pr.Branch)
		if len(links) == len(locked.PrLinks) {
			return nil
		}
		_, err = tx.Exec("UPDATE tasks SET pr_url = ?, pr_links = ?, pr_links_detached = 0 WHERE id = ?",
			pullRequestURLValue(links), encodePullRequestLinks(links), task.ID)
		return err
	})
	d.mu.Unlock()
	if err != nil {
		return pr, err
	}
	if links != nil {
		task.PrLinks = links
		task.PrURL = pullRequestURLValue(links)
	}
	return pr, nil
}

// evidenceTerms words a refusal in the terms of the forge that answered, so a
// GitLab user is told about a merge request rather than a GitHub PR.
func evidenceTerms(forge string) (source, request string) {
	if forge == "gitlab" {
		return "GitLab", "merge request"
	}
	return "forge", "PR"
}

func validatePullRequestEvidence(pr trackerapi.PullRequest, branch, url string, recorded []models.TaskPullRequest, ready bool) error {
	source, request := evidenceTerms(pr.Forge)
	// A merged PR is accepted: the human merge is the boundary adjustment stops at, not a reason to strand the task.
	if (!pr.Open && !pr.Merged) || pr.Branch != branch || pr.URL == "" || pr.URL != url {
		return fmt.Errorf("%s does not confirm the matching open or merged %s", source, request)
	}
	if err := models.AcceptPullRequest(recorded, pr.URL, pr.Branch); err != nil {
		return err
	}
	if ready && pr.Draft {
		return fmt.Errorf("adjustment %s is still a draft", request)
	}
	return nil
}

// validateStagePR checks the pull request a stage names and returns its URL,
// plus a notice when the evidence is weaker than usual: a pull request in
// another repository whose head no local checkout could confirm. The notice is
// part of the stage's report, never a reason to refuse it.
func (d *DB) validateStagePR(task *models.Task, actorID, skillID, repoPath, branch, url string) (string, string, error) {
	if !d.stagePRRequired(task, skillID) {
		return url, "", nil
	}
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return "", "", fmt.Errorf("read project for the stage PR lookup: %w", err)
	}
	target, err := d.resolveStagePRTarget(project, url)
	if err != nil {
		return "", "", err
	}
	return d.validateStagePRAt(task, actorID, skillID, repoPath, branch, url, target)
}

// stagePRRequired says whether a skill's stage needs pull request evidence.
func (d *DB) stagePRRequired(task *models.Task, skillID string) bool {
	skillID = models.NormalizeSkillID(skillID)
	return skillID == "create_pr" || skillID == "adjust" || skillID == "pickup" || skillID == "implement" || (skillID == "specify" && d.prCreationOwner(task) == "specify")
}

// validateStagePRAt is validateStagePR once the repository the pull request
// is read in is known, which a multi-repo ticket decides per repository.
func (d *DB) validateStagePRAt(task *models.Task, actorID, skillID, repoPath, branch, url string, target stagePRTarget) (string, string, error) {
	skillID = models.NormalizeSkillID(skillID)
	pr, err := d.lookupStagePR(task, actorID, repoPath, branch, target)
	if err != nil {
		return "", "", err
	}
	if url == "" {
		url = pr.URL
	}
	ready := skillID == "adjust" || skillID == "pickup"
	if err = validatePullRequestEvidence(pr, branch, url, task.PrLinks, ready); err != nil {
		return "", "", err
	}
	var evidence struct {
		Repository string
		Found      bool
		SHA        string
		Branch     string
		Clean      bool
	}
	op := agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "git_evidence", UserID: actorID}
	if target.foreign {
		op.Repository, op.Branch = target.link.Identity(), branch
	}
	if err = d.callAgent(op, &evidence); err != nil {
		return "", "", err
	}
	if target.foreign {
		if evidence.Repository != op.Repository {
			return "", "", errAgentTooOld
		}
		if !evidence.Found {
			// Q2 of #392: without a checkout the agent can prove is that
			// repository's, the forge's word stands, and the report says so.
			return url, fmt.Sprintf("Head commit not verified on a local checkout: no checkout of %s on %s is known to the agent (pin the task's repository to verify it).", op.Repository, branch), nil
		}
	}
	if evidence.Branch != branch || evidence.SHA != pr.SHA {
		_, request := evidenceTerms(pr.Forge)
		return "", "", fmt.Errorf("%s does not contain the agent checkout commit", request)
	}
	if ready && !evidence.Clean {
		return "", "", fmt.Errorf("agent checkout contains uncommitted changes")
	}

	return url, "", nil
}

// errAgentTooOld is a failed lookup: an agent that ignores the repository of a
// question would answer for the project checkout instead.
var errAgentTooOld = fmt.Errorf("local agent is too old to look up a pull request in another repository; update it")

// stagePRTarget says where a stage's pull request is read: the project
// repository, as always, or the repository its link names.
type stagePRTarget struct {
	link    models.PullRequestLink
	url     string
	foreign bool
}

// resolveStagePRTarget decides whether prURL names a pull request outside the
// project repository, and whether the project may record one (Q1 of #392): a
// project without a code remote, or not mono-repo, may; a mono-repo project
// with a remote keeps the same-repository rule. A link that is not a
// recognized pull request keeps the project path, as before: the forge answer
// never matches it, so the evidence check refuses it there.
func (d *DB) resolveStagePRTarget(project *models.Project, prURL string) (stagePRTarget, error) {
	prURL = strings.TrimSpace(prURL)
	if prURL == "" {
		return stagePRTarget{}, nil
	}
	link, ok := models.ParsePullRequestLink(prURL, "")
	own := projectRepositoryIdentities(project)
	if !ok || slices.Contains(own, link.Identity()) {
		return stagePRTarget{}, nil
	}
	if len(own) > 0 && project != nil && project.MonoRepo {
		return stagePRTarget{}, fmt.Errorf("pull request %s is not in the project repository %s; only a project without a code remote, or not mono-repo, may record one from another repository", prURL, own[0])
	}
	return stagePRTarget{link: link, url: prURL, foreign: true}, nil
}

// projectRepositoryIdentities names the project's own repository in
// models.RepositoryIdentity form: its code remote when that names a host, and
// its GitHub repository, which is where its pull requests live when the remote
// is a mirror or an unbranded host (see stagePRForge). Empty when it has none.
func projectRepositoryIdentities(p *models.Project) []string {
	if p == nil {
		return nil
	}
	var own []string
	if remoteHost(p.GitRemoteUrl) != "" {
		own = append(own, models.RepositoryIdentity(p.GitRemoteUrl))
	}
	if repo := strings.TrimSpace(p.GithubRepo); repo != "" {
		if strings.Contains(repo, "://") || strings.Contains(repo, "@") {
			own = append(own, models.RepositoryIdentity(repo))
		} else {
			own = append(own, strings.ToLower("github.com/"+strings.Trim(repo, "/")))
		}
	}
	return own
}

func (d *DB) adjustmentCheckout(task *models.Task) string {
	if p, _ := d.GetProjectByID(task.ProjectID); p != nil {
		if p.GithubRepo != "" {
			return p.GithubRepo
		}
		return p.GitRemoteUrl
	}
	return ""
}

// lookupStagePR reads the pull request that carries a stage's evidence. It is
// resolved for the task's own project and for whoever asked, because a lookup
// with neither reached only the server environment: a deployment configured
// through the project, or through the person's profile, had no credential on
// this path at all while every other tracker call had one.
//
// The forge is the one hosting the code, whatever tracks the issues: GitHub is
// read by the server, any other forge by the local agent, which already holds
// the CLI login for it.
//
// A pull request in another repository is read there instead: on GitHub by the
// server for that owner/repo, on GitLab by the local agent for that project.
func (d *DB) lookupStagePR(task *models.Task, userID, repo, branch string, target stagePRTarget) (trackerapi.PullRequest, error) {
	if d.prEvidenceLookup != nil {
		prURL := ""
		if target.foreign {
			repo, prURL = target.link.Identity(), target.url
		}
		return d.prEvidenceLookup(repo, branch, prURL)
	}
	if target.foreign {
		if target.link.Forge == "github" {
			return d.trackerAs(userID, "github", task.ProjectID).BranchPullRequest(target.link.Repository, branch)
		}
		return d.agentBranchPullRequest(task, userID, branch, "gitlab", target.link.Identity())
	}
	// An unreadable project is a failed lookup: guessing a forge would send the
	// question to the wrong one and report its answer as the task's evidence.
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return trackerapi.PullRequest{}, fmt.Errorf("read project for the stage PR lookup: %w", err)
	}
	forge := stagePRForge(project)
	if forge == "github" {
		return d.trackerAs(userID, "github", task.ProjectID).BranchPullRequest(repo, branch)
	}
	return d.agentBranchPullRequest(task, userID, branch, forge, "")
}

// stagePRForge chooses where stage evidence is read from the code remote:
// "github" for the server's GitHub client, "gitlab" or "" (a forge the remote
// does not name) for the local agent. A configured GitHub repository only
// decides when the remote names no forge, so a stale one never sends a GitLab
// project to GitHub, while a GitHub project keeps the path it always had.
func stagePRForge(p *models.Project) string {
	if p == nil {
		return ""
	}
	host := remoteHost(p.GitRemoteUrl)
	switch {
	case strings.Contains(host, "github"):
		return "github"
	case strings.Contains(host, "gitlab"):
		return "gitlab"
	case strings.TrimSpace(p.GithubRepo) != "":
		return "github"
	}
	return ""
}

// remoteHost is the host of a Git remote, in URL or scp-like form, lowercased;
// empty when the remote names none, such as a local path or a bare owner/repo.
func remoteHost(remote string) string {
	r := strings.ToLower(strings.TrimSpace(remote))
	if i := strings.Index(r, "://"); i >= 0 {
		r = r[i+3:]
		if j := strings.Index(r, "/"); j >= 0 {
			r = r[:j]
		}
	} else if j := strings.Index(r, ":"); j > 1 && !strings.Contains(r[:j], "/") {
		r = r[:j]
	} else {
		return ""
	}
	if k := strings.LastIndex(r, "@"); k >= 0 {
		r = r[k+1:]
	}
	if k := strings.Index(r, ":"); k >= 0 {
		r = r[:k]
	}
	return r
}

// agentBranchPullRequest asks the local agent which request carries the branch.
// The agent tells a refusal (the forge answered, nothing usable) from a failed
// lookup, and the two stay distinct here: neither is permission to proceed, but
// only the first means the request is really absent.
//
// A repository asks about a merge request in another GitLab repository; the
// answer must echo it, or the agent answered for the project checkout.
func (d *DB) agentBranchPullRequest(task *models.Task, userID, branch, forge, repository string) (trackerapi.PullRequest, error) {
	var answer struct {
		Repository string
		Forge      string
		URL        string
		Branch     string
		SHA        string
		Open       bool
		Draft      bool
		Merged     bool
		Refusal    string
	}
	if err := d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "pr_evidence", Branch: branch, UserID: userID, Repository: repository}, &answer); err != nil {
		lookup := "pull request"
		if forge == "gitlab" {
			lookup = "GitLab merge request"
		}
		return trackerapi.PullRequest{}, fmt.Errorf("%s lookup failed on the local agent: %w", lookup, err)
	}
	if answer.Repository != repository {
		return trackerapi.PullRequest{}, errAgentTooOld
	}
	if answer.Forge != "gitlab" {
		answer.Forge = ""
	}
	if answer.Refusal != "" {
		if answer.Forge == "gitlab" {
			return trackerapi.PullRequest{}, fmt.Errorf("GitLab: %s", answer.Refusal)
		}
		return trackerapi.PullRequest{}, fmt.Errorf("%s", answer.Refusal)
	}
	return trackerapi.PullRequest{URL: answer.URL, Branch: answer.Branch, SHA: answer.SHA, Open: answer.Open, Draft: answer.Draft, Merged: answer.Merged, Forge: answer.Forge}, nil
}
