package agent

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func profileTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAgentProfileStoreValidationAndDiscovery(t *testing.T) {
	root := profileTestRoot(t)
	list, err := listAgentProfiles(root)
	if err != nil || len(list.Profiles) != 0 || list.Profiles == nil || list.Warnings == nil {
		t.Fatalf("empty list: %+v %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".sectile")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("listing created a directory")
	}
	for _, id := range []string{"", "A", "a--b", "-a", "a-", "../escape", "con", "nul", "com1", "lpt9", strings.Repeat("a", 65)} {
		if err := createAgentProfile(root, id, "instructions"); err == nil {
			t.Fatalf("accepted id %q", id)
		}
	}
	for _, prompt := range []string{"", " \n", string([]byte{255}), "a\x00b", strings.Repeat("x", agentProfileLimit+1)} {
		if err := createAgentProfile(root, "invalid", prompt); err == nil {
			t.Fatal("accepted invalid prompt")
		}
	}
	for _, id := range []string{"z", "a"} {
		if err := createAgentProfile(root, id, "  '$(touch bad)'\r\n你好\n"); err != nil {
			t.Fatal(err)
		}
	}
	for name, contents := range map[string][]byte{"broken.md": {255}, "Bad.md": []byte("prompt"), "blank.md": []byte(" "), "ignored.txt": []byte("no")} {
		if err := os.WriteFile(filepath.Join(root, ".sectile", "agents", name), contents, 0644); err != nil {
			t.Fatal(err)
		}
	}
	list, err = listAgentProfiles(root)
	if err != nil || len(list.Profiles) != 2 || list.Profiles[0].ID != "a" || len(list.Warnings) != 3 {
		t.Fatalf("list: %+v %v", list, err)
	}
	snapshot, err := loadAgentProfile(root, "a")
	if err != nil || snapshot.Prompt != "  '$(touch bad)'\n你好\n" {
		t.Fatalf("snapshot: %+v %v", snapshot, err)
	}
	if err := createAgentProfile(root, "a", "replacement"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sectile", "agents", "a.md"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	fresh, err := loadAgentProfile(root, "a")
	if err != nil || fresh.Prompt != "changed" || snapshot.Prompt == fresh.Prompt {
		t.Fatal("snapshot was not isolated")
	}
	if err := os.Remove(filepath.Join(root, ".sectile", "agents", "a.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAgentProfile(root, "a"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stale profile: %v", err)
	}
	other := profileTestRoot(t)
	if _, err := loadAgentProfile(other, "z"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cross-project read: %v", err)
	}
}

func TestAgentProfileExclusiveConcurrentPublication(t *testing.T) {
	root := profileTestRoot(t)
	var group sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		group.Go(func() { results <- createAgentProfile(root, "shared", "complete instructions") })
	}
	group.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, fs.ErrExist) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successful writers: %d", success)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".sectile", "agents"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}

// A link above or at the mapped checkout is the user's own path, such as /tmp
// on macOS; only links inside the repository are refused.
func TestAgentProfileFollowsSymlinkedCheckoutPath(t *testing.T) {
	for _, part := range []string{"ancestor", "root"} {
		t.Run(part, func(t *testing.T) {
			base := profileTestRoot(t)
			real := filepath.Join(base, "real", "repo")
			if err := os.MkdirAll(real, 0755); err != nil {
				t.Fatal(err)
			}
			mapped := filepath.Join(base, "link", "repo")
			if part == "ancestor" {
				if err := os.Symlink(filepath.Join(base, "real"), filepath.Join(base, "link")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(base, "link"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(real, mapped); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if list, err := listAgentProfiles(mapped); err != nil || len(list.Profiles) != 0 {
				t.Fatalf("empty catalogue: %+v %v", list, err)
			}
			if err := createAgentProfile(mapped, "reviewer", "review code"); err != nil {
				t.Fatal(err)
			}
			if list, err := listAgentProfiles(mapped); err != nil || len(list.Profiles) != 1 {
				t.Fatalf("catalogue: %+v %v", list, err)
			}
			snapshot, err := loadAgentProfile(mapped, "reviewer")
			if err != nil || snapshot.Prompt != "review code" {
				t.Fatalf("launch read: %+v %v", snapshot, err)
			}
			if _, err := os.Stat(filepath.Join(real, ".sectile", "agents", "reviewer.md")); err != nil {
				t.Fatalf("profile not written in the resolved checkout: %v", err)
			}
		})
	}
}

func TestAgentProfileRejectsSymlinkDirectoriesAndFiles(t *testing.T) {
	for _, part := range []string{".sectile", "agents", "profile"} {
		t.Run(part, func(t *testing.T) {
			root := profileTestRoot(t)
			outside := profileTestRoot(t)
			if err := createAgentProfile(root, "safe", "original"); err != nil {
				t.Fatal(err)
			}
			var target string
			switch part {
			case ".sectile":
				target = filepath.Join(root, ".sectile")
			case "agents":
				target = filepath.Join(root, ".sectile", "agents")
			case "profile":
				target = filepath.Join(root, ".sectile", "agents", "safe.md")
			}
			moved := filepath.Join(outside, "moved")
			if err := os.Rename(target, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, target); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if _, err := loadAgentProfile(root, "safe"); err == nil {
				t.Fatal("followed a symbolic link")
			}
			if part != "profile" {
				if err := createAgentProfile(root, "escape", "unexpected"); err == nil {
					t.Fatal("created through a symbolic link")
				}
			}
		})
	}
}

func TestAgentProfileFailedPublicationCleansTemporaryFile(t *testing.T) {
	root := profileTestRoot(t)
	if err := createAgentProfile(root, "good", "original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".sectile", "agents", "occupied.md"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := createAgentProfile(root, "occupied", "content"); err == nil {
		t.Fatal("replaced existing directory")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".sectile", "agents"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".profile-") {
			t.Fatal("temporary file leaked")
		}
	}
}

func TestProfileSnapshotIsNotDesktopPayload(t *testing.T) {
	run := &controlledRun{}
	applyProfile(run, &profileSnapshot{ID: "reviewer", Prompt: "PRIVATE PROMPT", DeveloperInstructions: "PRIVATE CONFIG"})
	raw, err := json.Marshal(run.desktop)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "PRIVATE") || !strings.Contains(string(raw), `"agentProfileId":"reviewer"`) {
		t.Fatalf("desktop payload: %s", raw)
	}
}

func TestAgentProfileExternalWriterIsNeverOverwritten(t *testing.T) {
	root := profileTestRoot(t)
	if err := createAgentProfile(root, "seed", "seed"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".sectile", "agents", "race.md")
	start := make(chan struct{})
	external := make(chan error, 1)
	go func() {
		<-start
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			_, err = file.WriteString("external")
			_ = file.Close()
		}
		external <- err
	}()
	close(start)
	agentErr := createAgentProfile(root, "race", "agent")
	externalErr := <-external
	if (agentErr == nil) == (externalErr == nil) {
		t.Fatalf("expected exactly one writer: agent=%v external=%v", agentErr, externalErr)
	}
	snapshot, err := loadAgentProfile(root, "race")
	if err != nil {
		t.Fatal(err)
	}
	expected := "agent"
	if externalErr == nil {
		expected = "external"
	}
	if snapshot.Prompt != expected {
		t.Fatal("winning writer was overwritten")
	}
}

func TestProfileIdentitySurvivesReadOnlyHistoryWithoutPrompt(t *testing.T) {
	store := openRunStore(t.TempDir())
	run := &controlledRun{desktop: desktopRun{ID: "profile-history", Status: "completed"}}
	applyProfile(run, &profileSnapshot{ID: "reviewer", Prompt: "PRIVATE SNAPSHOT"})
	if err := store.save(storedRun{Run: run.desktop}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.path(run.desktop.ID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "PRIVATE SNAPSHOT") {
		t.Fatal("prompt persisted in read-only history")
	}
	var record storedRun
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record.Run.AgentProfileID != "reviewer" || record.Run.AgentProfileName != "reviewer" {
		t.Fatal("profile identity was lost")
	}
}

func TestAgentProfileSizeBoundaryAndReadOnlyCheckout(t *testing.T) {
	root := profileTestRoot(t)
	if err := createAgentProfile(root, "maximum", strings.Repeat("x", agentProfileLimit)); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := loadAgentProfile(root, "maximum"); err != nil || len(snapshot.Prompt) != agentProfileLimit {
		t.Fatalf("size boundary: %v", err)
	}
	// Chmod is not an access boundary for root and does not model Windows ACLs.
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("readonly mode bits do not restrict this user/platform")
	}
	path := filepath.Join(root, ".sectile", "agents")
	if err := os.Chmod(path, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0755)
	if err := createAgentProfile(root, "readonly", "not saved"); err == nil {
		t.Fatal("reported success in a read-only directory")
	}
	if _, err := os.Stat(filepath.Join(path, "readonly.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("partial profile remained")
	}
}

func TestAgentProfileErrorStatusNamesTheCause(t *testing.T) {
	root := profileTestRoot(t)
	if err := createAgentProfile(root, "reviewer", "review code"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".sectile", "agents", "reviewer.md")
	if err := os.WriteFile(path, []byte("nul\x00byte"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".sectile", "agents", "folder.md"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		id, message string
		status      int
	}{
		{"reviewer", "invalid or unreadable", 422},
		{"folder", "invalid or unreadable", 422},
		{"missing", "no longer available", 404},
	} {
		_, err := loadAgentProfile(root, check.id)
		rec := httptest.NewRecorder()
		profileHTTPError(rec, err)
		if rec.Code != check.status || !strings.Contains(rec.Body.String(), check.message) {
			t.Fatalf("%s: %d %s", check.id, rec.Code, rec.Body.String())
		}
	}
}
