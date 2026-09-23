package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeInstall replaces npmInstall for one test. It records the folders it ran in
// and, unless fail names the folder, creates node_modules there as npm ci would.
type fakeInstall struct {
	mu        sync.Mutex
	dirs      []string
	fail      map[string]bool
	deadlines []bool
}

func useFakeInstall(t *testing.T) *fakeInstall {
	t.Helper()
	fake := &fakeInstall{fail: map[string]bool{}}
	previous := npmInstall
	npmInstall = func(ctx context.Context, dir string) error {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		_, hasDeadline := ctx.Deadline()
		fake.dirs = append(fake.dirs, dir)
		fake.deadlines = append(fake.deadlines, hasDeadline)
		if fake.fail[dir] {
			return errors.New("npm ci exited with status 1")
		}
		return os.MkdirAll(filepath.Join(dir, "node_modules", "typescript"), 0755)
	}
	t.Cleanup(func() { npmInstall = previous })
	return fake
}

func (f *fakeInstall) ran() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.dirs)
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
}

// npmPackage lays out a package folder with a manifest and an npm lockfile.
func npmPackage(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "package.json"))
	writeFile(t, filepath.Join(dir, "package-lock.json"))
}

func hasStamp(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "node_modules", installStamp))
	return err == nil
}

func TestProvisionInstallsEveryPackageFolderOfTheWorktree(t *testing.T) {
	fake := useFakeInstall(t)
	root, work := t.TempDir(), t.TempDir()
	npmPackage(t, root)
	npmPackage(t, filepath.Join(root, "web"))
	npmPackage(t, work)
	npmPackage(t, filepath.Join(work, "web"))
	npmPackage(t, filepath.Join(work, "desktop"))

	provisionWorktree(context.Background(), root, work)

	want := []string{work, filepath.Join(work, "desktop"), filepath.Join(work, "web")}
	if got := fake.ran(); !slices.Equal(got, want) {
		t.Fatalf("installed in %v, want %v", got, want)
	}
	for _, dir := range want {
		fi, err := os.Lstat(filepath.Join(dir, "node_modules"))
		if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s has no real node_modules: %v", dir, err)
		}
		if !hasStamp(dir) {
			t.Fatalf("%s has no install stamp", dir)
		}
	}
	for _, deadline := range fake.deadlines {
		if !deadline {
			t.Fatal("an install ran without a timeout")
		}
	}
	// The main checkout is neither installed into nor linked to.
	for _, dir := range []string{root, filepath.Join(root, "web")} {
		if _, err := os.Lstat(filepath.Join(dir, "node_modules")); !os.IsNotExist(err) {
			t.Fatalf("provisioning touched the main checkout at %s: %v", dir, err)
		}
	}
}

func TestProvisionIsANoOpWithoutPackages(t *testing.T) {
	fake := useFakeInstall(t)
	root, work := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(work, "go.mod"))
	writeFile(t, filepath.Join(work, "internal", "main.go"))

	provisionWorktree(context.Background(), root, work)

	if got := fake.ran(); len(got) != 0 {
		t.Fatalf("installed in %v on a repository without package.json", got)
	}
}

func TestProvisionSkipsFoldersNpmDoesNotOwn(t *testing.T) {
	fake := useFakeInstall(t)
	root, work := t.TempDir(), t.TempDir()
	// A root lockfile without a manifest is not a package (this repository).
	writeFile(t, filepath.Join(work, "package-lock.json"))
	// A manifest without an npm lockfile is left to its own package manager.
	writeFile(t, filepath.Join(work, "yarnapp", "package.json"))
	writeFile(t, filepath.Join(work, "yarnapp", "yarn.lock"))
	// Hidden directories, node_modules and anything deeper are not searched.
	npmPackage(t, filepath.Join(work, ".tasks"))
	npmPackage(t, filepath.Join(work, "node_modules"))
	npmPackage(t, filepath.Join(work, "packages", "deep"))

	provisionWorktree(context.Background(), root, work)

	if got := fake.ran(); len(got) != 0 {
		t.Fatalf("installed in %v", got)
	}
}

func TestProvisionSkipsTheMainCheckout(t *testing.T) {
	fake := useFakeInstall(t)
	root := t.TempDir()
	npmPackage(t, filepath.Join(root, "web"))

	provisionWorktree(context.Background(), root, root)

	if got := fake.ran(); len(got) != 0 {
		t.Fatalf("installed in the main checkout: %v", got)
	}
}

func TestProvisionReinstallsOnlyWhenDependenciesChange(t *testing.T) {
	fake := useFakeInstall(t)
	root, work := t.TempDir(), t.TempDir()
	web := filepath.Join(work, "web")
	npmPackage(t, web)
	// Leave the lockfile clearly older than the stamp written below.
	past := time.Now().Add(-time.Hour)
	for _, name := range []string{"package.json", "package-lock.json"} {
		if err := os.Chtimes(filepath.Join(web, name), past, past); err != nil {
			t.Fatal(err)
		}
	}

	provisionWorktree(context.Background(), root, work)
	provisionWorktree(context.Background(), root, work)
	if got := fake.ran(); len(got) != 1 {
		t.Fatalf("an up-to-date stamp still reinstalled: %v", got)
	}

	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(web, "package-lock.json"), future, future); err != nil {
		t.Fatal(err)
	}
	provisionWorktree(context.Background(), root, work)
	if got := fake.ran(); len(got) != 2 {
		t.Fatalf("a newer lockfile did not reinstall: %v", got)
	}
}

func TestProvisionNeverInstallsThroughALink(t *testing.T) {
	fake := useFakeInstall(t)
	root, work := t.TempDir(), t.TempDir()
	shared := filepath.Join(root, "desktop", "node_modules")
	writeFile(t, filepath.Join(shared, "electron", "index.js"))
	desktop := filepath.Join(work, "desktop")
	npmPackage(t, desktop)
	npmPackage(t, filepath.Join(work, "web"))
	if err := os.Symlink(shared, filepath.Join(desktop, "node_modules")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	provisionWorktree(context.Background(), root, work)

	if got := fake.ran(); !slices.Equal(got, []string{filepath.Join(work, "web")}) {
		t.Fatalf("installed in %v, want web only", got)
	}
	if _, err := os.Stat(filepath.Join(shared, "electron", "index.js")); err != nil {
		t.Fatalf("the link target was modified: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shared, installStamp)); !os.IsNotExist(err) {
		t.Fatalf("a stamp was written through the link: %v", err)
	}
}

func TestProvisionFailureWritesNoStampAndContinues(t *testing.T) {
	fake := useFakeInstall(t)
	root, work := t.TempDir(), t.TempDir()
	desktop, web := filepath.Join(work, "desktop"), filepath.Join(work, "web")
	npmPackage(t, desktop)
	npmPackage(t, web)
	fake.fail[desktop] = true

	provisionWorktree(context.Background(), root, work)

	if got := fake.ran(); !slices.Equal(got, []string{desktop, web}) {
		t.Fatalf("a failure stopped the remaining folders: %v", got)
	}
	if hasStamp(desktop) {
		t.Fatal("a failed install wrote a stamp")
	}
	if !hasStamp(web) {
		t.Fatal("the folder after the failure was not stamped")
	}

	// The failed folder is retried at the next preparation, the other is not.
	fake.fail[desktop] = false
	provisionWorktree(context.Background(), root, work)
	if got := fake.ran(); !slices.Equal(got, []string{desktop, web, desktop}) {
		t.Fatalf("retry ran in %v", got)
	}
}

func TestProvisionTimeoutIsNotFatal(t *testing.T) {
	root, work := t.TempDir(), t.TempDir()
	web := filepath.Join(work, "web")
	npmPackage(t, web)
	previousTimeout := provisionFolderTimeout
	provisionFolderTimeout = 10 * time.Millisecond
	previous := npmInstall
	npmInstall = func(ctx context.Context, dir string) error {
		<-ctx.Done()
		// A command killed on timeout exits with a signal error; reporting
		// nothing must not make a stamp appear either.
		return nil
	}
	t.Cleanup(func() {
		provisionFolderTimeout = previousTimeout
		npmInstall = previous
	})

	provisionWorktree(context.Background(), root, work)

	if hasStamp(web) {
		t.Fatal("a timed-out install wrote a stamp")
	}
}
