package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Connection is a workstation's stored attachment to a server: where the
// server is and the API key that names this workstation to it. It lives in
// the same settings file as the local overrides, where `sectile-agent pair`
// writes it and the standalone agent reads it. Sectile Desktop keeps its own
// copy in its own file and only reads this one (#746).
type Connection struct {
	Server   string `json:"server,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
	DeviceID string `json:"deviceId,omitempty"`
	// PairedAt is when the key was stored; zero for a key stored before #746,
	// which is older than any dated one.
	PairedAt time.Time `json:"pairedAt,omitempty"`
}

// NewerThan reports whether this connection's key was paired after the given
// moment. An undated key is never newer; any dated key is newer than a zero
// moment, so a key stored before #746 loses to every later pairing.
func (c Connection) NewerThan(pairedAt time.Time) bool {
	return !c.PairedAt.IsZero() && c.PairedAt.After(pairedAt)
}

// ErrNoStoredConnection reports a workstation that has never been paired.
var ErrNoStoredConnection = errors.New("no stored connection: run `sectile-agent pair` or pass --token")

// ReadConnection returns the stored server and key. A settings file without
// them, or no settings file at all, is ErrNoStoredConnection.
func ReadConnection() (Connection, error) {
	path, err := SettingsPath()
	if err != nil {
		return Connection{}, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Connection{}, ErrNoStoredConnection
	}
	if err != nil {
		return Connection{}, err
	}
	var connection Connection
	if err := json.Unmarshal(raw, &connection); err != nil {
		return Connection{}, err
	}
	connection.Server = strings.TrimRight(strings.TrimSpace(connection.Server), "/")
	connection.APIKey = strings.TrimSpace(connection.APIKey)
	if connection.APIKey == "" {
		return connection, ErrNoStoredConnection
	}
	return connection, nil
}

// WriteConnection stores the server and key beside the local overrides, with
// the moment they were paired, preserving every other field of the file except
// the keys only Sectile Desktop used (#746). The file is owner-readable only:
// the key authenticates as its owner on every machine surface.
func WriteConnection(connection Connection) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if strings.TrimSpace(connection.APIKey) != "" && connection.PairedAt.IsZero() {
		connection.PairedAt = time.Now().UTC()
	}
	return writeConnectionFields(func(fields map[string]json.RawMessage) {
		set := func(key, value string) {
			if value == "" {
				delete(fields, key)
				return
			}
			encoded, _ := json.Marshal(value)
			fields[key] = encoded
		}
		set("server", strings.TrimRight(strings.TrimSpace(connection.Server), "/"))
		set("apiKey", strings.TrimSpace(connection.APIKey))
		set("deviceId", strings.TrimSpace(connection.DeviceID))
		pairedAt := ""
		if !connection.PairedAt.IsZero() {
			pairedAt = connection.PairedAt.UTC().Format(time.RFC3339Nano)
		}
		set("pairedAt", pairedAt)
	})
}

// RecordDesktopConnection stores the server and the device of the pairing
// Sectile Desktop started the agent with, never its key, so that
// `sectile-agent pair` replaces that device's key rather than registering a
// second device (#746). A file that holds a key is left alone: its server and
// device are that key's, whichever pairing Desktop started on. A file that
// already says the same is not rewritten.
func RecordDesktopConnection(server, deviceID string) error {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	deviceID = strings.TrimSpace(deviceID)
	if server == "" || deviceID == "" {
		return nil
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	stored, err := ReadConnection()
	if err != nil && !errors.Is(err, ErrNoStoredConnection) {
		return err
	}
	if stored.APIKey != "" {
		return nil
	}
	if stored.Server == server && stored.DeviceID == deviceID {
		return nil
	}
	return writeConnectionFields(func(fields map[string]json.RawMessage) {
		fields["server"], _ = json.Marshal(server)
		fields["deviceId"], _ = json.Marshal(deviceID)
	})
}

// writeConnectionFields rewrites the settings file with change applied to its
// top-level fields, dropping the keys only Sectile Desktop used. The caller
// holds settingsMu.
func writeConnectionFields(change func(map[string]json.RawMessage)) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	fields := map[string]json.RawMessage{}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &fields); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	change(fields)
	for _, key := range desktopOnlyKeys {
		delete(fields, key)
	}
	raw, err = json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), ".settings-"+uuid.NewString()+".tmp")
	defer os.Remove(tmp)
	if err = os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
