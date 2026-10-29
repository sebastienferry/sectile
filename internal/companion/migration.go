package companion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"tasks/internal/storelock"
)

type MigrationBackup struct {
	Protocol int               `json:"protocol"`
	Stage    string            `json:"stage"`
	Files    map[string]string `json:"files"`
}

// PrepareMigration snapshots one stopped installation. It never changes the
// source manifest or opens SQLite. The native owner must hold runtime.lock.
func PrepareMigration(root string) error {
	if !filepath.IsAbs(root) {
		return errors.New("migration root must be absolute")
	}
	installation, err := readInstallation(root)
	if err != nil {
		return err
	}
	if err := requireClosedPort(fmt.Sprintf("127.0.0.1:%d", installation.Port)); err != nil {
		return err
	}
	// Discovery is untrusted input: accept only the loopback URL shape the agent writes.
	raw, err := os.ReadFile(filepath.Join(root, "agent-connection.json"))
	if err == nil {
		var discovery struct {
			URL string `json:"url"`
		}
		if err = json.Unmarshal(raw, &discovery); err != nil {
			return err
		}
		var port int
		if n, _ := fmt.Sscanf(discovery.URL, "http://127.0.0.1:%d", &port); n != 1 || port < 1 || port > 65535 {
			return errors.New("invalid agent discovery URL")
		}
		if err = requireClosedPort(fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	lock, err := storelock.Acquire(root)
	if err != nil {
		return err
	}
	defer lock.Close()
	destination := filepath.Join(root, "migration-backup")
	if _, err = os.Lstat(destination); err == nil {
		return VerifyMigrationBackup(destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	staging := filepath.Join(root, "migration-backup.next")
	if _, err = os.Lstat(staging); err == nil {
		return errors.New("incomplete migration backup requires inspection")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.Mkdir(staging, 0700); err != nil {
		return err
	}
	report := MigrationBackup{Protocol: 1, Stage: "backup_verified", Files: map[string]string{}}
	for _, name := range []string{"installation.json", "agent-connection.json", "server", "agent", "runs"} {
		source := filepath.Join(root, name)
		if _, err = os.Lstat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			target := filepath.Join(staging, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0700)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("migration refuses non-regular file %s", relative)
			}
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			defer input.Close()
			output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			hash := sha256.New()
			_, copyErr := io.Copy(io.MultiWriter(output, hash), input)
			syncErr := output.Sync()
			closeErr := output.Close()
			if err = errors.Join(copyErr, syncErr, closeErr); err != nil {
				return err
			}
			report.Files[relative] = hex.EncodeToString(hash.Sum(nil))
			return nil
		})
		if err != nil {
			return err
		}
	}
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	journal, err := os.OpenFile(filepath.Join(staging, "backup.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := journal.Write(bytes)
	err = errors.Join(writeErr, journal.Sync(), journal.Close())
	if err != nil {
		return err
	}
	if err = VerifyMigrationBackup(staging); err != nil {
		return err
	}
	return os.Rename(staging, destination)
}

func requireClosedPort(address string) error {
	conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
	if err == nil {
		conn.Close()
		return errors.New("stop the existing Local services before preparing migration")
	}
	return nil
}

func VerifyMigrationBackup(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "backup.json"))
	if err != nil {
		return err
	}
	var report MigrationBackup
	if err = json.Unmarshal(raw, &report); err != nil {
		return err
	}
	if report.Protocol != 1 || report.Stage != "backup_verified" || len(report.Files) == 0 {
		return errors.New("invalid migration backup manifest")
	}
	for relative, want := range report.Files {
		if !filepath.IsLocal(relative) {
			return errors.New("invalid backup file path")
		}
		path := filepath.Join(root, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("backup file is not regular")
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if err = errors.Join(readErr, closeErr); err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != want {
			return fmt.Errorf("backup integrity check failed for %s", relative)
		}
	}
	return nil
}
