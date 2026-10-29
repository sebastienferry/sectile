//go:build windows

package storelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(file *os.File) (func() error, error) {
	handle := windows.Handle(file.Fd())
	var overlapped windows.Overlapped
	err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if err != nil {
		return nil, err
	}
	return func() error { return windows.UnlockFileEx(handle, 0, 1, 0, &overlapped) }, nil
}

func isContended(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
