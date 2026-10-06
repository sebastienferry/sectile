package agent

import (
	"path/filepath"
	"strings"

	"tasks/internal/models"
)

// runFolder is one folder of a run as the desktop lists it under the
// run's path (#762): the folder to copy, what to call it and its role in the
// run, as SECTILE_REPOSITORIES names it.
type runFolder struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Attached bool   `json:"attached,omitempty"`
}

// runFolders turns the folder map a run was launched with into the folders the
// desktop lists: the run's own directory first, as its primary folder, then
// each entry's worktree, or its folder when it has none. An entry with no path
// here, a missing attached folder and a folder already listed are left out.
// Nil when the map holds no other folder than the directory.
func runFolders(directory string, entries []models.FolderMapEntry) []runFolder {
	var folders []runFolder
	if directory != "" {
		primary := runFolder{Path: directory, Name: filepath.Base(filepath.Clean(directory)), Role: models.FolderRolePrimary}
		for _, entry := range entries {
			if entry.Role == models.FolderRolePrimary && entry.Identity != "" {
				primary.Name = identityName(entry.Identity)
				break
			}
		}
		folders = append(folders, primary)
	}
	for _, entry := range entries {
		if entry.Kind == folderKindMissing {
			continue
		}
		path := entry.Path
		if entry.Worktree != "" {
			path = entry.Worktree
		}
		if entry.Role == models.FolderRolePrimary && entry.Worktree == "" {
			// The primary repository's own checkout is not the run's folder: a
			// lazily prepared ticket names no worktree yet, and listing the
			// checkout would offer a folder the run must not write in.
			continue
		}
		name := identityName(entry.Identity)
		switch {
		case name != "":
		case entry.Role == models.FolderRoleSpec:
			name = "specifications"
		default:
			name = filepath.Base(filepath.Clean(path))
		}
		folders = addFolder(folders, runFolder{Path: path, Name: name, Role: entry.Role, Attached: entry.Attached})
	}
	if len(folders) < 2 {
		return nil
	}
	return folders
}

// addFolder appends folder unless its path is empty or already listed.
func addFolder(folders []runFolder, folder runFolder) []runFolder {
	if folder.Path == "" {
		return folders
	}
	for _, listed := range folders {
		if filepath.Clean(listed.Path) == filepath.Clean(folder.Path) {
			return folders
		}
	}
	// A full copy: the list of a run is read outside the queue lock once
	// copied, so it is never grown in place.
	return append(folders[:len(folders):len(folders)], folder)
}

// identityName is the last segment of a repository identity, the name the
// desktop gives a repository elsewhere: "sectile" for
// "github.com/sebastienferry/sectile".
func identityName(identity string) string {
	identity = strings.TrimRight(strings.TrimSpace(identity), "/")
	if i := strings.LastIndex(identity, "/"); i >= 0 {
		return identity[i+1:]
	}
	return identity
}

// addRunFolder records a folder added to a running run on its list, after
// the run's directory when the list is still empty. The caller holds the
// queue lock.
func addRunFolder(run *controlledRun, folder runFolder) {
	folders := run.desktop.Folders
	if len(folders) == 0 && run.desktop.Directory != "" {
		folders = []runFolder{{Path: run.desktop.Directory, Name: filepath.Base(filepath.Clean(run.desktop.Directory)), Role: models.FolderRolePrimary}}
	}
	run.desktop.Folders = addFolder(folders, folder)
}

// runEnded says whether a run can no longer be added a folder: restored,
// canceled or exited. The caller holds the queue lock.
func runEnded(run *controlledRun) bool {
	if run.restored || run.canceled {
		return true
	}
	select {
	case <-run.exited:
		return true
	default:
		return false
	}
}

// recordTaskFolder adds a worktree prepared for a task during its runs
// (prepare_repository_worktree) to the folders of every run of that task that
// has not ended, as a changed repository's.
func (d *agentDaemon) recordTaskFolder(task models.Task, identity, path string) {
	if path == "" {
		return
	}
	folder := runFolder{Path: path, Name: identityName(identity), Role: models.FolderRoleChanged}
	if folder.Name == "" {
		folder.Name = filepath.Base(filepath.Clean(path))
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	for _, run := range d.queue.runs {
		if run.taskID != task.ID && run.desktop.TaskID != task.ID && (task.Key == "" || run.desktop.TaskKey != task.Key) {
			continue
		}
		if !runEnded(run) {
			addRunFolder(run, folder)
		}
	}
}

// refreshConversationFolders applies the folder map a conversation's turn read
// to its list. A project conversation's list is that map; a ticket's keeps its
// task folders, which the project map does not name, and gains the folders
// attached to the project since. The caller holds the queue lock.
func refreshConversationFolders(run *controlledRun, entries []models.FolderMapEntry) {
	fresh := runFolders(run.desktop.Directory, entries)
	if run.desktop.TaskID == "" {
		run.desktop.Folders = fresh
		return
	}
	for _, folder := range fresh {
		addRunFolder(run, folder)
	}
}
