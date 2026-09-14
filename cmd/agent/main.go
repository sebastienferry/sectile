package main

import (
	"context"
	"log"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "mcp":
			if err := runMCPCommand(context.Background(), args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		case "agent-exec":
			if err := runAgentExec(args[1:]); err != nil {
				log.Fatal(err)
			}
			return
		}
	}
	runAgentCommand(args)
}
