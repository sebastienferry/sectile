// Package testsqlite provides isolated copies of a migrated SQLite fixture.
// It deliberately does not import the application database package, so its
// in-package tests can use the same helper as handlers and other consumers.
package testsqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The image is immutable and process-local. No database, key file, worker or
// connection is shared between tests, and no generated schema is checked in.
var template struct {
	sync.Once
	image []byte
	err   error
}

// New creates a new fixture at path, then opens it through the usual production
// constructor. Use it only for behavior tests; migration, first-start, recovery
// and persistence tests must call their real constructor directly.
//
// The first call builds the template using open, closes all database connections
// and verifies that no uncheckpointed WAL remains before reading the file. Only
// the database bytes are copied: each open creates its own secret key and runtime
// state. Callers retain responsibility for Close and temporary directory cleanup.
// All calls in a test binary must use the same database constructor.
func New[T interface{ Close() error }](t testing.TB, path string, open func(string) (T, error)) (T, error) {
	t.Helper()
	var zero T
	template.Do(func() {
		dir, err := os.MkdirTemp("", "sectile-sqlite-template-*")
		if err != nil {
			template.err = err
			return
		}
		defer os.RemoveAll(dir)
		source := filepath.Join(dir, "template.db")
		database, err := open(source)
		if err != nil {
			template.err = err
			return
		}
		if err := database.Close(); err != nil {
			template.err = err
			return
		}
		if info, err := os.Stat(source + "-wal"); err == nil && info.Size() != 0 {
			template.err = fmt.Errorf("template still has an uncheckpointed WAL after close")
			return
		} else if err != nil && !os.IsNotExist(err) {
			template.err = err
			return
		}
		template.image, template.err = os.ReadFile(source)
	})
	if template.err != nil {
		return zero, fmt.Errorf("initialize SQLite test template: %w", template.err)
	}
	// Refuse to overwrite an existing database: reopening a fixture must never
	// silently reset it, and migration fixtures must never be replaced.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return zero, err
	}
	_, writeErr := file.Write(template.image)
	closeErr := file.Close()
	if writeErr != nil {
		return zero, writeErr
	}
	if closeErr != nil {
		return zero, closeErr
	}
	return open(path)
}
