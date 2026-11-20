// Command sectile-plugin writes the Claude plugin distributing Sectile's
// workflow skills and MCP server declaration, rendered from the built-in
// catalogue.
//
//	go run ./cmd/sectile-plugin -version 1.4.0 -out DIR [-lite] [-marketplace] [-force]
//
// With -marketplace, DIR becomes a one-plugin marketplace that
// `claude plugin marketplace add DIR` accepts, the plugin under plugins/sectile.
// With -lite, the plugin is Sectile Lite (plugins/sectile-lite): the same
// ticket skills on Jira, for a team without a Sectile server.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"tasks/internal/skills"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("sectile-plugin", flag.ContinueOnError)
	version := flags.String("version", "", "plugin version, SemVer without a leading v")
	out := flags.String("out", "", "directory to write the plugin into")
	marketplace := flags.Bool("marketplace", false, "wrap the plugin in a one-plugin marketplace")
	lite := flags.Bool("lite", false, "write the Sectile Lite plugin, which runs on Jira without a Sectile server")
	force := flags.Bool("force", false, "write into a directory that is not empty")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required")
	}

	render := skills.RenderPlugin
	switch {
	case *lite && *marketplace:
		render = skills.RenderLiteMarketplace
	case *lite:
		render = skills.RenderLitePlugin
	case *marketplace:
		render = skills.RenderMarketplace
	}
	files, err := render(*version)
	if err != nil {
		return err
	}

	if entries, err := os.ReadDir(*out); err == nil && len(entries) > 0 && !*force {
		return fmt.Errorf("%s is not empty; pass -force to write into it", *out)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(*out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, files[name], 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("Wrote %d files into %s\n", len(names), *out)
	return nil
}
