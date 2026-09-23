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

	_, err := agentconfig.ResolveLocations(provider)
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

	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		root, _ = os.Getwd()
	}
	result, err := d.initializeProvider(root, config, provider)
	return result.Message, err
}

// initializationStep distinguishes a failure from a step that was never attempted.
type initializationStep struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type initializationResult struct {
	Provider string             `json:"provider"`
	Success  bool               `json:"success"`
	MCP      initializationStep `json:"mcp"`
	Skills   initializationStep `json:"skills"`
	Message  string             `json:"message"`
}

// initializeProvider is the shared CLI/desktop initialization operation. Callers
// resolve the connection, fetch fresh server configuration and select a checkout.
func (d *agentDaemon) initializeProvider(root string, config agentconfig.Config, provider string) (result initializationResult, err error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	result.Provider = provider
	result.MCP = initializationStep{Status: "not_run", Message: "Not run"}
	result.Skills = initializationStep{Status: "not_run", Message: "Not run"}
	defer func() {
		if err != nil {
			result.Message = fmt.Sprintf("Initialization failed for %s: %v. MCP: %s. Skills: %s", provider, err, result.MCP.Message, result.Skills.Message)
		}
	}()
	loc, err := agentconfig.ResolveLocations(provider)
	if err != nil {
		return result, err
	}
	// Explicit initialization always targets one provider, regardless of server defaults.
	config.AIProvider = provider
	config.SetupProviders = []string{provider}

	executable, err := os.Executable()
	if err != nil {
		return result, err
	}
	if temporaryExecutable(executable) && !runningUnderTest() {
		return result, fmt.Errorf("agent is running from a temporary build at %s; run a built binary so native clients keep resolving it", executable)
	}

	mcpPath, err := agentconfig.BootstrapMCP(provider, executable, d.link.serverURL, d.link.token)
	if err != nil {
		result.MCP = initializationStep{Status: "failed", Message: err.Error()}
		return result, fmt.Errorf("bootstrap MCP for provider %q: %w", provider, err)
	}
	result.MCP = initializationStep{Status: "success", Message: "Registered in " + mcpPath}

	if _, err := agentconfig.ScaffoldProvider(root, config, provider); err != nil {
		result.Skills = initializationStep{Status: "failed", Message: err.Error()}
		return result, fmt.Errorf("install skills for provider %q: %w", provider, err)
	}

	result.Success = true
	if loc.InstallsSkills() {
		result.Skills = initializationStep{Status: "success", Message: fmt.Sprintf("%d installed in %s", len(config.Skills), filepath.Join(loc.Home, loc.SkillDir))}
		result.Message = fmt.Sprintf("Successfully initialized %s.\n- MCP registration: %s\n- Skills installed: %d in %s",
			provider, mcpPath, len(config.Skills), filepath.Join(loc.Home, loc.SkillDir))
	} else {
		result.Skills = initializationStep{Status: "skipped", Message: "Provider has no user skill directory convention"}
		result.Message = fmt.Sprintf("Successfully initialized %s.\n- MCP registration: %s\n- Skills: provider %q has no user skill directory convention.", provider, mcpPath, provider)
	}
	return result, nil
}
