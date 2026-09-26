package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// attachedFoldersCapability tells the desktop this agent serves
// /desktop/folders (#484).
const attachedFoldersCapability = "attached-folders"

// Kinds of an attached folder, as the desktop settings and the folder map
// state them.
const (
	folderKindGit     = "git"
	folderKindFolder  = "folder"
	folderKindMissing = "missing"
)

// attachedFolder is a folder attached to a project on this workstation
// (#484), read from the disk at each use: a remote changed after the folder
// was attached is followed.
type attachedFolder struct {
	// Stored is the path as the settings keep it.
	Stored string
	// Path is where the folder is: the top level of a Git checkout, which a
	// subfolder was attached from, else Stored.
	Path string
	Kind string
	// Remote is the origin URL of a Git folder, "" when it has none.
	Remote   string
	Identity string
}

// describeFolder reads what a folder is: a Git checkout with or without an
// origin, a plain folder, or missing.
func describeFolder(ctx context.Context, path string) attachedFolder {
	folder := attachedFolder{Stored: path, Path: path, Kind: folderKindMissing}
	if info, err := os.Stat(path); err != nil || !info.IsDir() || !filepath.IsAbs(path) {
		return folder
	}
	folder.Kind = folderKindFolder
	top, err := gitLocal(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(top) == "" {
		return folder
	}
	folder.Kind, folder.Path = folderKindGit, filepath.Clean(strings.TrimSpace(top))
	if remote, err := gitLocal(ctx, folder.Path, "remote", "get-url", "origin"); err == nil && strings.TrimSpace(remote) != "" {
		folder.Remote = strings.TrimSpace(remote)
		folder.Identity = models.RepositoryIdentity(folder.Remote)
	}
	return folder
}

// attachedFolders describes the folders attached to a project here, in the
// order they were added.
func attachedFolders(ctx context.Context, overrides agentconfig.Settings, projectID string) []attachedFolder {
	var folders []attachedFolder
	for _, path := range overrides.ProjectSettings[projectID].Folders {
		if path = strings.TrimSpace(path); path != "" {
			folders = append(folders, describeFolder(ctx, path))
		}
	}
	return folders
}

// repositoryFolder is the folder holding identity's checkout here: its
// mapping, the project root for the code repository, else a Git folder
// attached to the project whose origin is identity. A mapping wins over an
// attached folder of the same repository.
func repositoryFolder(ctx context.Context, overrides agentconfig.Settings, projectID, projectRoot, code, identity string) (string, bool) {
	if root, ok := repositoryRoot(overrides, projectRoot, code, identity); ok {
		return root, true
	}
	if identity == "" {
		return "", false
	}
	for _, folder := range attachedFolders(ctx, overrides, projectID) {
		if folder.Kind == folderKindGit && folder.Identity == identity {
			return folder.Path, true
		}
	}
	return "", false
}
