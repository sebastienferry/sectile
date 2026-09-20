package agent

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/agentconfig"
)

// Init runs `sectile-agent init`: it bootstraps native MCP and managed skills locally
// for the specified AI provider.
func Init(args []string) (string, error) {
	return InitContext(context.Background(), args)
}

// InitContext executes the initialization with the provided context.
func InitContext(ctx context.Context, args []string) (string, error) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	providerFlag := fs.String("provider", "", "Provider identifier to bootstrap (e.g. claude, agy, codex, cursor, gemini, vibe)")
	serverURL := fs.String("url", "", "Remote Sectile server URL (e.g. https://sectile.example.com); defaults to the paired server")
	token := fs.String("token", "", "Workstation API key (defaults to TOKEN, then to the key stored by `sectile-agent pair`)")
	projectID := fs.String("project", "", "Project primary key (defaults to matching local repository or the first project)")
	repoRoot := fs.String("repo", "", "Local repository root (defaults to current Git checkout)")
	fs.SetOutput(os.Stderr)
	var flagArgs []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return "", err
	}

	provider := strings.TrimSpace(*providerFlag)
	if provider == "" && len(positional) > 0 {
		provider = strings.TrimSpace(positional[0])
	}
	if provider == "" {
		return "", fmt.Errorf("--provider is required (e.g. claude, agy, codex, cursor, gemini, vibe)")
	}
	provider = strings.ToLower(provider)

	loc, err := agentconfig.ResolveLocations(provider)
	if err != nil {
		return "", err
	}

	stored, _ := agentconfig.ReadConnection()
	resolvedURL := resolveServerURL(*serverURL)
	if resolvedURL == "" {
		resolvedURL = stored.Server
	}
	if resolvedURL == "" {
		return "", fmt.Errorf("no server URL: pair this workstation with `sectile-agent pair` or pass --url")
	}

	resolvedToken := resolveCredential(*token, os.Getenv("TOKEN"), resolvedURL, stored)
	if resolvedToken == "" {
		return "", fmt.Errorf("no API key for this server: pair this workstation with `sectile-agent pair` or pass --token")
	}

	root := strings.TrimSpace(*repoRoot)
	if root == "" {
		wd, _ := os.Getwd()
		root = findRepoRoot(wd)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}

	d := &agentDaemon{
		link: serverLink{
			serverURL: strings.TrimRight(resolvedURL, "/"),
			token:     resolvedToken,
		},
		repoRoot: root,
	}

	selectedProject := strings.TrimSpace(*projectID)
	if selectedProject == "" {
		projects, err := d.discoverProjects(ctx)
		if err != nil {
			return "", fmt.Errorf("discover projects: %w", err)
		}
		// Match by remote URL if inside git repository
		remote, remoteErr := gitLocal(ctx, root, "remote", "get-url", "origin")
		if remoteErr == nil && remote != "" {
			for _, p := range projects.Projects {
				if p.GitRemoteURL != "" && repositoryIdentity(remote) == repositoryIdentity(p.GitRemoteURL) {
					selectedProject = p.ID
					break
				}
			}
		}
		// Match by local mapped projects in settings
		if selectedProject == "" {
			if settings, err := agentconfig.ReadSettings(root); err == nil {
				for id, pPath := range settings.Projects {
					if pPath != "" && filepath.Clean(pPath) == filepath.Clean(root) {
						selectedProject = id
						break
					}
				}
			}
		}
		// Fallback to "default" if available, or the first project
		if selectedProject == "" {
			for _, p := range projects.Projects {
				if p.ID == "default" {
					selectedProject = p.ID
					break
				}
			}
		}
		if selectedProject == "" && len(projects.Projects) > 0 {
			selectedProject = projects.Projects[0].ID
		}
		if selectedProject == "" {
			return "", fmt.Errorf("no project found on server to fetch configuration and skills from")
		}
	}

	config, err := d.fetchConfig(ctx, selectedProject, "")
	if err != nil {
		return "", fmt.Errorf("fetch project %q configuration: %w", selectedProject, err)
	}

	// Apply provider override for local initialization
	config.AIProvider = provider
	config.SetupProviders = []string{provider}

	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	if temporaryExecutable(executable) && !runningUnderTest() {
		return "", fmt.Errorf("agent is running from a temporary build at %s; run a built binary so native clients keep resolving it", executable)
	}

	mcpPath, err := agentconfig.BootstrapMCP(provider, executable, d.link.serverURL, d.link.token)
	if err != nil {
		return "", fmt.Errorf("bootstrap MCP for provider %q: %w", provider, err)
	}

	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		root, _ = os.Getwd()
	}
	if _, err := agentconfig.Scaffold(root, config); err != nil {
		return "", fmt.Errorf("install skills for provider %q: %w", provider, err)
	}

	if loc.InstallsSkills() {
		return fmt.Sprintf("Successfully initialized %s.\n- MCP registration: %s\n- Skills installed: %d in %s",
			provider, mcpPath, len(config.Skills), filepath.Join(loc.Home, loc.SkillDir)), nil
	}
	return fmt.Sprintf("Successfully initialized %s.\n- MCP registration: %s\n- Skills: provider %q has no user skill directory convention.",
		provider, mcpPath, provider), nil
}
