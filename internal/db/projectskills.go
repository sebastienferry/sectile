package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tasks/internal/models"
)

// Contenu des skills édité dans l'outil.
//
// La base est la source de vérité, par projet, et le template intégré sert de
// valeur par défaut. Les SKILL.md du dépôt sont un produit de cette source :
// l'installation les régénère. Une édition faite à la main sur disque n'est pas
// écrasée en silence pour autant, elle est signalée comme divergente et peut
// être réimportée.
func (d *DB) ensureProjectSkillsTable() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS project_skills (
		project_id TEXT NOT NULL,
		skill_id   TEXT NOT NULL,
		content    TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (project_id, skill_id)
	)`)
}

type projectSkillOverride struct {
	content   string
	updatedAt string
}

func (d *DB) projectSkillOverrides(projectID string) map[string]projectSkillOverride {
	out := map[string]projectSkillOverride{}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return out
	}
	d.ensureProjectSkillsTable()

	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT skill_id, content, updated_at FROM project_skills WHERE project_id = ?`, projectID)
	d.mu.RUnlock()
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, content, updated string
		if err := rows.Scan(&id, &content, &updated); err == nil {
			out[id] = projectSkillOverride{content: content, updatedAt: updated}
		}
	}
	return out
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
// one. An empty specFramework is read from the project.
func (d *DB) EffectiveProjectSkills(projectIDOrPath, specFramework string) []ProjectSkillTemplate {
	projectID, _, framework := d.projectSkillContext(projectIDOrPath)
	if strings.TrimSpace(specFramework) != "" {
		framework = specFramework
	}
	overrides := d.projectSkillOverrides(projectID)

	timing := "implemented"
	if project, err := d.GetProjectByID(projectID); err == nil && project != nil && project.PRCreationStage == "specified" {
		timing = "specified"
	}
	out := ProjectSkillTemplates(framework)
	for i := range out {
		if ov, ok := overrides[out[i].ID]; ok && strings.TrimSpace(ov.content) != "" {
			out[i].Content = ov.content
		}
	}
	for i := range out {
		if out[i].ID != "specify" && out[i].ID != "implement" && out[i].ID != "create_pr" && out[i].ID != "pickup" && out[i].ID != "pickup_issues" {
			continue
		}
		out[i].Content += "\n## Project pull request policy\nPR creation stage: " + timing + ". Read this setting from taskflow_get_project_context before executing. "
		if timing == "specified" {
			out[i].Content += "After the specification is written and validated, commit and push the specification on the task branch and open a draft PR/MR for specification review. Reuse an existing PR/MR for that branch. Include its URL as prUrl in the specified transition. Keep it draft while implementing; update the same PR/MR and mark it ready only after implementation and review. Do not mark the task reviewed merely because a draft exists.\n"
		} else {
			out[i].Content += "Create the PR/MR after implementation and review, reusing any existing PR/MR for the task branch. Do not create one during specification.\n"
		}
	}

	return out
}

// ListProjectSkillEditor feeds the in-app editor.
func (d *DB) ListProjectSkillEditor(projectIDOrPath string) ([]models.SkillEditorEntry, error) {
	projectID, repoPath, framework := d.projectSkillContext(projectIDOrPath)
	overrides := d.projectSkillOverrides(projectID)
	defaults := ProjectSkillTemplates(framework)

	entries := make([]models.SkillEditorEntry, 0, len(StageSkills))
	for i, stage := range StageSkills {
		def := defaults[i]
		content := def.Content
		updatedAt := ""
		isCustom := false
		if ov, ok := overrides[stage.ID]; ok && strings.TrimSpace(ov.content) != "" {
			content = ov.content
			updatedAt = ov.updatedAt
			isCustom = true
		}

		entry := models.SkillEditorEntry{
			ID:             stage.ID,
			Name:           def.Name,
			DirName:        stage.DirName,
			Command:        stage.Command,
			Description:    stage.Description,
			FromStage:      stage.FromStage,
			ToStage:        stage.ToStage,
			Scope:          stage.Scope,
			Interactive:    stage.Interactive,
			Content:        content,
			DefaultContent: def.Content,
			IsCustom:       isCustom,
			UpdatedAt:      updatedAt,
			Paths:          []string{},
		}

		// État sur disque : le premier fichier trouvé fait référence, et on
		// signale l'écart plutôt que de le corriger d'autorité.
		for _, dir := range SkillDirsFor(repoPath, stage.DirName) {
			p := filepath.Join(dir, "SKILL.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			entry.Paths = append(entry.Paths, p)
			if !entry.Installed {
				entry.Installed = true
				entry.RepoPath = p
				if strings.TrimSpace(string(raw)) != strings.TrimSpace(content) {
					entry.Diverged = true
					entry.RepoContent = string(raw)
				}
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// SaveProjectSkillContent stores the edited content and regenerates the file in
// the repository, so the agent reads what the editor shows.
func (d *DB) SaveProjectSkillContent(projectIDOrPath, skillID, content string) (*models.SkillEditorEntry, error) {
	stage, ok := StageSkillByID(skillID)
	if !ok {
		return nil, fmt.Errorf("skill %q inconnue", skillID)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("contenu vide : utilise la réinitialisation pour revenir au modèle intégré")
	}

	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	if projectID == "" {
		return nil, fmt.Errorf("projet introuvable")
	}
	d.ensureProjectSkillsTable()

	d.mu.Lock()
	_, err := d.conn.Exec(`
		INSERT INTO project_skills (project_id, skill_id, content, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(project_id, skill_id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at
	`, projectID, stage.ID, content, time.Now().Format(time.RFC3339))
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	written, writeErr := d.WriteProjectSkillToRepo(projectIDOrPath, stage.ID)
	entry, findErr := d.projectSkillEntry(projectIDOrPath, stage.ID)
	if findErr != nil {
		return nil, findErr
	}
	if writeErr != nil && written == 0 {
		// La base a bien enregistré : l'éditeur reste utilisable même si le
		// dépôt n'est pas accessible, on le dit dans l'entrée renvoyée.
		entry.RepoPath = ""
	}
	return entry, nil
}

// ResetProjectSkillContent drops the override and puts the built-in template
// back, in the database and in the repository.
func (d *DB) ResetProjectSkillContent(projectIDOrPath, skillID string) (*models.SkillEditorEntry, error) {
	stage, ok := StageSkillByID(skillID)
	if !ok {
		return nil, fmt.Errorf("skill %q inconnue", skillID)
	}
	projectID, _, _ := d.projectSkillContext(projectIDOrPath)
	d.ensureProjectSkillsTable()

	d.mu.Lock()
	_, err := d.conn.Exec(`DELETE FROM project_skills WHERE project_id = ? AND skill_id = ?`, projectID, stage.ID)
	d.mu.Unlock()
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	_, _ = d.WriteProjectSkillToRepo(projectIDOrPath, stage.ID)
	return d.projectSkillEntry(projectIDOrPath, stage.ID)
}

// ImportProjectSkillFromRepo takes the file on disk as the new content: the way
// out when a SKILL.md was edited by hand and that edit is the one to keep.
func (d *DB) ImportProjectSkillFromRepo(projectIDOrPath, skillID string) (*models.SkillEditorEntry, error) {
	stage, ok := StageSkillByID(skillID)
	if !ok {
		return nil, fmt.Errorf("skill %q inconnue", skillID)
	}
	_, repoPath, _ := d.projectSkillContext(projectIDOrPath)

	for _, dir := range SkillDirsFor(repoPath, stage.DirName) {
		p := filepath.Join(dir, "SKILL.md")
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(raw)) == "" {
			continue
		}
		return d.SaveProjectSkillContent(projectIDOrPath, stage.ID, string(raw))
	}
	return nil, fmt.Errorf("aucun SKILL.md de %s trouvé sous %s", stage.DirName, repoPath)
}

// WriteProjectSkillToRepo regenerates one skill in every agent directory of the
// main checkout and of each worktree. It returns how many files were written.
func (d *DB) WriteProjectSkillToRepo(projectIDOrPath, skillID string) (int, error) {
	stage, ok := StageSkillByID(skillID)
	if !ok {
		return 0, fmt.Errorf("skill %q inconnue", skillID)
	}
	_, repoPath, _ := d.projectSkillContext(projectIDOrPath)

	content := ""
	for _, s := range d.EffectiveProjectSkills(projectIDOrPath, "") {
		if s.ID == stage.ID {
			content = s.Content
			break
		}
	}
	if content == "" {
		return 0, fmt.Errorf("contenu introuvable pour %s", stage.ID)
	}

	_, _, framework := d.projectSkillContext(projectIDOrPath)

	written := 0
	var lastErr error
	for _, root := range getGitWorktreePaths(repoPath) {
		// La commande slash suit le même contenu : c'est par elle que l'étape est
		// appelée, la skill seule n'est pas invocable par « /nom ».
		if cmdContent, ok := commandContentFromSkill(stage, content, framework); ok {
			cmdPath := SkillCommandPath(root, stage.DirName)
			if err := os.MkdirAll(filepath.Dir(cmdPath), 0755); err == nil {
				if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err == nil {
					written++
				}
			}
		}

		for _, dir := range SkillDirsFor(root, stage.DirName) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				lastErr = err
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
				lastErr = err
				continue
			}
			written++
		}
	}
	if written == 0 && lastErr != nil {
		return 0, lastErr
	}
	return written, nil
}

// WriteAllProjectSkillsToRepo regenerates all workflow skills across all agent directories.
func (d *DB) WriteAllProjectSkillsToRepo(projectIDOrPath string) (int, error) {
	total := 0
	for _, s := range StageSkills {
		n, err := d.WriteProjectSkillToRepo(projectIDOrPath, s.ID)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
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
func commandContentFromSkill(stage StageSkill, content, specFramework string) (string, bool) {
	if strings.TrimSpace(content) == "" {
		return CommandContentFor(stage.ID, specFramework)
	}

	body := content
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---\n"); end >= 0 {
			body = body[4+end+5:]
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", skillYAMLString(stage.Description))
	b.WriteString("argument-hint: <TICKET-KEY> [contexte]\n")
	b.WriteString("---\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n\n## Ticket\n$ARGUMENTS\n")
	return b.String(), true
}
