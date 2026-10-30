package companion

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/storelock"
)

type updateFixture struct {
	root                                            string
	server, agent                                   *httptest.Server
	ownership                                       *storelock.Lock
	shutdowns                                       atomic.Int32
	busy, mismatch, standalone, redirect, stayAlive bool
	release                                         bool
}

func newUpdateFixture(t *testing.T) *updateFixture {
	t.Helper()
	f := &updateFixture{root: t.TempDir(), release: true}
	var err error
	f.ownership, err = storelock.Acquire(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+stringKey {
			t.Error("missing installation authentication")
			w.WriteHeader(401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"protocol": 1, "companion": !f.standalone, "ready": true})
	}))
	f.agent = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-agent-token" {
			t.Error("missing agent authentication")
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/desktop/status" {
			if f.redirect {
				http.Redirect(w, r, f.server.URL+"/redirect", 302)
				return
			}
			server := f.server.URL
			if f.mismatch {
				server = "https://fleet.example.invalid"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"server": server})
			return
		}
		if r.URL.Path != "/desktop/shutdown" || r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		f.shutdowns.Add(1)
		if f.busy {
			w.WriteHeader(409)
			return
		}
		w.WriteHeader(204)
		if !f.stayAlive {
			go func() {
				f.agent.Close()
				f.server.Close()
				if f.release {
					_ = f.ownership.Close()
				}
			}()
		}
	}))
	endpoint, _ := url.Parse(f.server.URL)
	port, _ := strconv.Atoi(endpoint.Port())
	write := func(name string, value any) {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(f.root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("installation.json", Installation{Port: port, Key: stringKey})
	write("agent-connection.json", map[string]string{"url": f.agent.URL, "token": "fixture-agent-token"})
	t.Cleanup(func() { f.agent.Close(); f.server.Close(); _ = f.ownership.Close() })
	return f
}

const stringKey = "fixture-installation-key-00000000000000"

func TestPrepareUpdateWaitsForStoreReleaseAfterTransportClosure(t *testing.T) {
	f := newUpdateFixture(t)
	f.release = false
	result := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() { result <- PrepareUpdate(ctx, f.root) }()
	deadline := time.After(2 * time.Second)
	for f.shutdowns.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("shutdown not requested")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	select {
	case err := <-result:
		t.Fatalf("returned while store still owned: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := f.ownership.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if f.shutdowns.Load() != 1 {
		t.Fatal("shutdown repeated")
	}
}
func TestPrepareUpdateRefusesBusyRuntime(t *testing.T) {
	f := newUpdateFixture(t)
	f.busy = true
	if err := PrepareUpdate(context.Background(), f.root); !errors.Is(err, ErrUpdateBusy) {
		t.Fatalf("expected busy refusal, got %v", err)
	}
	if lock, err := storelock.Acquire(f.root); !errors.Is(err, storelock.ErrOwned) {
		if lock != nil {
			lock.Close()
		}
		t.Fatalf("live ownership lost: %v", err)
	}
}
func TestPrepareUpdateRejectsUnrelatedAndRedirectedServices(t *testing.T) {
	for _, kind := range []string{"standalone", "mismatch", "redirect", "remote"} {
		t.Run(kind, func(t *testing.T) {
			f := newUpdateFixture(t)
			switch kind {
			case "standalone":
				f.standalone = true
			case "mismatch":
				f.mismatch = true
			case "redirect":
				f.redirect = true
			case "remote":
				raw := []byte(`{"url":"https://example.invalid","token":"private"}`)
				if err := os.WriteFile(filepath.Join(f.root, "agent-connection.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := PrepareUpdate(context.Background(), f.root); err == nil {
				t.Fatal("unrelated service accepted")
			}
			if f.shutdowns.Load() != 0 {
				t.Fatal("sent shutdown to unverified service")
			}
		})
	}
}
func TestPrepareUpdateDoesNotAcceptAcknowledgementWithoutClosure(t *testing.T) {
	f := newUpdateFixture(t)
	f.stayAlive = true
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := PrepareUpdate(ctx, f.root); err == nil {
		t.Fatal("live service reported ready for replacement")
	}
}
func TestPrepareUpdateCanConfirmPreviouslyStoppedRuntime(t *testing.T) {
	f := newUpdateFixture(t)
	f.agent.Close()
	f.server.Close()
	f.ownership.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := PrepareUpdate(ctx, f.root); err != nil {
		t.Fatal(err)
	}
	if f.shutdowns.Load() != 0 {
		t.Fatal("offline confirmation requested shutdown")
	}
}

func TestPrepareUpdateRealCompanionStopsCleanlyAndRetainsData(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "server"), 0700); err != nil {
		t.Fatal(err)
	}
	database, err := db.NewDB(filepath.Join(root, "server", "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	user, err := database.UpsertUser("local|update", "update@example.invalid", "Update fixture")
	if err != nil {
		t.Fatal(err)
	}
	token, credential, err := database.CreateAPIKey(user, "desktop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Retained update project", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	reservation.Close()
	installation := Installation{Port: port, Key: stringKey, Token: token, DeviceID: credential.ID, UserID: user}
	if err := writeDurable(root, "installation.json", installation); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECTILE_LOCAL_RUNTIME_KEY", stringKey)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	observedExit := false
	go func() { done <- Run(ctx, root) }()
	t.Cleanup(func() {
		cancel()
		if observedExit {
			return
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("companion cleanup timed out")
		}
	})
	deadline := time.Now().Add(8 * time.Second)
	for {
		raw, readErr := os.ReadFile(filepath.Join(root, "agent-connection.json"))
		var discovery struct{ URL, Token string }
		if readErr == nil && json.Unmarshal(raw, &discovery) == nil {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, discovery.URL+"/desktop/status", nil)
			req.Header.Set("Authorization", "Bearer "+discovery.Token)
			response, getErr := http.DefaultClient.Do(req)
			if getErr == nil {
				var status struct{ Connected bool }
				decodeErr := json.NewDecoder(response.Body).Decode(&status)
				response.Body.Close()
				if response.StatusCode == 200 && decodeErr == nil && status.Connected {
					break
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("real companion did not become ready")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := PrepareUpdate(ctx, root); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		observedExit = true
		if err != nil {
			t.Fatalf("requested shutdown was treated as failure: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("companion did not exit")
	}
	reopened, err := db.NewDB(filepath.Join(root, "server", "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	retained, err := reopened.GetProjectByID(project.ID)
	if err != nil || retained == nil || retained.Name != project.Name {
		t.Fatalf("project not retained: %v", err)
	}
	actual, err := readInstallation(root)
	if err != nil || actual != installation {
		t.Fatalf("installation identity changed: %v", err)
	}
}
