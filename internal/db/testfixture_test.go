package db

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"tasks/internal/secrets"
	"tasks/internal/testsqlite"
)

func TestSQLiteTemplateMatchesFreshSchema(t *testing.T) {
	fresh, err := NewDB(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	copied := testDB(t)
	schema := func(d *DB) []string {
		rows, err := d.conn.Query("SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master ORDER BY type, name")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var result []string
		for rows.Next() {
			var kind, name, table, sql string
			if err := rows.Scan(&kind, &name, &table, &sql); err != nil {
				t.Fatal(err)
			}
			result = append(result, fmt.Sprintf("%s|%s|%s|%s", kind, name, table, sql))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if !reflect.DeepEqual(schema(fresh), schema(copied)) {
		t.Fatal("template schema differs from a freshly migrated database")
	}
	for _, d := range []*DB{fresh, copied} {
		version, err := d.schemaVersion()
		if err != nil {
			t.Fatal(err)
		}
		if version != migrations[len(migrations)-1].version {
			t.Fatalf("schema version = %d", version)
		}
		var integrity string
		if err := d.conn.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
			t.Fatal(err)
		}
		if integrity != "ok" {
			t.Fatalf("integrity check: %s", integrity)
		}
	}
}

func TestSQLiteTemplateCopiesHaveIndependentDataAndKeys(t *testing.T) {
	// Prove that copies create their own key files even when a developer's shell
	// supplies a key. Do not alter the environment in parallel tests.
	t.Setenv(secrets.KeyEnvVar, "")
	first, second := testDB(t), testDB(t)
	if first.serverKey == second.serverKey {
		t.Fatal("fixtures share a secret key")
	}
	if first.conn == second.conn || first.jobQueue == second.jobQueue || first.instanceID == second.instanceID {
		t.Fatal("fixtures share runtime state")
	}
	if _, err := first.conn.Exec("UPDATE settings SET user_name = 'isolated' WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	for _, other := range []*DB{second, testDB(t)} {
		var name string
		if err := other.conn.QueryRow("SELECT user_name FROM settings WHERE id = 1").Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name == "isolated" {
			t.Fatal("a write escaped into another fixture or the template")
		}
	}
	path := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(path, []byte("do not replace"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := testsqlite.New(t, path, NewDB); !os.IsExist(err) {
		t.Fatalf("existing file: got %v, want os.ErrExist", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "do not replace" {
		t.Fatalf("existing file changed: %q, %v", contents, err)
	}
}

func TestSQLiteTemplateSupportsConcurrentCopies(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			d := testDB(t)
			if _, err := d.conn.Exec("CREATE TABLE only_this_copy (value INTEGER)"); err != nil {
				t.Fatal(err)
			}
			if _, err := d.conn.Exec("INSERT INTO only_this_copy VALUES (1)"); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := d.conn.QueryRow("SELECT COUNT(*) FROM only_this_copy").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("copy contains %d rows, want 1", count)
			}
		})
	}
}

func BenchmarkSQLiteFixture(b *testing.B) {
	for _, mode := range []string{"fresh", "template"} {
		b.Run(mode, func(b *testing.B) {
			// Warm the immutable image outside timing; CI package timing includes the
			// one-time build, while this benchmark measures steady-state fixture cost.
			if mode == "template" {
				d, err := testsqlite.New(b, filepath.Join(b.TempDir(), "warm.db"), NewDB)
				if err != nil {
					b.Fatal(err)
				}
				if err := d.Close(); err != nil {
					b.Fatal(err)
				}
			}
			root := b.TempDir()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dir := filepath.Join(root, fmt.Sprint(i))
				if err := os.Mkdir(dir, 0700); err != nil {
					b.Fatal(err)
				}
				path := filepath.Join(dir, "test.db")
				var d *DB
				var err error
				if mode == "fresh" {
					d, err = NewDB(path)
				} else {
					d, err = testsqlite.New(b, path, NewDB)
				}
				if err != nil {
					b.Fatal(err)
				}
				if err := d.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
