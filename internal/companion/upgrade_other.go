//go:build !darwin

package companion

import (
	"context"
	"errors"
)

// Offline migration works on every platform. Live legacy shutdown currently
// requires the macOS managed-process identity verifier.
func managedListener(int, string) (int, error) {
	return 0, errors.New("stop legacy services before migration on this platform")
}
func stopManaged(context.Context, int, string) error {
	return errors.New("live legacy shutdown is unavailable on this platform")
}
func syncDirectory(string) error { return nil }
