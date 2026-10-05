package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// Skill content edited in the tool.
//
// The database is the source of truth, per project, and the built-in template
// is the default. The copies on a workstation are a product of this source,
// written only by an explicit setup (the agent's init, the desktop's Initialize,
// an install request); a dispatch hands a custom skill to its run instead
// (#267). A copy edited by hand on disk is never overwritten silently: it is
// reported as diverged and can be imported back.
func (d *DB) ensureProjectSkillsTable() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS project_skills (
		mode TEXT NOT NULL DEFAULT '',
		project_id TEXT NOT NULL,
		skill_id   TEXT NOT NULL,
		content    TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (project_id, skill_id)
	)`)
	// mode : le mode d'exécution propre à la skill. Vide vaut « pas d'avis »,
	// ce qui laisse la précédence retomber sur le défaut du projet.
	if d.dialect.RunsLegacyMigrations() {
		_, _ = d.conn.Exec(`ALTER TABLE project_skills ADD COLUMN mode TEXT NOT NULL DEFAULT ''`)
	}
}

type projectSkillOverride struct {
	content   string
	updatedAt string
	mode      string
	// kind is what content replaces: the whole skill, or only its work
	// sections (#732).
	kind models.SkillOverrideKind
}

func (d *DB) projectSkillOverrides(projectID string) map[string]projectSkillOverride {
	out := map[string]projectSkillOverride{}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return out
	}
	d.ensureProjectSkillsTable()

	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT skill_id, content, updated_at, mode, override_kind FROM project_skills WHERE project_id = ?`, projectID)
	d.mu.RUnlock()
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, content, updated string
		var mode, kind sql.NullString
		if err := rows.Scan(&id, &content, &updated, &mode, &kind); err == nil {
			out[id] = projectSkillOverride{content: content, updatedAt: updated, mode: models.NormalizeSkillMode(mode.String), kind: models.SkillOverrideKind(kind.String)}
		}
	}
	return out
}

// ProjectSkillMode is the execution mode a skill carries for one project: the
// mode stored for it when there is one, otherwise the built-in mode of the
// catalogue entry. An empty result means the skill has no opinion, which lets
// the precedence fall through to the project default.
func (d *DB) ProjectSkillMode(projectIDOrPath, skillID string) string {
	// Resolve the id the way the setter does. skills.StageSkillByID knows the aliases
	// NormalizeSkillID does not (pickup-issue, pick, rewrite-story), and reading
	// under a different key than the one written makes a pinned mode silently
	// do nothing.
	stage, known := skills.StageSkillByID(skillID)
	if !known {
		return models.SkillModeUnset
	}
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if ov, ok := resolvedSkillOverride(d.projectSkillOverrides(projectID), stage.ID); ok && ov.mode != models.SkillModeUnset {
		return ov.mode
	}
	return models.NormalizeSkillMode(stage.Mode)
}

// SetProjectSkillMode pins the execution mode of one skill for a project. An
// empty mode clears the setting, which puts the skill back on the project
// default. The row is created when the skill has no edited content yet: the
// mode is a setting of its own, not a by-product of editing the content.
func (d *DB) SetProjectSkillMode(projectIDOrPath, skillID, mode string) error {
	stage, ok := skills.StageSkillByID(skillID)
	if !ok {
		return fmt.Errorf("skill inconnue : %s", skillID)
	}
	if !models.ValidSkillMode(mode) {
		return fmt.Errorf("mode invalide : %s", mode)
	}
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return fmt.Errorf("projet introuvable")
	}
	d.ensureProjectSkillsTable()

	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`
		INSERT INTO project_skills (project_id, skill_id, content, updated_at, mode)
		VALUES (?, ?, '', ?, ?)
		ON CONFLICT(project_id, skill_id) DO UPDATE SET mode = excluded.mode, updated_at = excluded.updated_at
	`, projectID, stage.ID, time.Now().Format(time.RFC3339), models.NormalizeSkillMode(mode))
	return err
}

// projectSkillContext resolves what a project needs to render and install its
// skills. It accepts an id or a raw repository path, like the rest of the skill
// API does.
func (d *DB) projectSkillContext(projectIDOrPath string) (projectID, repoPath, specFramework string) {
	projectID = projectIDOrPath
	repoPath = projectIDOrPath
	specFramework = "speckit"

	d.mu.RLock()
	proj, _ := d.getProjectByIDUnsafe(projectIDOrPath)
	d.mu.RUnlock()
	if proj != nil {
		projectID = proj.ID
		if strings.TrimSpace(proj.RepoPath) != "" {
			repoPath = proj.RepoPath
		}
		if strings.TrimSpace(proj.SpecFramework) != "" {
			specFramework = proj.SpecFramework
		}
	}
	if strings.TrimSpace(repoPath) == "" {
		repoPath = "."
	}
	return projectID, repoPath, specFramework
}

// EffectiveProjectSkills returns what should actually be written for a project:
// the built-in template, replaced by the project's edited content when there is
// one. A full replacement takes the place of the whole skill; a work-only
// override (#732) is composed with the Sectile contracts, and so is pickup when
// one of the stages it inlines carries one. An empty specFramework is read from
// the project.
func (d *DB) EffectiveProjectSkills(projectIDOrPath, specFramework string) []skills.ProjectSkillTemplate {
	projectID, _, framework := d.projectSkillContext(projectIDOrPath)
	if strings.TrimSpace(specFramework) != "" {
		framework = specFramework
	}
	overrides := d.projectSkillOverrides(projectID)
	work := projectWorkOverrides(overrides)

	timing := models.PRCreationImplemented
	if project, err := d.GetProjectByID(projectID); err == nil && project != nil && models.ValidPRCreationStage(project.PRCreationStage) {
		timing = project.PRCreationStage
	}
	out := skills.ProjectSkillTemplates(framework)
	for i := range out {
		ov, ok := resolvedSkillOverride(overrides, out[i].ID)
		if ok && ov.kind == models.SkillOverrideFull && strings.TrimSpace(ov.content) != "" {
			out[i].Content = ov.content
			if out[i].ID == "adjust" {
				stage, _ := skills.StageSkillByID("adjust")
				contract := skills.RenderSkillContent(stage, framework)
				if strings.TrimSpace(out[i].Content) != strings.TrimSpace(contract) {
					out[i].Content = adjustmentCustomContent(out[i].Content, contract)
				}
			}
			continue
		}
		if stage, known := skills.StageSkillByID(out[i].ID); known && composesWork(work, stage.ID) {
			out[i].Content = skills.RenderComposedSkillContent(stage, framework, work)
			// The content is a composite, not a replacement: an agent layers
			// its workstation's work sections on it section by section.
			out[i].OverrideKind = models.SkillOverrideWork
			if ok && ov.kind == models.SkillOverrideWork {
				out[i].WorkContent = ov.content
			}
		}
	}
	for i := range out {
		out[i].Content += skills.ProjectSkillPolicy(out[i].ID, timing)
	}

	return out
}

// ListProjectSkillEditor feeds the in-app editor.
func (d *DB) ListProjectSkillEditor(projectIDOrPath string) ([]models.SkillEditorEntry, error) {
	projectID, _, framework := d.projectSkillContext(projectIDOrPath)
	overrides := d.projectSkillOverrides(projectID)
	defaults := skills.ProjectSkillTemplates(framework)
	var localFiles map[string]agentprotocol.SkillFile
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// The server editor remains usable offline; disk evidence is optional.
	_ = d.callAgentContext(ctx, agentprotocol.Operation{ProjectID: projectID, Action: "skill_files"}, &localFiles)

	entries := make([]models.SkillEditorEntry, 0, len(skills.StageSkills))
	for i, stage := range skills.StageSkills {
		// A hand transition records a stage without running one: there is
		// nothing for a project to edit (#732).
		if stage.HandTransition {
			continue
		}
		def := defaults[i]
		content := def.Content
		updatedAt := ""
		isCustom := false
		kind := models.SkillOverrideFull
		if ov, ok := resolvedSkillOverride(overrides, stage.ID); ok && strings.TrimSpace(ov.content) != "" {
			content = ov.content
			updatedAt = ov.updatedAt
			isCustom = strings.TrimSpace(content) != strings.TrimSpace(def.Content)
			kind = ov.kind
		}
		overridable := skills.Overridable(stage)
		defaultWork := ""
		if overridable {
			defaultWork = skills.FormatWorkSections(stage, skills.BuiltinWorkSections(stage, framework))
		}

		mode := models.NormalizeSkillMode(stage.Mode)
		if ov, ok := resolvedSkillOverride(overrides, stage.ID); ok && ov.mode != models.SkillModeUnset {
			mode = ov.mode
		}
		origin, conflicts := adjustmentOverrideOrigin(overrides)
		entry := models.SkillEditorEntry{
			ID:             stage.ID,
			Name:           def.Name,
			DirName:        stage.DirName,
			Command:        stage.Command,
			Description:    stage.Description,
			FromStage:      stage.FromStage,
			ToStage:        stage.ToStage,
			Scope:          stage.Scope,
			Mode:           mode,
			Content:        content,
			DefaultContent: def.Content,
			IsCustom:       isCustom,
			UpdatedAt:      updatedAt,
			Paths:          []string{},

			OverrideKind:       kind,
			DefaultWorkContent: defaultWork,
			Overridable:        overridable,
		}

		if stage.ID == "adjust" {
			entry.OverrideOrigin = origin
			if p, _ := d.GetProjectByID(projectID); p != nil && origin != "adjust" {
				for _, id := range []string{"review"} {
					if command := strings.TrimSpace(p.SkillOverrides[id]); command != "" && strings.TrimSpace(p.SkillOverrides["adjust"]) == "" {
						entry.Content += "\n\nLegacy command override (" + id + "): " + command
						entry.RequiresReconciliation = true
						entry.IsCustom = true
					}
				}
			}
			entry.LegacyConflicts = conflicts
			entry.LegacyContents = map[string]string{}
			for _, id := range conflicts {
				entry.LegacyContents[id] = overrides[id].content
			}
			entry.RequiresReconciliation = entry.RequiresReconciliation || origin == "review"
			settings, _ := d.GetSettings()
			if settings != nil && strings.TrimSpace(settings.PromptCreatePR) != "" && origin != "adjust" {
				entry.RequiresReconciliation = true
				entry.Content += "\n\nLegacy global prompt:\n" + settings.PromptCreatePR
				entry.IsCustom = true
			}
		}

		if file, ok := localFiles[stage.ID]; ok && len(file.Paths) > 0 {
			entry.Paths = file.Paths
			entry.RepoPath = file.Paths[0]
			entry.Installed = true
			// The direct copy is the generic skill, shared by every project of
			// the workstation: it diverges when it is neither of the two forms
			// the direct setup writes, which is a hand edit or a copy an
			// earlier release rendered for one project.
			installed := strings.TrimSpace(file.Content)
			entry.Diverged = installed != strings.TrimSpace(skills.RenderDirectSkillContent(stage)) &&
				installed != strings.TrimSpace(skills.RenderDirectSkillCommand(stage))
			if entry.Diverged {
				entry.RepoContent = file.Content
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// SaveProjectSkillContent stores the edited content. Nothing is written on any
// workstation (#267): the next dispatch of the skill hands it to its run.
//
// kind says what the content replaces (#732). Left nil, a new override is
// work-only when the skill takes one and a full replacement otherwise, while an
// existing row keeps the kind it has. A work-only override must parse into the
// skill's work sections.
func (d *DB) SaveProjectSkillContent(projectIDOrPath, skillID, content string, kind *models.SkillOverrideKind) (*models.SkillEditorEntry, error) {
	stage, ok := skills.StageSkillByID(skillID)
	if !ok {
		return nil, fmt.Errorf("skill %q inconnue", skillID)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("contenu vide : utilise la réinitialisation pour revenir au modèle intégré")
	}
	if kind != nil && !models.ValidSkillOverrideKind(*kind) {
		return nil, fmt.Errorf("type de surcharge invalide : %s", *kind)
	}

	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return nil, fmt.Errorf("projet introuvable")
	}
	d.ensureProjectSkillsTable()

	effective := d.defaultSkillOverrideKind(projectID, stage)
	if kind != nil {
		effective = *kind
	}
	if effective == models.SkillOverrideWork {
		if !skills.Overridable(stage) || stage.ID == "transition" {
			return nil, fmt.Errorf("la skill %q ne se surcharge qu'en remplacement complet", stage.ID)
		}
		if _, err := skills.ParseWorkSections(stage, content); err != nil {
			return nil, fmt.Errorf("surcharge du travail invalide : %w", err)
		}
	}

	d.mu.Lock()
	_, err := d.conn.Exec(`
		INSERT INTO project_skills (project_id, skill_id, content, updated_at, mode, override_kind)
		VALUES (?, ?, ?, ?, '', ?)
		ON CONFLICT(project_id, skill_id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at, override_kind = excluded.override_kind
	`, projectID, stage.ID, content, time.Now().Format(time.RFC3339), string(effective))
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return d.projectSkillEntry(projectIDOrPath, stage.ID)
}

// defaultSkillOverrideKind is the kind a save that names none takes: the kind
// of the override already stored, or work-only for a new override of a skill
// that takes one. A row holding only a mode is no override yet.
func (d *DB) defaultSkillOverrideKind(projectID string, stage skills.StageSkill) models.SkillOverrideKind {
	if ov, ok := d.projectSkillOverrides(projectID)[stage.ID]; ok && strings.TrimSpace(ov.content) != "" {
		return ov.kind
	}
	if skills.Overridable(stage) && stage.ID != "transition" {
		return models.SkillOverrideWork
	}
	return models.SkillOverrideFull
}

// ResetProjectSkillContent drops the override and puts the built-in template
// back. Like a save, it writes nothing on any workstation.
func (d *DB) ResetProjectSkillContent(projectIDOrPath, skillID string) (*models.SkillEditorEntry, error) {
	stage, ok := skills.StageSkillByID(skillID)
	if !ok {
		return nil, fmt.Errorf("skill %q inconnue", skillID)
	}
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	d.ensureProjectSkillsTable()

	d.mu.Lock()
	var err error
	if stage.ID == "adjust" {
		// An explicit reset selects the default while retaining legacy entries.
		_, err = d.conn.Exec(`INSERT INTO project_skills(project_id, skill_id, content, updated_at) VALUES (?, 'adjust', ?, ?) ON CONFLICT(project_id,skill_id) DO UPDATE SET content=excluded.content,updated_at=excluded.updated_at,override_kind=''`, projectID, skills.RenderSkillContent(stage, ""), time.Now().Format(time.RFC3339))
	} else {
		_, err = d.conn.Exec(`DELETE FROM project_skills WHERE project_id = ? AND skill_id = ?`, projectID, stage.ID)
	}
	d.mu.Unlock()
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return d.projectSkillEntry(projectIDOrPath, stage.ID)
}

// ImportProjectSkillFromRepo takes the file on disk as the new content: the way
// out when a SKILL.md was edited by hand and that edit is the one to keep.
func (d *DB) ImportProjectSkillFromRepo(projectIDOrPath, skillID string) (*models.SkillEditorEntry, error) {
	var result struct{ Content string }
	if err := d.callAgent(agentprotocol.Operation{ProjectID: projectIDOrPath, Action: "read_skill", SkillID: skillID}, &result); err != nil {
		return nil, err
	}
	// The file on disk is a whole SKILL.md: it is stored as a full replacement.
	full := models.SkillOverrideFull
	return d.SaveProjectSkillContent(projectIDOrPath, skillID, result.Content, &full)
}

func (d *DB) projectSkillEntry(projectIDOrPath, skillID string) (*models.SkillEditorEntry, error) {
	entries, err := d.ListProjectSkillEditor(projectIDOrPath)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].ID == skillID {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("skill %q inconnue", skillID)
}

// commandContentFromSkill builds the slash command of a skill from the content
// actually stored, so an edited skill and its command never diverge.
func commandContentFromSkill(stage skills.StageSkill, content, specFramework string) (string, bool) {
	if strings.TrimSpace(content) == "" {
		return skills.CommandContentFor(stage.ID, specFramework)
	}

	body := content
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---\n"); end >= 0 {
			body = body[4+end+5:]
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", skills.YAMLString(stage.Description))
	b.WriteString("argument-hint: <TICKET-KEY> [contexte]\n")
	b.WriteString("---\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n\n## Ticket\n$ARGUMENTS\n")
	return b.String(), true
}

// Canonical customization wins; losing entries remain available for reconciliation.
func adjustmentOverrideOrigin(overrides map[string]projectSkillOverride) (string, []string) {
	origin := ""
	conflicts := []string{}
	for _, id := range []string{"adjust", "review"} {
		if strings.TrimSpace(overrides[id].content) != "" {
			if origin == "" {
				origin = id
			} else {
				conflicts = append(conflicts, id)
			}
		}
	}
	return origin, conflicts
}
func resolvedSkillOverride(overrides map[string]projectSkillOverride, id string) (projectSkillOverride, bool) {
	if id == "adjust" {
		origin, _ := adjustmentOverrideOrigin(overrides)
		value, ok := overrides[origin]
		return value, ok
	}
	value, ok := overrides[id]
	return value, ok
}

// projectWorkOverrides are a project's work-only overrides, parsed. A stored
// row that no longer parses (the catalogue changed under it) is skipped, so the
// skill falls back to the built-in rather than failing every run.
func projectWorkOverrides(overrides map[string]projectSkillOverride) skills.SkillOverrides {
	work := skills.SkillOverrides{}
	for id, ov := range overrides {
		if ov.kind != models.SkillOverrideWork || strings.TrimSpace(ov.content) == "" {
			continue
		}
		stage, ok := skills.StageSkillByID(id)
		if !ok {
			continue
		}
		if sections, err := skills.ParseWorkSections(stage, ov.content); err == nil {
			work[stage.ID] = sections
		}
	}
	return work
}

// composesWork says a skill is composed from work-only overrides: its own, or
// those of a stage it inlines.
func composesWork(work skills.SkillOverrides, id string) bool {
	if _, ok := work[id]; ok {
		return true
	}
	for _, inlined := range skills.ComposedStageIDs(id) {
		if _, ok := work[inlined]; ok {
			return true
		}
	}
	return false
}

// Preserve canonical metadata when a legacy document is reconciled under Adjust.
func adjustmentCustomContent(custom, contract string) string {
	strip := func(content string) string {
		if strings.HasPrefix(content, "---\n") {
			if end := strings.Index(content[4:], "\n---\n"); end >= 0 {
				return content[4+end+5:]
			}
		}
		return content
	}
	body := strip(contract)
	header := strings.TrimSuffix(contract, body)
	return header + "## Project instructions\n\n" + strip(custom) + "\n\n## Mandatory adjustment requirements\n\n" + body
}
