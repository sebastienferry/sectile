// Package sectile holds the repository-level assets the executables ship with.
//
// It exists for one reason: `go:embed` cannot reach outside the directory of
// the package that declares it, and CHANGELOG.md belongs at the root of the
// repository, where every convention and every reader expects to find it.
// Copying it into internal/ to satisfy the compiler would leave two files to
// keep in step, and the copy would be the one the product serves.
package sectile

import (
	_ "embed"
	"strings"
)

// Changelog is CHANGELOG.md as written, Markdown included. The server serves it
// verbatim and both interfaces render it: a release note reformatted on its way
// to the screen is a release note nobody can proofread before tagging.
//
//go:embed CHANGELOG.md
var Changelog string

// ChangelogMarkdown returns the changelog with its trailing blank lines
// trimmed, which is what a renderer wants.
func ChangelogMarkdown() string { return strings.TrimSpace(Changelog) + "\n" }
