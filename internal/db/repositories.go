package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// projectCodeRemote is the remote that heads a project's repositories: the
// code remote, else the GitHub repository the pull requests live in.
func projectCodeRemote(p *models.Project) string {
	if p == nil {
		return ""
	}
	if remote := strings.TrimSpace(p.GitRemoteUrl); remote != "" {
		return remote
	}
	if repo := strings.Trim(strings.TrimSpace(p.GithubRepo), "/"); repo != "" {
		if strings.Contains(repo, "://") || strings.Contains(repo, "@") {
			return repo
		}
		return "https://github.com/" + repo
	}
	return ""
}

// parseRepositoryURLs decodes projects.repositories, tolerating an empty or
// malformed column.
func parseRepositoryURLs(raw string) []string {
	var list []string
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &list) != nil {
		return []string{}
	}
	return list
}

// encodeRepositoryURLs stores the declared remotes other than the code remote,
// which is always derived from the project and never stored twice.
func encodeRepositoryURLs(codeRemote string, urls []string) string {
	stored := []string{}
	for _, repository := range models.NormalizeProjectRepositories(codeRemote, urls) {
		if repository.Identity != models.RepositoryIdentity(codeRemote) {
			stored = append(stored, repository.URL)
		}
	}
	payload, _ := json.Marshal(stored)
	return string(payload)
}

// parseIdentityList decodes tasks.changed_repositories.
func parseIdentityList(raw string) []string {
	var list []string
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &list) != nil {
		return nil
	}
	return list
}

// ErrDuplicateRepository refuses a second spelling of a repository a project
// already declares, rather than silently keeping one of them.
var ErrDuplicateRepository = errors.New("repository already declared")

// ErrInvalidSpecArtifacts refuses a specification artefacts setting other than
// the two a project can hold, naming them so an API client can correct itself.
var ErrInvalidSpecArtifacts = errors.New("specArtifacts must be keep or drop")

// ErrRepositoryNotInProject refuses to pin a ticket to a repository its
// project does not declare.
var ErrRepositoryNotInProject = errors.New("repository is not one of the project's repositories")

// checkRepositories refuses a repository declared twice. The code remote is
// always listed first and derived from the project, so a client that sends it
// back with the others (the project as it read it) is not declaring it twice:
// it is left out, as encodeRepositoryURLs leaves it out of the column.
func checkRepositories(codeRemote string, urls []string) error {
	code := models.RepositoryIdentity(codeRemote)
	declared := make([]string, 0, len(urls))
	for _, url := range urls {
		if strings.TrimSpace(codeRemote) == "" || models.RepositoryIdentity(url) != code {
			declared = append(declared, url)
		}
	}
	if duplicate := models.DuplicateRepository("", declared); duplicate != "" {
		return fmt.Errorf("%w: %s", ErrDuplicateRepository, duplicate)
	}
	return nil
}

// declaredRepositoryURLs are the remotes a project declares besides its code
// remote, as they are stored.
func declaredRepositoryURLs(p *models.Project) []string {
	code := models.RepositoryIdentity(projectCodeRemote(p))
	urls := []string{}
	for _, repository := range p.Repositories {
		if repository.Identity != code {
			urls = append(urls, repository.URL)
		}
	}
	return urls
}

// taskPin is the repository a ticket is pinned to, as far as its project is
// concerned: nothing for a pin to a repository the project no longer declares.
// The agent reads a pin the same way (models.ResolvePrimaryRepository).
func taskPin(project *models.Project, task *models.Task) string {
	if project == nil || task == nil {
		return ""
	}
	if repository, ok := models.FindProjectRepository(project.Repositories, task.Repository); ok {
		return repository.Identity
	}
	return ""
}

// taskChangedRepositories are the secondary repositories a ticket changed, the
// primary one left out. A folder attached on a workstation only is one of them
// as much as a project repository (#484): each needs its pull request.
func taskChangedRepositories(project *models.Project, task *models.Task) []string {
	if project == nil || task == nil {
		return nil
	}
	primary := TaskPrimaryRepository(project, task)
	var changed []string
	for _, identity := range task.ChangedRepositories {
		if identity == "" || identity == primary || slices.Contains(changed, identity) {
			continue
		}
		changed = append(changed, identity)
	}
	return changed
}

// ErrRepositoriesConverted refuses a second conversion of a project's legacy
// paths: the first workstation to convert wins, the others skip.
var ErrRepositoriesConverted = errors.New("the project's repositories were already converted")

// LegacyRepoPaths lists what a local agent must convert for a project: the
// project's repoPath and repoPaths and every ticket's repoPath, de-duplicated,
// each with the tickets that pinned it.
func (d *DB) LegacyRepoPaths(projectID string) (*models.LegacyRepoPaths, error) {
	project, err := d.GetProjectByID(projectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}
	result := &models.LegacyRepoPaths{Migrated: project.RepositoriesMigration != "", Paths: []models.LegacyRepoPath{}}
	if result.Migrated {
		return result, nil
	}
	index := map[string]int{}
	add := func(path, taskID string) {
		path = strings.TrimSpace(path)
		if path == "" || path == "." {
			return
		}
		i, ok := index[path]
		if !ok {
			i = len(result.Paths)
			index[path] = i
			result.Paths = append(result.Paths, models.LegacyRepoPath{Path: path})
		}
		if taskID != "" {
			result.Paths[i].TaskIDs = append(result.Paths[i].TaskIDs, taskID)
		}
	}
	add(project.RepoPath, "")
	for _, path := range project.RepoPaths {
		add(path, "")
	}
	rows, err := d.conn.Query("SELECT id, repo_path FROM tasks WHERE project_id = ? AND repo_path <> '' ORDER BY id", project.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		add(path, id)
	}
	return result, rows.Err()
}

// ApplyRepositoryConversion records a local agent's conversion in one
// transaction: the resolved remotes join the project's repositories, the
// tickets that pinned a resolved path are pinned to its repository, every
// legacy path is cleared and the report is kept. It applies once per project.
func (d *DB) ApplyRepositoryConversion(userID, projectID string, report models.RepositoryConversion) (*models.Project, error) {
	d.mu.Lock()
	project, err := d.getProjectByIDUnsafe(projectID)
	if err != nil || project == nil {
		d.mu.Unlock()
		if err == nil {
			err = fmt.Errorf("project not found")
		}
		return nil, err
	}
	codeRemote := projectCodeRemote(project)
	urls := make([]string, 0, len(project.Repositories)+len(report.Converted))
	for _, repository := range project.Repositories {
		urls = append(urls, repository.URL)
	}
	for _, converted := range report.Converted {
		urls = append(urls, converted.URL)
	}
	report.ConvertedAt = time.Now().UTC().Format(time.RFC3339)
	report.UserID = strings.TrimSpace(userID)
	if report.Converted == nil {
		report.Converted = []models.ConvertedRepoPath{}
	}
	if report.Dropped == nil {
		report.Dropped = []models.DroppedRepoPath{}
	}
	payload, _ := json.Marshal(report)
	err = d.conn.WithTx(func(tx *sqlTx) error {
		result, err := tx.Exec("UPDATE projects SET repositories = ?, repositories_migration = ?, repo_paths = '[]', updated_at = ? WHERE id = ? AND repositories_migration = ''",
			encodeRepositoryURLs(codeRemote, urls), string(payload), time.Now(), project.ID)
		if err != nil {
			return err
		}
		if count, _ := result.RowsAffected(); count == 0 {
			return ErrRepositoriesConverted
		}
		for _, converted := range report.Converted {
			identity := models.RepositoryIdentity(converted.URL)
			for _, taskID := range converted.TaskIDs {
				if _, err := tx.Exec("UPDATE tasks SET repository = ? WHERE id = ? AND project_id = ?", identity, taskID, project.ID); err != nil {
					return err
				}
			}
		}
		_, err = tx.Exec("UPDATE tasks SET repo_path = '' WHERE project_id = ?", project.ID)
		return err
	})
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return d.GetProjectByID(project.ID)
}

// AddChangedRepository records that a ticket has a worktree in a secondary
// repository, which then needs its own pull request.
func (d *DB) AddChangedRepository(taskID, identity string) error {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return fmt.Errorf("repository is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.WithTx(func(tx *sqlTx) error {
		var raw string
		if err := tx.QueryRow("SELECT changed_repositories FROM tasks WHERE id = ?", taskID).Scan(&raw); err != nil {
			return err
		}
		list := parseIdentityList(raw)
		for _, existing := range list {
			if existing == identity {
				return nil
			}
		}
		payload, _ := json.Marshal(append(list, identity))
		_, err := tx.Exec("UPDATE tasks SET changed_repositories = ?, updated_at = ? WHERE id = ?", string(payload), time.Now(), taskID)
		return err
	})
}

// TaskPrimaryRepository is the identity of the repository a ticket runs in:
// its pin, else its project's code remote. Empty when neither exists.
func TaskPrimaryRepository(project *models.Project, task *models.Task) string {
	if pin := taskPin(project, task); pin != "" {
		return pin
	}
	if remote := projectCodeRemote(project); remote != "" {
		return models.RepositoryIdentity(remote)
	}
	return ""
}

// PrepareRepositoryWorktree asks the caller's local agent for the ticket's
// worktree in a secondary repository, on the ticket's branch, and records that
// repository as changed. Git runs on the agent's machine, which holds the
// checkouts; the server only checks the request and relays it. The repository
// is one of the project's, or a Git folder attached to the project on the
// caller's workstation (#484), which only that agent knows: the server never
// learns its path, and records its identity once the agent returned a worktree.
func (d *DB) PrepareRepositoryWorktree(ctx context.Context, userID, taskKey, repository string) (*models.RepositoryWorktree, error) {
	task, err := d.GetTaskByID(taskKey)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}
	target, ok := models.FindProjectRepository(project.Repositories, repository)
	if !ok {
		identity := models.RepositoryIdentity(repository)
		if !remoteIdentity(identity) {
			return nil, fmt.Errorf("%q does not name a repository: give the repository's remote URL or host/path; a folder without a remote is changed in place, with no worktree and no pull request", strings.TrimSpace(repository))
		}
		target = models.ProjectRepository{URL: strings.TrimSpace(repository), Identity: identity}
	}
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	if branch == "" {
		return nil, fmt.Errorf("task %s has no branch yet: its primary worktree comes first", task.Key)
	}
	if target.Identity == TaskPrimaryRepository(project, task) {
		return nil, fmt.Errorf("%s is the primary repository of %s: it already has its worktree", target.Identity, task.Key)
	}
	var worktree models.RepositoryWorktree
	err = d.callAgentContext(ctx, agentprotocol.Operation{UserID: strings.TrimSpace(userID), ProjectID: project.ID, TaskID: task.ID,
		Action: "repository_worktree", Repository: target.Identity, Branch: branch}, &worktree)
	if err != nil {
		return nil, err
	}
	if worktree.Repository != target.Identity {
		return nil, fmt.Errorf("local agent is too old to prepare a worktree in another repository; update it")
	}
	if err := d.AddChangedRepository(task.ID, target.Identity); err != nil {
		return nil, err
	}
	return &worktree, nil
}

// remoteIdentity tells an identity derived from a remote (host/path) from what
// a folder path or a bare name reduces to, which names no repository.
func remoteIdentity(identity string) bool {
	host, path, ok := strings.Cut(identity, "/")
	return ok && host != "" && path != "" && !strings.ContainsAny(identity, `\~`) && !strings.HasPrefix(host, ".")
}
