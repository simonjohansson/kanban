package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/simonjohansson/kanban/backend/internal/server"
	"gopkg.in/yaml.v3"
)

func main() {
	var outPath string
	flag.StringVar(&outPath, "out", "/Users/simonjohansson/src/kanban/backend/api/openapi.yaml", "output path for OpenAPI YAML")
	flag.Parse()

	tmpDataDir, err := os.MkdirTemp("", "kanban-openapi-data-")
	if err != nil {
		log.Fatalf("create temp data dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDataDir) }()

	sqlitePath := filepath.Join(tmpDataDir, "projection.db")
	app, err := server.New(server.Options{DataDir: tmpDataDir, SQLitePath: sqlitePath})
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer func() { _ = app.Close() }()

	raw, err := yaml.Marshal(app.OpenAPI())
	if err != nil {
		log.Fatalf("marshal openapi: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		log.Fatalf("write openapi file: %v", err)
	}
}
