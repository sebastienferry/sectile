package handlers

import (
	"log"
	"net/http"

	"tasks/internal/db"
	"tasks/internal/models"
)

// enqueueMacroAxes queues the label write of each epic axis the request
// changed, and returns the note the macro response carries.
//
// A macro whose epic cannot carry a label keeps the values in Sectile and gets
// no queued write at all: that is the expected outcome on a GitHub milestone, a
// local project or an epic of a roadmap project that was not opted in, and a
// failed activity per edit would report it as an error.
func (h *Handler) enqueueMacroAxes(r *http.Request, projectID string, key string, saved *models.MacroMeta, priority bool, quarter bool, readiness bool) string {
	if !saved.AxesWritable {
		return "conservé dans Sectile, non écrit sur le tracker"
	}
	ops := []db.TrackerOp{}
	if priority {
		ops = append(ops, db.TrackerOp{Kind: db.TrackerOpEpicPriority, ProjectID: projectID, TaskKey: key, EpicKey: key, Priority: saved.Priority})
	}
	if quarter {
		ops = append(ops, db.TrackerOp{Kind: db.TrackerOpEpicQuarter, ProjectID: projectID, TaskKey: key, EpicKey: key, Quarter: saved.Quarter})
	}
	// The readiness is not among the axes a roadmap project's epic opens to
	// (#632): only an epic whose labels are ours takes it.
	if readiness && saved.LabelsWritable {
		ops = append(ops, db.TrackerOp{Kind: db.TrackerOpEpicReadiness, ProjectID: projectID, TaskKey: key, EpicKey: key, Readiness: saved.Readiness})
	}
	if len(ops) == 0 {
		return "conservé dans Sectile, non écrit sur le tracker"
	}
	for _, op := range ops {
		if _, err := h.db.EnqueueTrackerOp(h.actingContext(r), op); err != nil {
			log.Printf("[macros] label d'axe non mis en file pour %s: %v", key, err)
			return "labels non mis en file : " + err.Error()
		}
	}
	return "labels en file d'attente"
}
