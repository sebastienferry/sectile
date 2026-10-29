package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agentconfig"
)

func TestEmbeddedAgentHandsRestartToOwnerAfterGatewayStops(t *testing.T) {
	root := t.TempDir()
	discovery := filepath.Join(root, "agent-connection.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restarted := make(chan struct{}, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- RunEmbedded(ctx, EmbeddedConfig{
			ServerURL: "http://127.0.0.1:1", Token: "agent-test-key",
			ProjectID: "all", DeviceID: "embedded-test-device",
			DesktopInfo: discovery, DesktopToken: "desktop-test-key",
			SettingsRoot: root, RepoRoot: root,
			OnRestart: func() { restarted <- struct{}{} },
		})
	}()
	var info struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(discovery)
		if err == nil && json.Unmarshal(raw, &info) == nil && info.URL != "" {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("embedded agent stopped before discovery: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if info.URL == "" || info.Token != "desktop-test-key" {
		t.Fatalf("embedded discovery was not published")
	}
	request, err := http.NewRequest(http.MethodPost, info.URL+"/desktop/restart", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+info.Token)
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("restart returned %d", response.StatusCode)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("embedded agent did not stop after restart")
	}
	select {
	case <-restarted:
	default:
		t.Fatal("embedded restart was not handed to the owner")
	}
	if _, err := client.Get(info.URL + "/health"); err == nil {
		t.Fatal("embedded gateway still accepts requests after stop")
	}
}

func TestEmbeddedSettingsRemainInPrivateRoot(t *testing.T) {
	globalRoot := t.TempDir()
	t.Setenv("SECTILE_DESKTOP_DATA_DIR", globalRoot)
	privateRoot := t.TempDir()
	d := &agentDaemon{settingsRoot: privateRoot}
	_, err := d.updateSettings(func(settings *agentconfig.Settings) error {
		settings.Repositories = map[string]string{"github.com/example/one": privateRoot}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := d.withSettingsPath(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = rememberRepositoryFolderAt(settingsPathFromContext(ctx), privateRoot, "github.com/example/two", privateRoot); err != nil {
		t.Fatal(err)
	}
	settings, err := d.readSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Repositories) != 2 {
		t.Fatalf("private settings contain %d repositories, want two", len(settings.Repositories))
	}
	if _, err := os.Stat(filepath.Join(globalRoot, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("global settings were touched: %v", err)
	}
}
