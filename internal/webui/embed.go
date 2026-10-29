//go:build !sectile_headless

// Package webui carries the compiled interface inside the binary.
//
// A single file that needs nothing beside it is the difference between a program
// one distributes and a program one installs. The interface is therefore built
// into internal/webui/dist and embedded from there: go:embed cannot reach above
// its own package, which is why the build output lives here rather than in web/.
//
// The embedded copy can legitimately be empty, when the binary is built without
// building the interface first. FS reports it, and the server then falls back on
// serving the files from disk, which is what the development loop does anyway.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the compiled interface and whether it holds anything. An empty
// embed is a build without a front end, not an error.
func FS() (fs.FS, bool) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return sub, true
}

// Headless reports whether this binary was built without browser UI assets.
func Headless() bool { return false }
