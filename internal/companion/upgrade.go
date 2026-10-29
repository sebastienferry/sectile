package companion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"tasks/internal/db"
	"tasks/internal/storelock"
	"tasks/internal/version"
)

const generation = "generations/companion-v1"

type Selection struct {
	Protocol   int    `json:"protocol"`
	Kind       string `json:"kind"`
	Generation string `json:"generation"`
	Version    string `json:"version"`
}

// Upgrade is called under the desktop startup lock. It stops only authenticated
// managed source services, validates a separate candidate and commits selection.
// A committed selection always recovers forward; backups are never auto-restored.
func Upgrade(ctx context.Context, root, serverBinary, agentBinary string) error {
	return upgrade(ctx, root, serverBinary, agentBinary, nil)
}

func upgrade(ctx context.Context, root, serverBinary, agentBinary string, checkpoint func(string) error) error {
	state := func(stage string) error {
		if err := writeDurable(root, "migration-state.json", map[string]any{"protocol": 1, "stage": stage, "generation": generation, "version": version.String()}); err != nil {
			return err
		}
		if checkpoint != nil {
			return checkpoint(stage)
		}
		return nil
	}
	if !filepath.IsAbs(root) {
		return errors.New("upgrade root must be absolute")
	}
	if err := realDirectory(root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "generations"), 0700); err != nil {
		return err
	}
	if err := realDirectory(filepath.Join(root, "generations")); err != nil {
		return err
	}
	coordinator, err := storelock.Acquire(filepath.Join(root, ".migration-owner"))
	if err != nil {
		return err
	}
	defer coordinator.Close()
	selectionPath := filepath.Join(root, "runtime-selection.json")
	if raw, err := os.ReadFile(selectionPath); err == nil {
		var selected Selection
		if json.Unmarshal(raw, &selected) != nil || selected.Protocol != 1 || selected.Kind != "companion" || selected.Generation != generation {
			return errors.New("unsupported runtime selection")
		}
		if _, err = readInstallation(filepath.Join(root, generation)); err != nil {
			return err
		}
		if err = preventLegacyStart(root); err != nil {
			return err
		}
		return state("committed")
	} else if !os.IsNotExist(err) {
		return err
	}
	installation, err := readInstallation(root)
	if err != nil {
		return err
	}
	if err = state("prepared"); err != nil {
		return err
	}
	if err = stopSource(ctx, root, &installation, serverBinary, agentBinary); err != nil {
		return err
	}
	if err = state("source_stopped"); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(root, "migration-backup")); os.IsNotExist(err) {
		if err = os.RemoveAll(filepath.Join(root, "migration-backup.next")); err != nil {
			return err
		}
	}
	if err = archiveBinaries(root, serverBinary, agentBinary); err != nil {
		return err
	}
	if err = PrepareMigration(root); err != nil {
		return err
	}
	backup := filepath.Join(root, "migration-backup")
	if err = VerifyMigrationBackup(backup); err != nil {
		return err
	}
	// Never silently reuse a snapshot if an old runtime changed its source.
	if err = compareBackupSource(backup, root); err != nil {
		return err
	}
	candidate := filepath.Join(root, generation)
	// Candidate effects have not been enabled, so an interrupted validation is
	// safely recreated from the immutable verified backup.
	if err = os.RemoveAll(candidate); err != nil {
		return err
	}
	if err = copyGeneration(backup, candidate); err != nil {
		return err
	}
	if installation.AgentDeviceID == "" {
		if raw, err := os.ReadFile(filepath.Join(root, "migration-source-identity.json")); err == nil {
			var identity map[string]string
			if err = json.Unmarshal(raw, &identity); err != nil {
				return err
			}
			if identity["userId"] != installation.UserID {
				return errors.New("migration source identity changed")
			}
			installation.AgentDeviceID = identity["agentDeviceId"]
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if installation.AgentDeviceID != "" {
		raw, err := os.ReadFile(filepath.Join(candidate, "installation.json"))
		if err != nil {
			return err
		}
		var manifest map[string]any
		if err = json.Unmarshal(raw, &manifest); err != nil {
			return err
		}
		manifest["agent_device_id"] = installation.AgentDeviceID
		if err = writeDurable(candidate, "installation.json", manifest); err != nil {
			return err
		}
	}
	if err = state("backup_verified"); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(candidate, "server", "tasks.db")); err == nil {
		if _, err = os.Stat(filepath.Join(candidate, "server", "secret.key")); err != nil {
			return errors.New("existing encrypted store requires its original secret.key before migration")
		}
		database, err := db.Open(db.SQLiteConfig(filepath.Join(candidate, "server", "tasks.db")))
		if err != nil {
			return err
		}
		if err = database.Close(); err != nil {
			return err
		}
		check, err := sql.Open("sqlite", filepath.Join(candidate, "server", "tasks.db"))
		if err != nil {
			return err
		}
		var integrity string
		err = check.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity)
		closeErr := check.Close()
		if err = errors.Join(err, closeErr); err != nil {
			return err
		}
		if integrity != "ok" {
			return errors.New("candidate SQLite integrity check failed")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = syncGeneration(candidate); err != nil {
		return err
	}
	if err = state("validated"); err != nil {
		return err
	}
	if err = writeDurable(root, "runtime-selection.json", Selection{1, "companion", generation, version.String()}); err != nil {
		return err
	}
	if checkpoint != nil {
		if err = checkpoint("selection_committed"); err != nil {
			return err
		}
	}
	if err = preventLegacyStart(root); err != nil {
		return err
	}
	return state("committed")
}

func writeDurable(root, name string, value any) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".runtime-next-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(raw)
	err = errors.Join(writeErr, file.Sync(), file.Close())
	if err != nil {
		return err
	}
	if err = os.Rename(file.Name(), filepath.Join(root, name)); err != nil {
		return err
	}
	return syncDirectory(root)
}

func copyGeneration(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "backup.json" || relative == "agent-connection.json" {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("candidate refuses special files")
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		return errors.Join(copyErr, out.Sync(), out.Close())
	})
}

func compareBackupSource(backup, source string) error {
	raw, err := os.ReadFile(filepath.Join(backup, "backup.json"))
	if err != nil {
		return err
	}
	var report MigrationBackup
	if err = json.Unmarshal(raw, &report); err != nil {
		return err
	}
	for relative, want := range report.Files {
		file, err := os.Open(filepath.Join(source, relative))
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		if err = errors.Join(readErr, file.Close()); err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != want {
			return fmt.Errorf("source changed since backup: %s", relative)
		}
	}

	return nil
}

func preventLegacyStart(root string) error {
	source := filepath.Join(root, "server")
	archive := filepath.Join(root, "legacy-server")
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		if _, err = os.Stat(source); err == nil {
			if err = os.Rename(source, archive); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// An old launcher cannot open a directory as a SQLite database.
	if err := os.MkdirAll(filepath.Join(source, "tasks.db"), 0700); err != nil {
		return err
	}
	if err := writeDurable(root, "installation.json", map[string]any{"runtime": "companion", "requiresProtocol": 2}); err != nil {
		return err
	}
	return syncDirectory(root)
}

func stopSource(ctx context.Context, root string, installation *Installation, serverBinary, agentBinary string) error {
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(endpoint, token string, target any) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return errors.New("source authentication refused")
		}
		return json.NewDecoder(response.Body).Decode(target)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", installation.Port)
	serverAddress := fmt.Sprintf("127.0.0.1:%d", installation.Port)
	var serverPID int
	var activeActivities []struct {
		Status   string `json:"status"`
		ID       string `json:"id"`
		SkillID  string `json:"skillId"`
		UserID   string `json:"userId"`
		DeviceID string `json:"deviceId"`
	}
	idleRuns := map[string]bool{}
	if conn, err := net.DialTimeout("tcp", serverAddress, 300*time.Millisecond); err == nil {
		conn.Close()
		var proof struct {
			Protocol  int  `json:"protocol"`
			Companion bool `json:"companion"`
		}
		if err = get(base+"/api/local-runtime", installation.Key, &proof); err != nil {
			return err
		}
		if proof.Protocol != 1 || proof.Companion {
			return errors.New("unexpected source runtime")
		}
		serverPID, err = managedListener(installation.Port, serverBinary)
		if err != nil {
			return err
		}

		if installation.Token != "" {
			var agentStatus struct {
				Agents []struct {
					UserID   string `json:"userId"`
					DeviceID string `json:"deviceId"`
				} `json:"agents"`
			}
			if err = get(base+"/api/agent/status", installation.Token, &agentStatus); err != nil {
				return err
			}
			if len(agentStatus.Agents) > 1 {
				return errors.New("multiple source agents must be disconnected before migration")
			}
			for _, a := range agentStatus.Agents {
				if a.UserID != installation.UserID {
					return errors.New("source agent belongs to another user")
				}
				installation.AgentDeviceID = a.DeviceID
				if err = writeDurable(root, "migration-source-identity.json", map[string]string{"userId": installation.UserID, "agentDeviceId": a.DeviceID}); err != nil {
					return err
				}
			}
		}

		if installation.Token != "" {
			if err = get(base+"/api/activities?limit=1000000", installation.Token, &activeActivities); err != nil {
				return err
			}
			// Run inspection below identifies resumable idle free conversations.

		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "agent-connection.json"))
	if err == nil {
		var discovery struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err = json.Unmarshal(raw, &discovery); err != nil {
			return err
		}
		endpoint, err := url.Parse(discovery.URL)
		if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.User != nil || endpoint.Fragment != "" {
			return errors.New("invalid source agent discovery")
		}
		if conn, err := net.DialTimeout("tcp", endpoint.Host, 300*time.Millisecond); err == nil {
			conn.Close()
			var status struct {
				Server string `json:"server"`
			}
			if err = get(discovery.URL+"/desktop/status", discovery.Token, &status); err != nil {
				return err
			}
			if status.Server != base {
				return errors.New("agent belongs to another backend")
			}
			var runs []struct {
				ID           string `json:"id"`
				Status       string `json:"status"`
				Conversation bool   `json:"conversation"`
				TaskID       string `json:"taskId"`
			}
			if err = get(discovery.URL+"/desktop/runs", discovery.Token, &runs); err != nil {
				return err
			}
			for _, run := range runs {
				switch run.Status {
				case "completed", "failed", "canceled", "stopped", "exited":
					continue
				}
				if !run.Conversation || run.TaskID != "" {
					return errors.New("stop active executions before migration")
				}
				var conversation struct {
					Busy      bool              `json:"busy"`
					Approvals []json.RawMessage `json:"approvals"`
				}
				if err = get(discovery.URL+"/desktop/conversation?id="+url.QueryEscape(run.ID), discovery.Token, &conversation); err != nil {
					return err
				}
				if conversation.Busy || len(conversation.Approvals) > 0 {
					return errors.New("finish active conversation turns before migration")
				}
				idleRuns[run.ID] = true
			}
			for _, a := range activeActivities {
				if (a.Status == "running" || a.Status == "pending" || a.Status == "queued" || a.Status == "waiting") && !idleRuns[a.ID] && !(a.SkillID == "remote_run" && a.UserID == "" && a.DeviceID == "") {
					return errors.New("backend work is active; finish it before migration")
				}
			}
			var port int
			fmt.Sscanf(endpoint.Port(), "%d", &port)
			pid, err := managedListener(port, agentBinary)
			if err != nil {
				return err
			}
			if err = stopManaged(ctx, pid, agentBinary); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, a := range activeActivities {
		if (a.Status == "running" || a.Status == "pending" || a.Status == "queued" || a.Status == "waiting") && !idleRuns[a.ID] && !(a.SkillID == "remote_run" && a.UserID == "" && a.DeviceID == "") {
			return errors.New("backend work is active; finish it before migration")
		}
	}
	if serverPID != 0 {
		return stopManaged(ctx, serverPID, serverBinary)
	}
	return nil
}

// Retain the compatible legacy binaries outside the candidate generation.
func archiveBinaries(root, server, agent string) error {
	directory := filepath.Join(root, "migration-binaries")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	for name, source := range map[string]string{"sectile-server": server, "sectile-agent": agent} {
		if source == "" {
			continue
		}
		destination := filepath.Join(directory, name)
		if _, err := os.Stat(destination); err == nil {
			continue
		}
		in, err := os.Open(source)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		err = errors.Join(copyErr, out.Sync(), out.Close(), in.Close())
		if err != nil {
			return err
		}
	}
	return syncDirectory(directory)
}

func syncGeneration(root string) error {
	var directories []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, path)
		}
		return nil
	}); err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := syncDirectory(directories[i]); err != nil {
			return err
		}
	}
	return syncDirectory(filepath.Dir(root))
}

func realDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("migration requires real private directories")
	}
	return nil
}
