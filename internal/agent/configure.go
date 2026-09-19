package agent

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"tasks/internal/agentconfig"
)

// Configure runs `sectile-agent config`: it reads and writes the workstation
// settings the daemon already obeys, so a project's parallelism can be changed
// from the terminal instead of the desktop app. Parallelism stays
// workstation-owned: nothing here is sent to or read from the server, and the
// ceiling is the one every other surface enforces.
func Configure(args []string) (string, error) {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	projectID := fs.String("project", "", "Project primary key the setting applies to")
	parallelism := fs.Int("parallelism", 0, fmt.Sprintf("Background executions this workstation runs for the project, 1 to %d", agentconfig.MaxParallelism))
	repoRoot := fs.String("repo", "", "Local repository root holding the legacy .taskflow/agent.json (defaults to the current Git checkout)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	root := strings.TrimSpace(*repoRoot)
	if root == "" {
		cwd, _ := os.Getwd()
		root = findRepoRoot(cwd)
	}
	overrides, err := agentconfig.ReadSettings(root)
	if err != nil {
		return "", err
	}
	// Without a value to write the command reports what the workstation applies
	// today, so the setting can be read back from the same place it is set.
	if !flagProvided(fs, "parallelism") {
		return describeParallelism(overrides), nil
	}
	id := strings.TrimSpace(*projectID)
	if id == "" {
		return "", fmt.Errorf("--project is required to set a parallelism")
	}
	// The same bounds the desktop mapping enforces: an out-of-range value is
	// refused rather than silently clamped into the stored settings.
	if *parallelism < 1 || *parallelism > agentconfig.MaxParallelism {
		return "", fmt.Errorf("parallelism must be between 1 and %d", agentconfig.MaxParallelism)
	}
	if overrides.Parallelism == nil {
		overrides.Parallelism = map[string]int{}
	}
	overrides.Parallelism[id] = *parallelism
	if err := agentconfig.WriteSettings(overrides); err != nil {
		return "", err
	}
	path, _ := agentconfig.SettingsPath()
	message := fmt.Sprintf("%s now runs up to %s on this workstation, stored in %s.", id, executionCount(*parallelism), path)
	if overrides.Projects[id] == "" {
		message += "\nThis project has no local repository yet, so map it before it can run anything."
	} else if useWorktrees, ok := overrides.Worktrees[id]; ok && !useWorktrees {
		message += "\nWorktrees are off for this project, so the agent keeps running one execution at a time."
	}
	// Admission re-reads the settings, so a running agent picks the new limit up
	// for the next submission without being restarted.
	message += "\nExecutions submitted from now on use the new limit; the ones already queued keep the limit they were admitted with."
	return message, nil
}

// describeParallelism renders every project the workstation knows about, so a
// value that was never stored is visibly a default rather than a blank.
func describeParallelism(overrides agentconfig.Overrides) string {
	ids := map[string]bool{}
	for id := range overrides.Projects {
		ids[id] = true
	}
	for id := range overrides.Parallelism {
		ids[id] = true
	}
	path, _ := agentconfig.SettingsPath()
	if len(ids) == 0 {
		return fmt.Sprintf("No project is configured on this workstation (%s).\nMap one, then set its parallelism with: sectile-agent config --project <id> --parallelism <1-%d>", path, agentconfig.MaxParallelism)
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	var out strings.Builder
	fmt.Fprintf(&out, "Workstation settings: %s\n\n", path)
	w := tabwriter.NewWriter(&out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tREPOSITORY\tWORKTREES\tPARALLEL EXECUTIONS")
	for _, id := range sorted {
		repository := overrides.Projects[id]
		if repository == "" {
			repository = "(not mapped)"
		}
		worktrees := "server default"
		if value, ok := overrides.Worktrees[id]; ok {
			worktrees = "no"
			if value {
				worktrees = "yes"
			}
		}
		limit := "1 (default)"
		if value, ok := overrides.Parallelism[id]; ok {
			limit = fmt.Sprintf("%d", value)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, repository, worktrees, limit)
	}
	_ = w.Flush()
	fmt.Fprintf(&out, "\nA project without a stored value runs a single execution, and a project without\nworktrees runs a single execution whatever the value. Change one with:\n  sectile-agent config --project <id> --parallelism <1-%d>", agentconfig.MaxParallelism)
	return out.String()
}

// flagProvided separates "--parallelism 0", which is out of range and must be
// refused, from an absent flag, which asks for the current settings instead.
func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func executionCount(n int) string {
	if n == 1 {
		return "1 execution"
	}
	return fmt.Sprintf("%d executions", n)
}
