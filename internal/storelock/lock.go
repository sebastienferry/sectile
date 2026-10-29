package storelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrOwned = errors.New("local store is already owned by another runtime")

// Lock owns one private data root for the lifetime of a Local runtime.
// The lock file is never removed: operating-system locks, not file existence,
// distinguish a live owner from a process that crashed.
type Lock struct {
	file    *os.File
	release func() error
	once    sync.Once
	err     error
}

// Acquire must run before opening or recovering the database or journals.
// The caller supplies an explicit absolute root; no home-directory fallback
// is permitted. The root itself and the lock file may not be symlinks.
func Acquire(root string) (*Lock, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("local store root must be absolute")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create local store root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect local store root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("local store root is not a real directory")
	}
	path := filepath.Join(root, ".runtime.lock")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("local store lock is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect local store lock: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open local store lock: %w", err)
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect opened local store lock: %w", err)
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		_ = file.Close()
		return nil, fmt.Errorf("local store lock changed while opening")
	}
	release, err := lockFile(file)
	if err != nil {
		_ = file.Close()
		if isContended(err) {
			return nil, ErrOwned
		}
		return nil, fmt.Errorf("acquire local store lock: %w", err)
	}
	return &Lock{file: file, release: release}, nil
}

// Close releases ownership only after all store users have stopped.
func (l *Lock) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		unlockErr := l.release()
		closeErr := l.file.Close()
		l.err = errors.Join(unlockErr, closeErr)
	})
	return l.err
}
