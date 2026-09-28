package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/agenthttp"
	"tasks/internal/models"
)

// projectRepositories are the project's repositories as the configuration
// names them, the code remote first. An older server sends none: the code
// remote alone, as before.
func projectRepositories(c agentconfig.Config) []models.ProjectRepository {
	if len(c.Repositories) > 0 {
		return models.NormalizeProjectRepositories("", c.Repositories)
	}
	return models.NormalizeProjectRepositories(c.GitRemoteURL, nil)
}

// codeIdentity is the identity of the project's own repository, the one the
// project root is a checkout of.
func codeIdentity(c agentconfig.Config) string {
	if strings.TrimSpace(c.GitRemoteURL) != "" {
		return models.RepositoryIdentity(c.GitRemoteURL)
	}
	if repositories := projectRepositories(c); len(repositories) > 0 {
		return repositories[0].Identity
	}
	return ""
}

// repositoryRoot is the folder that holds a repository's checkout on this
// workstation: its mapping, else the project root for the project's own
// repository. A mapping whose folder is gone reads as unmapped.
func repositoryRoot(overrides agentconfig.Settings, projectRoot, code, identity string) (string, bool) {
	if mapped := strings.TrimSpace(overrides.Repositories[identity]); mapped != "" {
		if info, err := os.Stat(mapped); err == nil && info.IsDir() && filepath.IsAbs(mapped) {
			return mapped, true
		}
		return "", false
	}
	if identity != "" && identity == code && projectRoot != "" {
		return projectRoot, true
	}
	return "", false
}

// primaryRoot decides which checkout a ticket's worktree lives in, before
// anything is launched: the checkout of the repository the ticket is pinned
// to, mapped or attached here, else the project root, the code repository
// (#484). It fails, naming the repository, when the pinned one has no folder
// here.
func primaryRoot(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot string, task models.Task) (root, identity string, err error) {
	code := codeIdentity(config)
	mapped := func(identity string) bool {
		_, ok := repositoryFolder(ctx, overrides, config.ProjectID, projectRoot, code, identity)
		return ok
	}
	repository, outcome := models.ResolvePrimaryRepository(task.Repository, projectRepositories(config), mapped)
	switch outcome {
	case models.PrimaryDefault:
		return projectRoot, code, nil
	case models.PrimaryUnmapped:
		return "", "", fmt.Errorf("Le dépôt %s de la tâche n'est associé à aucun dossier sur ce poste : choisissez son dossier dans les réglages du projet de l'app desktop.", repository.Identity)
	}
	root, _ = repositoryFolder(ctx, overrides, config.ProjectID, projectRoot, code, repository.Identity)
	if repository.Identity != code {
		// The project's own checkout ignores .tasks/ through its .gitignore;
		// another repository has no reason to, so its status is kept clean
		// through info/exclude, as the specifications checkout is.
		if err := excludeTaskWorktrees(ctx, root); err != nil {
			return "", "", err
		}
	}
	return root, repository.Identity, nil
}

// buildFolderMap describes every folder of a ticket to the agent: each project
// repository with its role, its folder here and the ticket's worktree in it,
// then each folder attached to the project on this workstation (#484), then
// the specifications folder when it is a folder of its own.
func buildFolderMap(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot, primary, workDir string, task models.Task) []models.FolderMapEntry {
	code := codeIdentity(config)
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	changed := map[string]bool{}
	for _, identity := range task.ChangedRepositories {
		changed[identity] = true
	}
	changedWorktree := func(root string) string {
		if root == "" || branch == "" {
			return ""
		}
		worktree, _ := worktreeForBranch(ctx, root, branch)
		return worktree
	}
	var entries []models.FolderMapEntry
	seen := map[string]bool{}
	listed := map[string]bool{}
	for _, repository := range projectRepositories(config) {
		root, _ := repositoryFolder(ctx, overrides, config.ProjectID, projectRoot, code, repository.Identity)
		entry := models.FolderMapEntry{Remote: repository.URL, Identity: repository.Identity, Role: models.FolderRoleContext, Path: root}
		switch {
		case repository.Identity == primary:
			entry.Role, entry.Worktree = models.FolderRolePrimary, workDir
		case changed[repository.Identity]:
			entry.Role, entry.Worktree = models.FolderRoleChanged, changedWorktree(root)
		}
		if root != "" {
			seen[filepath.Clean(root)] = true
		}
		listed[repository.Identity] = true
		entries = append(entries, entry)
	}
	spec, err := localSpecRepo(overrides, config.ProjectID, projectRoot)
	if err != nil {
		spec = ""
	}
	// Folders compare as directories: an attached checkout is described at
	// its top level, which Git reports with its symbolic links resolved.
	taken := []string{projectRoot, spec}
	for root := range seen {
		taken = append(taken, root)
	}
	isTaken := func(folder string) bool {
		for _, other := range taken {
			if other != "" && (filepath.Clean(other) == filepath.Clean(folder) || sameDirectory(other, folder)) {
				return true
			}
		}
		return false
	}
	for _, folder := range attachedFolders(ctx, overrides, config.ProjectID) {
		// A project repository is already listed with its mapping, which wins
		// over an attached checkout of it; the project root, the
		// specifications folder and a folder already listed are not listed
		// again.
		if (folder.Identity != "" && (listed[folder.Identity] || folder.Identity == primary)) || isTaken(folder.Path) {
			continue
		}
		taken = append(taken, folder.Path)
		entry := models.FolderMapEntry{Remote: folder.Remote, Identity: folder.Identity, Path: folder.Path, Kind: folder.Kind, Attached: true}
		switch {
		case folder.Kind == folderKindMissing:
			entry.Role = models.FolderRoleContext
		case folder.Identity == "":
			entry.Role = models.FolderRoleLocal
		case changed[folder.Identity]:
			entry.Role, entry.Worktree = models.FolderRoleChanged, changedWorktree(folder.Path)
		default:
			entry.Role = models.FolderRoleContext
		}
		if folder.Identity != "" {
			listed[folder.Identity] = true
		}
		entries = append(entries, entry)
	}
	if spec != "" && !seen[filepath.Clean(spec)] {
		entries = append(entries, models.FolderMapEntry{Role: models.FolderRoleSpec, Path: spec})
	}
	return entries
}

// folderMapDirs are the folders a CLI is given beside its working directory:
// the context, local and specifications folders where they are, and the
// ticket's worktrees in its other changed repositories. A missing attached
// folder is named in the prompt only.
func folderMapDirs(entries []models.FolderMapEntry) []string {
	var dirs []string
	for _, entry := range entries {
		if entry.Kind == folderKindMissing {
			continue
		}
		switch entry.Role {
		case models.FolderRoleContext, models.FolderRoleSpec, models.FolderRoleLocal:
			if entry.Path != "" {
				dirs = append(dirs, entry.Path)
			}
		case models.FolderRoleChanged:
			if entry.Worktree != "" {
				dirs = append(dirs, entry.Worktree)
			} else if entry.Path != "" {
				dirs = append(dirs, entry.Path)
			}
		}
	}
	return dirs
}

// folderKindLabels says what an attached folder is, in the prompt.
var folderKindLabels = map[string]string{
	folderKindGit:     "attached Git repository",
	folderKindFolder:  "attached plain folder",
	folderKindMissing: "attached folder",
}

// folderMapPrompt is the folder map as the prompt states it. It is only
// written for a ticket with more than one folder: a single checkout needs no
// map.
func folderMapPrompt(entries []models.FolderMapEntry) string {
	if len(entries) < 2 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nFolders of this task (also in $SECTILE_REPOSITORIES):")
	for _, entry := range entries {
		name := entry.Identity
		switch {
		case name != "":
		case entry.Role == models.FolderRoleSpec:
			name = "specifications"
		default:
			name = entry.Path
		}
		where := entry.Path
		if entry.Worktree != "" {
			where = entry.Worktree
		}
		switch {
		case entry.Kind == folderKindMissing:
			where = "not found on this workstation"
		case where == "":
			where = "not mapped on this workstation"
		}
		role := entry.Role
		if entry.Attached {
			if entry.Kind == folderKindGit && entry.Identity == "" {
				role += ", attached Git repository without a remote"
			} else {
				role += ", " + folderKindLabels[entry.Kind]
			}
		}
		fmt.Fprintf(&b, "\n- %s (%s): %s", name, role, where)
	}
	b.WriteString("\nWork in the primary worktree. Context folders are read-only: to change one, call prepare_repository_worktree for its repository first and work in the worktree it returns. Local folders have no remote: change them in place, with no worktree and no pull request. Each changed repository needs its own pull request, given to transition_stage in prUrls.")
	return b.String()
}

// postAPI sends a JSON body to the server and decodes its answer into result
// when result is not nil.
func (d *agentDaemon) postAPI(ctx context.Context, path string, body, result any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.link.serverURL+path, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var detail struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&detail)
		return &apiStatusError{Status: resp.StatusCode, Detail: detail.Error}
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
}

// apiStatusError is a refused request, with the status the caller may act on.
type apiStatusError struct {
	Status int
	Detail string
}

func (e *apiStatusError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("Sectile API returned HTTP %d: %s", e.Status, e.Detail)
	}
	return fmt.Sprintf("Sectile API returned HTTP %d", e.Status)
}

// convertLegacyRepoPaths converts, once per project, the working directories
// typed before repositories existed (#456). Only a workstation can read a
// checkout's origin, so the first agent that sees the project resolves every
// path it can and drops the others, and the server keeps the first report it
// receives. A failure is logged, never a reason to refuse a launch.
func (d *agentDaemon) convertLegacyRepoPaths(ctx context.Context, config agentconfig.Config) {
	projectID := config.ProjectID
	var legacy models.LegacyRepoPaths
	if err := d.readAPI(ctx, "/api/projects/"+url.PathEscape(projectID)+"/legacy-repo-paths", &legacy); err != nil || legacy.Migrated {
		return
	}
	report := resolveLegacyRepoPaths(ctx, legacy.Paths)
	err := d.postAPI(ctx, "/api/projects/"+url.PathEscape(projectID)+"/repositories/convert", report, nil)
	var refused *apiStatusError
	switch {
	case errors.As(err, &refused) && refused.Status == http.StatusConflict:
		return
	case err != nil:
		log.Printf("[Agent] Could not convert the legacy paths of project %s: %v", projectID, err)
		return
	}
	// The tickets are now pinned to these repositories: this workstation has
	// just read where each one lives, so it keeps the folder, or their next
	// launch would ask for it.
	d.rememberConvertedFolders(ctx, config, report.Converted)
	for _, dropped := range report.Dropped {
		log.Printf("[Agent] Project %s: dropped legacy path %s (%s), pinned by %d task(s)", projectID, dropped.Path, dropped.Reason, len(dropped.TaskIDs))
	}
	log.Printf("[Agent] Project %s: %d legacy path(s) converted to repositories, %d dropped", projectID, len(report.Converted), len(report.Dropped))
}

// resolveLegacyRepoPaths reads each path's origin on this workstation.
func resolveLegacyRepoPaths(ctx context.Context, paths []models.LegacyRepoPath) models.RepositoryConversion {
	report := models.RepositoryConversion{Converted: []models.ConvertedRepoPath{}, Dropped: []models.DroppedRepoPath{}}
	for _, legacy := range paths {
		drop := func(reason string) {
			report.Dropped = append(report.Dropped, models.DroppedRepoPath{Path: legacy.Path, Reason: reason, TaskIDs: legacy.TaskIDs})
		}
		if info, err := os.Stat(legacy.Path); err != nil || !info.IsDir() || !filepath.IsAbs(legacy.Path) {
			drop("not found")
			continue
		}
		if _, err := gitLocal(ctx, legacy.Path, "rev-parse", "--show-toplevel"); err != nil {
			drop("not a git checkout")
			continue
		}
		remote, err := gitLocal(ctx, legacy.Path, "remote", "get-url", "origin")
		if err != nil || strings.TrimSpace(remote) == "" {
			drop("no origin")
			continue
		}
		report.Converted = append(report.Converted, models.ConvertedRepoPath{Path: legacy.Path, URL: strings.TrimSpace(remote), TaskIDs: legacy.TaskIDs})
	}
	return report
}

// repositoryWorktree answers repository_worktree: the ticket's worktree in a
// secondary repository, on the ticket's branch, created or reused like the
// primary one. The repository is one of the project's, or a Git folder
// attached to the project here (#484). The repository is echoed so the
// server can tell this agent from one that ignored the question.
func repositoryWorktree(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot string, task models.Task, repository string) (models.RepositoryWorktree, error) {
	repository = strings.TrimSpace(repository)
	for _, folder := range attachedFolders(ctx, overrides, config.ProjectID) {
		if folder.Identity == "" && filepath.IsAbs(repository) && (sameDirectory(folder.Stored, repository) || sameDirectory(folder.Path, repository)) {
			return models.RepositoryWorktree{}, fmt.Errorf("%s has no remote: change it in place, with no worktree and no pull request", folder.Stored)
		}
	}
	identity := models.RepositoryIdentity(repository)
	if target, ok := models.FindProjectRepository(projectRepositories(config), repository); ok {
		identity = target.Identity
	}
	root, ok := repositoryFolder(ctx, overrides, config.ProjectID, projectRoot, codeIdentity(config), identity)
	if !ok || identity == "" {
		return models.RepositoryWorktree{}, fmt.Errorf("Le dépôt %s n'est ni associé ni attaché à ce projet sur ce poste : attachez son dossier dans les réglages du projet de l'app desktop.", repository)
	}
	target := models.ProjectRepository{URL: repository, Identity: identity}
	if target.Identity != codeIdentity(config) {
		if err := excludeTaskWorktrees(ctx, root); err != nil {
			return models.RepositoryWorktree{}, err
		}
	}
	dir, branch, err := ensureLocalWorktree(ctx, root, task, true)
	if err != nil {
		return models.RepositoryWorktree{}, err
	}
	return models.RepositoryWorktree{Repository: target.Identity, Path: dir, Branch: branch}, nil
}

// removeRepositoryWorktrees answers remove_workspace for a ticket with
// worktrees in several repositories: each is removed where it is mapped or
// attached, and a repository that could not be cleaned is named rather than
// failing the rest.
func removeRepositoryWorktrees(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot string, task models.Task, repositories []string) models.WorktreeRemoval {
	result := models.WorktreeRemoval{Removed: []string{}}
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	code := codeIdentity(config)
	for _, identity := range repositories {
		root, ok := repositoryFolder(ctx, overrides, config.ProjectID, projectRoot, code, identity)
		if !ok {
			result.Failed = append(result.Failed, models.WorktreeRemovalFailed{Repository: identity, Error: "not found on this workstation"})
			continue
		}
		worktree, err := worktreeForBranch(ctx, root, branch)
		if err != nil {
			result.Failed = append(result.Failed, models.WorktreeRemovalFailed{Repository: identity, Error: err.Error()})
			continue
		}
		// No worktree on the branch, or the branch is the main checkout's:
		// there is nothing of the ticket's own to remove there.
		if worktree == "" || sameDirectory(worktree, root) {
			result.Removed = append(result.Removed, identity)
			continue
		}
		if _, err := gitLocal(ctx, root, "worktree", "remove", worktree); err != nil {
			result.Failed = append(result.Failed, models.WorktreeRemovalFailed{Repository: identity, Error: err.Error()})
			continue
		}
		result.Removed = append(result.Removed, identity)
	}
	return result
}

// taskFolderMap is the folder map of a launch, read from this workstation's
// mappings once the worktree exists. A failure leaves the map empty: it
// describes the launch, it never decides it.
func (d *agentDaemon) taskFolderMap(ctx context.Context, config agentconfig.Config, task models.Task, workDir string) []models.FolderMapEntry {
	root, overrides, err := d.localProjectRoot(ctx, config)
	if err != nil {
		return nil
	}
	_, primary, err := primaryRoot(ctx, config, overrides, root, task)
	if err != nil {
		return nil
	}
	return buildFolderMap(ctx, config, overrides, root, primary, workDir, task)
}

// rememberConvertedFolders maps each converted repository to the checkout the
// conversion read, unless this workstation already maps it. The project's own
// repository keeps its folder in the project mapping.
func (d *agentDaemon) rememberConvertedFolders(ctx context.Context, config agentconfig.Config, converted []models.ConvertedRepoPath) {
	code := codeIdentity(config)
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	unlock := agentconfig.LockSettings()
	defer unlock()
	overrides, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		log.Printf("[Agent] Could not read the settings to keep converted folders: %v", err)
		return
	}
	changed := false
	for _, entry := range converted {
		identity := models.RepositoryIdentity(entry.URL)
		if identity == "" || identity == code || strings.TrimSpace(overrides.Repositories[identity]) != "" {
			continue
		}
		top, err := gitLocal(ctx, entry.Path, "rev-parse", "--show-toplevel")
		if err != nil {
			continue
		}
		if overrides.Repositories == nil {
			overrides.Repositories = map[string]string{}
		}
		overrides.Repositories[identity] = filepath.Clean(top)
		changed = true
	}
	if changed {
		if err := agentconfig.WriteSettings(overrides); err != nil {
			log.Printf("[Agent] Could not keep converted folders: %v", err)
		}
	}
}
