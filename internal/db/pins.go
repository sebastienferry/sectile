package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
)

// PinnedLabel est le label dédié utilisé sur tous les trackers (Jira, GitHub, Local)
// pour marquer les tickets épinglés.
const PinnedLabel = "pinned"

// HasPinnedLabel vérifie si un des labels correspond à "pinned" (insensible à la casse et au préfixe #).
func HasPinnedLabel(labels []string) bool {
	for _, l := range labels {
		clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(l, "#")))
		if clean == PinnedLabel {
			return true
		}
	}
	return false
}

// AddPinnedLabel ajoute le label "pinned" s'il n'est pas déjà présent.
func AddPinnedLabel(labels []string) []string {
	if HasPinnedLabel(labels) {
		return labels
	}
	return append(labels, PinnedLabel)
}

// RemovePinnedLabel retire toutes les variantes du label "pinned".
func RemovePinnedLabel(labels []string) []string {
	var out []string
	for _, l := range labels {
		clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(l, "#")))
		if clean == PinnedLabel {
			continue
		}
		out = append(out, l)
	}
	return out
}

// Tickets épinglés, pour basculer vite entre les chantiers en cours.
//
// L'épinglage s'appuie désormais sur un label dédié ("pinned") synchronisé avec
// tous les trackers distants (Jira, GitHub), combiné à une table
// pinned_tasks et une colonne/index pinned sur tasks en base locale pour
// optimiser les requêtes.
func (d *DB) ensurePinsTable() {
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS pinned_tasks (
		task_id   TEXT PRIMARY KEY,
		pinned_at TEXT NOT NULL
	)`)
}

// migratePinnedTasks synchronise la table pinned_tasks, la colonne pinned et le label "pinned" sur les tâches existantes.
func (d *DB) migratePinnedTasks() {
	d.ensurePinsTable()

	// 1. Tâches présentes dans pinned_tasks : s'assurer que pinned = 1 et le label "pinned" est présent
	rows, err := d.conn.Query("SELECT id, labels FROM tasks WHERE id IN (SELECT task_id FROM pinned_tasks)")
	if err == nil {
		type taskInfo struct {
			id     string
			labels []string
		}
		var toUpdate []taskInfo
		for rows.Next() {
			var id, labelsJSON string
			if err := rows.Scan(&id, &labelsJSON); err == nil {
				var lbls []string
				_ = json.Unmarshal([]byte(labelsJSON), &lbls)
				if !HasPinnedLabel(lbls) {
					lbls = AddPinnedLabel(lbls)
					toUpdate = append(toUpdate, taskInfo{id: id, labels: lbls})
				}
			}
		}
		rows.Close()
		for _, u := range toUpdate {
			lj, _ := json.Marshal(u.labels)
			_, _ = d.conn.Exec("UPDATE tasks SET labels = ?, pinned = 1 WHERE id = ?", string(lj), u.id)
		}
	}

	// 2. Tâches ayant déjà le label "pinned" : s'assurer que pinned = 1 et qu'une entrée existe dans pinned_tasks
	rows2, err := d.conn.Query("SELECT id, labels, updated_at FROM tasks WHERE labels LIKE '%pinned%' OR labels LIKE '%Pinned%'")
	if err == nil {
		for rows2.Next() {
			var id, labelsJSON, updatedAt string
			if err := rows2.Scan(&id, &labelsJSON, &updatedAt); err == nil {
				var lbls []string
				_ = json.Unmarshal([]byte(labelsJSON), &lbls)
				if HasPinnedLabel(lbls) {
					_, _ = d.conn.Exec("UPDATE tasks SET pinned = 1 WHERE id = ?", id)
					_, _ = d.conn.Exec(`
						INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
						ON CONFLICT(task_id) DO NOTHING
					`, id, updatedAt)
				}
			}
		}
		rows2.Close()
	}
}

// PinnedTasks returns the pinned tickets, most recently pinned first.
//
// Les tâches sont renvoyées entières, pas seulement leurs identifiants : la
// barre d'épingles doit s'afficher même quand un ticket est filtré ou appartient
// à un autre projet que celui ouvert.
func (d *DB) PinnedTasks() ([]models.Task, error) {
	d.ensurePinsTable()

	d.mu.RLock()
	rows, err := d.conn.Query(`SELECT task_id FROM pinned_tasks ORDER BY pinned_at DESC`)
	if err != nil {
		d.mu.RUnlock()
		return []models.Task{}, nil
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	// Récupérer également les tickets avec pinned = 1 non encore présents dans pinned_tasks
	extraRows, err := d.conn.Query(`
		SELECT id FROM tasks WHERE pinned = 1 AND id NOT IN (SELECT task_id FROM pinned_tasks)
		ORDER BY updated_at DESC
	`)
	if err == nil {
		for extraRows.Next() {
			var id string
			if err := extraRows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		extraRows.Close()
	}
	d.mu.RUnlock()

	out := make([]models.Task, 0, len(ids))
	stale := []string{}
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		task, err := d.GetTaskByID(id)
		if err != nil || task == nil {
			// Ticket disparu d'une synchro à l'autre : l'épingle n'a plus d'objet.
			stale = append(stale, id)
			continue
		}
		if !HasPinnedLabel(task.Labels) {
			stale = append(stale, id)
			continue
		}
		task.Pinned = true
		out = append(out, *task)
	}
	for _, id := range stale {
		_ = d.SetTaskPinned(id, false)
	}
	return out, nil
}

// SetTaskPinned pins or unpins a ticket, updates its labels, database flag and synchronizes with trackers.
// SetTaskPinned pins or unpins with no acting user.
func (d *DB) SetTaskPinned(taskID string, pinned bool) error {
	return d.SetTaskPinnedBy(Actor{}, taskID, pinned)
}

// SetTaskPinnedBy pins on behalf of whoever asked: the label it writes on the
// tracker goes out under their credential.
func (d *DB) SetTaskPinnedBy(actor Actor, taskID string, pinned bool) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("identifiant de tâche manquant")
	}
	d.ensurePinsTable()

	d.mu.Lock()
	defer d.mu.Unlock()

	task, err := d.getTaskByIDUnsafe(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("tâche non trouvée: %s", taskID)
	}

	oldLabels := task.Labels
	hasPin := HasPinnedLabel(task.Labels)
	now := time.Now()

	if pinned {
		if !hasPin {
			task.Labels = AddPinnedLabel(task.Labels)
		}
		task.Pinned = true
		task.UpdatedAt = now

		_, _ = d.conn.Exec(`
			INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
			ON CONFLICT(task_id) DO UPDATE SET pinned_at = excluded.pinned_at
		`, task.ID, now.Format(time.RFC3339))

		labelsJSON, _ := json.Marshal(task.Labels)
		_, err = d.conn.Exec(`
			UPDATE tasks
			SET labels = ?, pinned = 1, updated_at = ?
			WHERE id = ?
		`, string(labelsJSON), now, task.ID)
		if err != nil {
			return err
		}

		if !hasPin {
			d.enqueueTrackerUpdateAsUnsafe(actor.ID, task, nil, task.Labels, nil, TrackerFieldChanges{})
		}
	} else {
		if hasPin {
			task.Labels = RemovePinnedLabel(task.Labels)
		}
		task.Pinned = false
		task.UpdatedAt = now

		_, _ = d.conn.Exec(`DELETE FROM pinned_tasks WHERE task_id = ?`, task.ID)

		labelsJSON, _ := json.Marshal(task.Labels)
		_, err = d.conn.Exec(`
			UPDATE tasks
			SET labels = ?, pinned = 0, updated_at = ?
			WHERE id = ?
		`, string(labelsJSON), now, task.ID)
		if err != nil {
			return err
		}

		if hasPin {
			var actualRemoved []string
			for _, ol := range oldLabels {
				clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(ol, "#")))
				if clean == PinnedLabel {
					actualRemoved = append(actualRemoved, ol)
				}
			}
			if len(actualRemoved) == 0 {
				actualRemoved = []string{PinnedLabel}
			}
			d.enqueueTrackerUpdateAsUnsafe(actor.ID, task, nil, task.Labels, actualRemoved, TrackerFieldChanges{})
		}
	}
	return nil
}

// ToggleTaskPinned flips the pin and says what the new state is.
// ToggleTaskPinned flips the pin with no acting user.
func (d *DB) ToggleTaskPinned(taskID string) (bool, error) {
	return d.ToggleTaskPinnedBy(Actor{}, taskID)
}

// ToggleTaskPinnedBy flips it on behalf of whoever asked.
func (d *DB) ToggleTaskPinnedBy(actor Actor, taskID string) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, fmt.Errorf("identifiant de tâche manquant")
	}
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil {
		return false, err
	}
	if task == nil {
		return false, fmt.Errorf("tâche %s non trouvée", taskID)
	}

	isCurrentlyPinned := HasPinnedLabel(task.Labels)
	if !isCurrentlyPinned {
		d.mu.RLock()
		var existing string
		_ = d.conn.QueryRow(`SELECT task_id FROM pinned_tasks WHERE task_id = ? OR task_id = ?`, task.ID, task.Key).Scan(&existing)
		d.mu.RUnlock()
		if existing != "" {
			isCurrentlyPinned = true
		}
	}

	newPinned := !isCurrentlyPinned
	if err := d.SetTaskPinned(task.ID, newPinned); err != nil {
		return false, err
	}
	return newPinned, nil
}
