//go:build sectile_headless

package webui

import "io/fs"

// FS returns no browser assets for the headless server build.
func FS() (fs.FS, bool) { return nil, false }

// Headless reports whether this binary was built without browser UI assets.
func Headless() bool { return true }
