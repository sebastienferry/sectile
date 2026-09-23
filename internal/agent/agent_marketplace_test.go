package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/testhome"
)

// fixtureMarketplace copies the format fixture out of the repository, so a
// local-directory source is read where nothing else already carries a revision.
func fixtureMarketplace(t *testing.T) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "marketplace")
	copyTree(t, filepath.Join("..", "marketplace", "testdata", "marketplace"), dst)
	return dst
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		from, to := filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyTree(t, from, to)
			continue
		}
		raw, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(to, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A local directory is the source that needs no network at all, and the one a
// team uses before it publishes anything.
func TestMarketplacePackReadsALocalDirectory(t *testing.T) {
	root := fixtureMarketplace(t)
	op := agentprotocol.Operation{Marketplace: "acme", Kind: "path", Locator: root, Plugin: "acme-flow"}

	pack, err := marketplacePack(context.Background(), op)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Marketplace != "acme" || pack.Plugin != "acme-flow" || pack.Version != "2.1.0" {
		t.Fatalf("coordinates: %+v", pack)
	}
	for _, dir := range []string{"clarify-issue", "specify-issue", "code-issue", "adjust-issue"} {
		if !strings.Contains(pack.Bodies[dir], "Acme checklist") {
			t.Fatalf("missing body for %s", dir)
		}
	}
	// A directory that is not a checkout pins nothing, and the preview has to
	// say so rather than record an empty commit as if it were one.
	if pack.Commit != "" {
		t.Fatalf("a plain directory reported a commit: %q", pack.Commit)
	}
	if len(pack.Warnings) == 0 {
		t.Fatal("an unversioned source must warn that it is not reproducible")
	}
}

func TestMarketplaceCatalogReportsWhatEachPluginSupplies(t *testing.T) {
	root := fixtureMarketplace(t)
	catalog, err := marketplaceCatalog(context.Background(), agentprotocol.Operation{Marketplace: "acme", Kind: "path", Locator: root})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Name != "acme-workflow" || len(catalog.Plugins) != 5 {
		t.Fatalf("catalog: %+v", catalog)
	}
	byName := map[string]int{}
	for i, plugin := range catalog.Plugins {
		byName[plugin.Name] = i
	}
	if skills := catalog.Plugins[byName["acme-flow"]].Skills; len(skills) != 4 {
		t.Fatalf("acme-flow supplies %v", skills)
	}
	extras := catalog.Plugins[byName["acme-extras"]]
	if len(extras.Skills) != 1 || len(extras.Ignored) != 2 {
		t.Fatalf("acme-extras: %+v", extras)
	}
	if docs := catalog.Plugins[byName["acme-docs"]]; docs.Error == "" || len(docs.Skills) != 0 {
		t.Fatalf("a plugin Sectile cannot use must be listed with its reason: %+v", docs)
	}
}

// The cache directory is derived from the registry key; a key carrying a path
// separator is refused before anything is written.
func TestMarketplaceCacheDirRefusesAnEscapingName(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)

	for _, name := range []string{"", "..", ".", "../escape", `team\pack`, "team/pack"} {
		if _, err := marketplaceCacheDir(name); err == nil {
			t.Fatalf("name %q accepted", name)
		}
	}
	dir, err := marketplaceCacheDir("acme workflow")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".taskflow", "marketplaces", "acme-workflow"); dir != want {
		t.Fatalf("cache dir %q, want %q", dir, want)
	}
}

// The registry actions need no project: the daemon answers them without
// resolving a project or a local checkout first.
func TestMarketplaceRegistryActionsNeedNoProject(t *testing.T) {
	testhome.Set(t, t.TempDir())
	root := fixtureMarketplace(t)
	daemon := &agentDaemon{}

	if _, err := daemon.executeOperation(context.Background(), agentprotocol.Operation{Action: "marketplace_catalog", Marketplace: "acme", Kind: "path", Locator: root}); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := daemon.executeOperation(context.Background(), agentprotocol.Operation{Action: "marketplace_forget", Marketplace: "acme"}); err != nil {
		t.Fatalf("forget: %v", err)
	}
	if _, err := daemon.executeOperation(context.Background(), agentprotocol.Operation{Action: "marketplace_pack", Marketplace: "acme", Kind: "path", Locator: root, Plugin: "acme-flow"}); err == nil {
		t.Fatal("a project operation ran without a project")
	}
}

// Re-registering a name with another locator reads the new repository, not the
// one the cache was first cloned from.
func TestMarketplaceCacheFollowsALocatorChange(t *testing.T) {
	ctx := context.Background()
	testhome.Set(t, t.TempDir())

	publish := func(message string) (string, string) {
		repo := fixtureMarketplace(t)
		for _, args := range [][]string{
			{"init", "-b", "main"},
			{"add", "-A"},
			{"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", message},
		} {
			if _, err := gitLocal(ctx, repo, args...); err != nil {
				t.Fatal(err)
			}
		}
		head, err := gitLocal(ctx, repo, "rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		return repo, head
	}
	first, firstHead := publish("First source")
	second, secondHead := publish("Second source")
	if firstHead == secondHead {
		t.Fatal("the two sources must differ")
	}

	op := agentprotocol.Operation{Marketplace: "acme", Kind: "git", Locator: first}
	if _, commit, _, err := ensureMarketplace(ctx, op); err != nil || commit != firstHead {
		t.Fatalf("first source: %s, %v", commit, err)
	}
	op.Locator = second
	_, commit, _, err := ensureMarketplace(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	if commit != secondHead {
		t.Fatalf("after the locator change the cache read %s, want %s", commit, secondHead)
	}
}

// A locator or a commit git would read as an option never reaches git, even
// when the server let it through.
func TestEnsureMarketplaceRefusesAnOptionLikeArgument(t *testing.T) {
	testhome.Set(t, t.TempDir())
	root := fixtureMarketplace(t)

	for _, op := range []agentprotocol.Operation{
		{Marketplace: "acme", Kind: "git", Locator: "--upload-pack=touch pwned"},
		{Marketplace: "acme", Kind: "path", Locator: root, Commit: "--output=pwned"},
		{Marketplace: "acme", Kind: "git", Locator: root, Commit: "main"},
	} {
		if _, _, _, err := ensureMarketplace(context.Background(), op); err == nil {
			t.Fatalf("accepted %+v", op)
		}
	}
}

// A pinned revision already in the cache is read without touching the remote:
// installing skills on a pinned project must work with the network gone.
func TestMarketplacePackReusesTheCacheForAPinnedRevision(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	testhome.Set(t, home)

	origin := fixtureMarketplace(t)
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Publish the pack"},
	} {
		if _, err := gitLocal(ctx, origin, args...); err != nil {
			t.Fatal(err)
		}
	}

	op := agentprotocol.Operation{Marketplace: "acme", Kind: "git", Locator: origin, Plugin: "acme-flow"}
	first, err := marketplacePack(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	if first.Commit == "" {
		t.Fatal("a git source must resolve a commit to pin")
	}
	cache := filepath.Join(home, ".taskflow", "marketplaces", "acme")
	if _, err := os.Stat(filepath.Join(cache, ".git")); err != nil {
		t.Fatalf("nothing was cached: %v", err)
	}

	// Anything reaching the remote now fails, so a second read that succeeds
	// proves it was served from the cache.
	if err := os.Rename(origin, origin+"-gone"); err != nil {
		t.Fatal(err)
	}
	op.Commit = first.Commit
	second, err := marketplacePack(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	if second.Commit != first.Commit || len(second.Bodies) != len(first.Bodies) {
		t.Fatalf("pinned read drifted: %s vs %s", second.Commit, first.Commit)
	}

	// Without a pin, the same unreachable remote is a failure and not a silent
	// fallback to whatever the cache happens to hold.
	op.Commit = ""
	if _, err := marketplacePack(ctx, op); err == nil {
		t.Fatal("an unreachable marketplace was reported as resolved")
	}
}
