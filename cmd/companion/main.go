// Command sectile-companion composes the Local backend and workstation agent.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"tasks/internal/agentexec"
	"tasks/internal/agentmcp"
	"tasks/internal/companion"
	"tasks/internal/localruntime"
	"tasks/internal/runner"
	"tasks/internal/version"
)

func main() {
	runner.ExtendProcessPath()
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "--version", "version":
			fmt.Println("sectile-companion " + version.String())
			return
		case "--local-runtime-info":
			_ = json.NewEncoder(os.Stdout).Encode(localruntime.Info())
			return
		case "provider-guard":
			os.Exit(agentexec.RunProviderGuard(args[1:]))
		case "agent-exec":
			if err := agentexec.Run(args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		case "mcp":
			if err := agentmcp.Run(context.Background(), args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		}
	}
	flags := flag.NewFlagSet("sectile-companion", flag.ExitOnError)
	root := flags.String("data-dir", "", "private Local installation directory")
	_ = flags.Parse(args)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := companion.Run(ctx, *root); err != nil {
		log.Fatal(err)
	}
}
