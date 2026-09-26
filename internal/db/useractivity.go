package db

import (
	"time"
)

// sessionTouchInterval is how often a browser session in use records that it
// was seen. It bounds the writes UserForWebSession makes, and it is also the
// precision of every "last active" the admin page shows.
var sessionTouchInterval = time.Minute

// ActiveUserWindow is how recently a session must have reached the server for
// its user to count as active. An open tab keeps its session seen through the
// event stream, so the window only has to outlast a few missed touches.
const ActiveUserWindow = 5 * time.Minute

// UserActivity is when each account last used a browser session, keyed by user
// id. An account that never did, or whose sessions have all been purged, is
// absent.
//
// The latest mark is picked here rather than with MAX(): SQLite answers an
// aggregate as text, having lost the column's DATETIME type, while the plain
// column scans as a time on both engines.
func (d *DB) UserActivity() (map[string]time.Time, error) {
	rows, err := d.conn.Query(`SELECT user_id, last_seen_at FROM web_sessions WHERE last_seen_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	activity := map[string]time.Time{}
	for rows.Next() {
		var userID string
		var seen time.Time
		if err := rows.Scan(&userID, &seen); err != nil {
			return nil, err
		}
		if seen.After(activity[userID]) {
			activity[userID] = seen
		}
	}
	return activity, rows.Err()
}

// ActiveUserCount is the number of distinct accounts holding a session that is
// still valid and was seen within the window. It is computed in the database,
// so every server instance sharing it gives the same answer.
func (d *DB) ActiveUserCount(window time.Duration) (int, error) {
	now := time.Now().UTC()
	var count int
	err := d.conn.QueryRow(`SELECT COUNT(DISTINCT s.user_id) FROM web_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.revoked_at IS NULL AND s.expires_at > ? AND s.last_seen_at >= ? AND u.blocked_at IS NULL`,
		now, now.Add(-window)).Scan(&count)
	return count, err
}

// UserCounts is the roster at a glance.
type UserCounts struct {
	Total   int `json:"total"`
	Admins  int `json:"admins"`
	Blocked int `json:"blocked"`
}

// CountUsers counts the accounts, the admins who can still sign in, and the
// blocked accounts.
func (d *DB) CountUsers() (UserCounts, error) {
	var counts UserCounts
	err := d.conn.QueryRow(`SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN role = ? AND blocked_at IS NULL THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN blocked_at IS NOT NULL THEN 1 ELSE 0 END), 0)
		FROM users`, RoleAdmin).Scan(&counts.Total, &counts.Admins, &counts.Blocked)
	return counts, err
}

// ActiveRunCounts counts the runs that are not over, by status. Every active
// status is present, at zero when no run holds it, so a gauge built from it
// drops back to zero instead of keeping its last value.
func (d *DB) ActiveRunCounts() (map[string]int, error) {
	counts := make(map[string]int, len(activeRunStatuses))
	for _, status := range activeRunStatuses {
		counts[status] = 0
	}
	rows, err := d.conn.Query(`SELECT status, COUNT(*) FROM task_activities WHERE ` + activeRunPredicate() + ` GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}
