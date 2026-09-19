// Command sectile-agent is the workstation executable. It dispatches to one of
// five independent roles and owns no logic of its own: the daemon that talks
// to the server, the one-time pairing that stores the workstation API key, the
// reader and writer of the workstation settings, the stdio MCP bridge a coding
// CLI spawns, and the terminal-side supervisor of a single agent-owned command.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"tasks/internal/agent"
	"tasks/internal/agentexec"
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
		case "config":
			message, err := agent.Configure(args[1:])
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
		}
	}
	agent.Run(args)
}
