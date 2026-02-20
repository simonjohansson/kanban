package main

import (
	"os"

	"github.com/simonjohansson/kanban/backend/internal/kanban"
)

func main() {
	os.Exit(kanban.Run(os.Args[1:], os.Stdout, os.Stderr, os.Environ()))
}
