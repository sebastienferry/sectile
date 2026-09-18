// Command sectile-agent is the workstation executable. It dispatches to one of
// five independent roles and owns no logic of its own: the daemon that talks
// to the server, the one-time pairing that stores the workstation API key, the
// stdio MCP bridge a coding CLI spawns, the terminal-side supervisor of a
// single agent-owned command, and the Claude Code hook.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"tasks/internal/agent"
	"tasks/internal/agentexec"
	"tasks/internal/agenthook"
	"tasks/internal/agentmcp"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "pair":
			message, err := agent.Pair(args[1:])
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(message)
			return
		case "mcp":
			if err := agentmcp.Run(context.Background(), args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		case "agent-exec":
			if err := agentexec.Run(args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		case "sectile-hook":
			// A Claude Code hook reads the exit code back and treats stdout as
			// the hook's answer, so this one neither fails nor prints: it makes
			// its report and returns. The name is also the marker that tells a
			// Sectile-owned registration from a hook the user wrote.
			agenthook.Run()
			return
		}
	}
	agent.Run(args)
}
