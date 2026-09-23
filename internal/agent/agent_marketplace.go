package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/agentprotocol"
	"tasks/internal/marketplace"
	"tasks/internal/models"
)

// A marketplace is read on the workstation, never by the server: the two skill
// pack actions sit here beside sync_config and spec_install for the same reason
// those do — the network and the checkouts belong to the machine the developer
// works on.
//
// The cache lives under ~/.taskflow/marketplaces/<name>/, next to the agent
// connection file. A pinned project reads its own cache and never refetches: a
// revision that was applied once stays what it was.

// marketplaceCacheRoot is where every cached marketplace lives.
func marketplaceCacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".taskflow", "marketplaces"), nil
}

// marketplaceCacheDir turns a registry name into a directory. A name carrying a
// path separator is refused rather than sanitized: it means the server sent
// something that is not a registry key, and guessing what was meant is how a
// cache ends up outside its own root.
func marketplaceCacheDir(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("marketplace name is required")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || !filepath.IsLocal(name) {
		return "", fmt.Errorf("invalid marketplace name %q", name)
	}
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, name)
	root, err := marketplaceCacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, safe), nil
}

// ensureMarketplace makes the marketplace readable locally and returns the
// directory to parse, the commit it stands at, and what the caller should be
// told about the result — an unversioned source, for one, cannot be pinned.
func ensureMarketplace(ctx context.Context, op agentprotocol.Operation) (root, commit string, warnings []string, err error) {
	kind := strings.ToLower(strings.TrimSpace(op.Kind))
	locator := strings.TrimSpace(op.Locator)
	if locator == "" {
		return "", "", nil, fmt.Errorf("marketplace locator is required")
	}
	// Both values reach git as arguments; the server validates them too, but a
	// value git would read as an option is refused here whoever sent it.
	if strings.HasPrefix(locator, "-") {
		return "", "", nil, fmt.Errorf("invalid marketplace locator %q", locator)
	}
	if op.Commit = strings.TrimSpace(op.Commit); op.Commit != "" && !models.IsCommitSHA(op.Commit) {
		return "", "", nil, fmt.Errorf("invalid commit %q: expected a hexadecimal commit id", op.Commit)
	}

	if kind == models.MarketplaceKindPath || kind == "" && filepath.IsAbs(locator) {
		fi, err := os.Stat(locator)
		if err != nil || !fi.IsDir() {
			return "", "", nil, fmt.Errorf("no directory at %s", locator)
		}
		commit, _ = gitLocal(ctx, locator, "rev-parse", "HEAD")
		commit = strings.TrimSpace(commit)
		if commit == "" {
			warnings = append(warnings, "the source is a plain directory: nothing pins a revision, and the pack is not reproducible")
		} else if op.Commit != "" && op.Commit != commit {
			warnings = append(warnings, fmt.Sprintf("a local directory is read where it stands (%s), not at the pinned %s", short(commit), short(op.Commit)))
		}
		return locator, commit, warnings, nil
	}

	url, err := marketplaceURL(kind, locator)
	if err != nil {
		return "", "", nil, err
	}
	dir, err := marketplaceCacheDir(op.Marketplace)
	if err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", "", nil, err
	}

	// Re-registering a name with another locator must not keep reading the
	// repository the cache was cloned from, so a checkout of another origin is
	// dropped and cloned again. A path source has no cache to realign: it is
	// read where the registry says, every time.
	if isGitCheckout(dir) {
		if origin, err := gitLocal(ctx, dir, "remote", "get-url", "origin"); err != nil || strings.TrimSpace(origin) != url {
			_ = os.RemoveAll(dir)
		}
	}
	if !isGitCheckout(dir) {
		// A leftover that is not a checkout is not something to repair in
		// place; the cache is disposable by construction.
		_ = os.RemoveAll(dir)
		if _, err := gitLocal(ctx, filepath.Dir(dir), "clone", "--filter=blob:none", "--", url, filepath.Base(dir)); err != nil {
			return "", "", nil, err
		}
	} else if op.Commit == "" || !hasCommit(ctx, dir, op.Commit) {
		// A pinned revision already in the cache needs no network at all: this
		// is what lets a pinned project install its skills offline.
		if _, err := gitLocal(ctx, dir, "fetch", "--prune", "origin"); err != nil {
			if op.Commit == "" || !hasCommit(ctx, dir, op.Commit) {
				return "", "", nil, err
			}
			warnings = append(warnings, "the marketplace could not be fetched; the pinned revision was read from the cache")
		}
	}

	target := op.Commit
	if target == "" {
		if target, err = remoteHead(ctx, dir); err != nil {
			return "", "", nil, err
		}
	}
	// The revision is resolved to a full commit id behind --end-of-options
	// first, so what checkout receives can never be read as an option.
	sha, err := gitLocal(ctx, dir, "rev-parse", "--verify", "--end-of-options", target+"^{commit}")
	if err != nil {
		return "", "", nil, err
	}
	if _, err := gitLocal(ctx, dir, "checkout", "--detach", strings.TrimSpace(sha), "--"); err != nil {
		return "", "", nil, err
	}
	commit, err = gitLocal(ctx, dir, "rev-parse", "HEAD")
	return dir, strings.TrimSpace(commit), warnings, err
}

// marketplaceURL turns a registration into something git can clone.
func marketplaceURL(kind, locator string) (string, error) {
	switch kind {
	case models.MarketplaceKindGithub:
		if strings.Count(strings.Trim(locator, "/"), "/") != 1 {
			return "", fmt.Errorf("a github marketplace is registered as owner/repo, got %q", locator)
		}
		return "https://github.com/" + strings.Trim(locator, "/") + ".git", nil
	case models.MarketplaceKindGit, "":
		return locator, nil
	default:
		return "", fmt.Errorf("unknown marketplace kind %q", kind)
	}
}

func isGitCheckout(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && (fi.IsDir() || fi.Mode().IsRegular())
}

func hasCommit(ctx context.Context, dir, commit string) bool {
	if strings.TrimSpace(commit) == "" {
		return false
	}
	_, err := gitLocal(ctx, dir, "cat-file", "-e", commit+"^{commit}")
	return err == nil
}

// remoteHead is the revision a marketplace head resolves to. The format pins
// nothing itself, so Sectile decides what "the latest" means once, here.
func remoteHead(ctx context.Context, dir string) (string, error) {
	if ref, err := gitLocal(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && strings.TrimSpace(ref) != "" {
		return strings.TrimSpace(ref), nil
	}
	for _, candidate := range []string{"origin/main", "origin/master"} {
		if _, err := gitLocal(ctx, dir, "rev-parse", "--verify", candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no default branch on the marketplace remote")
}

func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// marketplaceForget drops the cache of a marketplace the registry no longer
// carries. The cache is disposable by construction, so a name that was never
// cached is not an error.
func marketplaceForget(op agentprotocol.Operation) (any, error) {
	dir, err := marketplaceCacheDir(op.Marketplace)
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	return map[string]any{"removed": dir}, nil
}

// marketplaceCatalog answers the plugin picker in one call: every plugin of the
// marketplace with the workflow skills it would supply and the directories
// Sectile would ignore.
func marketplaceCatalog(ctx context.Context, op agentprotocol.Operation) (models.MarketplaceCatalog, error) {
	root, commit, _, err := ensureMarketplace(ctx, op)
	if err != nil {
		return models.MarketplaceCatalog{}, err
	}
	manifest, err := marketplace.ParseMarketplace(root)
	if err != nil {
		return models.MarketplaceCatalog{}, err
	}
	catalog := models.MarketplaceCatalog{
		Name:        manifest.Name,
		Owner:       manifest.Owner.Name,
		Description: manifest.Description,
		Commit:      commit,
		Plugins:     make([]models.MarketplacePlugin, 0, len(manifest.Plugins)),
	}
	for _, ref := range manifest.Plugins {
		entry := models.MarketplacePlugin{
			Name:        ref.Name,
			Source:      ref.Source,
			Description: ref.Description,
			Version:     ref.Version,
			Skills:      []string{},
		}
		pack, err := marketplace.ResolvePlugin(root, ref)
		if err != nil {
			// A plugin Sectile cannot use is listed with its reason, not
			// hidden: the picker has to say why it is not selectable.
			entry.Error = err.Error()
		}
		entry.Skills = pack.Dirs()
		if pack.Plugin.Version != "" {
			entry.Version = pack.Plugin.Version
		}
		for _, ignored := range pack.Ignored {
			entry.Ignored = append(entry.Ignored, ignored.Dir)
		}
		catalog.Plugins = append(catalog.Plugins, entry)
	}
	return catalog, nil
}

// marketplacePack resolves one plugin into the bodies a project would run.
func marketplacePack(ctx context.Context, op agentprotocol.Operation) (models.SkillPack, error) {
	if strings.TrimSpace(op.Plugin) == "" {
		return models.SkillPack{}, fmt.Errorf("plugin name is required")
	}
	root, commit, warnings, err := ensureMarketplace(ctx, op)
	if err != nil {
		return models.SkillPack{}, err
	}
	manifest, err := marketplace.ParseMarketplace(root)
	if err != nil {
		return models.SkillPack{}, err
	}
	ref, ok := manifest.PluginByName(op.Plugin)
	if !ok {
		return models.SkillPack{}, fmt.Errorf("marketplace %q has no plugin %q", manifest.Name, op.Plugin)
	}
	pack, err := marketplace.ResolvePlugin(root, ref)
	if err != nil {
		return models.SkillPack{}, err
	}

	name := strings.TrimSpace(op.Marketplace)
	if name == "" {
		name = manifest.Name
	}
	result := models.SkillPack{
		Marketplace: name,
		Plugin:      ref.Name,
		Version:     pack.Plugin.Version,
		Commit:      commit,
		Bodies:      pack.Bodies(),
		Rejected:    pack.Rejected,
		Warnings:    warnings,
	}
	for _, ignored := range pack.Ignored {
		result.Ignored = append(result.Ignored, ignored.Dir)
	}
	if len(result.Rejected) == 0 {
		result.Rejected = nil
	}
	return result, nil
}
