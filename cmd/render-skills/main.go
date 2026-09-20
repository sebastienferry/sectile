// Command render-skills exports or prints rendered workflow skills
// from the embedded markdown fragments.
package main

import (
	"fmt"
	"os"

	"tasks/internal/agent"
)

func main() {
	if err := agent.RenderSkills(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
