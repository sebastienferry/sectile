package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/models"
)

type MacroMetadata struct {
	Title          *string `json:"title,omitempty"`
	Description    *string `json:"description,omitempty"`
	FramingComment *string `json:"framingComment,omitempty"`
	Horizon        *string `json:"horizon,omitempty"`
	Closed         *bool   `json:"closed,omitempty"`
	Priority       *string `json:"priority,omitempty"`
	Quarter        *string `json:"quarter,omitempty"`
	Readiness      *string `json:"readiness,omitempty"`
}

func (edit MacroMetadata) validate() error {
	if edit.Title != nil && strings.TrimSpace(*edit.Title) == "" {
		return fmt.Errorf("titre de la macro obligatoire")
	}
	if edit.Horizon != nil && strings.TrimSpace(*edit.Horizon) != "" && normalizeHorizon(*edit.Horizon) == "" {
		return fmt.Errorf("horizon de macro invalide : %s", *edit.Horizon)
	}
	for _, axis := range []struct {
		value     *string
		normalize func(string) (string, error)
	}{
		{edit.Priority, NormalizeEpicPriority}, {edit.Quarter, NormalizeQuarter}, {edit.Readiness, NormalizeReadiness},
	} {
		if axis.value != nil {
			if _, err := axis.normalize(*axis.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DB) UpdateMacroMetadata(ctx context.Context, projectID, key string, edit MacroMetadata) (*models.MacroMeta, string, error) {
	if edit == (MacroMetadata{}) {
		return nil, "", fmt.Errorf("aucune métadonnée de macro à modifier")
	}
	if err := edit.validate(); err != nil {
		return nil, "", err
	}
	macro, err := d.GetMacro(projectID, key)
	if err != nil {
		return nil, "", err
	}
	return d.SaveMacroEdit(ctx, macro.ProjectID, macro.Key, edit, nil)
}

func (d *DB) SaveMacroEdit(ctx context.Context, projectID, key string, edit MacroMetadata, todos *[]models.MacroTodo) (*models.MacroMeta, string, error) {
	if err := edit.validate(); err != nil {
		return nil, "", err
	}
	saved, err := d.UpdateMacro(ctx, projectID, key, edit.Title, edit.Horizon, edit.Description, edit.FramingComment, todos, edit.Closed)
	if err != nil {
		d.FillMacroFlags(projectID, saved, bulkMacroEdit(ctx))
		return saved, "", err
	}
	note := ""
	if edit.Horizon != nil {
		if d.MacroIsForeign(projectID, key) {
			note = "conservé dans Sectile, non écrit sur le tracker"
		} else {
			note = "label roadmap en file d'attente"
			if _, err := d.EnqueueTrackerOp(ctx, TrackerOp{Kind: TrackerOpEpicHorizon, ProjectID: projectID, TaskKey: key, EpicKey: key, Horizon: saved.Horizon}); err != nil {
				note = "label roadmap non mis en file : " + err.Error()
			}
		}
	}
	if edit.Priority != nil || edit.Quarter != nil || edit.Readiness != nil {
		axes, err := d.SaveMacroAxes(projectID, key, edit.Priority, edit.Quarter, edit.Readiness)
		if err != nil {
			return saved, note, err
		}
		saved = axes
		d.FillMacroFlags(projectID, saved, bulkMacroEdit(ctx))
		axisNote := d.enqueueMacroAxes(ctx, projectID, key, saved, edit.Priority != nil, edit.Quarter != nil, edit.Readiness != nil)
		if note != "" {
			note += "; "
		}
		note += axisNote
	} else {
		d.FillMacroFlags(projectID, saved, bulkMacroEdit(ctx))
	}
	return saved, note, nil
}

func (d *DB) enqueueMacroAxes(ctx context.Context, projectID, key string, saved *models.MacroMeta, priority, quarter, readiness bool) string {
	if !saved.AxesWritable {
		return "conservé dans Sectile, non écrit sur le tracker"
	}
	ops := []TrackerOp{}
	if priority {
		ops = append(ops, TrackerOp{Kind: TrackerOpEpicPriority, ProjectID: projectID, TaskKey: key, EpicKey: key, Priority: saved.Priority})
	}
	if quarter {
		ops = append(ops, TrackerOp{Kind: TrackerOpEpicQuarter, ProjectID: projectID, TaskKey: key, EpicKey: key, Quarter: saved.Quarter})
	}
	if readiness && saved.LabelsWritable {
		ops = append(ops, TrackerOp{Kind: TrackerOpEpicReadiness, ProjectID: projectID, TaskKey: key, EpicKey: key, Readiness: saved.Readiness})
	}
	if len(ops) == 0 {
		return "conservé dans Sectile, non écrit sur le tracker"
	}
	for _, op := range ops {
		if _, err := d.EnqueueTrackerOp(ctx, op); err != nil {
			return "labels non mis en file : " + err.Error()
		}
	}
	return "labels en file d'attente"
}
