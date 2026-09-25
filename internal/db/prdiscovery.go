package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// Pull-request rediscovery.
//
// Pull-request knowledge used to live only in the local store: the import path
// never writes `pr_url` / `pr_links`, deliberately, so that a synchronisation
// can never erase the evidence a workflow produced. The blind spot was an
// instance that never had that evidence — a project recreated on a second
// machine imports its tickets with no pull request at all, and nothing repaired
// it.
//
// Discovery is therefore a distinct step that runs *after* the import returns,
// never inside `ImportOrUpdateTasks`: the import keeps its property, and this
// file alone owns the additive write. It is bounded (`pullRequestDiscoveryGate`),
// best effort (a failure is a warning on the sync activity, never a sync
// failure) and additive (`models.AcceptPullRequest` still guards a task that
// already holds links).

// pullRequestDiscoveryReason says why a task was skipped, for a test to assert
// on the gate rather than on the absence of a call.
type pullRequestDiscoveryReason string

const (
	discoveryAllowed        pullRequestDiscoveryReason = ""
	discoveryNoCapability   pullRequestDiscoveryReason = "tracker cannot discover pull requests"
	discoveryAlreadyLinked  pullRequestDiscoveryReason = "task already holds pull request links"
	discoveryBeforeCreation pullRequestDiscoveryReason = "task has not reached the pull request creation stage"
	discoveryDetached       pullRequestDiscoveryReason = "pull request links were detached by hand"
)

// pullRequestDiscoverer resolves the tracker read, or nil when this tracker
// cannot answer. The injectable hook comes first so tests need no live forge.
func (d *DB) pullRequestDiscoverer(ts tracker.TicketingSystem) func(context.Context, *models.Project, string) ([]models.TaskPullRequest, error) {
	if d.prDiscoveryLookup != nil {
		return func(_ context.Context, proj *models.Project, key string) ([]models.TaskPullRequest, error) {
			projectID := ""
			if proj != nil {
				projectID = proj.ID
			}
			return d.prDiscoveryLookup(projectID, key)
		}
	}
	if ts == nil || !ts.Supports(tracker.CapPullRequests) {
		return nil
	}
	finder, ok := ts.(tracker.PullRequestDiscoverer)
	if !ok {
		return nil
	}
	return func(ctx context.Context, proj *models.Project, key string) ([]models.TaskPullRequest, error) {
		return finder.IssuePullRequests(ctx, tracker.IssuePullRequestsRequest{Project: proj, Key: key})
	}
}

// pullRequestDiscoveryGate is the bounding rule: discovery costs one tracker
// call per task, so it only runs where a link is both plausible and missing. A
// rediscovery a person asked for (US5) bypasses everything but the capability.
func (d *DB) pullRequestDiscoveryGate(proj *models.Project, task *models.Task, force bool) pullRequestDiscoveryReason {
	if force {
		return discoveryAllowed
	}
	if len(task.PrLinks) > 0 {
		return discoveryAlreadyLinked
	}
	if d.pullRequestLinksDetached(task.ID) {
		return discoveryDetached
	}
	creation := "implemented"
	if proj != nil && strings.TrimSpace(proj.PRCreationStage) != "" {
		creation = proj.PRCreationStage
	}
	if stageRank(d.StageOfTask(task)) < stageRank(creation) {
		return discoveryBeforeCreation
	}
	return discoveryAllowed
}

// discoverTaskPullRequests runs the gate, then the tracker read. halt says the
// tracker asked to be left alone — a rate limit or a credential it refuses —
// and the caller stops discovering for the rest of the pass rather than
// repeating the same failure once per ticket.
func (d *DB) discoverTaskPullRequests(ctx context.Context, proj *models.Project, ts tracker.TicketingSystem, task *models.Task, force bool) (found []models.TaskPullRequest, halt bool, err error) {
	if task == nil || strings.TrimSpace(task.Key) == "" {
		return nil, false, nil
	}
	discover := d.pullRequestDiscoverer(ts)
	view := taskViewRepository(proj, task)
	if discover == nil && view == "" {
		return nil, false, nil
	}
	if reason := d.pullRequestDiscoveryGate(proj, task, force); reason != discoveryAllowed {
		return nil, false, nil
	}
	if discover != nil {
		found, err = discover(ctx, proj, task.Key)
		if err != nil {
			if isRateLimited(err) {
				d.enterAutoSyncBackoff()
				return nil, true, err
			}
			return nil, isUnauthorized(err), err
		}
	}
	if view != "" && len(found) == 0 {
		// Issue references only name pull requests in the tracker's own
		// repository: one opened in the view's repository is found by branch.
		pr, err := d.discoverViewRepositoryPullRequest(task, view)
		if err != nil {
			return nil, false, err
		}
		if pr != nil {
			found = append(found, *pr)
		}
	}
	return found, false, nil
}

// discoverViewRepositoryPullRequest reads the pull request of the ticket's
// branch in its recorded view repository (#429), as the user who launched it:
// GitHub on the server, GitLab through that user's agent. No branch, or no
// open or merged request on it, is nothing to attach.
func (d *DB) discoverViewRepositoryPullRequest(task *models.Task, view string) (*models.TaskPullRequest, error) {
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	if branch == "" {
		return nil, nil
	}
	pr, err := d.lookupStagePR(task, d.taskViewRepositoryUser(task.ID), "", branch, repositoryTarget(view))
	if err != nil {
		return nil, fmt.Errorf("%s : %w", view, err)
	}
	if strings.TrimSpace(pr.URL) == "" || (!pr.Open && !pr.Merged) || pr.Branch != branch {
		return nil, nil
	}
	return &models.TaskPullRequest{URL: pr.URL, Branch: pr.Branch}, nil
}

// isUnauthorized reports a credential the tracker refuses, as opposed to a
// transient failure. Reading pull requests needs a scope reading issues does
// not, so a token can sync tickets happily and be refused here — once per
// ticket, which is the noise this detection removes.
func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "credential")
}

// applyDiscoveredPullRequests writes what discovery found, additively.
//
// A task that already holds links keeps the anti-substitution guard: a
// candidate on a branch no link mentions is refused and reported, never
// written. A task that holds none is seeded, oldest first, so `prUrl` resolves
// to the newest. Nothing found, or nothing new, writes nothing at all.
func (d *DB) applyDiscoveredPullRequests(task *models.Task, found []models.TaskPullRequest) (attached []string, warnings []string, err error) {
	if task == nil || len(found) == 0 {
		return nil, nil, nil
	}
	// Discovery read the forge from a snapshot of the task. The merge runs on
	// the row locked and read again, so a link another writer attached in the
	// meantime, on this instance or another, is kept rather than overwritten.
	var links []models.TaskPullRequest
	var branchValue *string
	d.mu.Lock()
	err = d.conn.WithTx(func(tx *sqlTx) error {
		locked, err := d.lockTaskUnsafe(tx, task.ID)
		if err != nil || locked == nil {
			return err
		}
		links = models.NormalizePullRequestLinks(locked.PrLinks)
		seeding := len(links) == 0
		branch := ""
		for _, candidate := range found {
			url := strings.TrimSpace(candidate.URL)
			if url == "" {
				continue
			}
			if !seeding {
				if refusal := models.AcceptPullRequest(links, url, candidate.Branch); refusal != nil {
					warnings = append(warnings, fmt.Sprintf("%s : %v", task.Key, refusal))
					continue
				}
			}
			grown := models.AppendPullRequestLink(links, url, candidate.Branch)
			if len(grown) == len(links) {
				continue
			}
			links = grown
			attached = append(attached, url)
			if branch == "" {
				branch = strings.TrimSpace(candidate.Branch)
			}
		}
		if len(attached) == 0 {
			return nil
		}

		// The branch a rediscovered pull request carries seeds `branch_name` only
		// when the task has none: it makes the branch lookup work from the next
		// synchronisation onwards, and a recorded branch is never overwritten.
		branchValue = locked.BranchName
		if branchValue == nil || strings.TrimSpace(*branchValue) == "" {
			if branch != "" {
				seeded := branch
				branchValue = &seeded
			}
		}

		// One statement for `pr_url` and `pr_links`, as every other write path
		// does, so the current pull request can never diverge from the set. The
		// detachment flag falls with it: a link is attached again.
		_, err = tx.Exec("UPDATE tasks SET pr_url = ?, pr_links = ?, branch_name = ?, pr_links_detached = 0 WHERE id = ?",
			pullRequestURLValue(links), encodePullRequestLinks(links), branchValue, task.ID)
		return err
	})
	d.mu.Unlock()
	if err != nil {
		return nil, warnings, err
	}
	if len(attached) == 0 {
		return nil, warnings, nil
	}
	task.PrLinks = links
	task.PrURL = pullRequestURLValue(links)
	task.BranchName = branchValue
	return attached, warnings, nil
}

// rediscoverPullRequests is the whole step for one task: gate, read, write, and
// what the synchronisation activity should say about it.
func (d *DB) rediscoverPullRequests(ctx context.Context, proj *models.Project, ts tracker.TicketingSystem, task *models.Task, force bool) (steps []string, halt bool) {
	found, halt, err := d.discoverTaskPullRequests(ctx, proj, ts, task, force)
	if err != nil {
		return []string{fmt.Sprintf("⚠️ Pull requests %s : %v", task.Key, err)}, halt
	}
	attached, warnings, err := d.applyDiscoveredPullRequests(task, found)
	for _, warning := range warnings {
		steps = append(steps, "⚠️ Pull requests "+warning)
	}
	if err != nil {
		return append(steps, fmt.Sprintf("⚠️ Pull requests %s : écriture locale échouée : %v", task.Key, err)), false
	}
	if len(attached) > 0 {
		steps = append(steps, fmt.Sprintf("🔗 %s — pull request(s) rattachée(s) : %s", task.Key, strings.Join(attached, ", ")))
	}
	return steps, false
}

// rediscoverProjectPullRequests runs the step over the tasks a full
// synchronisation imported. The incremental pass never comes through here: it
// re-reads tickets one by one, and a discovery call per ticket per minute is
// exactly the cost the bounding rule exists to avoid.
func (d *DB) rediscoverProjectPullRequests(ctx context.Context, proj *models.Project, ts tracker.TicketingSystem, imported []models.Task) []string {
	// A tracker that cannot discover still leaves the view repositories of
	// #429 to search, one lookup per ticket that recorded one.
	canDiscover := d.pullRequestDiscoverer(ts) != nil
	var steps []string
	seen := map[string]bool{}
	for i := range imported {
		task, err := d.GetTaskByID(imported[i].ID)
		if err != nil || task == nil || (!canDiscover && taskViewRepository(proj, task) == "") {
			continue
		}
		found, halt := d.rediscoverPullRequests(ctx, proj, ts, task, false)
		for _, step := range found {
			// A refused credential would otherwise be reported once per
			// ticket; the pass says it once.
			if seen[step] {
				continue
			}
			seen[step] = true
			steps = append(steps, step)
		}
		if halt {
			steps = append(steps, "⚠️ Pull requests : découverte interrompue pour cette passe")
			break
		}
	}
	return steps
}

// pullRequestLinksDetached reads the flag a human raised by detaching every
// link. It is read here rather than carried on models.Task: nothing outside
// this step needs it, and the task payload stays as it is.
func (d *DB) pullRequestLinksDetached(taskID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var detached int
	if err := d.conn.QueryRow("SELECT pr_links_detached FROM tasks WHERE id = ?", taskID).Scan(&detached); err != nil {
		return false
	}
	return detached != 0
}

// discoverAndApply is the step a person asked for on one work item: the gate
// steps aside, and a failure is returned to the caller, which logs it rather
// than turning it into a synchronisation failure.
func (d *DB) discoverAndApply(ctx context.Context, proj *models.Project, ts tracker.TicketingSystem, task *models.Task) ([]string, []string, error) {
	found, _, err := d.discoverTaskPullRequests(ctx, proj, ts, task, true)
	if err != nil {
		return nil, nil, err
	}
	return d.applyDiscoveredPullRequests(task, found)
}
