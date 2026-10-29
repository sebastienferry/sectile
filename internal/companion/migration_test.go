package companion

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationBackupRequiresStoppedServicesAndDetectsCorruption(t *testing.T) {
	root := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	manifest := Installation{Port: listener.Addr().(*net.TCPAddr).Port, Key: "01234567890123456789012345678901"}
	raw, _ := json.Marshal(manifest)
	if err = os.WriteFile(filepath.Join(root, "installation.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = PrepareMigration(root); err == nil {
		t.Fatal("migration accepted a live source")
	}
	listener.Close()
	if err = os.Mkdir(filepath.Join(root, "server"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "server", "secret.key")
	if err = os.WriteFile(source, []byte("fixture-private-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = PrepareMigration(root); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "migration-backup")
	if err = VerifyMigrationBackup(backup); err != nil {
		t.Fatal(err)
	}
	if err = PrepareMigration(root); err != nil {
		t.Fatal("verified retry failed:", err)
	}
	if err = os.WriteFile(filepath.Join(backup, "server", "secret.key"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = VerifyMigrationBackup(backup); err == nil {
		t.Fatal("corruption was accepted")
	}
	preserved, err := os.ReadFile(source)
	if err != nil || string(preserved) != "fixture-private-key" {
		t.Fatal("source changed")
	}
}
