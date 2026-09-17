package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Connection is a workstation's stored attachment to a server: where the
// server is and the API key that names this workstation to it. It lives in
// the same settings file as the local overrides, so the desktop app, the
// standalone agent and `sectile-agent pair` all read one place.
type Connection struct {
	Server   string `json:"server,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
	DeviceID string `json:"deviceId,omitempty"`
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

// WriteConnection stores the server and key beside the local overrides,
// preserving every other field of the file. The file is owner-readable only:
// the key authenticates as its owner on every machine surface.
func WriteConnection(connection Connection) error {
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
