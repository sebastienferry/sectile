package db

import (
	"encoding/json"
	"strings"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// SaveCapabilities stores what a workstation reported it will run, one row per
// project, replacing its previous report for each (#305).
func (d *DB) SaveCapabilities(userID string, report agentconfig.CapabilityReport) error {
	now := time.Now().UTC()
	return d.conn.WithTx(func(tx *sqlTx) error {
		for _, c := range report.Projects {
			projectID := strings.TrimSpace(c.ProjectID)
			if projectID == "" {
				continue
			}
			skillModels, _ := json.Marshal(nonNilMap(c.SkillModels))
			list, _ := json.Marshal(nonNilList(c.Models))
			if _, err := tx.Exec(`INSERT INTO agent_capabilities (user_id, device_id, project_id, provider, model, skill_models, models, model_slot, headless, reported_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (user_id, device_id, project_id) DO UPDATE SET provider = excluded.provider, model = excluded.model,
					skill_models = excluded.skill_models, models = excluded.models, model_slot = excluded.model_slot,
					headless = excluded.headless, reported_at = excluded.reported_at`,
				userID, strings.TrimSpace(report.DeviceID), projectID, strings.TrimSpace(c.Provider), strings.TrimSpace(c.Model),
				string(skillModels), string(list), boolInt(c.ModelSlot), boolInt(c.Headless), now); err != nil {
				return err
			}
		}
		return nil
	})
}

// EngineReport returns what a person's workstation reported for a project.
// With a device, that workstation's report; without one, the latest report of
// any of their workstations. False when nothing was reported.
func (d *DB) EngineReport(userID, projectID, deviceID string) (models.EngineReport, bool) {
	query := `SELECT provider, model, skill_models, models, model_slot, headless, reported_at FROM agent_capabilities
		WHERE user_id = ? AND project_id = ?`
	args := []any{userID, projectID}
	if deviceID != "" {
		query += " AND device_id = ?"
		args = append(args, deviceID)
	}
	query += " ORDER BY reported_at DESC LIMIT 1"
	var report models.EngineReport
	var skillModels, list string
	var slot, headless int
	if err := d.conn.QueryRow(query, args...).Scan(&report.Provider, &report.Model, &skillModels, &list, &slot, &headless, &report.ReportedAt); err != nil {
		return models.EngineReport{State: models.EngineUnknown}, false
	}
	_ = json.Unmarshal([]byte(skillModels), &report.SkillModels)
	_ = json.Unmarshal([]byte(list), &report.Models)
	report.SkillModels = nonNilMap(report.SkillModels)
	report.Models = nonNilList(report.Models)
	report.ModelSlot, report.Headless = slot == 1, headless == 1
	report.State = models.EngineReported
	return report, true
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func nonNilList(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
