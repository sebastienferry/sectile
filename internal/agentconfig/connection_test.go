package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
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
