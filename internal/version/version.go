// Package version carries the one version string every Sectile executable
// reports: the server, the agent and, through them, both user interfaces.
//
// The value is the Git tag the release was built from. Nothing derives it from
// a file in the repository: a version committed in a manifest drifts from the
// tag the moment somebody forgets to bump it, and the binary would then claim
// a release it is not. The tag is the release, and the link step is what puts
// it into the binary.
package version

import (
	"runtime/debug"
	"strings"
)

// Version, Commit and Date are set at link time by the release build:
//
//	go build -ldflags "-X tasks/internal/version.Version=v1.4.0 \
//	                   -X tasks/internal/version.Commit=$(git rev-parse HEAD) \
//	                   -X tasks/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// A build that skips them says "dev", which is exactly what `go run`, `make
// build` and a developer's checkout are. Showing "dev" is the point: a binary
// must never claim a release number it was not cut from.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Dev is the version a build carries when no tag was injected.
const Dev = "dev"

// Info is what an executable reports about itself, and what /api/version
// serves. Commit and Date are omitted when unknown rather than sent empty, so
// a caller can tell "not recorded" from "recorded as nothing".
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
}

// Current reports the build's identity.
//
// When the link step left Commit or Date empty it falls back to the stamps the
// Go toolchain embeds on its own (`vcs.revision`, `vcs.time`), which is what a
// `go install` from a checkout carries. The version itself never falls back:
// the module version of a binary built outside a release is a pseudo-version,
// and reporting that as the product version would be a lie dressed as a fact.
func Current() Info {
	info := Info{Version: strings.TrimSpace(Version), Commit: strings.TrimSpace(Commit), Date: strings.TrimSpace(Date)}
	if info.Version == "" {
		info.Version = Dev
	}
	if info.Commit != "" && info.Date != "" {
		return info
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = setting.Value
			}
		}
	}
	return info
}

// String is the one-line form the binaries print for --version: the release,
// then the commit it was cut from when that is known.
func String() string {
	info := Current()
	if info.Commit == "" {
		return info.Version
	}
	commit := info.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return info.Version + " (" + commit + ")"
}
