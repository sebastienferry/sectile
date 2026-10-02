package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

// pairingServer answers the one call pairing makes, recording what it was sent.
func pairingServer(t *testing.T, code string) (*httptest.Server, *map[string]string) {
	t.Helper()
	var seen map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/pair" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&seen)
		w.Header().Set("Content-Type", "application/json")
		if seen["code"] != code {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid or expired pairing code"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"sectile_issued","deviceId":"dev_1","userId":"usr_1"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestPairStoresTheKeyTheDaemonThenStartsFrom(t *testing.T) {
	testhome.Temp(t)
	srv, seen := pairingServer(t, "code-1")

	message, err := Pair([]string{"--url", srv.URL + "/", "--code", " code-1 ", "--label", "laptop"})
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if (*seen)["code"] != "code-1" || (*seen)["label"] != "laptop" {
		t.Fatalf("server received %v", *seen)
	}
	if !strings.Contains(message, srv.URL) || !strings.Contains(message, "dev_1") {
		t.Fatalf("message does not name the server and workstation: %s", message)
	}

	stored, err := agentconfig.ReadConnection()
	if err != nil {
		t.Fatalf("read stored connection: %v", err)
	}
	if stored.Server != srv.URL || stored.APIKey != "sectile_issued" || stored.DeviceID != "dev_1" {
		t.Fatalf("stored = %+v", stored)
	}
	path, _ := agentconfig.SettingsPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("settings readable by others: %o", info.Mode().Perm())
	}
	// The daemon picks the stored key for the paired server and nothing else.
	if got := resolveCredential("", "", srv.URL, stored); got != "sectile_issued" {
		t.Fatalf("resolved %q for the paired server", got)
	}
	if got := resolveCredential("", "", "https://other.example.test", stored); got != "" {
		t.Fatalf("stored key lent to another server: %q", got)
	}
	if got := resolveCredential("", "sectile_env", srv.URL, stored); got != "sectile_env" {
		t.Fatalf("environment did not win over the stored key: %q", got)
	}
	if got := resolveCredential("sectile_flag", "sectile_env", srv.URL, stored); got != "sectile_flag" {
		t.Fatalf("flag did not win: %q", got)
	}
}

func TestPairKeepsTheOtherSettingsAndRefusesABadCode(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)
	path := filepath.Join(home, ".config", "sectile", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"projects":{"p1":"/repo"},"terminal":"iterm"}`), 0600); err != nil {
		t.Fatal(err)
	}
	srv, _ := pairingServer(t, "code-2")

	if _, err := Pair([]string{"--url", srv.URL, "--code", "wrong"}); err == nil || !strings.Contains(err.Error(), "pairing code") {
		t.Fatalf("bad code error = %v", err)
	}
	if _, err := agentconfig.ReadConnection(); err == nil {
		t.Fatal("a refused pairing stored a key")
	}
	// Without a code the browser signs in; one that never comes back times out.
	stubBrowser(t, func(string) error { return nil })
	shortenBrowserSignIn(t, 50*time.Millisecond)
	if _, err := Pair([]string{"--url", srv.URL}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("pairing without a code nor a browser = %v", err)
	}
	if _, err := agentconfig.ReadConnection(); err == nil {
		t.Fatal("an unfinished browser sign-in stored a key")
	}
	if _, err := Pair([]string{"--code", "code-2"}); err == nil {
		t.Fatal("pairing without a server succeeded")
	}

	if _, err := Pair([]string{"--url", srv.URL, "--code", "code-2"}); err != nil {
		t.Fatalf("pair: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["terminal"] != "iterm" || fields["apiKey"] != "sectile_issued" {
		t.Fatalf("settings after pairing: %s", raw)
	}
	if projects, _ := fields["projects"].(map[string]any); projects["p1"] != "/repo" {
		t.Fatalf("project mappings lost: %s", raw)
	}
}

func TestExpiryNoticeOnlyInsideTheWarningWindow(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	if got := expiryNotice(nil, now); got != "" {
		t.Fatalf("a key without expiry warned: %q", got)
	}
	far := now.Add(30 * 24 * time.Hour)
	if got := expiryNotice(&far, now); got != "" {
		t.Fatalf("a key with a month left warned: %q", got)
	}
	nine := now.Add(9*24*time.Hour + time.Hour)
	if got := expiryNotice(&nine, now); !strings.Contains(got, "10 days") {
		t.Fatalf("nine days and an hour left = %q, want a 10 days warning", got)
	}
	soon := now.Add(3 * time.Hour)
	if got := expiryNotice(&soon, now); !strings.Contains(got, "1 day") {
		t.Fatalf("three hours left = %q", got)
	}
}

// stubBrowser replaces the browser `pair` opens for one test.
func stubBrowser(t *testing.T, open func(string) error) {
	t.Helper()
	previous := openBrowser
	openBrowser = open
	t.Cleanup(func() { openBrowser = previous })
}

// Pairing again names the device the workstation held, so the server revokes
// its key; a device stored for another server is not sent (#717).
func TestPairSendsTheStoredDeviceSoTheServerRevokesIt(t *testing.T) {
	testhome.Temp(t)
	srv, seen := pairingServer(t, "code-1")
	if err := agentconfig.WriteConnection(agentconfig.Connection{Server: srv.URL, APIKey: "sectile_old", DeviceID: "dev_0"}); err != nil {
		t.Fatal(err)
	}

	message, err := Pair([]string{"--url", srv.URL + "/", "--code", "code-1", "--label", "laptop"})
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if (*seen)["deviceId"] != "dev_0" {
		t.Fatalf("server received %v", *seen)
	}
	if !strings.Contains(message, "previous key of this workstation is revoked") {
		t.Fatalf("message does not say the previous key is revoked: %s", message)
	}

	other, otherSeen := pairingServer(t, "code-2")
	message, err = Pair([]string{"--url", other.URL, "--code", "code-2", "--label", "laptop"})
	if err != nil {
		t.Fatalf("pair with another server: %v", err)
	}
	if _, sent := (*otherSeen)["deviceId"]; sent {
		t.Fatalf("a device of another server was sent: %v", *otherSeen)
	}
	if strings.Contains(message, "revoked") {
		t.Fatalf("message claims a revocation another server did not make: %s", message)
	}
}

// Without --code, `pair` signs in through the browser and spends the code the
// server sends back to the loopback; --no-browser only prints the URL.
func TestPairWithoutACodeSignsInThroughTheBrowser(t *testing.T) {
	testhome.Temp(t)
	srv, seen := pairingServer(t, "good-code")

	stubBrowser(t, func(string) error {
		t.Error("--no-browser opened the browser")
		return nil
	})
	shortenBrowserSignIn(t, 50*time.Millisecond)
	if _, err := Pair([]string{"--url", srv.URL, "--no-browser"}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("--no-browser without a sign-in = %v", err)
	}

	shortenBrowserSignIn(t, 5*time.Second)
	stubBrowser(t, func(target string) error {
		if status := followSignIn(t, target, "good-code", ""); status != http.StatusOK {
			t.Errorf("callback answered %d", status)
		}
		return nil
	})
	if _, err := Pair([]string{"--url", srv.URL, "--label", "laptop"}); err != nil {
		t.Fatalf("pair: %v", err)
	}
	if (*seen)["code"] != "good-code" || (*seen)["label"] != "laptop" {
		t.Fatalf("server received %v", *seen)
	}
	stored, err := agentconfig.ReadConnection()
	if err != nil || stored.APIKey != "sectile_issued" {
		t.Fatalf("stored = %+v, %v", stored, err)
	}
}

// The key a registration held is revoked by pairing again: an existing
// registration for this server follows the new key, and a provider without
// one still has none.
func TestPairRefreshesAnExistingMCPRegistration(t *testing.T) {
	home := testhome.Temp(t)
	srv, _ := pairingServer(t, "code-1")
	claude := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claude, []byte(`{"mcpServers":{"sectile":{"type":"http","url":"`+srv.URL+`/mcp","headers":{"Authorization":"Bearer old"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	agy := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(agy), 0700); err != nil {
		t.Fatal(err)
	}
	unregistered := `{"mcpServers":{"other":{"command":"other-server"}}}`
	if err := os.WriteFile(agy, []byte(unregistered), 0600); err != nil {
		t.Fatal(err)
	}

	message, err := Pair([]string{"--url", srv.URL, "--code", "code-1"})
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	raw, err := os.ReadFile(claude)
	if err != nil || !strings.Contains(string(raw), "Bearer sectile_issued") || strings.Contains(string(raw), "Bearer old") {
		t.Fatalf("registration not refreshed: %s %v", raw, err)
	}
	if !strings.Contains(message, "MCP configuration updated for: claude") {
		t.Fatalf("message does not name the refreshed provider: %s", message)
	}
	if raw, err := os.ReadFile(agy); err != nil || string(raw) != unregistered {
		t.Fatalf("a provider without a registration was given one: %s %v", raw, err)
	}
}
