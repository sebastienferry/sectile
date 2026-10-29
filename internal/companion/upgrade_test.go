package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agentexec"
	"tasks/internal/db"
	"tasks/internal/models"
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
			if err := database.SetUserTrackerCredential(user, "gitlab", "https://gitlab.example.test", "migration@example.com", "fixture-personal-token", ""); err != nil {
				t.Fatal(err)
			}
			if err := database.SaveJiraOAuthApp("fixture-client", "fixture-client-secret", "https://sectile.example.test/auth/jira/callback", user); err != nil {
				t.Fatal(err)
			}
			project, err := database.CreateProject(models.CreateProjectRequest{Name: "Migration workspace", IssueTracker: "local"})
			if err != nil {
				t.Fatal(err)
			}
			task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Retained ticket", Source: "local"})
			if err != nil {
				t.Fatal(err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}
			if err = writeDurable(root, "installation.json", Installation{Port: 59999, Key: "01234567890123456789012345678901", Token: token, DeviceID: credential.ID, UserID: user}); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "runs"), 0700); err != nil {
				t.Fatal(err)
			}
			conversationSnapshot := []byte(`{"version":1,"run":{"conversation":true,"provider":"codex","model":"fixture-model","id":"conversation-fixture","projectId":"` + project.ID + `","directory":"/tmp/fixture","status":"completed"},"conversation":{"session":"fixture-session","mode":"plan"}}`)
			conversationJournal := []byte("{\"seq\":1,\"kind\":\"line\",\"text\":\"saved history\"}\n")
			if !json.Valid(conversationSnapshot) || !json.Valid(bytes.TrimSpace(conversationJournal)) {
				t.Fatal("run-store fixture is invalid JSON")
			}
			for name, data := range map[string][]byte{"conversation-fixture.json": conversationSnapshot, "conversation-fixture.json.journal": conversationJournal} {
				if err := os.WriteFile(filepath.Join(root, "runs", name), data, 0600); err != nil {
					t.Fatal(err)
				}
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
			if binary := os.Getenv("SECTILE_TEST_PACKAGED_COMPANION"); binary != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				command := agentexec.Hidden(exec.CommandContext(ctx, binary, "--upgrade", "--data-dir", root))
				if output, commandErr := command.CombinedOutput(); commandErr != nil {
					t.Fatalf("packaged upgrade failed: %v\n%s", commandErr, output)
				}
				t.Log("Recovery completed using the packaged companion")
			} else if err = Upgrade(context.Background(), root, "", ""); err != nil {
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
			if site, email, personalToken, err := reopened.UserTrackerCredentialsFor(user, "gitlab"); err != nil || site != "https://gitlab.example.test" || email != "migration@example.com" || personalToken != "fixture-personal-token" {
				t.Fatal("personal tracker credential was not retained")
			}
			if app, _, err := reopened.JiraOAuthApp(); err != nil || app.ClientID != "fixture-client" || app.ClientSecret != "fixture-client-secret" {
				t.Fatal("encrypted OAuth app was not retained")
			}
			if restoredProject, err := reopened.GetProjectByID(project.ID); err != nil || restoredProject == nil || restoredProject.Name != "Migration workspace" {
				t.Fatal("project was not retained")
			}
			if restoredTask, err := reopened.GetTaskByID(task.ID); err != nil || restoredTask == nil || restoredTask.Title != "Retained ticket" {
				t.Fatal("ticket was not retained")
			}
			for name, expected := range map[string][]byte{"conversation-fixture.json": conversationSnapshot, "conversation-fixture.json.journal": conversationJournal} {
				stored, err := os.ReadFile(filepath.Join(candidate, "runs", name))
				if err != nil || !bytes.Equal(stored, expected) {
					t.Fatalf("run-store file %s was not retained: %v", name, err)
				}
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
