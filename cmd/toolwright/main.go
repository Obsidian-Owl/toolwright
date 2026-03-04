package main

import (
	"os"

	"github.com/Obsidian-Owl/toolwright/internal/cli"
)

func main() {
	cmd := cli.BuildRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(cli.ExitError)
	}
}
