package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func checkPresenceDeviceUpgrade(t *testing.T, d *DB) {
	t.Helper()
	// Recreate the deployed version-50 table with populated legacy rows.
	for _, query := range []string{
		`DROP INDEX task_launch_affinity`,
		`ALTER TABLE task_activities DROP COLUMN device_id`,
		`DROP TABLE agent_presence`,
		`DROP TABLE IF EXISTS agent_workstation_presence`,
		`CREATE TABLE agent_presence (user_id TEXT NOT NULL, project_id TEXT NOT NULL, instance_id TEXT NOT NULL, device_id TEXT NOT NULL DEFAULT '', connected_at DATETIME NOT NULL, disconnected_at DATETIME, PRIMARY KEY (user_id, project_id))`,
		`DELETE FROM schema_migrations WHERE version >= 51`,
	} {
		if _, err := d.conn.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if version, _ := d.schemaVersion(); version == 50 {
			_ = d.applyMigrations(migrations, 50)
		}
	})
	older := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	newer := older.Add(time.Minute)
	for _, row := range []struct {
		user, project, device string
		at                    time.Time
		off                   any
	}{
		{"u", "old", "laptop", older, nil},
		{"u", "z-tie", "laptop", newer, nil},
		{"u", "a-tie", "laptop", newer, newer},
		{"u", "desktop-project", "desktop", older, nil},
		{"u", "legacy", "", older, nil},
		{"other", "a-tie", "laptop", older, nil},
	} {
		if _, err := d.conn.Exec(`INSERT INTO agent_presence (user_id, project_id, instance_id, device_id, connected_at, disconnected_at) VALUES (?, ?, 'holder', ?, ?, ?)`, row.user, row.project, row.device, row.at, row.off); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.applyMigrations(migrations, 50); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM agent_presence`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
	var project, instance string
	var at time.Time
	var off sql.NullTime
	if err := d.conn.QueryRow(`SELECT project_id, instance_id, connected_at, disconnected_at FROM agent_presence WHERE user_id='u' AND device_id='laptop'`).Scan(&project, &instance, &at, &off); err != nil {
		t.Fatal(err)
	}
	if project != "a-tie" || instance != "holder" || !at.Equal(newer) || !off.Valid || !off.Time.Equal(newer) {
		t.Fatalf("retained row: %s %s %s %+v", project, instance, at, off)
	}
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM agent_presence WHERE user_id='u' AND device_id='unknown'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unknown fallback: %d %v", count, err)
	}
	// The new unique key rejects duplicate devices even under another project.
	if _, err := d.conn.Exec(`INSERT INTO agent_presence (user_id,project_id,instance_id,device_id,connected_at) VALUES ('u','another','holder','laptop',?)`, newer); err == nil {
		t.Fatal("duplicate user/device accepted")
	}
	if err := d.applyMigrations(migrations, migrations[len(migrations)-1].version); err != nil {
		t.Fatalf("repeated migration: %v", err)
	}
}

func TestPresenceDeviceUpgradeSQLite(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	checkPresenceDeviceUpgrade(t, d)
}
func TestPostgresPresenceDeviceUpgrade(t *testing.T) { checkPresenceDeviceUpgrade(t, openPostgres(t)) }

func checkDevicePresenceGenerations(t *testing.T, a, b *DB) {
	t.Helper()
	first := time.Now().UTC().Truncate(time.Microsecond)
	replacement := first.Add(time.Millisecond)
	if _, _, err := a.AgentConnected("devices", "all", "laptop", first); err != nil {
		t.Fatal(err)
	}
	if _, had, err := b.AgentConnected("devices", "all", "desktop", first); err != nil || had {
		t.Fatalf("distinct device takeover: %v %v", had, err)
	}
	if _, ok := a.AgentOwner("devices", "project"); ok {
		t.Fatal("ambiguous presence selected an owner")
	}
	if _, _, err := a.AgentConnected("devices", "all", "laptop", replacement); err != nil {
		t.Fatal(err)
	}
	a.AgentDisconnected("devices", "all", "laptop", first)
	locations := a.ConnectedAgentLocations()
	count := 0
	for _, location := range locations {
		if location.UserID == "devices" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("stale disconnection removed device: %+v", locations)
	}
	previous, had, err := b.AgentConnected("devices", "project", "laptop", replacement.Add(time.Millisecond))
	if err != nil || !had || previous.DeviceID != "laptop" || !previous.ConnectedAt.Equal(replacement) {
		t.Fatalf("same-device takeover: %+v %v %v", previous, had, err)
	}
	a.AgentDisconnected("devices", "all", "laptop", replacement)
	count = 0
	for _, location := range a.ConnectedAgentLocations() {
		if location.UserID == "devices" {
			count++
		}
	}
	if count != 2 {
		t.Fatal("old instance removed replacement")
	}
}
func TestPresenceDeviceGenerationsSQLite(t *testing.T) {
	a, b := twoServingStores(t)
	checkDevicePresenceGenerations(t, a, b)
}
func TestPostgresPresenceDeviceGenerations(t *testing.T) {
	a, b := openPostgresPair(t)
	_, _ = a.conn.Exec(`DELETE FROM agent_presence`)
	_, _ = a.conn.Exec(`DELETE FROM server_instances`)
	stopA, err := a.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stopA()
	stopB, err := b.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stopB()
	checkDevicePresenceGenerations(t, a, b)
}

func TestPostgresConcurrentSameDeviceTakeoversAreSerialized(t *testing.T) {
	a, b := openPostgresPair(t)
	_, _ = a.conn.Exec(`DELETE FROM agent_presence`)
	_, _ = a.conn.Exec(`DELETE FROM server_instances`)
	stopA, err := a.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stopA()
	stopB, err := b.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stopB()
	const n = 20
	results := make(chan error, n)
	race(n, a, b, func(i int, d *DB) { _, _, err := d.AgentConnected("concurrent", "all", "same"); results <- err })
	for i := 0; i < n; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := a.conn.QueryRow(`SELECT COUNT(*) FROM agent_presence WHERE user_id='concurrent' AND device_id='same'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("slots=%d err=%v", count, err)
	}
}
