package main

import (
	"os"

	"github.com/simonjohansson/kanban/cli/internal/kb"
)

func main() {
	os.Exit(kb.Run(os.Args[1:], os.Stdout, os.Stderr, os.Environ()))
}
