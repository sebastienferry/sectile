package db

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// Workflow skills sourced from a marketplace.
//
// The registry is deployment-wide, the pin is per project, and nothing becomes
// effective without an explicit apply: a skill body is a prompt run by a CLI
// launched with permissions skipped, so a body nobody read is the one mistake
// this feature must not make. Fetching and previewing therefore write nothing
// at all, and the server never reaches the network itself — every read of a
// marketplace happens on the workstation, through the local agent.

// AddSkillMarketplace registers a source. It resolves the marketplace once
// before storing anything, so a locator that does not answer, or a repository
// without the manifest, is refused with the reason rather than registered and
// broken later.
func (d *DB) AddSkillMarketplace(entry models.SkillMarketplace) (*models.SkillMarketplace, error) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return nil, fmt.Errorf("a marketplace needs a name")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		// The name is the cache directory on the workstation as well as the
		// key a project pins against.
		return nil, fmt.Errorf("invalid marketplace name %q", name)
	}
	kind := strings.ToLower(strings.TrimSpace(entry.Kind))
	switch kind {
	case models.MarketplaceKindGithub, models.MarketplaceKindGit, models.MarketplaceKindPath:
	default:
		return nil, fmt.Errorf("unknown marketplace kind %q", entry.Kind)
	}
	locator := strings.TrimSpace(entry.Locator)
	if locator == "" {
		return nil, fmt.Errorf("a marketplace needs a locator")
	}

	var catalog models.MarketplaceCatalog
	if err := d.callAgent(agentprotocol.Operation{
		Action:      "marketplace_catalog",
		Marketplace: name,
		Kind:        kind,
		Locator:     locator,
	}, &catalog); err != nil {
		return nil, err
	}

	owner := strings.TrimSpace(entry.Owner)
	if owner == "" {
		owner = catalog.Owner
	}
	description := strings.TrimSpace(entry.Description)
	if description == "" {
		description = catalog.Description
	}
	now := time.Now().Format(time.RFC3339)

	d.mu.Lock()
	_, err := d.conn.Exec(`
		INSERT INTO skill_marketplaces (name, kind, locator, owner, description, last_commit, last_fetched_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			kind = excluded.kind,
			locator = excluded.locator,
			owner = excluded.owner,
			description = excluded.description,
			last_commit = excluded.last_commit,
			last_fetched_at = excluded.last_fetched_at
	`, name, kind, locator, owner, description, catalog.Commit, now, now)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return d.SkillMarketplace(name)
}

// SkillMarketplace reads one registered source.
func (d *DB) SkillMarketplace(name string) (*models.SkillMarketplace, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.skillMarketplaceUnsafe(name)
}

func (d *DB) skillMarketplaceUnsafe(name string) (*models.SkillMarketplace, error) {
	row := d.conn.QueryRow(`SELECT name, kind, locator, owner, description, last_commit, last_fetched_at, created_at FROM skill_marketplaces WHERE name = ?`, strings.TrimSpace(name))
	var m models.SkillMarketplace
	if err := row.Scan(&m.Name, &m.Kind, &m.Locator, &m.Owner, &m.Description, &m.LastCommit, &m.LastFetchedAt, &m.CreatedAt); err != nil {
		return nil, fmt.Errorf("unknown marketplace %q", name)
	}
	return &m, nil
}

// ListSkillMarketplaces returns the registry. Reading it is open to anyone
// signed in; writing it is a deployment setting.
func (d *DB) ListSkillMarketplaces() ([]models.SkillMarketplace, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`SELECT name, kind, locator, owner, description, last_commit, last_fetched_at, created_at FROM skill_marketplaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.SkillMarketplace{}
	for rows.Next() {
		var m models.SkillMarketplace
		if err := rows.Scan(&m.Name, &m.Kind, &m.Locator, &m.Owner, &m.Description, &m.LastCommit, &m.LastFetchedAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RemoveSkillMarketplace unregisters a source and reports the projects that
// pinned a plugin of it. Their applied bodies stay exactly as they were: a
// project must not silently change the prompts it runs because someone else
// tidied the registry. Their pin is reported as orphaned instead.
func (d *DB) RemoveSkillMarketplace(name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if _, err := d.SkillMarketplace(name); err != nil {
		return nil, err
	}
	pinned, err := d.projectsPinning(name)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	_, err = d.conn.Exec(`DELETE FROM skill_marketplaces WHERE name = ?`, name)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	// The cache on the workstation is disposable by construction; losing the
	// agent is not a reason to keep the registry row.
	_ = d.callAgent(agentprotocol.Operation{Action: "marketplace_forget", Marketplace: name}, nil)
	return pinned, nil
}

func (d *DB) projectsPinning(marketplace string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`SELECT project_id FROM project_skill_packs WHERE marketplace = ? ORDER BY project_id`, marketplace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// MarketplaceCatalog lists the plugins of one registered marketplace, with the
// workflow skills each would supply. It refreshes what the registry knows about
// the source and writes nothing on any project.
func (d *DB) MarketplaceCatalog(name string) (*models.MarketplaceCatalog, error) {
	entry, err := d.SkillMarketplace(name)
	if err != nil {
		return nil, err
	}
	var catalog models.MarketplaceCatalog
	if err := d.callAgent(agentprotocol.Operation{
		Action:      "marketplace_catalog",
		Marketplace: entry.Name,
		Kind:        entry.Kind,
		Locator:     entry.Locator,
	}, &catalog); err != nil {
		return nil, err
	}

	d.mu.Lock()
	_, _ = d.conn.Exec(`UPDATE skill_marketplaces SET last_commit = ?, last_fetched_at = ?, owner = COALESCE(NULLIF(owner, ''), ?) WHERE name = ?`,
		catalog.Commit, time.Now().Format(time.RFC3339), catalog.Owner, entry.Name)
	d.mu.Unlock()
	return &catalog, nil
}

// resolvePack asks the workstation for one plugin, at a given revision or at
// the marketplace head.
func (d *DB) resolvePack(projectID, marketplace, plugin, commit string) (models.SkillPack, error) {
	entry, err := d.SkillMarketplace(marketplace)
	if err != nil {
		return models.SkillPack{}, err
	}
	var pack models.SkillPack
	err = d.callAgent(agentprotocol.Operation{
		ProjectID:   projectID,
		Action:      "marketplace_pack",
		Marketplace: entry.Name,
		Kind:        entry.Kind,
		Locator:     entry.Locator,
		Plugin:      plugin,
		Commit:      strings.TrimSpace(commit),
	}, &pack)
	return pack, err
}

// packBaselines maps the bodies of a pack, keyed by skill directory, onto the
// workflow skill ids. A directory Sectile does not own never gets here: the
// agent already reported it as ignored.
func packBaselines(pack models.SkillPack) map[string]string {
	out := map[string]string{}
	for dir, body := range pack.Bodies {
		if stage, ok := skills.StageSkillByDirName(dir); ok && strings.TrimSpace(body) != "" {
			out[stage.ID] = body
		}
	}
	return out
}

// PreviewSkillPack shows what a pack would change and writes nothing: not the
// project's skills, not its files on disk, not its pin. Closing the preview
// leaves the project exactly as it was.
func (d *DB) PreviewSkillPack(projectIDOrPath, marketplace, plugin, commit string) (*models.SkillPackPreview, error) {
	projectID, _, framework := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return nil, fmt.Errorf("projet introuvable")
	}
	pack, err := d.resolvePack(projectID, marketplace, plugin, commit)
	if err != nil {
		return nil, err
	}

	baselines := packBaselines(pack)
	current := d.resolvedSkillBaselines(projectID, framework)
	proposed := skills.ProjectSkillTemplatesOver(framework, baselines)

	preview := models.SkillPackPreview{Pack: pack, Entries: []models.SkillPackPreviewEntry{}}
	for i, stage := range skills.StageSkills {
		if _, supplied := baselines[stage.ID]; !supplied {
			preview.Missing = append(preview.Missing, stage.DirName)
			continue
		}
		preview.Entries = append(preview.Entries, models.SkillPackPreviewEntry{
			SkillID:  stage.ID,
			DirName:  stage.DirName,
			Name:     proposed[i].Name,
			Current:  current[i].Content,
			Proposed: proposed[i].Content,
			Changed:  strings.TrimSpace(current[i].Content) != strings.TrimSpace(proposed[i].Content),
		})
	}
	return &preview, nil
}

// ApplySkillPack makes a pack the project's baseline: the one action of this
// feature that writes. It re-resolves at the previewed revision, stores the
// bodies under whatever the project edited itself, records the pin, and
// reinstalls the files exactly as saving an edited skill does.
func (d *DB) ApplySkillPack(projectIDOrPath, marketplace, plugin, commit string) (*models.SkillPackPin, error) {
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return nil, fmt.Errorf("projet introuvable")
	}
	pack, err := d.resolvePack(projectID, marketplace, plugin, commit)
	if err != nil {
		return nil, err
	}
	baselines := packBaselines(pack)
	if len(baselines) == 0 {
		return nil, fmt.Errorf("plugin %q supplies no Sectile workflow skill", plugin)
	}

	origin := packOriginLabel(pack)
	applied := make([]string, 0, len(baselines))
	now := time.Now().Format(time.RFC3339)

	d.ensureProjectSkillsTable()
	d.mu.Lock()
	for _, stage := range skills.StageSkills {
		body, supplied := baselines[stage.ID]
		if !supplied {
			// A pack that no longer ships a skill returns it to the built-in
			// body rather than keeping an orphaned one.
			_, err = d.conn.Exec(`UPDATE project_skills SET pack_content = '', pack_origin = '' WHERE project_id = ? AND skill_id = ?`, projectID, stage.ID)
			if err == nil {
				_, err = d.conn.Exec(`DELETE FROM project_skills WHERE project_id = ? AND skill_id = ? AND content = '' AND pack_content = '' AND mode = ''`, projectID, stage.ID)
			}
			if err != nil {
				break
			}
			continue
		}
		applied = append(applied, stage.DirName)
		// updated_at stays the date of the project's own edit: applying a pack
		// is not editing the skill.
		if _, err = d.conn.Exec(`
			INSERT INTO project_skills (project_id, skill_id, content, updated_at, mode, pack_content, pack_origin)
			VALUES (?, ?, '', ?, '', ?, ?)
			ON CONFLICT(project_id, skill_id) DO UPDATE SET pack_content = excluded.pack_content, pack_origin = excluded.pack_origin
		`, projectID, stage.ID, now, body, origin); err != nil {
			break
		}
	}
	if err == nil {
		appliedJSON, _ := json.Marshal(applied)
		_, err = d.conn.Exec(`
			INSERT INTO project_skill_packs (project_id, marketplace, plugin, version, commit_sha, applied_at, applied_skills)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(project_id) DO UPDATE SET
				marketplace = excluded.marketplace,
				plugin = excluded.plugin,
				version = excluded.version,
				commit_sha = excluded.commit_sha,
				applied_at = excluded.applied_at,
				applied_skills = excluded.applied_skills
		`, projectID, pack.Marketplace, pack.Plugin, pack.Version, pack.Commit, now, string(appliedJSON))
	}
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	_, writeErr := d.WriteAllProjectSkillsToRepo(projectID)
	d.recordSkillPackActivity(projectID, pack, applied, writeErr)
	return d.SkillPackPin(projectID)
}

// UnpinSkillPack puts the built-in catalogue back as the baseline. The
// project's own edits are untouched: adopting a pack is reversible, and
// reverting it is not a way of losing a customization.
func (d *DB) UnpinSkillPack(projectIDOrPath string) error {
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return fmt.Errorf("projet introuvable")
	}
	d.ensureProjectSkillsTable()

	d.mu.Lock()
	_, err := d.conn.Exec(`UPDATE project_skills SET pack_content = '', pack_origin = '' WHERE project_id = ?`, projectID)
	if err == nil {
		_, err = d.conn.Exec(`DELETE FROM project_skills WHERE project_id = ? AND content = '' AND pack_content = '' AND mode = ''`, projectID)
	}
	if err == nil {
		_, err = d.conn.Exec(`DELETE FROM project_skill_packs WHERE project_id = ?`, projectID)
	}
	d.mu.Unlock()
	if err != nil {
		return err
	}
	_, _ = d.WriteAllProjectSkillsToRepo(projectID)
	return nil
}

// SkillPackPin returns what a project applied, or nil when it runs the built-in
// catalogue. A pin whose marketplace is gone from the registry is reported as
// orphaned rather than silently reverted.
func (d *DB) SkillPackPin(projectIDOrPath string) (*models.SkillPackPin, error) {
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return nil, fmt.Errorf("projet introuvable")
	}

	d.mu.RLock()
	row := d.conn.QueryRow(`SELECT project_id, marketplace, plugin, version, commit_sha, applied_at, applied_skills FROM project_skill_packs WHERE project_id = ?`, projectID)
	pin := models.SkillPackPin{Skills: []string{}}
	var skills string
	err := row.Scan(&pin.ProjectID, &pin.Marketplace, &pin.Plugin, &pin.Version, &pin.Commit, &pin.AppliedAt, &skills)
	if err == nil {
		_, lookupErr := d.skillMarketplaceUnsafe(pin.Marketplace)
		pin.Orphaned = lookupErr != nil
	}
	d.mu.RUnlock()
	if err != nil {
		return nil, nil
	}
	_ = json.Unmarshal([]byte(skills), &pin.Skills)
	if pin.Skills == nil {
		pin.Skills = []string{}
	}
	return &pin, nil
}

// packOriginLabel is what the editor shows beside a skill whose baseline came
// from a marketplace.
func packOriginLabel(pack models.SkillPack) string {
	label := pack.Marketplace + "/" + pack.Plugin
	if pack.Version != "" {
		label += "@" + pack.Version
	}
	if len(pack.Commit) >= 7 {
		label += "+" + pack.Commit[:7]
	}
	return label
}

// recordSkillPackActivity leaves an auditable trace of an application, the way
// installing a specification framework does: what was applied, what the pack
// shipped that Sectile ignored, and what it refused.
func (d *DB) recordSkillPackActivity(projectID string, pack models.SkillPack, applied []string, writeErr error) {
	var lines []string
	lines = append(lines, fmt.Sprintf("### Pack %s appliqué\n", packOriginLabel(pack)))
	lines = append(lines, fmt.Sprintf("- Marketplace : `%s`", pack.Marketplace))
	lines = append(lines, fmt.Sprintf("- Plugin : `%s`", pack.Plugin))
	if pack.Commit != "" {
		lines = append(lines, fmt.Sprintf("- Révision : `%s`", pack.Commit))
	}
	lines = append(lines, "", fmt.Sprintf("Skills appliquées (%d) : %s", len(applied), strings.Join(applied, ", ")))
	if len(pack.Ignored) > 0 {
		ignored := append([]string{}, pack.Ignored...)
		sort.Strings(ignored)
		lines = append(lines, fmt.Sprintf("Répertoires ignorés : %s", strings.Join(ignored, ", ")))
	}
	for dir, reason := range pack.Rejected {
		lines = append(lines, fmt.Sprintf("> Refusé — %s : %s", dir, reason))
	}
	for _, warning := range pack.Warnings {
		lines = append(lines, fmt.Sprintf("> %s", warning))
	}

	steps := []string{fmt.Sprintf("✅ %d skill(s) appliquée(s)", len(applied))}
	status := models.ActivityStatusCompleted
	errText := ""
	if writeErr != nil {
		// The bodies are stored; only the repository files are missing, and the
		// activity has to say which of the two failed.
		status = models.ActivityStatusFailed
		errText = writeErr.Error()
		steps = append(steps, "❌ Régénération des fichiers du dépôt")
		lines = append(lines, fmt.Sprintf("> Erreur d'écriture : %s", errText))
	}

	now := time.Now()
	act := models.TaskActivity{
		ID:          uuid.New().String(),
		TaskID:      "skill-pack-" + projectID,
		ProjectID:   projectID,
		SkillID:     "apply_skill_pack",
		SkillName:   "Apply skill pack",
		Action:      fmt.Sprintf("Application du pack %s", packOriginLabel(pack)),
		Status:      string(status),
		Summary:     fmt.Sprintf("%d skill(s) depuis %s", len(applied), pack.Marketplace),
		Output:      strings.Join(lines, "\n"),
		Steps:       steps,
		CreatedAt:   now,
		StartedAt:   &now,
		CompletedAt: &now,
		Error:       errText,
	}

	d.mu.Lock()
	_ = d.addTaskActivityDirect(act)
	d.mu.Unlock()
}
