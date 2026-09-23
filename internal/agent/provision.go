package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// A task worktree is a fresh checkout: nothing gitignored comes with it, so
// every JavaScript package in it starts without node_modules and cannot build
// or lint. Provisioning gives each worktree its own install, run inside that
// worktree. The main checkout is never linked to or written to: a link would
// share one node_modules across every worktree, and npm ci run through it would
// empty the main checkout's copy.

// installStamp follows the Makefile's web-deps and desktop-deps convention, so a
// worktree installed by make and one installed by the agent agree on what is
// up to date.
const installStamp = ".install-stamp"

// The timeouts are variables so tests can shorten them. A first install of this
// repository's web package takes about four minutes on a Windows workstation,
// so a folder gets more than twice that. The total stays below the server's
// budget for prepare_workspace.
var (
	provisionFolderTimeout = 10 * time.Minute
	provisionTotalTimeout  = 15 * time.Minute
)

// npmInstall runs a clean install in dir. It is a variable so tests run without
// npm and without the network.
var npmInstall = func(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "npm", "ci")
	cmd.Dir = dir
	// npm starts node, which may outlive a killed npm and keep the output pipe
	// open; WaitDelay stops that from holding the launch past the timeout.
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, outputTail(string(out), 5))
	}
	return nil
}

// provisionLocks serialises provisioning per worktree, so two preparations of
// the same task never run npm ci in one folder at once, while preparations of
// different tasks proceed in parallel.
var provisionLocks sync.Map

// provisionWorktree installs the JavaScript dependencies of every package folder
// in workDir. It never fails the caller: a worktree without dependencies is a
// degraded worktree, not a reason to refuse a launch, so every problem is
// logged and the next preparation tries again.
func provisionWorktree(ctx context.Context, root, workDir string) {
	if workDir == "" || sameDirectory(workDir, root) {
		// The main checkout keeps being managed by make web-deps and desktop-deps.
		return
	}
	lock, _ := provisionLocks.LoadOrStore(filepath.Clean(workDir), &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	total, cancel := context.WithTimeout(ctx, provisionTotalTimeout)
	defer cancel()
	for _, dir := range packageFolders(workDir) {
		install, skip := installReason(dir)
		if skip != "" {
			log.Printf("[Agent] Skipping dependency install in %s: %s", dir, skip)
			continue
		}
		if !install {
			continue
		}
		if err := total.Err(); err != nil {
			log.Printf("[Agent] Skipping dependency install in %s: %v", dir, err)
			continue
		}
		log.Printf("[Agent] Installing dependencies in %s (npm ci)", dir)
		folder, cancelFolder := context.WithTimeout(total, provisionFolderTimeout)
		err := npmInstall(folder, dir)
		if err == nil {
			err = folder.Err()
		}
		cancelFolder()
		if err != nil {
			log.Printf("[Agent] Dependency install failed in %s, continuing without it: %v", dir, err)
			continue
		}
		if err := touchStamp(filepath.Join(dir, "node_modules", installStamp)); err != nil {
			log.Printf("[Agent] Dependencies installed in %s but the stamp was not written: %v", dir, err)
		}
	}
}

// packageFolders lists the worktree root and its direct subdirectories that
// hold a package.json. Hidden directories and node_modules are not searched.
func packageFolders(workDir string) []string {
	var folders []string
	if isRegularFile(filepath.Join(workDir, "package.json")) {
		folders = append(folders, workDir)
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return folders
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		dir := filepath.Join(workDir, name)
		if isRegularFile(filepath.Join(dir, "package.json")) {
			folders = append(folders, dir)
		}
	}
	return folders
}

// installReason says whether the package folder dir needs an install. A
// non-empty skip names why the folder is left alone and is worth a log line;
// install false with no skip means it is already up to date.
func installReason(dir string) (install bool, skip string) {
	lockfile := filepath.Join(dir, "package-lock.json")
	lock, err := os.Stat(lockfile)
	if err != nil || !lock.Mode().IsRegular() {
		return false, "no package-lock.json, only npm installs are supported"
	}
	modules := filepath.Join(dir, "node_modules")
	fi, err := os.Lstat(modules)
	if os.IsNotExist(err) {
		return true, ""
	}
	if err != nil {
		return false, err.Error()
	}
	// Windows reports a junction as irregular rather than as a symlink. npm ci
	// deletes node_modules first, so installing through a link would empty
	// whatever it points at, the main checkout's copy included.
	if !fi.IsDir() || fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return false, "node_modules is not a plain directory; remove it to let the agent install"
	}
	stamp, err := os.Stat(filepath.Join(modules, installStamp))
	if err != nil {
		return true, ""
	}
	manifest, err := os.Stat(filepath.Join(dir, "package.json"))
	if err != nil {
		return true, ""
	}
	return manifest.ModTime().After(stamp.ModTime()) || lock.ModTime().After(stamp.ModTime()), ""
}

func touchStamp(path string) error {
	if err := os.WriteFile(path, nil, 0644); err != nil {
		return err
	}
	now := time.Now()
	return os.Chtimes(path, now, now)
}

func isRegularFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// outputTail keeps the last lines of a command's output, where npm prints the
// reason it failed.
func outputTail(out string, lines int) string {
	all := strings.Split(strings.TrimSpace(out), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, " | ")
}
