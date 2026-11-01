package main

import (
	"fmt"
	"os"

	"github.com/sudame/ghq-worktree/internal/cli"
)

var version = "dev"

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghq-worktree:", err)
		os.Exit(1)
	}
	env := cli.Env{Version: version, Dir: dir, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	if err := cli.Run(os.Args[1:], env); err != nil {
		fmt.Fprintln(os.Stderr, "ghq-worktree:", err)
		os.Exit(1)
	}
}
