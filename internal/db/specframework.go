package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// resolveSpecFrameworkTarget turns a project id, project slug or bare path into
// the working directory, framework and AI agent to use for a Spec-Driven Design
// toolchain operation. Explicit request fields always win over project defaults.
func (d *DB) resolveSpecFrameworkTarget(req models.SpecFrameworkInstallRequest) (repoPath string, framework string, aiAgent string) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	repoPath = strings.TrimSpace(req.RepoPath)
	framework = strings.TrimSpace(req.Framework)
	aiAgent = strings.TrimSpace(req.AIAgent)

	target := strings.TrimSpace(req.ProjectID)
	if target == "" {
		target = repoPath
	}

	if proj, _ := d.getProjectByIDUnsafe(target); proj != nil {
		if repoPath == "" {
			repoPath = proj.RepoPath
		}
		if framework == "" {
			framework = proj.SpecFramework
		}
		if aiAgent == "" {
			aiAgent = proj.AIProvider
		}
	}

	if s, _ := d.getSettingsUnsafe(); s != nil {
		if repoPath == "" {
			repoPath = s.RepoPath
		}
		if framework == "" {
			framework = s.SpecFramework
		}
		if aiAgent == "" {
			aiAgent = s.AIProvider
		}
	}

	if repoPath == "" {
		repoPath = "."
	}
	framework = models.NormalizeSpecFramework(framework)

	return repoPath, framework, aiAgent
}

// GetSpecFrameworkStatus reports whether the SDD toolchain CLI is reachable and
// whether the project working directory has already been initialized. Passing an
// empty framework reports on both Spec Kit and OpenSpec.
func (d *DB) GetSpecFrameworkStatus(projectID, framework string) []models.SpecFrameworkStatus {
	var result []models.SpecFrameworkStatus
	err := d.callAgent(agentprotocol.Operation{ProjectID: projectID, Action: "spec_status", Framework: framework}, &result)
	if err != nil {
		return []models.SpecFrameworkStatus{{Framework: framework, InstallHint: err.Error()}}
	}
	return result
}

func (d *DB) InstallSpecFramework(req models.SpecFrameworkInstallRequest) (*models.SpecFrameworkInstallResult, error) {
	if req.Framework != "" && !isKnownFrameworkAlias(req.Framework) {
		return nil, fmt.Errorf("unknown specification framework %q", req.Framework)
	}
	var result models.SpecFrameworkInstallResult
	if req.ProjectID == "" {
		req.ProjectID = req.RepoPath
	}
	err := d.callAgent(agentprotocol.Operation{ProjectID: req.ProjectID, Action: "spec_install", Framework: req.Framework, Provider: req.AIAgent, Force: req.Force}, &result)
	if err != nil {
		return nil, err
	}
	d.recordSpecFrameworkActivity(req.ProjectID, &result)
	return &result, nil
}

func isKnownFrameworkAlias(framework string) bool {
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "speckit", "spec-kit", "spec kit", "specify", "openspec", "open-spec", "open spec":
		return true
	}
	return false
}

// recordSpecFrameworkActivity writes a standalone activity row (no parent task)
// describing the installation, so it is visible and auditable in the UI.
func (d *DB) recordSpecFrameworkActivity(projectID string, res *models.SpecFrameworkInstallResult) {
	if res == nil {
		return
	}

	var steps []string
	var outputLines []string

	outputLines = append(outputLines, fmt.Sprintf("### Installation %s\n", res.FrameworkLabel))
	outputLines = append(outputLines, fmt.Sprintf("- Répertoire de travail : `%s`", res.RepoPath))
	if res.Version != "" {
		outputLines = append(outputLines, fmt.Sprintf("- Version : `%s`", res.Version))
	}
	outputLines = append(outputLines, "")

	for _, st := range res.Steps {
		icon := "❌"
		switch {
		case st.Skipped:
			icon = "⏭️"
		case st.Success:
			icon = "✅"
		}
		steps = append(steps, fmt.Sprintf("%s %s", icon, st.Label))
		outputLines = append(outputLines, fmt.Sprintf("%s **%s**", icon, st.Label))
		outputLines = append(outputLines, fmt.Sprintf("```\n$ %s\n```", st.Command))
		if st.Output != "" {
			outputLines = append(outputLines, fmt.Sprintf("```\n%s\n```", st.Output))
		}
		if st.Error != "" {
			outputLines = append(outputLines, fmt.Sprintf("> Erreur : %s", st.Error))
		}
		outputLines = append(outputLines, "")
	}

	if len(res.MarkerPaths) > 0 {
		outputLines = append(outputLines, fmt.Sprintf("Fichiers détectés : %s", strings.Join(res.MarkerPaths, ", ")))
	}

	if res.AlreadyInit && len(res.Steps) == 0 {
		steps = append(steps, "⏭️ Déjà initialisé, aucune commande exécutée")
	}

	status := models.ActivityStatusCompleted
	if !res.Installed {
		status = models.ActivityStatusFailed
	}

	// Installations are project-scoped, not task-scoped: reuse the same
	// synthetic task_id convention as the tracker sync activities.
	// Installing a framework is work on a project, never on a ticket. It used to
	// say so through a "spec-framework-<x>" task_id, the same made-up identifier
	// #310 removes from the column.
	// projectID is a project identifier when the caller had one and a repository
	// path otherwise (see InstallSpecFramework). Only the first is an
	// attachment: a path is not a project, and project_id is a foreign key now.
	attachedProject := ""
	if p, err := d.GetProjectByID(strings.TrimSpace(projectID)); err == nil && p != nil {
		attachedProject = p.ID
	}

	now := time.Now()
	act := models.TaskActivity{
		ID:          uuid.New().String(),
		ProjectID:   attachedProject,
		SkillID:     "install_spec_framework",
		SkillName:   fmt.Sprintf("Install %s", res.FrameworkLabel),
		Action:      fmt.Sprintf("Installation de %s dans %s", res.FrameworkLabel, res.RepoPath),
		Status:      string(status),
		Summary:     res.Message,
		Output:      strings.Join(outputLines, "\n"),
		Steps:       steps,
		CreatedAt:   now,
		StartedAt:   &now,
		CompletedAt: &now,
		Error:       res.Error,
	}

	d.mu.Lock()
	_ = d.addTaskActivityDirect(act)
	d.mu.Unlock()
}
