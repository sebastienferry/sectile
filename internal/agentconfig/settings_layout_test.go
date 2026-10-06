package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// layoutsOf reads the layout stamps of the settings file as written.
func layoutsOf(t *testing.T, path string) fileLayout {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stamp fileLayout
	if err := json.Unmarshal(raw, &stamp); err != nil {
		t.Fatal(err)
	}
	return stamp
}

func noop(*Settings) error { return nil }

// Every write records the highest layout the file reached, in a key an older
// agent keeps since it owns only its own keys (#744).
func TestWriteSettingsRecordsTheHighestLayout(t *testing.T) {
	path := writeSettingsFile(t, `{"layout":2,"server":"https://s"}`)
	if _, err := UpdateSettings(t.TempDir(), noop); err != nil {
		t.Fatal(err)
	}
	if stamp := layoutsOf(t, path); stamp.Layout != SettingsLayout || stamp.MaxLayout != SettingsLayout {
		t.Fatalf("stamps after a save: %+v", stamp)
	}
	if slices.Contains(ownedKeys, maxLayoutKey) || slices.Contains(legacyKeys, maxLayoutKey) {
		t.Fatalf("%s must stay outside the keys an agent replaces", maxLayoutKey)
	}
}

// A running agent never moves project Sandbox values: only the start-up
// migration folds them (#744).
func TestAnOrdinaryReadOrSaveDoesNotFold(t *testing.T) {
	path := writeSettingsFile(t, `{"layout":3,"projectSettings":{"a":{"path":"/a","claudeSandbox":{"allow":["Read"]}}}}`)
	s, err := ReadSettings(t.TempDir())
	if err != nil || s.Defaults.ClaudeSandbox != nil || s.Project("a").ClaudeSandbox == nil {
		t.Fatalf("a read moved the values: %+v %v", s, err)
	}
	if _, err := UpdateSettings(t.TempDir(), noop); err != nil {
		t.Fatal(err)
	}
	s, _ = ReadSettings(t.TempDir())
	if s.Defaults.ClaudeSandbox != nil || !reflect.DeepEqual(s.Project("a").ClaudeSandbox.Allow, []string{"Read"}) {
		t.Fatalf("a save moved the values: %+v", s)
	}
	if stamp := layoutsOf(t, path); stamp.Layout != SettingsLayout {
		t.Fatalf("layout after the save: %+v", stamp)
	}
}

// olderAgentRewrite is what an agent that predates #730 leaves after a save
// over a folded file: layout 3, no workstation Sandbox values, the maxLayout
// key kept, and a rule "Always allow" added to a project.
const olderAgentRewrite = `{"layout":3,"maxLayout":4,"server":"https://s",` +
	`"defaults":{"terminal":"ghostty"},` +
	`"projectSettings":{"a":{"path":"/a","claudeSandbox":{"allow":["WebFetch(domain:pypi.org)"]}}}}`

// The next save of a running agent keeps the file an older agent left as a
// backup and logs it, once; the project rule stays on its project.
func TestASaveAfterAnOlderAgentKeepsATrace(t *testing.T) {
	path := writeSettingsFile(t, olderAgentRewrite)
	// The backup of the original fold already holds the plain name.
	if err := os.WriteFile(path+".bak-layout3", []byte(`{"layout":3}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateSettings(t.TempDir(), noop); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(path + ".bak-layout3-*")
	if len(backups) != 1 {
		t.Fatalf("one timestamped backup expected: %v", backups)
	}
	if raw, _ := os.ReadFile(backups[0]); string(raw) != olderAgentRewrite {
		t.Fatalf("the backup is not the file the older agent left:\n%s", raw)
	}
	if raw, _ := os.ReadFile(path + ".bak-layout3"); string(raw) != `{"layout":3}` {
		t.Fatal("the earlier backup was overwritten")
	}
	s, _ := ReadSettings(t.TempDir())
	if s.Defaults.ClaudeSandbox != nil || !reflect.DeepEqual(s.Project("a").ClaudeSandbox.Allow, []string{"WebFetch(domain:pypi.org)"}) {
		t.Fatalf("after the save: defaults %+v, project %+v", s.Defaults.ClaudeSandbox, s.Project("a").ClaudeSandbox)
	}
	if _, err := UpdateSettings(t.TempDir(), noop); err != nil {
		t.Fatal(err)
	}
	if again, _ := filepath.Glob(path + ".bak-layout3-*"); len(again) != 1 {
		t.Fatalf("a second save must leave no new backup: %v", again)
	}
}

// At start, a file an older agent rewrote is reported, backed up and
// rewritten, and its project values are not folded again.
func TestMigrationReportsADowngradeWithoutFolding(t *testing.T) {
	path := writeSettingsFile(t, olderAgentRewrite)
	migrated, report, err := MigrateSettingsReport(t.TempDir())
	if !migrated || err != nil || report.Downgraded != 4 || report.SandboxFolded {
		t.Fatalf("migration: %v %+v %v", migrated, report, err)
	}
	if raw, _ := os.ReadFile(path + ".bak-layout3"); string(raw) != olderAgentRewrite {
		t.Fatalf("backup:\n%s", raw)
	}
	if extra, _ := filepath.Glob(path + ".bak-layout3-*"); len(extra) != 0 {
		t.Fatalf("the migration's own write must not back up twice: %v", extra)
	}
	s, _ := ReadSettings(t.TempDir())
	if s.Defaults.ClaudeSandbox != nil || s.Project("a").ClaudeSandbox == nil {
		t.Fatalf("the project rule moved: %+v", s)
	}
	if stamp := layoutsOf(t, path); stamp.Layout != SettingsLayout || stamp.MaxLayout != SettingsLayout {
		t.Fatalf("stamps: %+v", stamp)
	}
}

// An agent older than the file refuses to save over it and leaves it as it
// is, at start and on every save (#744).
func TestNewerSettingsAreNeverOverwritten(t *testing.T) {
	for _, raw := range []string{
		`{"layout":5,"defaults":{"futureField":true}}`,
		`{"layout":3,"maxLayout":5,"defaults":{"terminal":"ghostty"}}`,
	} {
		path := writeSettingsFile(t, raw)
		if err := WriteSettings(Settings{}); !errors.Is(err, ErrSettingsNewer) {
			t.Errorf("%s: WriteSettings = %v", raw, err)
		}
		if _, err := UpdateSettings(t.TempDir(), noop); !errors.Is(err, ErrSettingsNewer) {
			t.Errorf("%s: UpdateSettings = %v", raw, err)
		}
		migrated, report, err := MigrateSettingsReport(t.TempDir())
		if migrated || err != nil || report.NewerLayout != 5 {
			t.Errorf("%s: migration = %v %+v %v", raw, migrated, report, err)
		}
		if after, _ := os.ReadFile(path); string(after) != raw {
			t.Errorf("the newer file was changed:\n%s", after)
		}
		if backups, _ := filepath.Glob(path + ".bak-*"); len(backups) != 0 {
			t.Errorf("%s: backups written: %v", raw, backups)
		}
	}
}

// What only Sectile Desktop used leaves the file at the first agent save and
// at start, without a layout backup, while the connection stays (#746).
func TestAgentWritesDropTheDesktopOnlyKeys(t *testing.T) {
	for _, name := range []string{"migration", "save"} {
		t.Run(name, func(t *testing.T) {
			path := currentSettingsFileWith(t, `{"server":"https://sectile.example.test","deviceId":"dev_1","apiKey":"cli","pairedAt":"2026-10-06T07:00:00Z","appearance":"dark","consoleView":"terminal","repo":"/r","binary":"/b","secret":"enc"}`)
			if name == "migration" {
				migrated, report, err := MigrateSettingsReport(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				if migrated || !report.DesktopKeysRemoved {
					t.Fatalf("migrated = %v, report = %+v", migrated, report)
				}
				if backups, _ := filepath.Glob(path + ".bak-layout*"); len(backups) > 0 {
					t.Fatalf("backup written: %v", backups)
				}
			} else if _, err := UpdateSettings(t.TempDir(), func(*Settings) error { return nil }); err != nil {
				t.Fatal(err)
			}
			fields := readFields(t, path)
			for _, key := range desktopOnlyKeys {
				if _, kept := fields[key]; kept {
					t.Fatalf("%s kept: %v", key, fields)
				}
			}
			for _, key := range []string{"server", "deviceId", "apiKey", "pairedAt"} {
				if _, kept := fields[key]; !kept {
					t.Fatalf("%s lost: %v", key, fields)
				}
			}
		})
	}
}

func TestMigrationLeavesAFileWithoutDesktopKeysAlone(t *testing.T) {
	path := currentSettingsFileWith(t, `{"server":"https://sectile.example.test"}`)
	before, _ := os.ReadFile(path)
	if migrated, report, err := MigrateSettingsReport(t.TempDir()); err != nil || migrated || report.DesktopKeysRemoved {
		t.Fatalf("migrated = %v, report = %+v, err = %v", migrated, report, err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("file rewritten: %s", after)
	}
}

// currentSettingsFileWith writes a settings file as this agent saves it, with
// extra top-level fields merged in, so that the start-up migration has no
// layout conversion to make.
func currentSettingsFileWith(t *testing.T, extra string) string {
	t.Helper()
	path := writeSettingsFile(t, `{}`)
	if _, err := UpdateSettings(t.TempDir(), func(*Settings) error { return nil }); err != nil {
		t.Fatal(err)
	}
	fields := map[string]json.RawMessage{}
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(extra), &fields); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(fields)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
