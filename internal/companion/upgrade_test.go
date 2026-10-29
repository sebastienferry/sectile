package companion

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tasks/internal/db"
)

func TestUpgradeRecoversEveryDurableBoundaryAndPreservesIdentity(t *testing.T) {
	for _, stage := range []string{"prepared", "source_stopped", "backup_verified", "validated", "selection_committed", "committed"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "server"), 0700); err != nil {
				t.Fatal(err)
			}
			database, err := db.NewDB(filepath.Join(root, "server", "tasks.db"))
			if err != nil {
				t.Fatal(err)
			}
			user, err := database.UpsertUser("local|migration", "migration@example.com", "Migration")
			if err != nil {
				t.Fatal(err)
			}
			token, credential, err := database.CreateAPIKey(user, "desktop", db.DefaultAPIKeyTTL)
			if err != nil {
				t.Fatal(err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}
			if err = writeDurable(root, "installation.json", Installation{Port: 59999, Key: "01234567890123456789012345678901", Token: token, DeviceID: credential.ID, UserID: user}); err != nil {
				t.Fatal(err)
			}
			originalDatabase, err := os.ReadFile(filepath.Join(root, "server", "tasks.db"))
			if err != nil {
				t.Fatal(err)
			}
			originalInstallation, err := os.ReadFile(filepath.Join(root, "installation.json"))
			if err != nil {
				t.Fatal(err)
			}
			originalKey, err := os.ReadFile(filepath.Join(root, "server", "secret.key"))
			if err != nil {
				t.Fatal(err)
			}
			simulated := errors.New("simulated migration interruption")
			err = upgrade(context.Background(), root, "", "", func(at string) error {
				if at == stage {
					return simulated
				}
				return nil
			})
			if !errors.Is(err, simulated) {
				t.Fatalf("checkpoint %s returned %v", stage, err)
			}
			selectionPath := filepath.Join(root, "runtime-selection.json")
			if stage == "prepared" || stage == "source_stopped" || stage == "backup_verified" || stage == "validated" {
				if _, statErr := os.Stat(selectionPath); !os.IsNotExist(statErr) {
					t.Fatalf("pre-commit checkpoint %s unexpectedly committed runtime selection: %v", stage, statErr)
				}
				sourceDatabase, readErr := os.ReadFile(filepath.Join(root, "server", "tasks.db"))
				if readErr != nil || !bytes.Equal(sourceDatabase, originalDatabase) {
					t.Fatalf("pre-commit source database changed at %s: %v", stage, readErr)
				}
				sourceInstallation, readErr := os.ReadFile(filepath.Join(root, "installation.json"))
				if readErr != nil || !bytes.Equal(sourceInstallation, originalInstallation) {
					t.Fatalf("pre-commit source installation changed at %s: %v", stage, readErr)
				}
			} else {
				if _, statErr := os.Stat(selectionPath); statErr != nil {
					t.Fatalf("post-commit checkpoint %s lost runtime selection: %v", stage, statErr)
				}
				selected, readErr := readInstallation(filepath.Join(root, generation))
				if readErr != nil || selected.Token != token || selected.UserID != user {
					t.Fatalf("post-commit candidate unavailable at %s: %v", stage, readErr)
				}
			}
			if err = Upgrade(context.Background(), root, "", ""); err != nil {
				t.Fatal(err)
			}
			candidate := filepath.Join(root, generation)
			restored, err := readInstallation(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if restored.Token != token || restored.UserID != user || restored.DeviceID != credential.ID {
				t.Fatal("installation identity changed")
			}
			key, err := os.ReadFile(filepath.Join(candidate, "server", "secret.key"))
			if err != nil || string(key) != string(originalKey) {
				t.Fatal("encryption key changed")
			}
			reopened, err := db.NewDB(filepath.Join(candidate, "server", "tasks.db"))
			if err != nil {
				t.Fatal(err)
			}
			found, err := reopened.LookupDeviceToken(token)
			if err != nil || found.UserID != user || found.ID != credential.ID {
				t.Fatal("credential no longer resolves")
			}
			if err = reopened.Close(); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(filepath.Join(root, "server", "tasks.db"))
			if err != nil || !info.IsDir() {
				t.Fatal("legacy database reopen is not fenced")
			}
			if err = VerifyMigrationBackup(filepath.Join(root, "migration-backup")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
