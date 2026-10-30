// Command sectile-companion composes the Local backend and workstation agent.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tasks/internal/agent"
	"tasks/internal/agentattach"
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
		case "--version", "-version", "version":
			fmt.Println("sectile-companion " + version.String())
			return
		case "--local-runtime-info":
			_ = json.NewEncoder(os.Stdout).Encode(localruntime.Info())
			return
		case "attach":
			if err := agentattach.Run(args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		case "pair":
			message, err := agent.Pair(args[1:])
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(message)
			return
		case "init":
			message, err := agent.Init(args[1:])
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(message)
			return
		case "render-skills", "skills":
			if err := agent.RenderSkills(args[1:]); err != nil {
				log.Fatal(err)
			}
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
	prepareUpdate := flags.Bool("prepare-update", false, "stop an idle Local companion before manual app replacement")
	upgrade := flags.Bool("upgrade", false, "migrate the stopped managed Local installation")
	legacyServer := flags.String("legacy-server", "", "managed legacy server executable")
	legacyAgent := flags.String("legacy-agent", "", "managed legacy agent executable")
	root := flags.String("data-dir", "", "private Local installation directory")
	_ = flags.Parse(args)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if *prepareUpdate {
		if *upgrade {
			log.Fatal("choose either migration or update preparation")
		}
		updateCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		if err := companion.PrepareUpdate(updateCtx, *root); err != nil {
			code := "unconfirmed"
			if errors.Is(err, companion.ErrUpdateBusy) {
				code = "busy"
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ready": false, "reason": code})
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ready": true})
		return
	}
	if *upgrade {
		if err := companion.Upgrade(ctx, *root, *legacyServer, *legacyAgent); err != nil {
			log.Fatal(err)
		}
		return
	}
	for {
		err := companion.Run(ctx, *root)
		if err == companion.ErrRestartRequested && ctx.Err() == nil {
			continue
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}
}
