package db

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// A work item may carry a team, and it is never mandatory: a project can hold
// tickets with no team at all, and those keep behaving exactly as before. When a
// team is there, it is the only handle on the people concerned by the ticket, so
// the sync stores the teams it meets and reads their members from the tracker.
// Two tables hold that: `teams` for the team itself and `team_members` for the
// people, keyed by the tracker's account id.

const teamsAPITimeout = 90 * time.Second

func (d *DB) ensureTeamsTables() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS teams (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		members_synced_at DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`)
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS team_members (
		team_id TEXT NOT NULL,
		account_id TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		avatar_url TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 1,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (team_id, account_id)
	);`)
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_team_members_name ON team_members(display_name);")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_teams_project ON teams(project_id);")
}

// registerTeamsFromTasks records the teams the sync just met, with their ids.
// The name alone is stored on the task; the id is what the members endpoint
// takes, and it only ever arrives here through a sync.
func (d *DB) registerTeamsFromTasks(projectID string, tasks []models.Task) []models.TrackerTeam {
	seen := map[string]string{}
	for _, t := range tasks {
		id := strings.TrimSpace(t.TeamID)
		name := strings.TrimSpace(t.Team)
		if id == "" || name == "" {
			continue
		}
		seen[id] = name
	}
	if len(seen) == 0 {
		return nil
	}

	d.mu.Lock()
	d.ensureTeamsTables()
	now := time.Now()
	for id, name := range seen {
		_, _ = d.conn.Exec(`
			INSERT INTO teams (id, project_id, name, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET name = excluded.name, project_id = excluded.project_id, updated_at = excluded.updated_at
		`, id, projectID, name, now)
	}
	d.mu.Unlock()

	teams := make([]models.TrackerTeam, 0, len(seen))
	for id, name := range seen {
		teams = append(teams, models.TrackerTeam{ID: id, Name: name})
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name })
	return teams
}

// storeTeamMembers replaces the stored members of a team. A full replace is what
// keeps a departure visible: keeping the union of every sync would leave people
// in a team they left months ago.
func (d *DB) storeTeamMembers(teamID string, members []models.TeamMember) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return fmt.Errorf("identifiant d'équipe manquant")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.ensureTeamsTables()

	now := time.Now()
	if _, err := d.conn.Exec("DELETE FROM team_members WHERE team_id = ?", teamID); err != nil {
		return err
	}
	for _, m := range members {
		accountID := strings.TrimSpace(m.AccountID)
		if accountID == "" {
			continue
		}
		activeVal := 0
		if m.Active {
			activeVal = 1
		}
		if _, err := d.conn.Exec(`
			INSERT INTO team_members (team_id, account_id, display_name, email, avatar_url, active, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(team_id, account_id) DO UPDATE SET
				display_name = excluded.display_name,
				email = excluded.email,
				avatar_url = excluded.avatar_url,
				active = excluded.active,
				updated_at = excluded.updated_at
		`, teamID, accountID, strings.TrimSpace(m.DisplayName), strings.TrimSpace(m.Email), m.AvatarURL, activeVal, now); err != nil {
			return err
		}
	}
	_, _ = d.conn.Exec("UPDATE teams SET members_synced_at = ? WHERE id = ?", now, teamID)
	return nil
}

// RefreshProjectTeamMembers reads the people of every team met on the project's
// work items. It is called by the Jira sync, right after the import: the teams
// are only known once the work items have been read.
//
// A failure on one team is not fatal for the others, and no team at all is not
// an error either: a project may simply not use the field.
func (d *DB) RefreshProjectTeamMembers(ctx context.Context, projectID string, tasks []models.Task) (string, error) {
	teams := d.registerTeamsFromTasks(projectID, tasks)
	if len(teams) == 0 {
		return "aucune équipe portée par les tickets, rien à rafraîchir", nil
	}

	ts, proj, err := d.trackerReaderFor(projectID)
	if err != nil {
		return "", err
	}
	if !ts.Supports(tracker.CapTeam) {
		return "", tracker.Unsupported(ts.Name(), tracker.CapTeam)
	}

	ctx, cancel := context.WithTimeout(ctx, teamsAPITimeout)
	defer cancel()

	people := 0
	refreshed := 0
	var failures []string
	for _, team := range teams {
		members, err := ts.TeamMembers(ctx, tracker.TeamRequest{Project: proj, TeamID: team.ID})
		if err != nil {
			// The team stays, its members unknown: a sync never fails on this.
			failures = append(failures, fmt.Sprintf("%s: %v", team.Name, err))
			log.Printf("[teams] membres de %s (%s) non lus: %v", team.Name, team.ID, err)
			continue
		}
		for i := range members {
			members[i].TeamName = team.Name
		}
		if err := d.storeTeamMembers(team.ID, members); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", team.Name, err))
			continue
		}
		refreshed++
		people += len(members)
	}

	note := fmt.Sprintf("%d équipe(s) rafraîchie(s), %d personne(s)", refreshed, people)
	if len(failures) > 0 {
		note += fmt.Sprintf(", %d échec(s) : %s", len(failures), strings.Join(failures, " | "))
	}
	return note, nil
}

// RefreshTeamMembersNow re-reads one team on demand, for the refresh button of
// the team view.
// RefreshTeamMembersNow runs with no acting user, which is what an unattended caller
// does: a tracker whose credential is personal then uses the server one.
func (d *DB) RefreshTeamMembersNow(projectID string, teamID string) (*models.TrackerTeam, error) {
	return d.RefreshTeamMembersNowAs(context.Background(), projectID, teamID)
}

// RefreshTeamMembersNowAs runs on behalf of whoever asked, so a personal tracker
// credential can be resolved for the call.
func (d *DB) RefreshTeamMembersNowAs(ctx context.Context, projectID string, teamID string) (*models.TrackerTeam, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("identifiant d'équipe manquant")
	}

	ts, proj, err := d.trackerReaderFor(projectID)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapTeam) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapTeam)
	}

	d.mu.Lock()
	d.ensureTeamsTables()
	name := ""
	_ = d.conn.QueryRow("SELECT name FROM teams WHERE id = ?", teamID).Scan(&name)
	d.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, teamsAPITimeout)
	defer cancel()

	members, err := ts.TeamMembers(ctx, tracker.TeamRequest{Project: proj, TeamID: teamID})
	if err != nil {
		return nil, err
	}
	for i := range members {
		members[i].TeamName = name
	}
	if err := d.storeTeamMembers(teamID, members); err != nil {
		return nil, err
	}

	teams, err := d.ListProjectTeams(projectID, true)
	if err != nil {
		return nil, err
	}
	for i := range teams {
		if teams[i].ID == teamID {
			return &teams[i], nil
		}
	}
	return &models.TrackerTeam{ID: teamID, Name: name, MemberCount: len(members), Members: members}, nil
}

// ListProjectTeams returns the teams carried by the project's work items, with
// their members when withMembers is set. Teams are listed from the work items
// rather than from the tracker's directory: an organisation has hundreds of
// teams, and only the ones actually on the board are useful here.
func (d *DB) ListProjectTeams(projectID string, withMembers bool) ([]models.TrackerTeam, error) {
	d.mu.Lock()
	d.ensureTeamsTables()
	d.mu.Unlock()

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Le regroupement se fait sur le nom, pas sur l'identifiant : les tickets
	// synchronisés avant que l'identifiant soit stocké n'en portent pas, et les
	// regrouper par identifiant vide les mélangerait tous dans une seule équipe.
	query := `
		SELECT t.team AS team_name, MAX(t.team_id) AS team_id, COUNT(*) AS task_count
		FROM tasks t
		WHERE TRIM(t.team) != ''`
	args := []interface{}{}
	if projectID != "" && projectID != "all" {
		query += " AND (t.project_id = ? OR t.project_id = (SELECT slug FROM projects WHERE id = ?) OR t.project_id = (SELECT id FROM projects WHERE slug = ?))"
		args = append(args, projectID, projectID, projectID)
	}
	query += " GROUP BY t.team ORDER BY task_count DESC"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	teams := []models.TrackerTeam{}
	for rows.Next() {
		var id, name string
		var count int
		if err := rows.Scan(&name, &id, &count); err != nil {
			continue
		}
		teams = append(teams, models.TrackerTeam{ID: strings.TrimSpace(id), Name: strings.TrimSpace(name), TaskCount: count})
	}
	rows.Close()

	for i := range teams {
		if teams[i].ID == "" {
			// Ancien ticket synchronisé avant que l'identifiant soit stocké : le
			// nom reste utilisable comme filtre, mais ses membres ne peuvent pas
			// être lus tant qu'une synchro n'a pas ramené l'identifiant.
			continue
		}
		var syncedAt *time.Time
		_ = d.conn.QueryRow("SELECT members_synced_at FROM teams WHERE id = ?", teams[i].ID).Scan(&syncedAt)
		if syncedAt != nil {
			teams[i].SyncedAt = syncedAt.Format(time.RFC3339)
		}

		members, err := d.teamMembersUnsafe(teams[i].ID)
		if err != nil {
			continue
		}
		teams[i].MemberCount = len(members)
		if withMembers {
			for j := range members {
				members[j].TeamName = teams[i].Name
			}
			teams[i].Members = members
		}
	}

	return teams, nil
}

func (d *DB) teamMembersUnsafe(teamID string) ([]models.TeamMember, error) {
	rows, err := d.conn.Query(`
		SELECT team_id, account_id, display_name, email, avatar_url, active
		FROM team_members WHERE team_id = ?
		ORDER BY active DESC, display_name ASC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.TeamMember{}
	for rows.Next() {
		var m models.TeamMember
		var active int
		if err := rows.Scan(&m.TeamID, &m.AccountID, &m.DisplayName, &m.Email, &m.AvatarURL, &active); err != nil {
			continue
		}
		m.Active = active == 1
		members = append(members, m)
	}
	return members, nil
}

// MembersForTeamName resolves a team by the label a work item carries, which is
// what the ticket panel has at hand. Ambiguity is possible in theory (two teams
// sharing a name) and harmless here: the union of both is still the right list of
// candidates to assign to.
func (d *DB) MembersForTeamName(teamName string) ([]models.TeamMember, error) {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" {
		return []models.TeamMember{}, nil
	}

	d.mu.Lock()
	d.ensureTeamsTables()
	d.mu.Unlock()

	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`
		SELECT m.team_id, m.account_id, m.display_name, m.email, m.avatar_url, m.active, t.name
		FROM team_members m
		JOIN teams t ON t.id = m.team_id
		WHERE t.name = ?
		ORDER BY m.active DESC, m.display_name ASC
	`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.TeamMember{}
	for rows.Next() {
		var m models.TeamMember
		var active int
		if err := rows.Scan(&m.TeamID, &m.AccountID, &m.DisplayName, &m.Email, &m.AvatarURL, &active, &m.TeamName); err != nil {
			continue
		}
		m.Active = active == 1
		members = append(members, m)
	}
	return members, nil
}

// AccountIDForAssignee resolves a display name into the tracker account id the
// assignment API needs. It looks in the members of the ticket's own team first,
// then across every known team, because a person can be moved between teams
// while the ticket keeps its own.
func (d *DB) AccountIDForAssignee(displayName string, teamName string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return ""
	}

	d.mu.Lock()
	d.ensureTeamsTables()
	d.mu.Unlock()

	d.mu.RLock()
	defer d.mu.RUnlock()

	if strings.TrimSpace(teamName) != "" {
		accountID := ""
		_ = d.conn.QueryRow(`
			SELECT m.account_id FROM team_members m
			JOIN teams t ON t.id = m.team_id
			WHERE t.name = ? AND m.display_name = ?
			ORDER BY m.active DESC LIMIT 1
		`, strings.TrimSpace(teamName), displayName).Scan(&accountID)
		if strings.TrimSpace(accountID) != "" {
			return accountID
		}
	}

	accountID := ""
	_ = d.conn.QueryRow(`
		SELECT account_id FROM team_members
		WHERE display_name = ?
		ORDER BY active DESC LIMIT 1
	`, displayName).Scan(&accountID)
	return strings.TrimSpace(accountID)
}

// GetTeamWorkload groups a team's work items by the person who owns them. The
// members with nothing assigned are kept in the list: an empty column is the
// point of the view, not a gap in it.
func (d *DB) GetTeamWorkload(projectID string, teamName string) (*models.TeamWorkload, error) {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" {
		return nil, fmt.Errorf("nom d'équipe manquant")
	}

	tasks, err := d.GetTasks("", "", "", "", projectID, "", teamName, "", "", nil, nil, false)
	if err != nil {
		return nil, err
	}

	members, err := d.MembersForTeamName(teamName)
	if err != nil {
		return nil, err
	}

	teamID := ""
	for _, m := range members {
		teamID = m.TeamID
		break
	}

	// Le rapprochement se fait sur le nom affiché : c'est ce que le tracker
	// écrit sur le ticket, et l'identifiant de compte n'est pas stocké dessus.
	loadByName := map[string]*models.TeamMemberLoad{}
	order := []string{}
	for _, m := range members {
		key := strings.ToLower(strings.TrimSpace(m.DisplayName))
		if key == "" {
			continue
		}
		if _, exists := loadByName[key]; exists {
			continue
		}
		loadByName[key] = &models.TeamMemberLoad{Member: m, Tasks: []models.Task{}, ByStatus: map[string]int{}}
		order = append(order, key)
	}

	outsideByName := map[string]*models.TeamMemberLoad{}
	outsideOrder := []string{}
	unassigned := []models.Task{}

	for _, t := range tasks {
		name := strings.TrimSpace(t.Assignee)
		if name == "" {
			unassigned = append(unassigned, t)
			continue
		}
		key := strings.ToLower(name)
		load, ok := loadByName[key]
		if !ok {
			load, ok = outsideByName[key]
			if !ok {
				load = &models.TeamMemberLoad{
					Member:   models.TeamMember{DisplayName: name, AvatarURL: t.AssigneeAvatar, Active: true},
					Tasks:    []models.Task{},
					ByStatus: map[string]int{},
				}
				outsideByName[key] = load
				outsideOrder = append(outsideOrder, key)
			}
		}
		load.Tasks = append(load.Tasks, t)
		load.ByStatus[string(t.Status)]++
		load.Total++
	}

	loads := make([]models.TeamMemberLoad, 0, len(order))
	for _, key := range order {
		loads = append(loads, *loadByName[key])
	}
	// Le plus chargé d'abord : c'est la question que la vue pose.
	sort.SliceStable(loads, func(i, j int) bool { return loads[i].Total > loads[j].Total })

	outside := make([]models.TeamMemberLoad, 0, len(outsideOrder))
	for _, key := range outsideOrder {
		outside = append(outside, *outsideByName[key])
	}
	sort.SliceStable(outside, func(i, j int) bool { return outside[i].Total > outside[j].Total })

	syncedAt := ""
	if teamID != "" {
		d.mu.RLock()
		var ts *time.Time
		_ = d.conn.QueryRow("SELECT members_synced_at FROM teams WHERE id = ?", teamID).Scan(&ts)
		d.mu.RUnlock()
		if ts != nil {
			syncedAt = ts.Format(time.RFC3339)
		}
	}

	return &models.TeamWorkload{
		Team: models.TrackerTeam{
			ID:          teamID,
			Name:        teamName,
			MemberCount: len(members),
			TaskCount:   len(tasks),
			SyncedAt:    syncedAt,
		},
		Members:    loads,
		Unassigned: unassigned,
		Outside:    outside,
	}, nil
}

// SearchAssignableUsers looks up who a work item can be assigned to. The members
// of the ticket's own team come first with no query at all, because that is the
// answer in the overwhelming majority of cases; typing then searches the whole
// instance, which is what makes assigning outside the team possible.
// SearchAssignableUsers runs with no acting user, which is what an unattended caller
// does: a tracker whose credential is personal then uses the server one.
func (d *DB) SearchAssignableUsers(taskIDOrKey string, query string) ([]models.TeamMember, error) {
	return d.SearchAssignableUsersAs(context.Background(), taskIDOrKey, query)
}

// SearchAssignableUsersAs runs on behalf of whoever asked, so a personal tracker
// credential can be resolved for the call.
func (d *DB) SearchAssignableUsersAs(ctx context.Context, taskIDOrKey string, query string) ([]models.TeamMember, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		// Pas de frappe : l'équipe du ticket suffit, et elle est déjà en base.
		if members, err := d.MembersForTeamName(task.Team); err == nil && len(members) > 0 {
			return members, nil
		}
	}

	// Typed, or no team known: ask the tracker who the work item accepts.
	ts, err := d.TrackerForTask(task)
	if err != nil || ts == nil || !ts.Supports(tracker.CapAssign) {
		return []models.TeamMember{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, teamsAPITimeout)
	defer cancel()
	people, err := ts.SearchAssignable(ctx, task.Key, query, 20)
	if err != nil {
		if tracker.IsUnsupported(err) {
			return []models.TeamMember{}, nil
		}
		return nil, err
	}
	members := make([]models.TeamMember, 0, len(people))
	for _, p := range people {
		members = append(members, models.TeamMember{AccountID: p.ID, DisplayName: p.DisplayName, Email: p.Email, AvatarURL: p.AvatarURL, Active: p.Active, TeamName: task.Team, TeamID: task.TeamID})
	}
	return members, nil
}

// SearchTrackerTeams looks up the teams of the instance by name.
// SearchTrackerTeams runs with no acting user, which is what an unattended caller
// does: a tracker whose credential is personal then uses the server one.
func (d *DB) SearchTrackerTeams(projectID string, query string) ([]models.TrackerTeam, error) {
	return d.SearchTrackerTeamsAs(context.Background(), projectID, query)
}

// SearchTrackerTeamsAs runs on behalf of whoever asked, so a personal tracker
// credential can be resolved for the call.
func (d *DB) SearchTrackerTeamsAs(ctx context.Context, projectID string, query string) ([]models.TrackerTeam, error) {
	ts, proj, err := d.trackerReaderFor(projectID)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapTeam) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapTeam)
	}
	// The caller's context, not a blank one: it names who is searching, and
	// shadowing it here sent the search out under the server's credential.
	ctx, cancel := context.WithTimeout(ctx, teamsAPITimeout)
	defer cancel()
	return ts.SearchTeams(ctx, tracker.TeamSearchRequest{Project: proj, Query: query})
}

// SetTasksTeam records the team locally on a batch of work items and queues the
// tracker write. The team is optional, so an empty teamID is a valid instruction:
// it clears the field. One activity carries the whole batch, which is what makes
// triage usable on fifty tickets.
func (d *DB) SetTasksTeam(ctx context.Context, projectID string, taskIDs []string, teamID string, teamName string) (*models.TaskActivity, error) {
	if len(taskIDs) == 0 {
		return nil, fmt.Errorf("aucun ticket sélectionné")
	}

	teamID = strings.TrimSpace(teamID)
	teamName = strings.TrimSpace(teamName)
	// Le nom peut manquer quand l'appelant n'a que l'identifiant : les équipes
	// déjà connues le donnent.
	if teamID != "" && teamName == "" {
		d.mu.RLock()
		d.ensureTeamsTables()
		_ = d.conn.QueryRow("SELECT name FROM teams WHERE id = ?", teamID).Scan(&teamName)
		d.mu.RUnlock()
	}

	now := time.Now()
	firstKey := ""
	resolved := make([]string, 0, len(taskIDs))
	for _, id := range taskIDs {
		task, err := d.GetTaskByID(id)
		if err != nil || task == nil {
			continue
		}
		if task.Source != "jira" {
			continue
		}
		if firstKey == "" {
			firstKey = task.Key
		}
		resolved = append(resolved, task.ID)

		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET team = ?, team_id = ?, updated_at = ? WHERE id = ?", teamName, teamID, now, task.ID)
		d.mu.Unlock()
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("le champ Équipe n'existe que sur un ticket Jira")
	}

	singleID := ""
	if len(resolved) == 1 {
		singleID = resolved[0]
	}

	return d.EnqueueTrackerOp(ctx, TrackerOp{
		Kind:      TrackerOpSetTeam,
		ProjectID: projectID,
		TaskID:    singleID,
		TaskKey:   firstKey,
		TaskIDs:   resolved,
		TeamID:    teamID,
		TeamName:  teamName,
	})
}

// SetTaskTeam is the single work item case, which is what the ticket panel uses.
func (d *DB) SetTaskTeam(ctx context.Context, taskIDOrKey string, teamID string, teamName string) (*models.Task, *models.TaskActivity, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche non trouvée")
	}

	activity, err := d.SetTasksTeam(ctx, task.ProjectID, []string{task.ID}, teamID, teamName)
	if err != nil {
		return nil, nil, err
	}

	updated, err := d.GetTaskByID(task.ID)
	if err != nil {
		return nil, activity, err
	}
	return updated, activity, nil
}

// SetTasksSprint records the sprint locally and queues the tracker write, for one
// work item or for a batch. Sprint membership belongs to the tracker's Agile API,
// so the same rule as everywhere applies: local first, tracker in the queue.
func (d *DB) SetTasksSprint(ctx context.Context, projectID string, taskIDs []string, sprintID string, sprintName string) (*models.TaskActivity, error) {
	if len(taskIDs) == 0 {
		return nil, fmt.Errorf("aucun ticket sélectionné")
	}

	sprintID = strings.TrimSpace(sprintID)
	sprintName = strings.TrimSpace(sprintName)

	// Le nom peut manquer quand l'appelant n'a que l'identifiant : le board du
	// projet le porte, puisque la synchro en importe la liste.
	if sprintID != "" && sprintName == "" {
		if proj, _ := d.GetProjectByID(projectID); proj != nil {
			for _, sp := range proj.Sprints {
				if sp.ID == sprintID {
					sprintName = sp.Name
					break
				}
			}
		}
	}

	firstKey := ""
	resolved := make([]string, 0, len(taskIDs))
	now := time.Now()
	for _, id := range taskIDs {
		task, err := d.GetTaskByID(id)
		if err != nil || task == nil {
			continue
		}
		if firstKey == "" {
			firstKey = task.Key
		}
		resolved = append(resolved, task.ID)

		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET sprint = ?, updated_at = ? WHERE id = ?", sprintName, now, task.ID)
		d.mu.Unlock()
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("aucun ticket trouvé")
	}

	singleID := ""
	if len(resolved) == 1 {
		singleID = resolved[0]
	}

	return d.EnqueueTrackerOp(ctx, TrackerOp{
		Kind:       TrackerOpSetSprint,
		ProjectID:  projectID,
		TaskID:     singleID,
		TaskKey:    firstKey,
		TaskIDs:    resolved,
		SprintID:   sprintID,
		SprintName: sprintName,
	})
}

// SetTaskSprint is the single work item case, which is what the ticket panel uses.
func (d *DB) SetTaskSprint(ctx context.Context, taskIDOrKey string, sprintID string, sprintName string) (*models.Task, *models.TaskActivity, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche non trouvée")
	}

	activity, err := d.SetTasksSprint(ctx, task.ProjectID, []string{task.ID}, sprintID, sprintName)
	if err != nil {
		return nil, nil, err
	}

	updated, err := d.GetTaskByID(task.ID)
	if err != nil {
		return nil, activity, err
	}
	return updated, activity, nil
}

// jiraRESTClientForTask builds a client for the tracker the ticket belongs to.
// writerForTask returns the tracker writer of the ticket's own tracker. Every
// queued write goes through it, which is what keeps the operations from naming
// Jira: they ask the writer whether the operation exists, and the writer answers
// for its own tracker.
func (d *DB) writerForTask(task *models.Task) (tracker.Writer, error) {
	if task == nil {
		return nil, fmt.Errorf("tâche manquante")
	}
	ts, err := d.TrackerForTask(task)
	if err != nil {
		return nil, err
	}
	return ts, nil
}
