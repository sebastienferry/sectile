package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

// A key written by `sectile-agent pair` must not be outranked by one the
// desktop encrypted earlier (#717).
func TestWriteConnectionRetiresTheEncryptedSecret(t *testing.T) {
	// writeSettingsFile points the home at a temporary directory (testhome.Temp).
	path := writeSettingsFile(t, `{"secret":"old","custom":1}`)

	if err := WriteConnection(Connection{Server: "https://sectile.example.test/", APIKey: "sectile_new", DeviceID: "dev_1"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, kept := fields["secret"]; kept {
		t.Fatalf("the superseded secret was kept: %s", raw)
	}
	if fields["apiKey"] != "sectile_new" || fields["server"] != "https://sectile.example.test" || fields["deviceId"] != "dev_1" {
		t.Fatalf("connection not stored: %s", raw)
	}
	if fields["custom"] != float64(1) {
		t.Fatalf("unrelated field lost: %s", raw)
	}
}

// Pairing again reads the device the workstation held even when it has no
// key any more, so the server can revoke that device's key.
func TestReadConnectionReturnsTheStoredDeviceWithoutAKey(t *testing.T) {
	writeSettingsFile(t, `{"server":"https://sectile.example.test/","deviceId":"dev_0"}`)

	connection, err := ReadConnection()
	if !errors.Is(err, ErrNoStoredConnection) {
		t.Fatalf("err = %v, want ErrNoStoredConnection", err)
	}
	if connection.DeviceID != "dev_0" || connection.Server != "https://sectile.example.test" {
		t.Fatalf("connection = %+v", connection)
	}
}

// readFields reads the settings file's top-level fields.
func readFields(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

// A pairing records when it was made, so the newest of a Desktop pairing and
// a `sectile-agent pair` wins (#746), and drops what only Desktop used.
func TestWriteConnectionStampsThePairingAndDropsDesktopKeys(t *testing.T) {
	path := writeSettingsFile(t, `{"appearance":"dark","consoleView":"terminal","repo":"/r","binary":"/b","layout":4,"defaults":{"parallelism":2}}`)
	before := time.Now().UTC().Add(-time.Second)

	if err := WriteConnection(Connection{Server: "https://sectile.example.test", APIKey: "sectile_new", DeviceID: "dev_1"}); err != nil {
		t.Fatal(err)
	}
	fields := readFields(t, path)
	for _, key := range desktopOnlyKeys {
		if _, kept := fields[key]; kept {
			t.Fatalf("%s kept: %v", key, fields)
		}
	}
	if fields["defaults"] == nil || fields["layout"] != float64(4) {
		t.Fatalf("agent settings lost: %v", fields)
	}
	connection, err := ReadConnection()
	if err != nil {
		t.Fatal(err)
	}
	if !connection.NewerThan(before) {
		t.Fatalf("pairedAt = %v, want after %v", connection.PairedAt, before)
	}
}

func TestNewerThanTreatsAnUndatedKeyAsTheOldest(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name     string
		stored   time.Time
		since    time.Time
		expected bool
	}{
		{"later", now, now.Add(-time.Minute), true},
		{"earlier", now.Add(-time.Minute), now, false},
		{"undated stored key", time.Time{}, now, false},
		{"undated stored key, unknown start", time.Time{}, time.Time{}, false},
		{"dated stored key, unknown start", now, time.Time{}, true},
	}
	for _, c := range cases {
		if got := (Connection{PairedAt: c.stored}).NewerThan(c.since); got != c.expected {
			t.Errorf("%s: NewerThan = %v, want %v", c.name, got, c.expected)
		}
	}
}

// The agent Desktop starts records the server and device of its pairing,
// never a key (#746).
func TestRecordDesktopConnectionStoresServerAndDeviceOnly(t *testing.T) {
	path := writeSettingsFile(t, `{"layout":4,"secret":"desktop"}`)

	if err := RecordDesktopConnection("https://sectile.example.test/", "dev_desktop"); err != nil {
		t.Fatal(err)
	}
	fields := readFields(t, path)
	if fields["server"] != "https://sectile.example.test" || fields["deviceId"] != "dev_desktop" {
		t.Fatalf("connection not recorded: %v", fields)
	}
	for _, key := range []string{"apiKey", "pairedAt", "secret"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("%s present: %v", key, fields)
		}
	}
}

func TestRecordDesktopConnectionKeepsAKeyForAnotherServer(t *testing.T) {
	const stored = `{"server":"https://other.example.test","apiKey":"cli","deviceId":"dev_cli"}`
	path := writeSettingsFile(t, stored)

	if err := RecordDesktopConnection("https://sectile.example.test", "dev_desktop"); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != stored {
		t.Fatalf("file rewritten: %s", raw)
	}
}

func TestRecordDesktopConnectionLeavesAnUpToDateFileAlone(t *testing.T) {
	const stored = `{"server":"https://sectile.example.test","apiKey":"cli","deviceId":"dev_1"}`
	path := writeSettingsFile(t, stored)

	if err := RecordDesktopConnection("https://sectile.example.test", "dev_1"); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != stored {
		t.Fatalf("file rewritten: %s", raw)
	}
	if err := RecordDesktopConnection("https://sectile.example.test", ""); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != stored {
		t.Fatalf("file rewritten without a device: %s", raw)
	}
}
